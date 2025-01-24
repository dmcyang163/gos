// compression.go
package main

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/golang/snappy"
)

// 定义压缩池和解压池
var (
	compressorPool   = newBufferPool() // 用于压缩的缓冲池
	decompressorPool = newBufferPool() // 用于解压缩的缓冲池
)

// compressMessage compresses a message using Snappy.
func compressMessage(msg Message) ([]byte, error) {
	// 将消息编码为 JSON
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}

	// 从 compressorPool 中获取缓冲区
	buffer := compressorPool.Get().([]byte)
	defer compressorPool.Put(buffer) // 使用完毕后放回池中

	// 使用 Snappy 压缩数据
	compressed := snappy.Encode(buffer, data)
	return compressed, nil
}

// decompressMessage decompresses a message using Snappy.
func decompressMessage(data []byte) (Message, error) {
	// 从 decompressorPool 中获取缓冲区
	buffer := decompressorPool.Get().([]byte)
	defer decompressorPool.Put(buffer) // 使用完毕后放回池中

	// 使用 Snappy 解压缩数据
	decoded, err := snappy.Decode(buffer, data)
	if err != nil {
		return Message{}, fmt.Errorf("failed to decode message: %w", err)
	}

	// 解码消息
	var msg Message
	if err := json.Unmarshal(decoded, &msg); err != nil {
		return Message{}, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return msg, nil
}

// newBufferPool 创建一个新的 sync.Pool，用于复用 []byte
func newBufferPool() *sync.Pool {
	return &sync.Pool{
		New: func() interface{} {
			return make([]byte, 4096) // 返回的是 []byte
		},
	}
}
