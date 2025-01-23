package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

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
	processedMsgs  sync.Map // 用于存储已处理的消息ID

	// 依赖注入
	net   *NetworkManager // 管理网络连接
	peers *PeerManager    // 管理已知节点

	// 消息路由器
	router *MessageRouter
}

// NewNode creates a new Node instance.
func NewNode(config *Config, names []NameEntry) *Node {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		ForceColors:     true,
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	// Set log level
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
	router.RegisterHandler("peer_list", &PeerListHandler{})
	router.RegisterHandler("peer_list_request", &PeerListRequestHandler{})
	router.RegisterHandler("chat", &ChatHandler{})
	router.RegisterHandler("ping", &PingHandler{})
	router.RegisterHandler("pong", &PongHandler{})

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
		net:            &NetworkManager{Conns: make(map[string]net.Conn)},
		peers:          &PeerManager{},
		router:         router,
	}
}

// startServer starts the TCP server to listen for incoming connections.
func (n *Node) startServer() {
	ln, err := net.Listen("tcp", ":"+n.Port)
	if err != nil {
		n.logger.WithError(err).Error("Error starting server")
		return
	}
	defer ln.Close()

	n.logger.Infof("Server started on port %s. My name is: %s", n.Port, n.Name)
	n.logger.Infof("Description: %s", n.Description)
	n.logger.Infof("Special Ability: %s", n.SpecialAbility)
	n.logger.Infof("Tone: %s", n.Tone)

	for {
		conn, err := ln.Accept()
		if err != nil {
			n.logger.WithError(err).Error("Error accepting connection")
			continue
		}

		remoteAddr := conn.RemoteAddr().String()
		n.logger.Infof("New connection from: %s", remoteAddr)

		n.net.addConn(conn)
		go n.handleConnection(conn)
	}
}

// handleConnection handles incoming messages from a connection.
func (n *Node) handleConnection(conn net.Conn) {
	defer func() {
		conn.Close()
		n.net.removeConn(conn)
		n.peers.RemovePeer(conn.RemoteAddr().String())
		n.logger.Infof("Connection closed: %s", conn.RemoteAddr().String())
	}()

	reader := bufio.NewReader(conn)
	for {
		// 读取 4 字节的长度字段
		lengthBytes := make([]byte, 4)
		_, err := io.ReadFull(reader, lengthBytes)
		if err != nil {
			n.logger.WithError(err).Error("Error reading message length")
			return
		}

		// 解析消息长度
		length := binary.BigEndian.Uint32(lengthBytes)

		// 读取消息体
		msgBytes := make([]byte, length)
		_, err = io.ReadFull(reader, msgBytes)
		if err != nil {
			n.logger.WithError(err).Error("Error reading message body")
			return
		}

		// 解压消息
		msg, err := decompressMessage(msgBytes)
		if err != nil {
			n.logger.WithError(err).Error("Error decompressing message")
			continue
		}

		// 检查消息是否已处理
		if _, loaded := n.processedMsgs.LoadOrStore(msg.ID, struct{}{}); loaded {
			n.logger.Debugf("Message already processed: %s", msg.ID)
			continue
		}

		// 使用消息路由器处理消息
		n.router.RouteMessage(n, conn, msg)
	}
}

// shouldReplyToMessage 决定是否对消息进行回复
func shouldReplyToMessage(msg Message) bool {
	// 这里可以根据消息内容或其他条件决定是否回复
	// 例如，只有在消息包含特定关键词时才回复
	return strings.Contains(msg.Data, "hello")
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
		n.logger.WithError(err).Errorf("Error connecting to peer %s", peerAddr)
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
	msg := Message{Type: "peer_list_request", Data: "", Sender: n.Name, Address: ":" + n.Port, ID: generateMessageID()}
	if err := n.net.SendMessage(conn, msg); err != nil {
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

	msg := Message{Type: "peer_list", Data: string(peerList), Sender: n.Name, Address: ":" + n.Port, ID: generateMessageID()}
	if err := n.net.SendMessage(conn, msg); err != nil {
		n.logger.WithError(err).Error("Error sending peer list")
	}
}

// send broadcasts a message to all connected peers.
func (n *Node) send(message string) {
	msg := Message{Type: "chat", Data: message, Sender: n.Name, Address: ":" + n.Port, ID: generateMessageID()}
	conns := n.net.GetConns()
	for _, conn := range conns {
		go func(c net.Conn) {
			if err := n.net.SendMessage(c, msg); err != nil {
				n.logger.WithError(err).Error("Error sending message")
				n.net.removeConn(c)
			}
		}(conn)
	}
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
				msg := Message{Type: "ping", Data: "", Sender: n.Name, Address: ":" + n.Port, ID: generateMessageID()}
				if err := n.net.SendMessage(c, msg); err != nil {
					n.logger.WithError(err).Error("Heartbeat failed")
					n.net.removeConn(c)
					n.peers.RemovePeer(c.RemoteAddr().String())
				}
			}(conn)
		}
	}
}

// findDialogueForSender finds a dialogue for the sender based on their name.
func findDialogueForSender(sender string) string {
	for _, entry := range names {
		if strings.Contains(sender, entry.Name) {
			if len(entry.Dialogues) > 0 {
				return entry.Dialogues[rand.Intn(len(entry.Dialogues))]
			}
		}
	}
	return "你好，我是" + sender + "。"
}

// generateMessageID generates a unique message ID.
func generateMessageID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
