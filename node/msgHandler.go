// msgHandler.go
package main

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/jxskiss/base62"
	"github.com/sirupsen/logrus"
)

var jsonBufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// MessageHandler defines the interface for handling messages.
type MessageHandler interface {
	HandleMessage(n *Node, conn net.Conn, msg Message)
}

func sendMessage(n *Node, conn net.Conn, msgType string, data string) {
	n.net.SendMessage(conn, Message{
		Type:    msgType,
		Data:    data,
		Sender:  n.User.Name,
		Address: ":" + n.Port,
		ID:      generateMessageID(),
	})
}

// PeerListHandler handles "peer_list" messages.
type PeerListHandler struct{}

func (h *PeerListHandler) HandleMessage(n *Node, conn net.Conn, msg Message) {
	var peers []string
	buffer := jsonBufferPool.Get().(*bytes.Buffer)
	defer jsonBufferPool.Put(buffer)
	buffer.Reset()

	if err := json.NewDecoder(strings.NewReader(msg.Data)).Decode(&peers); err != nil {
		n.logger.WithError(err).Error("Error decoding peer list")
		return
	}

	for _, peer := range peers {
		if peer == ":"+n.Port {
			continue // Skip self
		}

		if _, loaded := n.peers.KnownPeers.LoadOrStore(peer, PeerInfo{Address: peer, LastSeen: time.Now()}); !loaded {
			n.logger.Infof("Discovered new peer: %s", peer)
			go n.connectToPeer(peer)
		}
	}
}

// PeerListRequestHandler handles "peer_list_request" messages.
type PeerListRequestHandler struct{}

func (h *PeerListRequestHandler) HandleMessage(n *Node, conn net.Conn, msg Message) {
	n.logger.Infof("Received peer list request from: %s", conn.RemoteAddr().String())
	n.sendPeerList(conn)
}

// ChatHandler handles "chat" messages.
type ChatHandler struct{}

func (h *ChatHandler) HandleMessage(n *Node, conn net.Conn, msg Message) {
	go func() {
		if msg.Sender == n.User.Name && msg.Address == ":"+n.Port {
			n.logger.Debugf("Ignoring message from self: %s", msg.Sender)
			return
		}

		// 记录聊天消息到独立的日志文件
		n.chatLogger.WithFields(map[string]interface{}{
			"timestamp": time.Now().Format("2006-01-02 15:04:05.000"),
			"sender":    msg.Sender,
			"address":   msg.Address,
			"message":   msg.Data,
		}).Info("Chat message")

		// 彩色显示接收到的消息
		color.Cyan("%s: %s\n", msg.Sender, msg.Data)

		if shouldReplyToMessage(msg) {
			dialogue := n.User.FindDialogueForSender(msg.Sender)
			sendMessage(n, conn, MessageTypeChat, dialogue)
		}
	}()
}

// PingHandler handles "ping" messages.
type PingHandler struct{}

func (h *PingHandler) HandleMessage(n *Node, conn net.Conn, msg Message) {
	n.logger.Debugf("Received ping from: %s", conn.RemoteAddr().String())
	// 回复 Pong 消息
	sendMessage(n, conn, MessageTypePong, "")
}

// PongHandler handles "pong" messages.
type PongHandler struct{}

func (h *PongHandler) HandleMessage(n *Node, conn net.Conn, msg Message) {
	n.logger.Debugf("Received pong from: %s", conn.RemoteAddr().String())
	// 更新节点的 LastSeen 时间
	n.peers.UpdateLastSeen(conn.RemoteAddr().String(), time.Now())
}

type FileTransferHandler struct {
	fileBuffers sync.Map // 用于存储文件块的缓冲区
}

