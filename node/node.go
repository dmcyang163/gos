package main

import (
	"bufio"
	"bytes"
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
	Port             string
	logger           *logrus.Logger
	config           *Config
	Name             string
	Description      string
	SpecialAbility   string
	Tone             string
	Dialogues        []string
	processedMsgs    sync.Map
	namesMap         map[string]NameEntry
	net              *NetworkManager
	peers            *PeerManager
	router           *MessageRouter
	pool             *ants.Pool
	compressorPool   *sync.Pool
	decompressorPool *sync.Pool
}

// newBufferPool 创建一个新的 sync.Pool，用于复用 bytes.Buffer
func newBufferPool() *sync.Pool {
	return &sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(nil)
		},
	}
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

	// 初始化 sync.Pool 用于复用缓冲区
	compressorPool := newBufferPool()
	decompressorPool := newBufferPool()

	return &Node{
		Port:             config.Port,
		logger:           logger,
		config:           config,
		Name:             name,
		Description:      entry.Description,
		SpecialAbility:   entry.SpecialAbility,
		Tone:             entry.Tone,
		Dialogues:        entry.Dialogues,
		processedMsgs:    sync.Map{},
		namesMap:         namesMap,
		net:              NewNetworkManager(),
		peers:            &PeerManager{},
		router:           router,
		pool:             pool,
		compressorPool:   compressorPool,
		decompressorPool: decompressorPool,
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

	// 读取消息长度
	lengthBytes := make([]byte, 4)
	_, err := io.ReadFull(reader, lengthBytes)
	if err != nil {
		n.logger.WithFields(logrus.Fields{
			"trace_id": traceID,
			"error":    err,
		}).Error("Error reading message length")
		return Message{}, fmt.Errorf("error reading message length: %w", err)
	}

	length := binary.BigEndian.Uint32(lengthBytes)

	// 读取消息体
	msgBytes := make([]byte, length)
	_, err = io.ReadFull(reader, msgBytes)
	if err != nil {
		n.logger.WithFields(logrus.Fields{
			"trace_id": traceID,
			"error":    err,
		}).Error("Error reading message body")
		return Message{}, fmt.Errorf("error reading message body: %w", err)
	}

	// 解压消息
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
	if err := n.net.SendMessage(conn, msg, compressMessage); err != nil {
		n.logger.WithError(err).Error("Error sending message")
		n.net.removeConn(conn)
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
