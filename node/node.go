// node.go
package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// 全局名字列表
var names []NameEntry

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

	net    *NetworkManager
	peers  *PeerManager
	router *MessageRouter
	pool   *ants.Pool // Goroutine 池

	// 使用 sync.Pool 复用压缩和解压缩的缓冲区
	compressorPool   *sync.Pool
	decompressorPool *sync.Pool
}

// NewNode creates a new Node instance.
func NewNode(config *Config, names []NameEntry) *Node {
	logger := logrus.New()

	// 配置日志轮转
	logger.SetOutput(&lumberjack.Logger{
		Filename:   "node.log", // 日志文件路径
		MaxSize:    100,        // 每个日志文件的最大大小（MB）
		MaxBackups: 3,          // 保留的旧日志文件数量
		MaxAge:     28,         // 保留日志的最大天数
		Compress:   true,       // 是否压缩旧日志
	})

	// 同时输出到控制台
	logger.SetOutput(io.MultiWriter(os.Stdout, logger.Out))

	// 设置日志格式为 JSON
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "level",
			logrus.FieldKeyMsg:   "message",
		},
	})

	// 设置日志级别
	switch config.LogLevel {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}

	// 随机选择一个名字
	entry := names[rand.Intn(len(names))]
	name := fmt.Sprintf("%s·%s", entry.Name, config.Port)

	// 初始化消息路由器
	router := NewMessageRouter()
	router.RegisterHandler(MessageTypePeerList, &PeerListHandler{})
	router.RegisterHandler(MessageTypePeerListReq, &PeerListRequestHandler{})
	router.RegisterHandler(MessageTypeChat, &ChatHandler{})
	router.RegisterHandler(MessageTypePing, &PingHandler{})
	router.RegisterHandler(MessageTypePong, &PongHandler{})
	router.RegisterHandler(MessageTypeFileTransfer, &FileTransferHandler{})
	router.RegisterHandler(MessageTypeNodeStatus, &NodeStatusHandler{})

	// 初始化 Goroutine 池
	pool, err := ants.NewPool(100) // 创建 Goroutine 池，最大并发数为 100
	if err != nil {
		logger.WithError(err).Fatal("Failed to create Goroutine pool")
	}

	// 初始化 sync.Pool 用于复用缓冲区
	compressorPool := &sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer) // 用于压缩的缓冲区
		},
	}
	decompressorPool := &sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer) // 用于解压缩的缓冲区
		},
	}

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
		net:              NewNetworkManager(),
		peers:            &PeerManager{},
		router:           router,
		pool:             pool,
		compressorPool:   compressorPool,
		decompressorPool: decompressorPool,
	}
}

// SetLogLevel dynamically sets the log level.
func (n *Node) SetLogLevel(level string) {
	switch level {
	case "debug":
		n.logger.SetLevel(logrus.DebugLevel)
	case "info":
		n.logger.SetLevel(logrus.InfoLevel)
	case "warn":
		n.logger.SetLevel(logrus.WarnLevel)
	case "error":
		n.logger.SetLevel(logrus.ErrorLevel)
	default:
		n.logger.SetLevel(logrus.InfoLevel)
	}
	n.logger.WithField("level", level).Info("Log level changed")
}

