// node.go
package main

import (
	"fmt"
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
		// 使用 NetworkManager 读取消息
		msg, err := n.net.ReadMessage(conn)
		if err != nil {
			n.logger.WithFields(logrus.Fields{
				"trace_id":    traceID,
				"error":       err,
				"remote_addr": conn.RemoteAddr().String(),
			}).Error("Error reading message, closing connection")
			return
		}

		if _, loaded := n.processedMsgs.LoadOrStore(msg.ID, struct{}{}); loaded {
			n.logger.WithFields(logrus.Fields{
				"trace_id":   traceID,
				"message_id": msg.ID,
			}).Debug("Message already processed")
			continue
		}

		// 使用 Goroutine 池处理消息
		n.handleMessageWithPool(conn, msg)
	}
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
				if err := n.net.SendMessage(c, msg); err != nil {
					n.logger.WithError(err).Error("Heartbeat failed")
					n.net.removeConn(c)
					n.peers.RemovePeer(c.RemoteAddr().String())
				}
			}(conn)
		}
	}
}

// BroadcastMessage broadcasts a message to all connected peers.
func (n *Node) BroadcastMessage(message string) {
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
			if err := n.net.SendMessage(conn, msg); err != nil {
				n.logger.WithError(err).Error("Error sending message")
			}
		})
	}
}
