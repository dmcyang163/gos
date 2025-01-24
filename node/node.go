package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/sirupsen/logrus"
)

// Node represents a peer in the P2P network.
type Node struct {
	Port           string
	logger         *logrus.Logger
	config         *Config
	Name           string
	Description    string
	SpecialAbility string
	Tone           string
	Dialogues      []string
	processedMsgs  sync.Map
	namesMap       map[string]NameEntry
	net            *NetworkManager
	peers          *PeerManager
	router         *MessageRouter
	pool           *ants.Pool
	readBufferPool *sync.Pool // 用于读取消息的缓冲池
	sendBufferPool *sync.Pool // 用于发送消息的缓冲池
}

// NewNode creates a new Node instance.
func NewNode(config *Config, names []NameEntry) *Node {
	logger := setupLogger(config)

	// 随机选择一个名字
	entry := names[rand.Intn(len(names))]
	name := fmt.Sprintf("%s·%s", entry.Name, config.Port)

	// 初始化 namesMap
	namesMap := make(map[string]NameEntry)
	for _, entry := range names {
		namesMap[entry.Name] = entry
	}

	// 初始化消息路由器
	router := NewMessageRouter()

	// 初始化 Goroutine 池
	pool, err := ants.NewPool(100)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create Goroutine pool")
	}

	// 初始化读取和发送消息的缓冲池
	readBufferPool := newBufferPool()
	sendBufferPool := newBufferPool()

	return &Node{
		Port:           config.Port,
		logger:         logger,
		config:         config,
		Name:           name,
		Description:    entry.Description,
		SpecialAbility: entry.SpecialAbility,
		Tone:           entry.Tone,
		Dialogues:      entry.Dialogues,
		processedMsgs:  sync.Map{},
		namesMap:       namesMap,
		net:            NewNetworkManager(),
		peers:          &PeerManager{},
		router:         router,
		pool:           pool,
		readBufferPool: readBufferPool, // 初始化 readBufferPool
		sendBufferPool: sendBufferPool, // 初始化 sendBufferPool
	}
}

// findDialogueForSender finds a dialogue for the sender based on their name.
func (n *Node) findDialogueForSender(sender string) string {
	for name, entry := range n.namesMap {
		if strings.Contains(sender, name) {
			if len(entry.Dialogues) > 0 {
				return entry.Dialogues[rand.Intn(len(entry.Dialogues))]
			}
		}
	}
	return "你好，我是" + sender + "。"
}

// startServer starts the TCP server to listen for incoming connections.
func (n *Node) startServer() {
	ln, err := net.Listen("tcp", ":"+n.Port)
	if err != nil {
		n.logger.WithFields(logrus.Fields{
			"port":  n.Port,
			"error": err,
		}).Error("Error starting server")
		return
	}
	defer ln.Close()

	n.logger.WithFields(logrus.Fields{
		"port":            n.Port,
		"name":            n.Name,
		"description":     n.Description,
		"special_ability": n.SpecialAbility,
		"tone":            n.Tone,
	}).Info("Server started")

	for {
		conn, err := ln.Accept()
		if err != nil {
			n.logger.WithFields(logrus.Fields{
				"port":  n.Port,
				"error": err,
			}).Error("Error accepting connection")
			continue
		}

		remoteAddr := conn.RemoteAddr().String()
		n.logger.WithFields(logrus.Fields{
			"remote_addr": remoteAddr,
		}).Info("New connection")

		n.net.addConn(conn)
		go n.handleConnection(conn)
	}
}

// handleConnection handles incoming messages from a connection.
func (n *Node) handleConnection(conn net.Conn) {
	traceID := generateTraceID()
	n.logger.WithField("trace_id", traceID).Info("Handling connection")

	defer n.closeConnection(conn, traceID)

	for {
		if err := n.readAndProcessMessage(conn, traceID); err != nil {
			return
		}
	}
}

// readAndProcessMessage 从连接中读取并处理消息
func (n *Node) readAndProcessMessage(conn net.Conn, traceID string) error {
	msg, err := n.readMessage(conn, traceID)
	if err != nil {
		n.logger.WithFields(logrus.Fields{
			"trace_id":    traceID,
			"error":       err,
			"remote_addr": conn.RemoteAddr().String(),
		}).Error("Error reading message, closing connection")
		return err
	}

	if _, loaded := n.processedMsgs.LoadOrStore(msg.ID, struct{}{}); loaded {
		n.logger.WithFields(logrus.Fields{
			"trace_id":   traceID,
			"message_id": msg.ID,
		}).Debug("Message already processed")
		return nil
	}

	// 使用 Goroutine 池处理消息
	n.handleMessageWithPool(conn, msg)
	return nil
}

// closeConnection 关闭连接并清理资源
func (n *Node) closeConnection(conn net.Conn, traceID string) {
	conn.Close()
	n.net.removeConn(conn)
	n.peers.RemovePeer(conn.RemoteAddr().String())
	n.logger.WithFields(logrus.Fields{
		"remote_addr": conn.RemoteAddr().String(),
		"trace_id":    traceID,
	}).Info("Connection closed")
}

