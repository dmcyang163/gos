package main

import (
	"encoding/json"
	"net"
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

		if _, loaded := n.peers.KnownPeers.LoadOrStore(peer, struct{}{}); !loaded {
			n.logger.Infof("Discovered new peer: %s", peer)
			go n.connectToPeer(peer)
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
	// 检查消息是否来自当前节点
	if msg.Sender == n.Name && msg.Address == ":"+n.Port {
		n.logger.Debugf("Ignoring message from self: %s", msg.Sender)
		return
	}

	n.logger.Infof("Received chat message from %s (%s): %s", msg.Sender, msg.Address, msg.Data)

	// 只有在特定条件下才生成回复消息
	if shouldReplyToMessage(msg) {
		dialogue := findDialogueForSender(msg.Sender)
		n.net.SendMessage(conn, Message{Type: "chat", Data: dialogue, Sender: n.Name, Address: ":" + n.Port, ID: generateMessageID()})
	}
}

// PingHandler handles "ping" messages.
type PingHandler struct{}

func (h *PingHandler) HandleMessage(n *Node, conn net.Conn, msg Message) {
	n.logger.Debugf("Received ping from: %s", conn.RemoteAddr().String())
	n.net.SendMessage(conn, Message{Type: "pong", Data: "", Sender: n.Name, Address: ":" + n.Port, ID: generateMessageID()})
}

// PongHandler handles "pong" messages.
type PongHandler struct{}

func (h *PongHandler) HandleMessage(n *Node, conn net.Conn, msg Message) {
	n.logger.Debugf("Received pong from: %s", conn.RemoteAddr().String())
}
