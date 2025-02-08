// peer.go
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

// PeerInfo 存储节点的信息和最后活跃时间
type PeerInfo struct {
	Address  string
	LastSeen time.Time
}

// PeerManager handles peer discovery and management.
type PeerManager struct {
	KnownPeers sync.Map
	logger     Logger
	executor   TaskExecutor
}

// NewPeerManager 创建一个新的 PeerManager
func NewPeerManager(logger Logger, executor TaskExecutor) *PeerManager {
	return &PeerManager{
		KnownPeers: sync.Map{},
		logger:     logger,
		executor:   executor,
	}
}

// AddPeer adds a new peer to the known peers list.
func (pm *PeerManager) AddPeer(peer string) {
	pm.KnownPeers.Store(peer, PeerInfo{Address: peer, LastSeen: time.Now()})
}

// RemovePeer removes a peer from the known peers list.
func (pm *PeerManager) RemovePeer(peer string) {
	pm.KnownPeers.Delete(peer)
}

// GetPeers returns a list of known peers.
func (pm *PeerManager) GetPeers() []string {
	peers := make([]string, 0)
	pm.KnownPeers.Range(func(k, v interface{}) bool {
		peers = append(peers, k.(string))
		return true
	})
	return peers
}

// UpdateLastSeen updates the LastSeen timestamp for a peer.
func (pm *PeerManager) UpdateLastSeen(peer string, lastSeen time.Time) {
	if info, ok := pm.KnownPeers.Load(peer); ok {
		peerInfo := info.(PeerInfo)
		peerInfo.LastSeen = lastSeen
		pm.KnownPeers.Store(peer, peerInfo)
	}
}

// CheckPeerHealth removes inactive peers.
func (pm *PeerManager) CheckPeerHealth(timeout time.Duration) {
	pm.KnownPeers.Range(func(k, v interface{}) bool {
		peerInfo := v.(PeerInfo)
		if time.Since(peerInfo.LastSeen) > timeout {
			pm.KnownPeers.Delete(k)
		}
		return true
	})
}

// connectToPeer attempts to connect to a peer.
func (n *Node) connectToPeer(peerAddr string) error {
	if peerAddr == ":"+n.Port {
		n.logger.Debugf("Skipping connection to self: %s", peerAddr)
		return nil
	}

	if _, loaded := n.peers.KnownPeers.Load(peerAddr); loaded {
		n.logger.Debugf("Already connected to peer: %s", peerAddr)
		return nil
	}

	conn, err := n.establishPeerConnection(peerAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to peer: %w", err)
	}

	n.net.addConn(conn)
	n.peers.AddPeer(peerAddr)
	n.logger.Infof("Successfully connected to peer: %s", peerAddr)

	// 请求 Peer 列表
	n.requestPeerList(conn)

	err = n.executor.Submit(func() {
		n.handleConnection(conn)
	})
	if err != nil {
		n.logger.Errorf("Failed to submit connection handling task to executor: %v", err)
	}

	return nil
}

func (n *Node) establishPeerConnection(peerAddr string) (net.Conn, error) {
	n.logger.Infof("Attempting to connect to peer: %s", peerAddr)
	conn, err := net.Dial("tcp", peerAddr)
	if err != nil {
		n.logger.Errorf("Error connecting to peer: %v", err)
		return nil, err
	}
	return conn, nil
}

// requestPeerList requests the peer list from a connection.
func (n *Node) requestPeerList(conn net.Conn) {
	msg := Message{Type: MessageTypePeerListReq, Data: "", Sender: n.User.Name, Address: ":" + n.Port, ID: generateMessageID()}
	if err := n.net.SendMessage(conn, msg); err != nil {
		n.logger.Errorf("Error requesting peer list: %v", err)
	}
}

// encodePeerList 将 Peer 列表编码为 JSON 字符串
func (n *Node) encodePeerList() (string, error) {
	peers := n.peers.GetPeers()
	if len(peers) == 0 {
		return "", nil
	}

	peerList, err := json.Marshal(peers)
	if err != nil {
		n.logger.Errorf("Error encoding peer list: %v", err)
		return "", err
	}
	return string(peerList), nil
}

// sendPeerList sends the current peer list to a connection.
func (n *Node) sendPeerList(conn net.Conn) error {
	peerList, err := n.encodePeerList()
	if err != nil {
		return err
	}

	if peerList == "" {
		n.logger.Debugf("No peers to send to %s", conn.RemoteAddr().String())
		return nil
	}

	msg := Message{
		Type:    MessageTypePeerList,
		Data:    peerList,
		Sender:  n.User.Name,
		Address: ":" + n.Port,
		ID:      generateMessageID(),
	}

	if err := n.net.SendMessage(conn, msg); err != nil {
		n.logger.Errorf("Error sending peer list: %v", err)
		return err
	}
	return nil
}
