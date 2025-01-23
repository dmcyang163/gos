package main

import (
	"encoding/binary"
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

// SendMessage sends a message to a connection.
func (nm *NetworkManager) SendMessage(conn net.Conn, msg Message) error {
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

// GetConns returns a list of active connections.
func (nm *NetworkManager) GetConns() []net.Conn {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	conns := make([]net.Conn, 0, len(nm.Conns))
	for _, conn := range nm.Conns {
		conns = append(conns, conn)
	}
	return conns
}