// startLogLevelAPI starts an HTTP server to dynamically adjust the log level.
func (n *Node) startLogLevelAPI(port string) {
	http.HandleFunc("/loglevel", func(w http.ResponseWriter, r *http.Request) {
		level := r.URL.Query().Get("level")
		if level == "" {
			http.Error(w, "Missing level parameter", http.StatusBadRequest)
			return
		}
		n.SetLogLevel(level)
		w.Write([]byte("Log level updated to " + level))
	})

	go http.ListenAndServe(":"+port, nil)
	n.logger.WithField("port", port).Info("Log level API started")
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

	defer func() {
		conn.Close()
		n.net.removeConn(conn)
		n.peers.RemovePeer(conn.RemoteAddr().String())
		n.logger.WithFields(logrus.Fields{
			"remote_addr": conn.RemoteAddr().String(),
			"trace_id":    traceID,
		}).Info("Connection closed")
	}()

	for {
		msg, err := n.readMessage(conn, traceID)
		if err != nil {
			n.logger.WithFields(logrus.Fields{
				"trace_id": traceID,
				"error":    err,
			}).Error("Error reading message")
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

// readMessage reads a message from the connection.
func (n *Node) readMessage(conn net.Conn, traceID string) (Message, error) {
	reader := bufio.NewReader(conn)

	// 读取消息长度
	lengthBytes := make([]byte, 4)
	_, err := io.ReadFull(reader, lengthBytes)
	if err != nil {
		return Message{}, fmt.Errorf("error reading message length: %w", err)
	}

	length := binary.BigEndian.Uint32(lengthBytes)

	// 读取消息体
	msgBytes := make([]byte, length)
	_, err = io.ReadFull(reader, msgBytes)
	if err != nil {
		return Message{}, fmt.Errorf("error reading message body: %w", err)
	}

	// 解压消息
	msg, err := decompressMessage(msgBytes)
	if err != nil {
		return Message{}, fmt.Errorf("error decompressing message: %w", err)
	}

	return msg, nil
}

// handleMessageWithPool uses a Goroutine pool to handle messages.
func (n *Node) handleMessageWithPool(conn net.Conn, msg Message) {
	_ = n.pool.Submit(func() {
		n.router.RouteMessage(n, conn, msg)
	})
}

// generateTraceID generates a unique trace ID.
func generateTraceID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// startDiscovery periodically broadcasts the peer list to all connected peers.
func (n *Node) startDiscovery() {
	for {
		time.Sleep(10 * time.Second)
		conns := n.net.GetConns()
		for _, conn := range conns {
			n.sendPeerList(conn)
		}
	}
}

// startHeartbeat periodically sends ping messages to all connected peers.
func (n *Node) startHeartbeat() {
	for {
		time.Sleep(5 * time.Second)
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

// connectToPeer attempts to connect to a peer.
func (n *Node) connectToPeer(peerAddr string) {
	if peerAddr == ":"+n.Port {
		n.logger.Debugf("Skipping connection to self: %s", peerAddr)
		return
	}

	if _, loaded := n.peers.KnownPeers.Load(peerAddr); loaded {
		n.logger.Debugf("Already connected to peer: %s", peerAddr)
		return
	}

	n.logger.Infof("Attempting to connect to peer: %s", peerAddr)
	conn, err := net.Dial("tcp", peerAddr)
	if err != nil {
		n.logger.WithFields(logrus.Fields{
			"peer_addr": peerAddr,
			"error":     err,
		}).Error("Error connecting to peer")
		return
	}

	n.net.addConn(conn)
	n.peers.AddPeer(peerAddr)
	n.logger.Infof("Successfully connected to peer: %s", peerAddr)

	// Request peer list from the connected peer
	n.requestPeerList(conn)

	go n.handleConnection(conn)
}

// requestPeerList requests the peer list from a connection.
func (n *Node) requestPeerList(conn net.Conn) {
	msg := Message{Type: MessageTypePeerListReq, Data: "", Sender: n.Name, Address: ":" + n.Port, ID: generateMessageID()}
	if err := n.net.SendMessage(conn, msg, compressMessage); err != nil {
		n.logger.WithError(err).Error("Error requesting peer list")
	}
}

// sendPeerList sends the current peer list to a connection.
func (n *Node) sendPeerList(conn net.Conn) {
	peers := n.peers.GetPeers()
	if len(peers) == 0 {
		n.logger.Debugf("No peers to send to %s", conn.RemoteAddr().String())
		return
	}

	peerList, err := json.Marshal(peers)
	if err != nil {
		n.logger.WithError(err).Error("Error encoding peer list")
		return
	}

	msg := Message{Type: MessageTypePeerList, Data: string(peerList), Sender: n.Name, Address: ":" + n.Port, ID: generateMessageID()}
	if err := n.net.SendMessage(conn, msg, compressMessage); err != nil {
		n.logger.WithError(err).Error("Error sending peer list")
	}
}

// send broadcasts a message to all connected peers.
func (n *Node) send(message string) {
	msg := Message{Type: MessageTypeChat, Data: message, Sender: n.Name, Address: ":" + n.Port, ID: generateMessageID()}
	conns := n.net.GetConns()
	for _, conn := range conns {
		go func(c net.Conn) {
			if err := n.net.SendMessage(c, msg, compressMessage); err != nil {
				n.logger.WithError(err).Error("Error sending message")
				n.net.removeConn(c)
			}
		}(conn)
	}
}
