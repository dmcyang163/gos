// msg.go
package main

import (
	"encoding/json"
	"fmt"
	"node/compressor"
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
	Type       string `json:"type"`                 // 消息类型
	Data       string `json:"data"`                 // 消息内容
	Sender     string `json:"sender"`               // 发送者名字
	Address    string `json:"address"`              // 发送者地址
	ID         string `json:"id"`                   // 消息ID，用于防止重复处理
	Compressed bool   `json:"compressed,omitempty"` // 是否已压缩

	// 文件传输相关字段
	FileName string `json:"file_name,omitempty"` // 文件名
	FileSize int64  `json:"file_size,omitempty"` // 文件大小
	Chunk    []byte `json:"chunk,omitempty"`     // 文件块数据
	ChunkID  int    `json:"chunk_id,omitempty"`  // 文件块ID
	IsLast   bool   `json:"is_last,omitempty"`   // 是否是最后一块
}

func CompressMsg(msg Message) ([]byte, error) {
	// 对于小消息或不需要压缩的消息类型，直接返回原始数据
	if msg.Type == MessageTypePing || msg.Type == MessageTypePong || msg.Type == MessageTypePeerListReq {
		msg.Compressed = false
		return json.Marshal(msg)
	}

	// 将消息编码为 JSON
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}

	// 调用 compression.go 中的 compress 函数
	compressed, err := compressor.Compress(data)
	if err != nil {
		return nil, fmt.Errorf("failed to compress message: %w", err)
	}

	// 返回压缩后的数据
	return compressed, nil
}

func DecompressMsg(data []byte) (Message, error) {
	// 先尝试解压数据
	decoded, err := compressor.Decompress(data)
	if err != nil {
		return Message{}, fmt.Errorf("failed to decompress message: %w", err)
	}

	// 解码消息
	var msg Message
	if err := json.Unmarshal(decoded, &msg); err != nil {
		return Message{}, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	// 标记消息为已解压
	msg.Compressed = false
	return msg, nil
}