// readMessage reads a message from the connection.
func (n *Node) readMessage(conn net.Conn, traceID string) (Message, error) {
	reader := bufio.NewReader(conn)

	// 从 readBufferPool 中获取缓冲区
	lengthBytes := n.readBufferPool.Get().([]byte)
	defer n.readBufferPool.Put(lengthBytes) // 使用完毕后放回池中

	// 读取消息长度
	_, err := io.ReadFull(reader, lengthBytes[:4]) // 只读取前 4 个字节
	if err != nil {
		n.logger.WithFields(logrus.Fields{
			"trace_id": traceID,
			"error":    err,
		}).Error("Error reading message length")
		return Message{}, fmt.Errorf("error reading message length: %w", err)
	}

	length := binary.BigEndian.Uint32(lengthBytes[:4])

	// 从 readBufferPool 中获取缓冲区
	msgBytes := n.readBufferPool.Get().([]byte)
	defer n.readBufferPool.Put(msgBytes) // 使用完毕后放回池中

	// 如果消息长度超过缓冲区大小，重新分配更大的缓冲区
	if int(length) > len(msgBytes) {
		msgBytes = make([]byte, length) // 动态分配更大的缓冲区
	} else {
		msgBytes = msgBytes[:length] // 调整切片长度为消息长度
	}

	// 读取消息体
	_, err = io.ReadFull(reader, msgBytes)
	if err != nil {
		n.logger.WithFields(logrus.Fields{
			"trace_id": traceID,
			"error":    err,
		}).Error("Error reading message body")
		return Message{}, fmt.Errorf("error reading message body: %w", err)
	}

	// 使用 decompressMessage 解压消息
	msg, err := decompressMessage(msgBytes)
	if err != nil {
		n.logger.WithFields(logrus.Fields{
			"trace_id": traceID,
			"error":    err,
		}).Error("Error decompressing message")
		return Message{}, fmt.Errorf("error decompressing message: %w", err)
	}

	return msg, nil
}

// handleMessageWithPool uses a Goroutine pool to handle messages.
func (n *Node) handleMessageWithPool(conn net.Conn, msg Message) {
	err := n.pool.Submit(func() {
		n.router.RouteMessage(n, conn, msg)
	})
	if err != nil {
		n.logger.WithError(err).Error("Failed to submit task to pool")
	}
}

// generateTraceID generates a unique trace ID.
func generateTraceID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// startDiscovery periodically broadcasts the peer list to all connected peers.
func (n *Node) startDiscovery() {
	for {
		time.Sleep(time.Duration(n.config.DiscoveryInterval) * time.Second)
		conns := n.net.GetConns()
		for _, conn := range conns {
			n.sendPeerList(conn)
		}
	}
}

// startHeartbeat periodically sends ping messages to all connected peers.
func (n *Node) startHeartbeat() {
	for {
		time.Sleep(time.Duration(n.config.HeartbeatInterval) * time.Second)
		conns := n.net.GetConns()
		for _, conn := range conns {
			go func(c net.Conn) {
				msg := Message{Type: MessageTypePing, Data: "", Sender: n.Name, Address: ":" + n.Port, ID: generateMessageID()}
				if err := n.net.SendMessage(c, msg, compressMessage); err != nil {
					n.logger.WithError(err).Error("Heartbeat failed")
					n.net.removeConn(c)
					n.peers.RemovePeer(c.RemoteAddr().String())
				}
			}(conn)
		}
	}
}

// sendMessageToPeer 向指定连接发送消息
func (n *Node) sendMessageToPeer(conn net.Conn, msg Message) {
	// 使用 compressMessage 压缩消息
	msgBytes, err := compressMessage(msg)
	if err != nil {
		n.logger.WithError(err).Error("Error compressing message")
		return
	}

	// 从 sendBufferPool 中获取缓冲区
	buffer := n.sendBufferPool.Get().([]byte)
	defer n.sendBufferPool.Put(buffer) // 使用完毕后放回池中

	// 如果消息长度超过缓冲区大小，重新分配更大的缓冲区
	if len(msgBytes) > len(buffer) {
		buffer = make([]byte, len(msgBytes)) // 动态分配更大的缓冲区
	} else {
		buffer = buffer[:len(msgBytes)] // 调整切片长度为消息长度
	}

	// 将消息内容复制到缓冲区
	copy(buffer, msgBytes)

	// 发送消息长度
	length := uint32(len(buffer))
	lengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBytes, length)
	if _, err := conn.Write(lengthBytes); err != nil {
		n.logger.WithError(err).Error("Error sending message length")
		return
	}

	// 发送消息体
	if _, err := conn.Write(buffer); err != nil {
		n.logger.WithError(err).Error("Error sending message body")
		return
	}
}

// send broadcasts a message to all connected peers.
func (n *Node) send(message string) {
	msg := Message{
		Type:    MessageTypeChat,
		Data:    message,
		Sender:  n.Name,
		Address: ":" + n.Port,
		ID:      generateMessageID(),
	}

	conns := n.net.GetConns()
	for _, conn := range conns {
		_ = n.pool.Submit(func() {
			n.sendMessageToPeer(conn, msg)
		})
	}
}
