package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// 全局名字列表
var names []NameEntry

// Node represents a peer in the P2P network.
type Node struct {
	Port           string
	Net            *NetworkManager
	Peers          *PeerManager
	logger         *logrus.Logger
	config         *Config
	Name           string
	Description    string
	SpecialAbility string
	Tone           string
	Dialogues      []string
	processedMsgs  sync.Map // 用于存储已处理的消息ID
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
	return &Node{
		Port:           config.Port,
		Net:            &NetworkManager{Conns: make(map[string]net.Conn)},
		Peers:          &PeerManager{},
		logger:         logger,
		config:         config,
		Name:           name,
		Description:    entry.Description,
		SpecialAbility: entry.SpecialAbility,
		Tone:           entry.Tone,
		Dialogues:      entry.Dialogues,
		processedMsgs:  sync.Map{},
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

		n.Net.addConn(conn)
		go n.handleConnection(conn)
	}
}

// handleConnection handles incoming messages from a connection.
func (n *Node) handleConnection(conn net.Conn) {
	defer func() {
		conn.Close()
		n.Net.removeConn(conn)
		n.Peers.KnownPeers.Delete(conn.RemoteAddr().String())
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

		// 处理消息
		switch msg.Type {
		case "peer_list":
			n.handlePeerMessage(msg.Data)
		case "peer_list_request":
			n.logger.Infof("Received peer list request from: %s", conn.RemoteAddr().String())
			n.sendPeerList(conn)
		case "chat":
			// 检查消息是否来自当前节点
			if msg.Sender == n.Name && msg.Address == ":"+n.Port {
				n.logger.Debugf("Ignoring message from self: %s", msg.Sender)
				continue
			}

			n.logger.Infof("Received chat message from %s (%s): %s", msg.Sender, msg.Address, msg.Data)

			// 只有在特定条件下才生成回复消息
			if shouldReplyToMessage(msg) {
				dialogue := findDialogueForSender(msg.Sender)
				n.sendMessage(conn, Message{Type: "chat", Data: dialogue, Sender: n.Name, Address: ":" + n.Port, ID: generateMessageID()})
			}
		case "ping":
			n.logger.Debugf("Received ping from: %s", conn.RemoteAddr().String())
			n.sendMessage(conn, Message{Type: "pong", Data: "", Sender: n.Name, Address: ":" + n.Port, ID: generateMessageID()})
		case "pong":
			n.logger.Debugf("Received pong from: %s", conn.RemoteAddr().String())
		default:
			n.logger.Warnf("Unknown message type: %s", msg.Type)
		}
	}
}

// shouldReplyToMessage 决定是否对消息进行回复
func shouldReplyToMessage(msg Message) bool {
	// 这里可以根据消息内容或其他条件决定是否回复
	// 例如，只有在消息包含特定关键词时才回复
	return strings.Contains(msg.Data, "hello")
}

// handlePeerMessage handles peer list messages.
func (n *Node) handlePeerMessage(message string) {
	var peers []string
	if err := json.Unmarshal([]byte(message), &peers); err != nil {
		n.logger.WithError(err).Error("Error decoding peer list")
		return
	}

	for _, peer := range peers {
		if peer == ":"+n.Port {
			continue // Skip self
		}

		if _, loaded := n.Peers.KnownPeers.LoadOrStore(peer, struct{}{}); !loaded {
			n.logger.Infof("Discovered new peer: %s", peer)
			go n.connectToPeer(peer)
		}
	}
}

// connectToPeer attempts to connect to a peer.
func (n *Node) connectToPeer(peerAddr string) {
	if peerAddr == ":"+n.Port {
		n.logger.Debugf("Skipping connection to self: %s", peerAddr)
		return
	}

	if _, loaded := n.Peers.KnownPeers.Load(peerAddr); loaded {
		n.logger.Debugf("Already connected to peer: %s", peerAddr)
		return
	}

	n.logger.Infof("Attempting to connect to peer: %s", peerAddr)
	conn, err := net.Dial("tcp", peerAddr)
	if err != nil {
		n.logger.WithError(err).Errorf("Error connecting to peer %s", peerAddr)
		return
	}

	n.Net.addConn(conn)
	n.Peers.KnownPeers.Store(peerAddr, struct{}{})
	n.logger.Infof("Successfully connected to peer: %s", peerAddr)

	// Request peer list from the connected peer
	n.requestPeerList(conn)

	go n.handleConnection(conn)
}

