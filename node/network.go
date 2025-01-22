package main

import (
	"net"
	"sync"
)

// NetworkManager handles network connections.
type NetworkManager struct {
	Conns map[string]net.Conn
	mu    sync.Mutex
}

// addConn adds a connection to the network.
func (nm *NetworkManager) addConn(conn net.Conn) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	nm.Conns[conn.RemoteAddr().String()] = conn
}

// removeConn removes a connection from the network.
func (nm *NetworkManager) removeConn(conn net.Conn) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	delete(nm.Conns, conn.RemoteAddr().String())
}
