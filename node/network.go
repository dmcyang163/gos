package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"node/compressor"
	"os"
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
		sendBufferPool: compressor.NewBufferPool(),
		readBufferPool: compressor.NewBufferPool(),
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

// SendFile sends a file in chunks using sendBufferPool.
func (nm *NetworkManager) SendFile(conn net.Conn, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	chunkSize := 1024 * 1024 // 1MB per chunk

	buffer, releaseBuffer := nm.getBuffer(nm.sendBufferPool, chunkSize)
	defer releaseBuffer()

	chunkID := 0

	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to read file: %w", err)
		}

		msg := Message{
			Type:     MessageTypeFileTransfer,
			FileName: fileInfo.Name(),
			FileSize: fileInfo.Size(),
			Chunk:    buffer[:n],
			ChunkID:  chunkID,
			IsLast:   err == io.EOF, // 如果是文件末尾，设置 IsLast 为 true
		}

		// 发送消息
		if err := nm.SendMessage(conn, msg); err != nil {
			return fmt.Errorf("failed to send file chunk: %w", err)
		}

		chunkID++

		// 如果是文件末尾，退出循环
		if err == io.EOF {
			fmt.Printf("send last chunk %d\n", chunkID)
			break
		}
	}

	return nil
}

// SendMessage sends a message to a connection.
func (nm *NetworkManager) SendMessage(conn net.Conn, msg Message) error {
	// 压缩消息
	msgBytes, err := CompressMsg(msg)
	if err != nil {
		return err
	}

	// 使用 SendRawMessage 发送压缩后的消息
	return nm.SendRawMessage(conn, msgBytes)
}

// SendRawMessage sends raw bytes to a connection without additional copying.
func (nm *NetworkManager) SendRawMessage(conn net.Conn, data []byte) error {
	// 发送消息长度
	if err := nm.writeLength(conn, uint32(len(data))); err != nil {
		return fmt.Errorf("error sending message length: %w", err)
	}

	// 直接发送消息体
	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("error sending message body: %w", err)
	}

	return nil
}

// writeLength writes the length of the message as a 4-byte big-endian integer.
func (nm *NetworkManager) writeLength(conn net.Conn, length uint32) error {
	lengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBytes, length)
	_, err := conn.Write(lengthBytes)
	return err
}

// ReadMessage reads a message from the connection.
func (nm *NetworkManager) ReadMessage(conn net.Conn) (Message, error) {
	reader := bufio.NewReader(conn)

	// 读取消息长度
	length, err := nm.readLength(reader)
	if err != nil {
		return Message{}, fmt.Errorf("error reading message length: %w", err)
	}

	buffer, releaseBuffer := nm.getBuffer(nm.readBufferPool, int(length))
	defer releaseBuffer()

	// 读取消息体
	if _, err := io.ReadFull(reader, buffer); err != nil {
		return Message{}, fmt.Errorf("error reading message body: %w", err)
	}

	// 解压消息
	msg, err := DecompressMsg(buffer)
	if err != nil {
		return Message{}, fmt.Errorf("error decompressing message: %w", err)
	}

	return msg, nil
}

// readLength reads the length of the message as a 4-byte big-endian integer.
func (nm *NetworkManager) readLength(reader *bufio.Reader) (uint32, error) {
	lengthBytes := make([]byte, 4)
	_, err := io.ReadFull(reader, lengthBytes)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(lengthBytes), nil
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

// getBuffer gets a buffer from the specified buffer pool and ensures it has at least the specified size.
func (nm *NetworkManager) getBuffer(pool *sync.Pool, size int) ([]byte, func()) {
	bufferPtr := pool.Get().(*[]byte)
	buffer := *bufferPtr

	if len(buffer) < size {
		buffer = make([]byte, size)
	} else {
		buffer = buffer[:size]
	}

	releaseBuffer := func() {
		pool.Put(&buffer)
	}

	return buffer, releaseBuffer
}
