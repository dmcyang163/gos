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
		return nil, fmt.Errorf("encode error: %w", err) // 编码错误
	}
	return data, nil
}

func decodeMessage(data []byte) (Message, error) {
	var msg Message
	err := json.Unmarshal(data, &msg)
	if err != nil {
		return Message{}, fmt.Errorf("decode error: %w", err) // 解码错误
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
		return nil, fmt.Errorf("compress error: %w", err) // 压缩错误
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

// pack 处理压缩和加密，返回带前缀的数据
func pack(data []byte, compressed, encrypted bool) ([]byte, error) {
	var err error
	if compressed {
		data, err = utils.Compress(data)
		if err != nil {
			return nil, fmt.Errorf("compress error: %w", err) // 压缩错误
		}
	}
	if encrypted {
		encryptedString, err := utils.Encrypt(data) // utils.Encrypt returns string
		if err != nil {
			return nil, fmt.Errorf("encrypt error: %w", err) // 加密错误
		}
		data = []byte(encryptedString) // Convert string to []byte
	}
	return data, nil
}

func PackMessage(msg Message) ([]byte, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal error: %w", err) // 序列化错误
	}

	compressed := shouldCompressMessage(msg.Type)
	encrypted := shouldEncryptMessage(msg.Type)

	var prefix string
	switch {
	case compressed && encrypted:
		prefix = "CE|" // 压缩加密
	case compressed:
		prefix = "C|" // 压缩未加密
	case encrypted:
		prefix = "E|" // 未压缩加密
	default:
		prefix = "N|" // 未压缩未加密
	}

	packedData, err := pack(data, compressed, encrypted)
	if err != nil {
		return nil, fmt.Errorf("pack error: %w", err)
	}

	return append([]byte(prefix), packedData...), nil
}

func parseMessagePrefix(data []byte) (string, []byte, error) {
	separatorIndex := bytes.IndexByte(data, '|')
	if separatorIndex == -1 {
		return "", nil, fmt.Errorf("invalid message format") // 无效的消息格式
	}

	prefix := string(data[:separatorIndex+1])
	remainingData := data[separatorIndex+1:]

	return prefix, remainingData, nil
}

// unpack 处理解压缩和解密
func unpack(data []byte, compressed, encrypted bool) ([]byte, error) {
	var err error
	if encrypted {
		dataString := string(data)                        // Convert []byte to string for Decrypt
		decryptedString, err := utils.Decrypt(dataString) // utils.Decrypt returns string
		if err != nil {
			return nil, fmt.Errorf("decrypt error: %w", err) // 解密错误
		}
		data = []byte(decryptedString) // Convert string back to []byte
	}
	if compressed {
		data, err = utils.Decompress(data)
		if err != nil {
			return nil, fmt.Errorf("decompress error: %w", err) // 解压缩错误
		}
	}
	return data, nil
}

func UnpackMessage(data []byte) (Message, error) {
	prefix, remainingData, err := parseMessagePrefix(data)
	if err != nil {
		return Message{}, err
	}

	compressed := false
	encrypted := false

	switch prefix {
	case "CE|":
		compressed = true
		encrypted = true
	case "C|":
		compressed = true
	case "E|":
		encrypted = true
	case "N|":
		// No compression or encryption
	default:
		return Message{}, fmt.Errorf("unknown prefix: %s", prefix) // 未知前缀
	}

	unpackedData, err := unpack(remainingData, compressed, encrypted)
	if err != nil {
		return Message{}, fmt.Errorf("unpack error: %w", err)
	}

	var msg Message
	err = json.Unmarshal(unpackedData, &msg)
	if err != nil {
		return Message{}, fmt.Errorf("unmarshal error: %w", err) // 反序列化错误
	}

	return msg, nil
}
