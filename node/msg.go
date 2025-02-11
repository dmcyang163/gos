// msg.go

package main

import (
	"encoding/json"
	"fmt"
	"node/utils"
)

// MessageType 定义消息类型
const (
	MessageTypeChat            = "chat"
	MessageTypePeerList        = "peer_list"
	MessageTypePeerListReq     = "peer_list_request"
	MessageTypePing            = "ping" // 心跳请求
	MessageTypePong            = "pong" // 心跳响应
	MessageTypeFileTransfer    = "file_transfer"
	MessageTypeFileTransferAck = "file_transfer_ack" // 文件传输确认
	MessageTypeFileTransferNak = "file_transfer_nak" // 文件传输否定确认
	MessageTypeNodeStatus      = "node_status"
)

// Message represents a message exchanged between nodes.
type Message struct {
	Type      string `json:"type"`                // 消息类型
	Data      string `json:"data"`                // 消息内容
	Sender    string `json:"sender"`              // 发送者名字
	Address   string `json:"address"`             // 发送者地址
	ID        string `json:"id"`                  // 消息ID，用于防止重复处理
	Encrypted bool   `json:"encrypted,omitempty"` // 标识消息是否已加密

	// 文件传输相关字段
	FileName string `json:"file_name,omitempty"` // 文件名
	FileSize int64  `json:"file_size,omitempty"` // 文件大小
	Chunk    []byte `json:"chunk,omitempty"`     // 文件块数据
	ChunkID  int    `json:"chunk_id,omitempty"`  // 文件块ID
	IsLast   bool   `json:"is_last,omitempty"`   // 是否是最后一块
	Checksum string `json:"checksum,omitempty"`  // 校验和
}

// 定义不需要压缩的消息类型
var uncompressedMessageTypes = map[string]bool{
	MessageTypePing:        true,
	MessageTypePong:        true,
	MessageTypePeerListReq: true,
}

func shouldCompressMessage(msgType string) bool {
	return !uncompressedMessageTypes[msgType]
}

// encodeMessage 将消息编码为 JSON
func encodeMessage(msg Message) ([]byte, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}
	return data, nil
}

// decodeMessage 将 JSON 数据解码为消息
func decodeMessage(data []byte) (Message, error) {
	var msg Message
	err := json.Unmarshal(data, &msg)
	if err != nil {
		return Message{}, fmt.Errorf("failed to decode message: %w", err)
	}
	return msg, nil
}

func CompressMsg(msg Message) ([]byte, error) {
	// 将消息编码为 JSON
	data, err := encodeMessage(msg)
	if err != nil {
		return nil, err
	}

	// 对于小消息或不需要压缩的消息类型，直接返回原始数据
	if !shouldCompressMessage(msg.Type) {
		return data, nil
	}

	// 调用 compression.go 中的 compress 函数
	compressed, err := utils.Compress(data)
	if err != nil {
		return nil, fmt.Errorf("failed to compress message: %w", err)
	}

	// 返回压缩后的数据
	return compressed, nil
}

func DecompressMsg(data []byte) (Message, error) {
	// 尝试解压缩数据
	decoded, err := utils.Decompress(data)
	if err == nil {
		// 如果解压缩成功，则解码解压缩后的消息
		return decodeMessage(decoded)
	}

	// 如果解压缩失败，则假定消息未压缩，直接解码消息
	return decodeMessage(data)
}
