package main

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Node represents a peer in the P2P network.
type Node struct {
	Port          string
	logger        Logger
	config        *Config
	User          *User
	processedMsgs sync.Map
	namesMap      map[string]NameEntry
	net           *NetworkManager
	peers         *PeerManager
	router        *MessageRouter
	executor      TaskExecutor
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

	user := &User{
		Name:           name,
		Description:    entry.Description,
		SpecialAbility: entry.SpecialAbility,
		Tone:           entry.Tone,
		Dialogues:      entry.Dialogues,
	}

	// 初始化消息路由器
	router := NewMessageRouter(logger, executor)

	// 初始化 NetworkManager 和 PeerManager
	netManager := NewNetworkManager(logger, executor)
	peerManager := NewPeerManager(logger, executor)

	return &Node{
		Port:          config.Port,
		logger:        logger,
		config:        config,
		User:          user,
		processedMsgs: sync.Map{},
		namesMap:      namesMap,
		net:           netManager,
		peers:         peerManager,
		router:        router,
		executor:      executor,
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
		"name":            n.User.Name,
		"description":     n.User.Description,
		"special_ability": n.User.SpecialAbility,
		"tone":            n.User.Tone,
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

// startDiscovery periodically broadcasts the peer list to all connected peers.
func (n *Node) startDiscovery() {
	ticker := time.NewTicker(time.Duration(n.config.DiscoveryInterval) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		conns := n.net.GetConns()
		for _, conn := range conns {
			err := n.executor.Submit(func() {
				n.sendPeerList(conn) // 调用 sendPeerList 函数
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
	ticker := time.NewTicker(time.Duration(n.config.HeartbeatInterval) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		conns := n.net.GetConns()
		for _, conn := range conns {
			err := n.executor.Submit(func() {
				msg := Message{Type: MessageTypePing, Data: "", Sender: n.User.Name, Address: ":" + n.Port, ID: generateMessageID()} // 调用 generateMessageID 函数
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
		Sender:  n.User.Name,
		Address: ":" + n.Port,
		ID:      generateMessageID(), // 调用 generateMessageID 函数
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

// SendDir sends all files in a directory to a peer.
func (n *Node) SendDir(peerAddr string, dirPath string) error {
	// 检查目录是否存在
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist: %s", dirPath)
	}

	// 获取目录名字
	dirName := filepath.Base(dirPath)

	// 遍历目录
	err := filepath.Walk(dirPath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 忽略目录，只发送文件
		if info.IsDir() {
			return nil
		}

		// 计算相对路径
		relPath, err := filepath.Rel(dirPath, filePath)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// 拼接目录名字和文件名
		fullFileName := filepath.Join(dirName, relPath)

		// 调用 SendFile 发送文件
		if err := n.SendFile(peerAddr, filePath); err != nil {
			return fmt.Errorf("failed to send file %s: %w", filePath, err)
		}

		n.logger.Infof("Sent file: %s", fullFileName)
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}

	return nil
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
		n.logger.WithFields(map[string]interface{}{
			"peer_addr": peerAddr,
			"file_path": filePath,
		}).Info("Starting to send file")

		// 使用现有的连接发送文件
		if err := n.net.SendFile(conn.(net.Conn), filePath); err != nil {
			n.logger.WithFields(map[string]interface{}{
				"peer_addr": peerAddr,
				"file_path": filePath,
				"error":     err,
			}).Error("Failed to send file")
		} else {
			n.logger.WithFields(map[string]interface{}{
				"peer_addr": peerAddr,
				"file_path": filePath,
			}).Info("File sent successfully")
		}
	})

	if err != nil {
		return fmt.Errorf("failed to submit file transfer task to executor: %w", err)
	}

	return nil
}

// generateTraceID generates a unique trace ID.
func generateTraceID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
