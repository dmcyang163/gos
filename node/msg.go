package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
)

// Message represents a message exchanged between nodes.
type Message struct {
	Type    string `json:"type"`    // 消息类型
	Data    string `json:"data"`    // 消息内容
	Sender  string `json:"sender"`  // 发送者名字
	Address string `json:"address"` // 发送者地址
	ID      string `json:"id"`      // 消息ID，用于防止重复处理
}

// compressMessage compresses a message using Gzip.
func compressMessage(msg Message) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if err := json.NewEncoder(gz).Encode(msg); err != nil {
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}
	gz.Close()
	return buf.Bytes(), nil
}

// decompressMessage decompresses a message using Gzip.
func decompressMessage(data []byte) (Message, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return Message{}, fmt.Errorf("failed to decompress message: %w", err)
	}
	defer gz.Close()

	var msg Message
	if err := json.NewDecoder(gz).Decode(&msg); err != nil {
		return Message{}, fmt.Errorf("failed to decode message: %w", err)
	}
	return msg, nil
}
