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
	MessageTypePing            = "ping"
	MessageTypePong            = "pong"
	MessageTypeFileTransfer    = "file_transfer"
	MessageTypeFileTransferAck = "file_transfer_ack"
	MessageTypeFileTransferNak = "file_transfer_nak"
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
	FileName string `json:"file_name,omitempty"`
	FileSize int64  `json:"file_size,omitempty"`
	RelPath  string `json:"rel_path,omitempty"`
	Chunk    []byte `json:"chunk,omitempty"`
	ChunkID  int    `json:"chunk_id,omitempty"`
	IsLast   bool   `json:"is_last,omitempty"`
	Checksum string `json:"checksum,omitempty"`
}

// 定义需要压缩的消息类型
var compressedMessageTypes = map[string]bool{
	MessageTypeChat:         true,
	MessageTypeFileTransfer: true,
}

// 定义需要加密的消息类型
var encryptedMessageTypes = map[string]bool{
	MessageTypeChat: true,
}

func shouldCompressMessage(msgType string) bool {
	return compressedMessageTypes[msgType]
}

func shouldEncryptMessage(msgType string) bool {
	return encryptedMessageTypes[msgType]
}

func encodeMessage(msg Message) ([]byte, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}
	return data, nil
}

func decodeMessage(data []byte) (Message, error) {
	var msg Message
	err := json.Unmarshal(data, &msg)
	if err != nil {
		return Message{}, fmt.Errorf("failed to decode message: %w", err)
	}
	return msg, nil
}

func CompressMessage(msg Message) ([]byte, error) {
	data, err := encodeMessage(msg)
	if err != nil {
		return nil, err
	}

	if !shouldCompressMessage(msg.Type) {
		return data, nil
	}

	compressed, err := utils.Compress(data)
	if err != nil {
		return nil, fmt.Errorf("failed to compress message: %w", err)
	}

	return compressed, nil
}

func DecompressMessage(data []byte) (Message, error) {
	decoded, err := utils.Decompress(data)
	if err == nil {
		return decodeMessage(decoded)
	}

	return decodeMessage(data)
}

func PackMessage(msg Message) ([]byte, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	prefix := genMessagePrefix(msg.Type)

	switch prefix {
	case "N|":
		return append([]byte(prefix), data...), nil
	case "C|":
		compressed, err := utils.Compress(data)
		if err != nil {
			return nil, fmt.Errorf("failed to compress: %w", err)
		}
		return append([]byte(prefix), compressed...), nil
	case "E|":
		encrypted, err := utils.Encrypt(data)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt: %w", err)
		}
		return append([]byte(prefix), encrypted...), nil
	case "CE|":
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

func genMessagePrefix(messageType string) string {
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

func parseMessagePrefix(data []byte) (string, []byte, error) {
	separatorIndex := bytes.IndexByte(data, '|')
	if separatorIndex == -1 {
		return "", nil, fmt.Errorf("invalid message format: missing separator '|'")
	}

	prefix := string(data[:separatorIndex+1])
	remainingData := data[separatorIndex+1:]

	return prefix, remainingData, nil
}

func UnpackMessage(data []byte) (Message, error) {
	var msg Message

	prefix, remainingData, err := parseMessagePrefix(data)
	if err != nil {
		return Message{}, fmt.Errorf("failed to parse prefix: %w", err)
	}

	switch prefix {
	case "N|":
		if err := json.Unmarshal(remainingData, &msg); err != nil {
			return Message{}, fmt.Errorf("failed to unmarshal message: %w", err)
		}
		return msg, nil
	case "C|":
		decompressed, err := utils.Decompress(remainingData)
		if err != nil {
			return Message{}, fmt.Errorf("failed to decompress: %w", err)
		}
		if err := json.Unmarshal(decompressed, &msg); err != nil {
			return Message{}, fmt.Errorf("failed to unmarshal message: %w", err)
		}
		return msg, nil
	case "E|":
		decrypted, err := utils.Decrypt(string(remainingData))
		if err != nil {
			return Message{}, fmt.Errorf("failed to decrypt: %w", err)
		}
		if err := json.Unmarshal(decrypted, &msg); err != nil {
			return Message{}, fmt.Errorf("failed to unmarshal message: %w", err)
		}
		return msg, nil
	case "CE|":
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
