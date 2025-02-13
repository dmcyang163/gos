// network.go
package main

import (
	"bufio"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"node/utils"
	"os"
	"sync"
	"time"
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
		sendBufferPool: utils.NewBufferPool(),
		readBufferPool: utils.NewBufferPool(),
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
func calculateChecksum(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
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

	// 初始块大小
	chunkSize := 1024 * 1024        // 1MB
	minChunkSize := 512 * 1024      // 最小块大小 512KB
	maxChunkSize := 4 * 1024 * 1024 // 最大块大小 4MB

	buffer, releaseBuffer := nm.getBuffer(nm.sendBufferPool, maxChunkSize)
	defer releaseBuffer()

	chunkID := 0
	startTime := time.Now()
	var totalBytesSent int64

	for {
		// 读取文件块
		n, err := nm.readFileChunk(file, buffer[:chunkSize])
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to read file chunk: %w", err)
		}

		// 发送文件块
		if err := nm.sendChunkWithRetry(conn, fileInfo, buffer[:n], chunkID, err == io.EOF); err != nil {
			return fmt.Errorf("failed to send file chunk: %w", err)
		}

		totalBytesSent += int64(n)
		chunkID++

		// 动态调整块大小
		chunkSize = nm.calculateChunkSize(chunkSize, minChunkSize, maxChunkSize, startTime, totalBytesSent)

		if err == io.EOF {
			break
		}
	}
	return nil
}

// readFileChunk 读取文件的指定块
func (nm *NetworkManager) readFileChunk(file *os.File, buffer []byte) (int, error) {
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return 0, fmt.Errorf("failed to read file: %w", err)
	}
	return n, err // 返回读取的字节数和可能的错误
}

// sendChunkWithRetry 发送文件块并支持重试机制
func (nm *NetworkManager) sendChunkWithRetry(conn net.Conn, fileInfo os.FileInfo, chunk []byte, chunkID int, isLast bool) error {
	// 计算文件块的校验和
	checksum := calculateChecksum(chunk)

	msg := Message{
		Type:     MessageTypeFileTransfer,
		FileName: fileInfo.Name(),
		FileSize: fileInfo.Size(),
		Chunk:    chunk,
		ChunkID:  chunkID,
		IsLast:   isLast,
		Checksum: checksum, // 添加校验和
	}

	// 重试机制
	retries := 3
	for i := 0; i < retries; i++ {
		if err := nm.SendMessage(conn, msg); err != nil {
			if i == retries-1 {
				return fmt.Errorf("failed to send file chunk after %d retries: %w", retries, err)
			}
			time.Sleep(100 * time.Millisecond) // 等待 100ms 后重试
			continue
		}
		return nil // 发送成功
	}
	return nil
}

// calculateChunkSize 动态计算块大小
func (nm *NetworkManager) calculateChunkSize(currentChunkSize, minChunkSize, maxChunkSize int, startTime time.Time, totalBytesSent int64) int {
	elapsedTime := time.Since(startTime).Seconds()
	if elapsedTime > 0 {
		currentSpeed := float64(totalBytesSent) / elapsedTime // 当前传输速度（字节/秒）
		if currentSpeed > 10*1024*1024 {                      // 如果传输速度大于 10MB/s，增加块大小
			return min(currentChunkSize*2, maxChunkSize)
		} else if currentSpeed < 1*1024*1024 { // 如果传输速度小于 1MB/s，减少块大小
			return max(currentChunkSize/2, minChunkSize)
		}
	}
	return currentChunkSize
}

// 辅助函数：返回两个数中的最小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// 辅助函数：返回两个数中的最大值
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// SendMessage sends a message to a connection.
func (nm *NetworkManager) SendMessage(conn net.Conn, msg Message) error {
	// 打包消息
	msgBytes, err := PackMessage(msg)
	if err != nil {
		return fmt.Errorf("error packing message: %w", err)
	}

	// 记录日志
	nm.logger.WithFields(map[string]interface{}{
		"original_size":   len(msg.Data),
		"compressed_size": len(msgBytes),
	}).Debug("Message compression details")

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
		nm.logger.WithFields(map[string]interface{}{
			"remote_addr": conn.RemoteAddr().String(),
			"error":       err,
		}).Error("Failed to read message length")
		return Message{}, fmt.Errorf("error reading message length: %w", err)
	}

	// 获取缓冲区
	buffer, releaseBuffer := nm.getBuffer(nm.readBufferPool, int(length))
	defer releaseBuffer()

	// 读取消息体
	if _, err := io.ReadFull(reader, buffer); err != nil {
		return Message{}, fmt.Errorf("error reading message body: %w", err)
	}

	// 打包消息
	msg, err := UnpackMessage(buffer)
	if err != nil {
		return Message{}, fmt.Errorf("error unpacking message: %w", err)
	}

	return msg, nil
}

// readLength reads the length of the message as a 4-byte big-endian integer.
// It includes retry mechanism and length validation.
func (nm *NetworkManager) readLength(reader *bufio.Reader) (uint32, error) {
	lengthBytes := make([]byte, 4)
	retries := 3                            // 最大重试次数
	const maxMessageSize = 10 * 1024 * 1024 // 10MB，最大消息长度
	const minMessageSize = 4                // 最小消息长度（4字节的长度字段）

	for i := 0; i < retries; i++ {
		// 尝试读取4字节的长度字段
		_, err := io.ReadFull(reader, lengthBytes)
		if err != nil {
			nm.logger.WithFields(map[string]interface{}{
				"retry": i + 1,
				"error": err,
			}).Warn("Failed to read length bytes, retrying...")
			time.Sleep(100 * time.Millisecond) // 等待100ms后重试
			continue
		}

		// 解析长度值
		length := binary.BigEndian.Uint32(lengthBytes)

		// 校验长度值
		if length < minMessageSize {
			return 0, fmt.Errorf("invalid message length: %d (too small)", length)
		}
		if length > maxMessageSize {
			return 0, fmt.Errorf("invalid message length: %d (exceeds max limit)", length)
		}

		// 返回有效的长度值
		return length, nil
	}

	// 重试次数用尽，返回错误
	return 0, fmt.Errorf("failed to read length after %d retries", retries)
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
