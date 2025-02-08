// msgHandler.go
package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
)

// MessageType 定义消息类型
const (
	MessageTypeChat         = "chat"
	MessageTypePeerList     = "peer_list"
	MessageTypePeerListReq  = "peer_list_request"
	MessageTypePing         = "ping" // 心跳请求
	MessageTypePong         = "pong" // 心跳响应
	MessageTypeFileTransfer = "file_transfer"
	MessageTypeNodeStatus   = "node_status"
)

// MessageHandler defines the interface for handling messages.
type MessageHandler interface {
	HandleMessage(n *Node, conn net.Conn, msg Message)
}

func sendMessage(n *Node, conn net.Conn, msgType string, data string) {
	n.net.SendMessage(conn, Message{
		Type:    msgType,
		Data:    data,
		Sender:  n.Name,
		Address: ":" + n.Port,
		ID:      generateMessageID(),
	})
}

// PeerListHandler handles "peer_list" messages.
type PeerListHandler struct{}

func (h *PeerListHandler) HandleMessage(n *Node, conn net.Conn, msg Message) {
	var peers []string
	if err := json.Unmarshal([]byte(msg.Data), &peers); err != nil {
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
		if msg.Sender == n.Name && msg.Address == ":"+n.Port {
			n.logger.Debugf("Ignoring message from self: %s", msg.Sender)
			return
		}

		// 记录收到的聊天消息
		n.logger.WithFields(logrus.Fields{
			"sender":  msg.Sender,
			"address": msg.Address,
			"message": msg.Data,
		}).Info("Received chat message")

		// 彩色显示接收到的消息
		color.Cyan("%s: %s\n", msg.Sender, msg.Data)

		if shouldReplyToMessage(msg) {
			dialogue := n.findDialogueForSender(msg.Sender)
			n.logger.WithFields(logrus.Fields{
				"reply": dialogue,
			}).Info("Sending reply")

			sendMessage(n, conn, MessageTypeChat, string(dialogue))
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
		"is_last": msg.IsLast, // 打印 IsLast 字段
	}).Info("Received file transfer chunk")

	// 检查连接状态
	if _, err := conn.Write([]byte{}); err != nil {
		fmt.Printf("Connection to %s is closed: %v", conn.RemoteAddr().String(), err)
		n.logger.Errorf("Connection to %s is closed: %v", conn.RemoteAddr().String(), err)
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
		fmt.Printf("last chunk!!!!%d\n", msg.ChunkID)

		go h.writeFile(n, msg.FileName, fb)
		h.fileBuffers.Delete(msg.FileName)

		fmt.Printf("rec fileName %s\n", msg.FileName)
	}
}

// writeFile 将文件块写入磁盘
func (h *FileTransferHandler) writeFile(n *Node, fileName string, fb *fileBuffer) {
	// 确保 received_files 目录存在
	os.MkdirAll("received_files", os.ModePerm)

	// 构建文件路径
	filePath := filepath.Join("received_files", fileName)

	// 创建目录（如果不存在）
	dir := filepath.Dir(filePath)
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
		"name":    n.Name,
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
	return fmt.Sprintf("%d-%d-%d", time.Now().UnixNano(), rand.Intn(1000000), os.Getpid())

}
