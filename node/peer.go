package main

import (
	"sync"
)

// PeerManager handles peer discovery and management.
type PeerManager struct {
	KnownPeers sync.Map
}

// AddPeer adds a new peer to the known peers list.
func (pm *PeerManager) AddPeer(peer string) {
	pm.KnownPeers.Store(peer, struct{}{})
}

// RemovePeer removes a peer from the known peers list.
func (pm *PeerManager) RemovePeer(peer string) {
	pm.KnownPeers.Delete(peer)
}
