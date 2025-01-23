// msgHandler.go
package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
)

// MessageType 定义消息类型
const (
	MessageTypeChat         = "chat"
	MessageTypePeerList     = "peer_list"
	MessageTypePeerListReq  = "peer_list_request"
	MessageTypePing         = "ping" // 心跳请求
	MessageTypePong         = "pong" // 心跳响应
	MessageTypeFileTransfer = "file_transfer"
	MessageTypeNodeStatus   = "node_status"
)

// MessageHandler defines the interface for handling messages.
type MessageHandler interface {
	HandleMessage(n *Node, conn net.Conn, msg Message)
}

// PeerListHandler handles "peer_list" messages.
type PeerListHandler struct{}

func (h *PeerListHandler) HandleMessage(n *Node, conn net.Conn, msg Message) {
	var peers []string
	if err := json.Unmarshal([]byte(msg.Data), &peers); err != nil {
		n.logger.WithError(err).Error("Error decoding peer list")
		return
	}

	for _, peer := range peers {
		if peer == ":"+n.Port {
			continue // Skip self
		}

		if _, loaded := n.peers.KnownPeers.LoadOrStore(peer, PeerInfo{Address: peer, LastSeen: time.Now()}); !loaded {
			n.logger.Infof("Discovered new peer: %s", peer)
			go n.connectToPeer(peer) // 这里修复了语法错误
		}
	}
}

// PeerListRequestHandler handles "peer_list_request" messages.
type PeerListRequestHandler struct{}

func (h *PeerListRequestHandler) HandleMessage(n *Node, conn net.Conn, msg Message) {
	n.logger.Infof("Received peer list request from: %s", conn.RemoteAddr().String())
	n.sendPeerList(conn)
}

// ChatHandler handles "chat" messages.
type ChatHandler struct{}

func (h *ChatHandler) HandleMessage(n *Node, conn net.Conn, msg Message) {
	go func() {
		if msg.Sender == n.Name && msg.Address == ":"+n.Port {
			n.logger.Debugf("Ignoring message from self: %s", msg.Sender)
			return
		}

		// 记录收到的聊天消息
		n.logger.WithFields(logrus.Fields{
			"sender":  msg.Sender,
			"address": msg.Address,
			"message": msg.Data,
		}).Info("Received chat message")

		// 彩色显示接收到的消息
		color.Cyan("%s: %s\n", msg.Sender, msg.Data)

		if shouldReplyToMessage(msg) {
			dialogue := findDialogueForSender(msg.Sender)
			n.logger.WithFields(logrus.Fields{
				"reply": dialogue,
			}).Info("Sending reply")
			n.net.SendMessage(conn, Message{Type: MessageTypeChat, Data: dialogue, Sender: n.Name, Address: ":" + n.Port, ID: generateMessageID()}, compressMessage) // 传递 compressFunc
		}
	}()
}

// PingHandler handles "ping" messages.
type PingHandler struct{}

func (h *PingHandler) HandleMessage(n *Node, conn net.Conn, msg Message) {
	n.logger.Debugf("Received ping from: %s", conn.RemoteAddr().String())
	// 回复 Pong 消息
	n.net.SendMessage(conn, Message{Type: MessageTypePong, Data: "", Sender: n.Name, Address: ":" + n.Port, ID: generateMessageID()}, compressMessage) // 传递 compressFunc
}

// PongHandler handles "pong" messages.
type PongHandler struct{}

func (h *PongHandler) HandleMessage(n *Node, conn net.Conn, msg Message) {
	n.logger.Debugf("Received pong from: %s", conn.RemoteAddr().String())
	// 更新节点的 LastSeen 时间
	n.peers.UpdateLastSeen(conn.RemoteAddr().String(), time.Now())
}

// FileTransferHandler handles "file_transfer" messages.
type FileTransferHandler struct{}

func (h *FileTransferHandler) HandleMessage(n *Node, conn net.Conn, msg Message) {
	n.logger.WithFields(logrus.Fields{
		"sender":  msg.Sender,
		"address": msg.Address,
		"file":    msg.Data,
	}).Info("Received file transfer request")
}

// NodeStatusHandler handles "node_status" messages.
type NodeStatusHandler struct{}

func (h *NodeStatusHandler) HandleMessage(n *Node, conn net.Conn, msg Message) {
	n.logger.WithFields(logrus.Fields{
		"sender":  msg.Sender,
		"address": msg.Address,
	}).Info("Received node status request")

	// 返回节点状态
	status := map[string]interface{}{
		"name":    n.Name,
		"port":    n.Port,
		"peers":   n.peers.GetPeers(),
		"conns":   len(n.net.GetConns()),
		"healthy": true,
	}

	statusBytes, err := json.Marshal(status)
	if err != nil {
		n.logger.WithError(err).Error("Error encoding node status")
		return
	}

	n.net.SendMessage(conn, Message{
		Type:    MessageTypeNodeStatus,
		Data:    string(statusBytes),
		Sender:  n.Name,
		Address: ":" + n.Port,
		ID:      generateMessageID(),
	}, compressMessage) // 传递 compressFunc
}

// shouldReplyToMessage decides whether to reply to a message.
func shouldReplyToMessage(msg Message) bool {
	// 只有在消息包含 "hello" 时才回复
	return strings.Contains(msg.Data, "hello")
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