// requestPeerList requests the peer list from a connection.
func (n *Node) requestPeerList(conn net.Conn) {
	msg := Message{Type: "peer_list_request", Data: "", Sender: n.Name, Address: ":" + n.Port, ID: generateMessageID()}
	if err := n.sendMessage(conn, msg); err != nil {
		n.logger.WithError(err).Error("Error requesting peer list")
	}
}

// sendPeerList sends the current peer list to a connection.
func (n *Node) sendPeerList(conn net.Conn) {
	peers := make([]string, 0)
	n.Peers.KnownPeers.Range(func(k, _ interface{}) bool {
		if k.(string) != ":"+n.Port && k.(string) != conn.RemoteAddr().String() {
			peers = append(peers, k.(string))
		}
		return true
	})

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
	if err := n.sendMessage(conn, msg); err != nil {
		n.logger.WithError(err).Error("Error sending peer list")
	}
}

// sendMessage sends a message to a connection.
func (n *Node) sendMessage(conn net.Conn, msg Message) error {
	msgBytes, err := compressMessage(msg)
	if err != nil {
		return err
	}

	// 添加 4 字节的长度字段
	length := uint32(len(msgBytes))
	lengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBytes, length)

	// 发送长度字段 + 消息体
	_, err = conn.Write(append(lengthBytes, msgBytes...))
	return err
}

// send broadcasts a message to all connected peers.
func (n *Node) send(message string) {
	msg := Message{Type: "chat", Data: message, Sender: n.Name, Address: ":" + n.Port, ID: generateMessageID()}
	n.Net.mu.Lock()
	defer n.Net.mu.Unlock()

	for _, conn := range n.Net.Conns {
		go func(c net.Conn) {
			if err := n.sendMessage(c, msg); err != nil {
				n.logger.WithError(err).Error("Error sending message")
				n.Net.removeConn(c)
			}
		}(conn)
	}
}

// startDiscovery periodically broadcasts the peer list to all connected peers.
func (n *Node) startDiscovery() {
	for {
		time.Sleep(10 * time.Second)
		n.Net.mu.Lock()
		connList := make([]net.Conn, 0, len(n.Net.Conns))
		for _, conn := range n.Net.Conns {
			connList = append(connList, conn)
		}
		n.Net.mu.Unlock()

		for _, conn := range connList {
			n.sendPeerList(conn)
		}
	}
}

// startHeartbeat periodically sends ping messages to all connected peers.
func (n *Node) startHeartbeat() {
	for {
		time.Sleep(5 * time.Second)
		n.Net.mu.Lock()
		for addr, conn := range n.Net.Conns {
			go func(c net.Conn, a string) {
				msg := Message{Type: "ping", Data: "", Sender: n.Name, Address: ":" + n.Port, ID: generateMessageID()}
				if err := n.sendMessage(c, msg); err != nil {
					n.logger.WithError(err).Errorf("Heartbeat failed for %s", a)
					n.Net.removeConn(c)
					n.Peers.KnownPeers.Delete(a)
				}
			}(conn, addr)
		}
		n.Net.mu.Unlock()
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

func main() {
	rand.Seed(time.Now().UnixNano())

	// 加载名字列表
	var err error
	names, err = LoadNames("names.json")
	if err != nil {
		fmt.Printf("Error loading names: %v\n", err)
		return
	}

	// 检查命令行参数
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <config_file>")
		return
	}

	// 加载配置文件
	config, err := LoadConfig(os.Args[1])
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	// 创建节点
	node := NewNode(config, names)

	// 启动服务器和其他协程
	go node.startServer()
	go node.startDiscovery()
	go node.startHeartbeat()

	// 连接到引导节点
	if config.BootstrapNode != "" {
		node.connectToPeer(config.BootstrapNode)
	}

	// 读取用户输入并发送消息
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		message := scanner.Text()
		node.send(message)
	}
}