func (h *FileTransferHandler) HandleMessage(n *Node, conn net.Conn, msg Message) {
	n.logger.WithFields(logrus.Fields{
		"sender":  msg.Sender,
		"address": msg.Address,
		"file":    msg.FileName,
		"chunk":   msg.ChunkID,
		"is_last": msg.IsLast,
	}).Info("Received file transfer chunk")

	// 检查连接状态
	if _, err := conn.Write([]byte{}); err != nil {
		n.logger.Errorf("Connection to %s is closed: %v", conn.RemoteAddr().String(), err)
		return
	}

	// 计算接收到的文件块的校验和
	receivedChecksum := calculateChecksum(msg.Chunk)
	if receivedChecksum != msg.Checksum {
		n.logger.Errorf("Checksum mismatch for chunk %d of file %s", msg.ChunkID, msg.FileName)
		return
	}

	// 获取或创建文件缓冲区
	buffer, _ := h.fileBuffers.LoadOrStore(msg.FileName, &fileBuffer{
		chunks: make(map[int][]byte),
		size:   msg.FileSize,
	})
	fb := buffer.(*fileBuffer)

	// 存储当前块
	fb.mu.Lock()
	fb.chunks[msg.ChunkID] = msg.Chunk
	fb.mu.Unlock()

	// 如果是最后一块，则写入文件
	if msg.IsLast {
		go h.writeFile(n, msg.FileName, fb, msg.RelPath)
		h.fileBuffers.Delete(msg.FileName)
	}
}

// writeFile 将文件块写入磁盘
func (h *FileTransferHandler) writeFile(n *Node, fileName string, fb *fileBuffer, relPath string) {
	// 确保 received_files 目录存在
	os.MkdirAll("received_files", os.ModePerm)

	// 构建文件路径
	var filePath string
	if relPath != "" && !strings.Contains(relPath, "..") { // 检查 relPath 是否合法
		filePath = filepath.Join("received_files", relPath) // 直接使用 relPath
	} else {
		filePath = filepath.Join("received_files", fileName)
	}

	// 创建文件的父目录（如果不存在）
	dir := filepath.Dir(filePath) // 获取文件的父目录
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		n.logger.WithError(err).Error("Failed to create directory")
		return
	}

	// 创建文件
	file, err := os.Create(filePath)
	if err != nil {
		n.logger.WithError(err).Error("Failed to create file")
		return
	}
	defer file.Close()

	// 按顺序写入文件块
	for i := 0; i < len(fb.chunks); i++ {
		if chunk, ok := fb.chunks[i]; ok {
			_, err := file.Write(chunk)
			if err != nil {
				n.logger.WithError(err).Error("Failed to write file chunk")
				return
			}
		}
	}

	n.logger.WithFields(logrus.Fields{
		"file": filePath,
	}).Info("File transfer completed")
}

// fileBuffer 用于存储文件块的缓冲区
type fileBuffer struct {
	mu     sync.Mutex
	chunks map[int][]byte // 文件块
	size   int64          // 文件总大小
}

// NodeStatusHandler handles "node_status" messages.
type NodeStatusHandler struct{}

func (h *NodeStatusHandler) HandleMessage(n *Node, conn net.Conn, msg Message) {
	n.logger.WithFields(logrus.Fields{
		"sender":  msg.Sender,
		"address": msg.Address,
	}).Info("Received node status request")

	// 返回节点状态
	status := map[string]interface{}{
		"name":    n.User.Name,
		"port":    n.Port,
		"peers":   n.peers.GetPeers(),
		"conns":   len(n.net.GetConns()),
		"healthy": true,
	}

	statusBytes, err := json.Marshal(status)
	if err != nil {
		n.logger.WithError(err).Error("Error encoding node status")
		return
	}

	sendMessage(n, conn, MessageTypeNodeStatus, string(statusBytes))
}

// shouldReplyToMessage decides whether to reply to a message.
func shouldReplyToMessage(msg Message) bool {
	// 只有在消息包含 "hello" 时才回复
	return strings.Contains(msg.Data, "hello")
}

// generateMessageID generates a unique message ID.
func generateMessageID() string {
	// return uuid.New().String()

	uuidBytes := uuid.New()
	encoded := base62.Encode(uuidBytes[:])
	// 将 UUID 转换为 base62 编码
	return string(encoded)
}
