// network.go

package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"node/utils"
	"os"
	"sync"
	"time"
)

// 定义消息的最大长度和最小长度
const (
	maxMessageSize = 10 * 1024 * 1024 // 10 MB
	minMessageSize = 4                // 最小长度为4字节（长度字段本身）
)

// NetworkManager handles network connections.
type NetworkManager struct {
	Conns    sync.Map
	logger   utils.Logger
	executor utils.TaskExecutor
	// mu       sync.Mutex
}

// NewNetworkManager creates a new NetworkManager instance.
func NewNetworkManager(logger utils.Logger, executor utils.TaskExecutor) *NetworkManager {
	return &NetworkManager{
		Conns:    sync.Map{},
		logger:   logger,
		executor: executor,
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

// SendFile sends a file in chunks asynchronously.
func (nm *NetworkManager) SendFile(conn net.Conn, filePath string, relPath string) error {
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

	buffer := make([]byte, maxChunkSize) // 直接分配缓冲区

	chunkID := 0
	startTime := time.Now()
	var totalBytesSent int64

	// 用于同步发送结果的 channel
	resultChan := make(chan error, 1)

	for {
		// nm.executor.PrintPoolStats()

		// 读取文件块
		n, err := nm.readFileChunk(file, buffer[:chunkSize])
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to read file chunk: %w", err)
		}

		// 异步发送文件块
		nm.executor.SubmitWithPriority(func() {
			err := nm.sendChunkWithRetry(conn, fileInfo, relPath, buffer[:n], chunkID, err == io.EOF)
			resultChan <- err
		}, 10)

		// 等待发送结果
		if err := <-resultChan; err != nil {
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

	nm.logger.WithFields(map[string]interface{}{
		"file_path":  filePath,
		"bytes_sent": totalBytesSent,
		"duration":   time.Since(startTime).String(),
	}).Info("File sent successfully")

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
func (nm *NetworkManager) sendChunkWithRetry(conn net.Conn, fileInfo os.FileInfo, relPath string, chunk []byte, chunkID int, isLast bool) error {
	// 计算文件块的校验和
	checksum := utils.CalculateChecksum(chunk)

	msg := Message{
		Type:     MessageTypeFileTransfer,
		FileName: fileInfo.Name(),
		FileSize: fileInfo.Size(),
		RelPath:  relPath,
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
				nm.logger.WithFields(map[string]interface{}{
					"file_name": fileInfo.Name(),
					"chunk_id":  chunkID,
					"retries":   retries,
					"error":     err,
				}).Error("Failed to send file chunk after retries")
				return err
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

// SendMessage sends a message to a connection.
func (nm *NetworkManager) SendMessage(conn net.Conn, msg Message) error {
	// 打包消息
	msgBytes, err := PackMessage(msg)
	if err != nil {
		return fmt.Errorf("error packing message: %w", err)
	}

	// 使用 SendRawMessage 发送压缩后的消息
	return nm.SendRawMessage(conn, msgBytes)
}

func (nm *NetworkManager) SendRawMessage(conn net.Conn, data []byte) error {
	// 直接分配长度头缓冲区
	headerBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(headerBuf[:4], uint32(len(data)))

	// 组合两个内存块
	buffers := net.Buffers{headerBuf[:4], data}

	// 单次系统调用发送
	_, err := buffers.WriteTo(conn)
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
		return Message{}, err
	}

	// 检查消息长度是否合法
	if length > maxMessageSize {
		return Message{}, fmt.Errorf("message length %d exceeds max limit %d", length, maxMessageSize)
	}

	// 直接分配缓冲区
	buffer := make([]byte, int(length))

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
func (nm *NetworkManager) readLength(reader *bufio.Reader) (uint32, error) {
	lengthBytes := make([]byte, 4)
	retries := 3 // 最大重试次数

	for i := 0; i < retries; i++ {
		// 尝试读取4字节的长度字段
		_, err := io.ReadFull(reader, lengthBytes)
		if err != nil {
			if err == io.EOF {
				return 0, fmt.Errorf("connection closed while reading length")
			}
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
