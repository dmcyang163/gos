// node.go
package main

import (
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
)

// Node represents a peer in the P2P network.
type Node struct {
	Port           string
	logger         Logger
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
	executor       TaskExecutor
}

// NewNode creates a new Node instance.
func NewNode(config *Config, names []NameEntry, logger Logger, executor TaskExecutor) *Node {
	// 随机选择一个名字
	entry := names[rand.Intn(len(names))]
	name := fmt.Sprintf("%s·%s", entry.Name, config.Port)

	// 初始化 namesMap
	namesMap := make(map[string]NameEntry)
	for _, entry := range names {
		namesMap[entry.Name] = entry
	}

	// 初始化消息路由器
	router := NewMessageRouter(logger, executor)

	// 初始化 NetworkManager 和 PeerManager
	netManager := NewNetworkManager(logger, executor)
	peerManager := NewPeerManager(logger, executor)

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
		net:            netManager,
		peers:          peerManager,
		router:         router,
		executor:       executor,
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
		n.logger.WithFields(map[string]interface{}{
			"port":  n.Port,
			"error": err,
		}).Error("Error starting server")
		return
	}
	defer ln.Close()

	n.logger.WithFields(map[string]interface{}{
		"port":            n.Port,
		"name":            n.Name,
		"description":     n.Description,
		"special_ability": n.SpecialAbility,
		"tone":            n.Tone,
	}).Info("Server started")

	for {
		conn, err := ln.Accept()
		if err != nil {
			n.logger.WithFields(map[string]interface{}{
				"port":  n.Port,
				"error": err,
			}).Error("Error accepting connection")
			continue
		}

		remoteAddr := conn.RemoteAddr().String()
		n.logger.WithFields(map[string]interface{}{
			"remote_addr": remoteAddr,
		}).Info("New connection")

		n.net.addConn(conn)
		err = n.executor.Submit(func() {
			n.handleConnection(conn)
		})
		if err != nil {
			n.logger.WithFields(map[string]interface{}{
				"error": err,
			}).Error("Failed to submit connection handling task to executor")
		}
	}
}

// handleConnection handles incoming messages from a connection.
func (n *Node) handleConnection(conn net.Conn) {
	traceID := generateTraceID()
	n.logger.WithFields(map[string]interface{}{
		"trace_id": traceID,
	}).Info("Handling connection")

	defer n.closeConnection(conn, traceID)

	for {
		msg, err := n.net.ReadMessage(conn)
		if err != nil {
			n.logger.WithFields(map[string]interface{}{
				"trace_id":    traceID,
				"error":       err,
				"remote_addr": conn.RemoteAddr().String(),
			}).Error("Error reading message, closing connection")
			return
		}

		n.handleMessageWithExecutor(conn, msg)

		if _, loaded := n.processedMsgs.LoadOrStore(msg.ID, struct{}{}); loaded {
			n.logger.WithFields(map[string]interface{}{
				"trace_id":   traceID,
				"message_id": msg.ID,
			}).Debug("Message already processed")
			continue
		}

	}
}

// closeConnection closes the connection and cleans up resources.
func (n *Node) closeConnection(conn net.Conn, traceID string) {
	conn.Close()
	n.net.removeConn(conn)
	n.peers.RemovePeer(conn.RemoteAddr().String())
	n.logger.WithFields(map[string]interface{}{
		"remote_addr": conn.RemoteAddr().String(),
		"trace_id":    traceID,
	}).Info("Connection closed")
}

// handleMessageWithExecutor uses a Goroutine pool to handle messages.
func (n *Node) handleMessageWithExecutor(conn net.Conn, msg Message) {
	err := n.executor.Submit(func() {
		n.router.RouteMessage(n, conn, msg)
	})
	if err != nil {
		n.logger.WithFields(map[string]interface{}{
			"error": err,
		}).Error("Failed to submit task to executor")
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
			err := n.executor.Submit(func() {
				n.sendPeerList(conn)
			})
			if err != nil {
				n.logger.WithFields(map[string]interface{}{
					"error": err,
				}).Error("Failed to submit discovery task to executor")
			}
		}
	}
}

// startHeartbeat periodically sends ping messages to all connected peers.
func (n *Node) startHeartbeat() {
	for {
		time.Sleep(time.Duration(n.config.HeartbeatInterval) * time.Second)
		conns := n.net.GetConns()
		for _, conn := range conns {
			err := n.executor.Submit(func() {
				msg := Message{Type: MessageTypePing, Data: "", Sender: n.Name, Address: ":" + n.Port, ID: generateMessageID()}
				if err := n.net.SendMessage(conn, msg); err != nil {
					n.logger.WithFields(map[string]interface{}{
						"error": err,
					}).Error("Heartbeat failed")
					n.net.removeConn(conn)
					n.peers.RemovePeer(conn.RemoteAddr().String())
				}
			})
			if err != nil {
				n.logger.WithFields(map[string]interface{}{
					"error": err,
				}).Error("Failed to submit heartbeat task to executor")
			}
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
		err := n.executor.Submit(func() {
			if err := n.net.SendMessage(conn, msg); err != nil {
				n.logger.WithFields(map[string]interface{}{
					"error": err,
				}).Error("Error sending message")
			}
		})
		if err != nil {
			n.logger.WithFields(map[string]interface{}{
				"error": err,
			}).Error("Failed to submit broadcast task to executor")
		}
	}
}

// SendFile sends a file to a peer using an existing connection.
func (n *Node) SendFile(peerAddr string, filePath string) error {
	// 检查是否已经连接到该 peer
	conn, ok := n.net.Conns.Load(peerAddr)
	if !ok {
		return fmt.Errorf("no connection to peer: %s", peerAddr)
	}

	// 将文件发送任务提交到 executor 中执行
	err := n.executor.Submit(func() {
		// 使用现有的连接发送文件
		if err := n.net.SendFile(conn.(net.Conn), filePath); err != nil {
			n.logger.Errorf("Failed to send file to %s: %v", peerAddr, err)
		} else {
			n.logger.Infof("File %s sent successfully to %s", filePath, peerAddr)
		}
	})

	if err != nil {
		return fmt.Errorf("failed to submit file transfer task to executor: %w", err)
	}

	return nil
}
