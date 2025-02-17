package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"node/utils"

	"github.com/google/uuid"
	"github.com/jxskiss/base62"
	"github.com/patrickmn/go-cache"
)

// Node represents a peer in the P2P network.
type Node struct {
	Port          string
	logger        utils.Logger // 系统日志
	chatLogger    utils.Logger // 聊天日志
	config        *Config
	User          *User        // 用户信息
	processedMsgs *cache.Cache // 使用 go-cache 存储已处理的消息 ID
	net           *NetworkManager
	peers         *PeerManager
	router        *MessageRouter
	executor      utils.TaskExecutor
}

// NewNode creates a new Node instance.
func NewNode(config *Config, logger utils.Logger, executor utils.TaskExecutor) *Node {
	user := NewUser("")

	// 初始化消息路由器
	router := NewMessageRouter(logger, executor)

	// 初始化 NetworkManager 和 PeerManager
	netManager := NewNetworkManager(logger, executor)
	peerManager := NewPeerManager(logger, executor)

	return &Node{
		Port:       config.Port,
		logger:     logger,                                 // 系统日志
		chatLogger: utils.NewLogger("log/chat.log", false), // 聊天日志
		config:     config,
		User:       user,

		processedMsgs: cache.New(5*time.Minute, 10*time.Minute), // 初始化 processedMsgs，设置默认过期时间为 5 分钟，清理间隔为 10 分钟
		net:           netManager,
		peers:         peerManager,
		router:        router,
		executor:      executor,
	}
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

	// 提取 NameEntry 变量
	nameEntry := n.User.namesMap[n.User.Name]
	n.logger.WithFields(map[string]interface{}{
		"port":            n.Port,
		"name":            nameEntry.Name,
		"description":     nameEntry.Description,
		"special_ability": nameEntry.SpecialAbility,
		"tone":            nameEntry.Tone,
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
	defer func() {
		if r := recover(); r != nil {
			n.logger.Errorf("Recovered from panic: %v", r)
		}
	}()

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
				"error":       err.Error(), // 显式记录错误信息
				"remote_addr": conn.RemoteAddr().String(),
			}).Error("Error reading message")
			continue
		}

		n.handleMessageWithExecutor(conn, msg)

		// 检查消息是否已经处理过
		if _, found := n.processedMsgs.Get(msg.ID); found {
			n.logger.WithFields(map[string]interface{}{
				"trace_id":   traceID,
				"message_id": msg.ID,
			}).Debug("Message already processed")
			continue
		}
		n.processedMsgs.Set(msg.ID, true, 5*time.Minute)
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
				err := n.sendPeerList(conn)
				if err != nil {
					n.logger.WithFields(map[string]interface{}{
						"error": err,
					}).Error("Error sending peer list")
				}
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
				msg := Message{
					Type:    MessageTypePing,
					Data:    "",
					Sender:  n.User.Name,
					Address: ":" + n.Port,
					ID:      generateMessageID(),
				}
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
func (n *Node) BroadcastMessage(message string) error {
	msg := Message{
		Type:    MessageTypeChat,
		Data:    message,
		Sender:  n.User.Name,
		Address: ":" + n.Port,
		ID:      generateMessageID(),
	}

	conns := n.net.GetConns()
	for _, conn := range conns {
		err := n.executor.Submit(func() {
			if err := n.net.SendMessage(conn, msg); err != nil {
				n.logger.WithFields(map[string]interface{}{
					"error": err.Error(),
				}).Error("Error sending message")
			}
		})
		if err != nil {
			n.logger.WithFields(map[string]interface{}{
				"error": err,
			}).Error("Failed to submit broadcast task to executor")
			return err
		}
	}
	return nil
}

// SendDir sends all files in a directory to a peer.
func (n *Node) SendDir(peerAddr string, dirPath string) error {
	// 检查目录是否存在
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist: %s", dirPath)
	}

	// 获取目录名字
	dirName := filepath.Base(dirPath) // dirName 是 "test-data"

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

		// 将 dirName 作为 relPath 的父目录
		fullRelPath := filepath.Join(dirName, relPath)

		// 调用 SendFile 发送文件，并传递完整相对路径
		if err := n.SendFile(peerAddr, filePath, fullRelPath); err != nil {
			return err
		}

		n.logger.Infof("Sent file: %s", fullRelPath)
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}

	return nil
}

// SendFile sends a file to a peer using an existing connection.
func (n *Node) SendFile(peerAddr string, filePath string, relPath string) error {
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
			"rel_path":  relPath,
		}).Info("Starting to send file")

		// 使用现有的连接发送文件
		if err := n.net.SendFile(conn.(net.Conn), filePath, relPath); err != nil {
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
	// return uuid.New().String()

	uuidBytes := uuid.New()
	encoded := base62.Encode(uuidBytes[:])
	// 将 UUID 转换为 base62 编码
	return string(encoded)
}
