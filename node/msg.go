package main

import (
	"bytes"
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
	Checksum string `json:"checksum,omitempty"`  // 校验和
}

// 定义不需要压缩的消息类型
var uncompressedMessageTypes = map[string]bool{
	MessageTypePing:        true,
	MessageTypePong:        true,
	MessageTypePeerListReq: true,
}

// 定义不需要加密的消息类型
var unencryptedMessageTypes = map[string]bool{
	MessageTypePing:            true,
	MessageTypePong:            true,
	MessageTypePeerListReq:     true,
	MessageTypePeerList:        true,
	MessageTypeFileTransfer:    true,
	MessageTypeFileTransferAck: true,
	MessageTypeFileTransferNak: true,
	MessageTypeNodeStatus:      true,
}

func shouldCompressMessage(msgType string) bool {
	return !uncompressedMessageTypes[msgType]
}

func shouldEncryptMessage(msgType string) bool {
	return !unencryptedMessageTypes[msgType]
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

// CompressMessage 压缩消息
func CompressMessage(msg Message) ([]byte, error) {
	// 将消息编码为 JSON
	data, err := encodeMessage(msg)
	if err != nil {
		return nil, err
	}

	// 对于小消息或不需要压缩的消息类型，直接返回原始数据
	if !shouldCompressMessage(msg.Type) {
		return data, nil
	}

	// 调用 compressor.go 中的 compress 函数
	compressed, err := utils.Compress(data)
	if err != nil {
		return nil, fmt.Errorf("failed to compress message: %w", err)
	}

	// 返回压缩后的数据
	return compressed, nil
}

// DecompressMessage 解压缩消息
func DecompressMessage(data []byte) (Message, error) {
	// 尝试解压缩数据
	decoded, err := utils.Decompress(data)
	if err == nil {
		// 如果解压缩成功，则解码解压缩后的消息
		return decodeMessage(decoded)
	}

	// 如果解压缩失败，则假定消息未压缩，直接解码消息
	return decodeMessage(data)
}

func PackMessage(msg Message) ([]byte, error) {
	// 将消息序列化为 JSON
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	// 根据消息类型生成前缀
	prefix := getMessagePrefix(msg.Type)

	// 根据前缀处理数据
	switch prefix {
	case "N|": // 未压缩未加密
		return append([]byte(prefix), data...), nil
	case "C|": // 压缩未加密
		compressed, err := utils.Compress(data)
		if err != nil {
			return nil, fmt.Errorf("failed to compress: %w", err)
		}
		return append([]byte(prefix), compressed...), nil
	case "E|": // 未压缩加密
		encrypted, err := utils.Encrypt(data)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt: %w", err)
		}
		return append([]byte(prefix), encrypted...), nil
	case "CE|": // 压缩加密
		compressed, err := utils.Compress(data)
		if err != nil {
			return nil, fmt.Errorf("failed to compress: %w", err)
		}
		encrypted, err := utils.Encrypt(compressed)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt: %w", err)
		}
		return append([]byte(prefix), encrypted...), nil
	default:
		return nil, fmt.Errorf("unknown prefix: %s", prefix)
	}
}

func getMessagePrefix(messageType string) string {
	compressed := shouldCompressMessage(messageType)
	encrypted := shouldEncryptMessage(messageType)

	switch {
	case compressed && encrypted:
		return "CE|" // 压缩加密
	case compressed && !encrypted:
		return "C|" // 压缩未加密
	case !compressed && encrypted:
		return "E|" // 未压缩加密
	default:
		return "N|" // 未压缩未加密
	}
}

// parsePrefix 从消息中解析前缀，并返回前缀和剩余数据
func parsePrefix(data []byte) (string, []byte, error) {
	separatorIndex := bytes.IndexByte(data, '|')
	if separatorIndex == -1 {
		return "", nil, fmt.Errorf("invalid message format: missing separator '|'")
	}

	prefix := string(data[:separatorIndex+1]) // 包含分隔符
	remainingData := data[separatorIndex+1:]  // 移除前缀和分隔符

	return prefix, remainingData, nil
}

func UnpackMessage(data []byte) (Message, error) {
	var msg Message

	// 解析前缀
	prefix, remainingData, err := parsePrefix(data)
	if err != nil {
		return Message{}, fmt.Errorf("failed to parse prefix: %w", err)
	}

	// 根据前缀处理数据
	switch prefix {
	case "N|": // 未压缩未加密
		if err := json.Unmarshal(remainingData, &msg); err != nil {
			return Message{}, fmt.Errorf("failed to unmarshal message: %w", err)
		}
		return msg, nil
	case "C|": // 压缩未加密
		decompressed, err := utils.Decompress(remainingData)
		if err != nil {
			return Message{}, fmt.Errorf("failed to decompress: %w", err)
		}
		if err := json.Unmarshal(decompressed, &msg); err != nil {
			return Message{}, fmt.Errorf("failed to unmarshal message: %w", err)
		}
		return msg, nil
	case "E|": // 未压缩加密
		decrypted, err := utils.Decrypt(string(remainingData))
		if err != nil {
			return Message{}, fmt.Errorf("failed to decrypt: %w", err)
		}
		if err := json.Unmarshal(decrypted, &msg); err != nil {
			return Message{}, fmt.Errorf("failed to unmarshal message: %w", err)
		}
		return msg, nil
	case "CE|": // 压缩加密
		decrypted, err := utils.Decrypt(string(remainingData))
		if err != nil {
			return Message{}, fmt.Errorf("failed to decrypt: %w", err)
		}
		decompressed, err := utils.Decompress(decrypted)
		if err != nil {
			return Message{}, fmt.Errorf("failed to decompress: %w", err)
		}
		if err := json.Unmarshal(decompressed, &msg); err != nil {
			return Message{}, fmt.Errorf("failed to unmarshal message: %w", err)
		}
		return msg, nil
	default:
		return Message{}, fmt.Errorf("unknown message prefix: %s", prefix)
	}
}
