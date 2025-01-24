// network.go
package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
)

// NetworkManager handles network connections.
type NetworkManager struct {
	Conns          sync.Map
	sendBufferPool *sync.Pool
	readBufferPool *sync.Pool
	logger         Logger
	executor       TaskExecutor
}

// NewNetworkManager creates a new NetworkManager instance.
func NewNetworkManager(logger Logger, executor TaskExecutor) *NetworkManager {
	return &NetworkManager{
		Conns:          sync.Map{},
		sendBufferPool: newBufferPool(),
		readBufferPool: newBufferPool(),
		logger:         logger,
		executor:       executor,
	}
}

// addConn adds a connection to the network.
func (nm *NetworkManager) addConn(conn net.Conn) {
	nm.Conns.Store(conn.RemoteAddr().String(), conn)
}

// removeConn removes a connection from the network.
func (nm *NetworkManager) removeConn(conn net.Conn) {
	nm.Conns.Delete(conn.RemoteAddr().String())
}

// SendMessage sends a message to a connection.
func (nm *NetworkManager) SendMessage(conn net.Conn, msg Message) error {
	// 压缩消息
	msgBytes, err := compressMessage(msg)
	if err != nil {
		return err
	}

	// 从缓冲池中获取缓冲区
	bufferPtr := nm.sendBufferPool.Get().(*[]byte)
	defer nm.sendBufferPool.Put(bufferPtr)
	buffer := *bufferPtr

	// 如果消息长度超过缓冲区大小，重新分配更大的缓冲区
	if len(msgBytes) > len(buffer) {
		buffer = make([]byte, len(msgBytes))
	} else {
		buffer = buffer[:len(msgBytes)]
	}

	// 将消息内容复制到缓冲区
	copy(buffer, msgBytes)

	// 发送消息长度
	length := uint32(len(buffer))
	lengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBytes, length)
	if _, err := conn.Write(lengthBytes); err != nil {
		return fmt.Errorf("error sending message length: %w", err)
	}

	// 发送消息体
	if _, err := conn.Write(buffer); err != nil {
		return fmt.Errorf("error sending message body: %w", err)
	}

	return nil
}

// ReadMessage reads a message from the connection.
func (nm *NetworkManager) ReadMessage(conn net.Conn) (Message, error) {
	reader := bufio.NewReader(conn)

	// 从缓冲池中获取缓冲区
	bufferPtr := nm.readBufferPool.Get().(*[]byte)
	defer nm.readBufferPool.Put(bufferPtr)
	buffer := *bufferPtr

	// 读取消息长度
	lengthBytes := buffer[:4]
	_, err := io.ReadFull(reader, lengthBytes)
	if err != nil {
		return Message{}, fmt.Errorf("error reading message length: %w", err)
	}

	length := binary.BigEndian.Uint32(lengthBytes)

	// 如果消息长度超过缓冲区大小，重新分配更大的缓冲区
	if int(length) > len(buffer) {
		buffer = make([]byte, length)
	} else {
		buffer = buffer[:length]
	}

	// 读取消息体
	_, err = io.ReadFull(reader, buffer)
	if err != nil {
		return Message{}, fmt.Errorf("error reading message body: %w", err)
	}

	// 解压消息
	msg, err := decompressMessage(buffer)
	if err != nil {
		return Message{}, fmt.Errorf("error decompressing message: %w", err)
	}

	return msg, nil
}

// GetConns returns a list of active connections.
func (nm *NetworkManager) GetConns() []net.Conn {
	conns := make([]net.Conn, 0)
	nm.Conns.Range(func(key, value interface{}) bool {
		conns = append(conns, value.(net.Conn))
		return true
	})
	return conns
}
