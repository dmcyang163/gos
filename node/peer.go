package main

import (
	"encoding/json"
	"fmt"
	"net"
	"node/utils"
	"sync"
	"time"
)

// PeerInfo 存储节点的信息和最后活跃时间
type PeerInfo struct {
	Address  string    // 节点地址
	LastSeen time.Time // 最后活跃时间
}

// PeerManager 处理节点的发现和管理
type PeerManager struct {
	KnownPeers sync.Map
	logger     utils.Logger
	executor   utils.TaskExecutor
}

// NewPeerManager 创建一个新的 PeerManager
func NewPeerManager(logger utils.Logger, executor utils.TaskExecutor) *PeerManager {
	return &PeerManager{
		KnownPeers: sync.Map{},
		logger:     logger,
		executor:   executor,
	}
}

// AddPeer 添加一个新的节点到已知节点列表
func (pm *PeerManager) AddPeer(peer string) {
	pm.KnownPeers.Store(peer, PeerInfo{Address: peer, LastSeen: time.Now()})
}

// RemovePeer 从已知节点列表中移除一个节点
func (pm *PeerManager) RemovePeer(peer string) {
	pm.KnownPeers.Delete(peer)
}

// GetPeers 返回已知节点列表
func (pm *PeerManager) GetPeers() []string {
	peers := make([]string, 0)
	pm.KnownPeers.Range(func(k, v interface{}) bool {
		peers = append(peers, k.(string))
		return true
	})
	return peers
}

// UpdateLastSeen 更新节点的最后活跃时间
func (pm *PeerManager) UpdateLastSeen(peer string, lastSeen time.Time) {
	if info, ok := pm.KnownPeers.Load(peer); ok {
		peerInfo := info.(PeerInfo)
		peerInfo.LastSeen = lastSeen
		pm.KnownPeers.Store(peer, peerInfo)
	}
}

// CheckPeerHealth 移除不活跃的节点
func (pm *PeerManager) CheckPeerHealth(timeout time.Duration) {
	pm.KnownPeers.Range(func(k, v interface{}) bool {
		peerInfo := v.(PeerInfo)
		if time.Since(peerInfo.LastSeen) > timeout {
			pm.KnownPeers.Delete(k)
		}
		return true
	})
}

// connectToPeer 尝试连接到一个节点
func (n *Node) connectToPeer(peerAddr string) error {
	// 如果尝试连接到自身，直接返回
	if peerAddr == ":"+n.Port {
		return nil
	}

	// 如果已经连接到该节点，直接返回
	if _, loaded := n.peers.KnownPeers.Load(peerAddr); loaded {
		return nil
	}

	// 建立连接
	conn, err := n.establishPeerConnection(peerAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to peer: %w", err)
	}

	// 添加连接并记录节点
	n.net.addConn(conn)
	n.peers.AddPeer(peerAddr)
	n.logger.Infof("Successfully connected to peer: %s", peerAddr)

	// 请求节点列表
	n.requestPeerList(conn)

	// 提交连接处理任务
	err = n.executor.Submit(func() {
		n.handleConnection(conn)
	})
	if err != nil {
		n.logger.Errorf("Failed to submit connection handling task: %v", err)
	}

	return nil
}

// establishPeerConnection 建立与节点的连接
func (n *Node) establishPeerConnection(peerAddr string) (net.Conn, error) {
	conn, err := net.Dial("tcp", peerAddr)
	if err != nil {
		n.logger.Errorf("Error connecting to peer: %v", err)
		return nil, err
	}
	return conn, nil
}

// requestPeerList 从连接中请求节点列表
func (n *Node) requestPeerList(conn net.Conn) {
	msg := Message{Type: MsgTypePeerListReq, Data: "", Sender: n.User.Name, Address: ":" + n.Port, ID: generateMessageID()}
	if err := n.net.SendMessage(conn, msg); err != nil {
		n.logger.Errorf("Error requesting peer list: %v", err)
	}
}

// encodePeerList 将节点列表编码为 JSON 字符串
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

// sendPeerList 发送当前节点列表到连接
func (n *Node) sendPeerList(conn net.Conn) error {
	peerList, err := n.encodePeerList()
	if err != nil {
		return err
	}

	if peerList == "" {
		return nil
	}

	msg := Message{
		Type:    MsgTypePeerList,
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
