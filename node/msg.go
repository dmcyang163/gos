package main

import (
	"encoding/json"
	"fmt"
)

// MessageType 定义消息类型
const (
	MessageTypeChat         = "chat"
	MessageTypePeerList     = "peer_list"
	MessageTypePeerListReq  = "peer_list_request"
	MessageTypePing         = "ping" // 心跳请求
	MessageTypePong         = "pong" // 心跳响应
	MessageTypeFileTransfer = "file_transfer"
	MessageTypeNodeStatus   = "node_status"
)

// Message represents a message exchanged between nodes.
type Message struct {
	Type    string `json:"type"`    // 消息类型
	Data    string `json:"data"`    // 消息内容
	Sender  string `json:"sender"`  // 发送者名字
	Address string `json:"address"` // 发送者地址
	ID      string `json:"id"`      // 消息ID，用于防止重复处理

	// 文件传输相关字段
	FileName string `json:"file_name,omitempty"` // 文件名
	FileSize int64  `json:"file_size,omitempty"` // 文件大小
	Chunk    []byte `json:"chunk,omitempty"`     // 文件块数据
	ChunkID  int    `json:"chunk_id,omitempty"`  // 文件块ID
	IsLast   bool   `json:"is_last,omitempty"`   // 是否是最后一块
}

// compressMessage 压缩消息
func CompressMsg(msg Message) ([]byte, error) {
	// 将消息编码为 JSON
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}

	// 调用 compression.go 中的 compress 函数
	compressed, err := compress(data)
	if err != nil {
		return nil, fmt.Errorf("failed to compress message: %w", err)
	}
	return compressed, nil
}

// decompressMessage 解压缩消息
func DecompressMsg(data []byte) (Message, error) {
	// 调用 compression.go 中的 decompress 函数
	decoded, err := decompress(data)
	if err != nil {
		return Message{}, fmt.Errorf("failed to decompress message: %w", err)
	}

	// 解码消息
	var msg Message
	if err := json.Unmarshal(decoded, &msg); err != nil {
		return Message{}, fmt.Errorf("failed to unmarshal message: %w", err)
	}
	return msg, nil
}
