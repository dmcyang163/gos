package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"node/event"
	"node/utils"

	"github.com/google/uuid"
	"github.com/jxskiss/base62"
	"github.com/patrickmn/go-cache"
)

// Node 代表 P2P 网络中的一个节点。
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

var eventBus = event.GetEventBus()

// subscribeEvents 订阅事件。
func (n *Node) subscribeEvents() {
	eventBus.Subscribe(event.EventTypePeerList, n.handlePeerList)
	eventBus.Subscribe(event.EventTypePeerListRequest, n.handlePeerListRequest)
	// eventBus.Subscribe(event.EventTypeChat, n.handleChat)
	// eventBus.Subscribe(event.EventTypePing, n.handlePing)
	// eventBus.Subscribe(event.EventTypePong, n.handlePong)
	// eventBus.Subscribe(event.EventTypeFileTransfer, n.handleFileTransfer)
	// eventBus.Subscribe(event.EventTypeNodeStatus, n.handleNodeStatus)
}

// handlePeerList 处理节点列表事件。
func (n *Node) handlePeerList(event event.Event) {
	peers := event.Payload.([]string)
	for _, peer := range peers {
		if peer == ":"+n.Port {
			continue // 跳过自身
		}

		if _, loaded := n.peers.KnownPeers.LoadOrStore(peer, PeerInfo{Address: peer, LastSeen: time.Now()}); !loaded {
			n.logger.Infof("Discovered new peer: %s", peer)
			go n.connectToPeer(peer)
		}
	}
}

// handlePeerListRequest 处理节点列表请求事件。
func (n *Node) handlePeerListRequest(event event.Event) {
	conn := event.Payload.(net.Conn)
	n.logger.Infof("Received peer list request from: %s", conn.RemoteAddr().String())
	n.sendPeerList(conn)
}

// NewNode 创建一个新的 Node 实例。
func NewNode(config *Config, logger utils.Logger, executor utils.TaskExecutor) *Node {
	user := NewUser("")

	// 初始化消息路由器
	router := NewMessageRouter(logger)

	// 初始化 NetworkManager 和 PeerManager
	netManager := NewNetworkManager(logger, executor)
	peerManager := NewPeerManager(logger, executor)

	node := &Node{
		Port:          config.Port,
		logger:        logger,
		chatLogger:    utils.NewLogger("log/chat.log", false),
		config:        config,
		User:          user,
		processedMsgs: cache.New(5*time.Minute, 10*time.Minute),
		net:           netManager,
		peers:         peerManager,
		router:        router,
		executor:      executor,
	}

	// 订阅事件
	node.subscribeEvents()

	return node
}

// startServer 启动 TCP 服务器以监听传入连接。
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

// handleConnection 处理来自连接的传入消息。
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

// closeConnection 关闭连接并清理资源。
func (n *Node) closeConnection(conn net.Conn, traceID string) {
	conn.Close()
	n.net.removeConn(conn)
	n.peers.RemovePeer(conn.RemoteAddr().String())
	n.logger.WithFields(map[string]interface{}{
		"remote_addr": conn.RemoteAddr().String(),
		"trace_id":    traceID,
	}).Info("Connection closed")
}

// handleMessageWithExecutor 使用 Goroutine 池来处理消息。
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

// startDiscovery 定期将节点列表广播到所有连接的节点。
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

