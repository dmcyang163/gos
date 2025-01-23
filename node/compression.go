// compression.go
package main

import (
	"encoding/json"
	"fmt"

	"github.com/golang/snappy"
)

// compressMessage compresses a message using Snappy.
func compressMessage(msg Message) ([]byte, error) {
	// 将消息编码为 JSON
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}

	// 使用 Snappy 压缩数据
	compressed := snappy.Encode(nil, data)
	return compressed, nil
}

// decompressMessage decompresses a message using Snappy.
func decompressMessage(data []byte) (Message, error) {
	// 使用 Snappy 解压缩数据
	decoded, err := snappy.Decode(nil, data)
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
