package main

import (
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
	KnownPeers sync.Map // map[string]PeerInfo
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