// startHeartbeat 定期向所有连接的节点发送 ping 消息。
func (n *Node) startHeartbeat() {
	ticker := time.NewTicker(time.Duration(n.config.HeartbeatInterval) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		conns := n.net.GetConns()
		for _, conn := range conns {
			err := n.executor.Submit(func() {
				msg := Message{
					Type:    MsgTypePing,
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

// BroadcastMessage 将消息广播到所有连接的节点。
func (n *Node) BroadcastMessage(message string) error {
	msg := Message{
		Type:    MsgTypeChat,
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

// SendFile 使用现有连接将文件发送到节点。
func (n *Node) SendFile(peerAddr string, filePath string, relPath string) error {
	// 检查是否已经连接到该 peer
	conn, ok := n.net.Conns.Load(peerAddr)
	if !ok {
		return fmt.Errorf("no connection to peer: %s", peerAddr)
	}

	progress, entry, err := n.prepareFileTransfer(filePath, relPath)
	if err != nil {
		return err
	}

	// 如果文件已传输完成，跳过
	if entry.Completed {
		n.logger.Infof("File already sent: %s", relPath)
		return nil
	}

	// 使用 conn.(net.Conn) 将 interface{} 转换为 net.Conn
	if err := n.SendFileWithProgress(conn.(net.Conn), filePath, relPath, &progress); err != nil {
		return err
	}

	// 传输完成后删除进度文件
	if err := deleteProgress(filePath); err != nil {
		return fmt.Errorf("failed to delete progress file: %w", err)
	}

	return nil
}

// prepareFileTransfer 准备文件传输，加载进度和初始化进度条目
func (n *Node) prepareFileTransfer(filePath string, relPath string) (Progress, *ProgressEntry, error) {
	// 加载传输进度
	progress, err := loadProgress(filePath)
	if err != nil {
		return Progress{}, nil, fmt.Errorf("failed to load progress: %w", err)
	}

	// 如果没有进度条目，初始化一个
	if len(progress.Entries) == 0 {
		progress = Progress{
			Type:    "file",   // 明确设置为 "file"
			Path:    filePath, // 使用文件的完整路径
			Entries: []ProgressEntry{{RelPath: relPath, Offset: 0, Completed: false}},
		}
	}
	n.logger.Debugf("正在发送文件: %s, 类型: %s", progress.Path, progress.Type)

	// 获取文件的传输进度
	entry := &progress.Entries[0]
	return progress, entry, nil
}

// SendDir 将目录中的所有文件发送到节点。
func (n *Node) SendDir(peerAddr string, dirPath string) error {
	// 检查是否已经连接到该 peer
	conn, ok := n.net.Conns.Load(peerAddr)
	if !ok {
		return fmt.Errorf("no connection to peer: %s", peerAddr)
	}

	progress, files, err := n.prepareDirTransfer(dirPath)
	if err != nil {
		return err
	}

	// 统一发送文件
	for i, file := range files {
		entry := &progress.Entries[i]

		if !entry.Completed {
			// 使用 conn.(net.Conn) 将 interface{} 转换为 net.Conn
			if err := n.SendFileWithProgress(conn.(net.Conn), file.filePath, file.fullRelPath, &progress); err != nil {
				return err
			}
			n.logger.Infof("Sent file: %s", file.fullRelPath)
		}
	}

	// 传输完成后删除进度文件
	if err := deleteProgress(dirPath); err != nil {
		return fmt.Errorf("failed to delete progress file: %w", err)
	}

	return nil
}

// prepareDirTransfer 准备目录传输，加载进度和收集文件信息
func (n *Node) prepareDirTransfer(dirPath string) (Progress, []struct {
	filePath    string
	fullRelPath string
}, error) {
	// 加载目录传输进度
	progress, err := loadProgress(dirPath)
	if err != nil {
		return Progress{}, nil, fmt.Errorf("failed to load directory progress: %w", err)
	}

	// 收集所有文件的路径和相对路径
	files, err := n.collectFiles(dirPath)
	if err != nil {
		return Progress{}, nil, fmt.Errorf("failed to collect files: %w", err)
	}

	// 初始化进度条目
	if len(progress.Entries) == 0 {
		progress.Entries = make([]ProgressEntry, len(files))
		for i, file := range files {
			progress.Entries[i] = ProgressEntry{
				RelPath:   file.fullRelPath,
				Offset:    0,
				Completed: false,
			}
		}
	}
	return progress, files, nil
}

// SendFileWithProgress 发送文件并更新传输进度
func (n *Node) SendFileWithProgress(conn net.Conn, filePath string, relPath string, progress *Progress) error {
	// 找到对应的进度条目
	var entry *ProgressEntry
	for i := range progress.Entries {
		if progress.Entries[i].RelPath == relPath {
			entry = &progress.Entries[i]
			break
		}
	}
	if entry == nil {
		return fmt.Errorf("progress entry not found for file: %s", relPath)
	}

	// 如果文件已传输完成，跳过
	if entry.Completed {
		n.logger.Infof("File already sent: %s", relPath)
		return nil
	}

	// 调用 network.go 中的 SendFile 函数，传入当前的偏移量，并接收文件信息
	fileInfo, err := n.net.SendFile(conn, filePath, relPath, entry.Offset)
	if err != nil {
		return fmt.Errorf("failed to send file: %w", err)
	}

	// 更新传输进度
	entry.Offset = (*fileInfo).Size()
	entry.Completed = true

	// 保存传输进度
	if err := saveProgress(*progress); err != nil {
		return fmt.Errorf("failed to save progress: %w", err)
	}

	return nil
}

// collectFiles 收集目录下的所有文件信息
func (n *Node) collectFiles(dirPath string) ([]struct {
	filePath    string
	fullRelPath string
}, error) {
	// 检查目录是否存在
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory does not exist: %s", dirPath)
	}

	// 获取目录名字
	dirName := filepath.Base(dirPath)

	// 收集所有文件的路径和相对路径
	var files []struct {
		filePath    string
		fullRelPath string
	}

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

		// 收集文件信息
		files = append(files, struct {
			filePath    string
			fullRelPath string
		}{filePath, fullRelPath})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return files, nil
}

// generateTraceID 生成唯一的跟踪 ID。
func generateTraceID() string {
	uuidBytes := uuid.New()
	encoded := base62.Encode(uuidBytes[:])
	return string(encoded)
}
