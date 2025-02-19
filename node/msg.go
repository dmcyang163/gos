package main

import (
	"bytes"
	"fmt"
	"node/utils"
	"sync"

	jsoniter "github.com/json-iterator/go" // 重命名为 jsoniter
)

// 使用 sync.Pool 复用 bytes.Buffer，减少内存分配
var (
	bufferPool = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
	// 使用 json-iterator/go 替代标准库的 encoding/json，提高性能
	// jniter = jsoniter.ConfigCompatibleWithStandardLibrary
	jniter = jsoniter.ConfigFastest
)

// MessageType 定义消息类型
const (
	MessageTypeChat            = "chat"              // 聊天消息
	MessageTypePeerList        = "peer_list"         // 节点列表
	MessageTypePeerListReq     = "peer_list_request" // 请求节点列表
	MessageTypePing            = "ping"              // 心跳消息
	MessageTypePong            = "pong"              // 心跳响应
	MessageTypeFileTransfer    = "file_transfer"     // 文件传输
	MessageTypeFileTransferAck = "file_transfer_ack" // 文件传输确认
	MessageTypeFileTransferNak = "file_transfer_nak" // 文件传输拒绝
	MessageTypeNodeStatus      = "node_status"       // 节点状态
)

// Message 表示节点之间交换的消息
type Message struct {
	Type    string `json:"type"`    // 消息类型
	Data    string `json:"data"`    // 消息内容
	Sender  string `json:"sender"`  // 发送者名字
	Address string `json:"address"` // 发送者地址
	ID      string `json:"id"`      // 消息ID，用于防止重复处理

	// 文件传输相关字段
	FileName string `json:"file_name,omitempty"` // 文件名（可选）
	FileSize int64  `json:"file_size,omitempty"` // 文件大小（可选）
	RelPath  string `json:"rel_path,omitempty"`  // 相对路径（可选）
	Chunk    []byte `json:"chunk,omitempty"`     // 文件分块数据（可选）
	ChunkID  int    `json:"chunk_id,omitempty"`  // 分块ID（可选）
	IsLast   bool   `json:"is_last,omitempty"`   // 是否为最后一个分块（可选）
	Checksum string `json:"checksum,omitempty"`  // 校验和（可选）
}

var (
	// 定义需要压缩的消息类型
	compressedMessageTypes = map[string]bool{
		MessageTypeChat:         true,
		MessageTypeFileTransfer: true,
	}

	// 定义需要加密的消息类型
	encryptedMessageTypes = map[string]bool{
		MessageTypeChat: true,
	}
)

// shouldCompressMessage 判断消息是否需要压缩
func shouldCompressMessage(msgType string) bool {
	return compressedMessageTypes[msgType]
}

// shouldEncryptMessage 判断消息是否需要加密
func shouldEncryptMessage(msgType string) bool {
	return encryptedMessageTypes[msgType]
}

// encodeMessage 将消息序列化为 JSON 字节
func encodeMessage(msg Message) ([]byte, error) {
	return jniter.Marshal(msg)
}

// decodeMessage 将 JSON 字节反序列化为消息
func decodeMessage(data []byte) (Message, error) {
	var msg Message
	if err := jniter.Unmarshal(data, &msg); err != nil {
		return Message{}, fmt.Errorf("deserialization error: %w", err)
	}
	return msg, nil
}

// pack 处理压缩和加密，返回带前缀的数据
func pack(data []byte, compressed, encrypted bool) ([]byte, error) {
	// 从池中获取 bytes.Buffer，避免频繁分配内存
	buf := bufferPool.Get().(*bytes.Buffer)
	defer bufferPool.Put(buf) // 使用完毕后放回池中
	buf.Reset()               // 重置缓冲区

	// 如果需要压缩，先压缩数据
	if compressed {
		compressedData, err := utils.Compress(data)
		if err != nil {
			return nil, fmt.Errorf("compression error: %w", err)
		}
		buf.Write(compressedData)
	} else {
		buf.Write(data)
	}

	// 如果需要加密，对数据进行加密
	if encrypted {
		encryptedData, err := utils.Encrypt(buf.Bytes())
		if err != nil {
			return nil, fmt.Errorf("encryption error: %w", err)
		}
		buf.Reset()
		buf.Write([]byte(encryptedData))
	}

	return buf.Bytes(), nil
}

// PackMessage 打包消息，包括压缩和加密
func PackMessage(msg Message) ([]byte, error) {
	data, err := encodeMessage(msg)
	if err != nil {
		return nil, fmt.Errorf("serialization error: %w", err)
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

// parseMessagePrefix 解析消息前缀
func parseMessagePrefix(data []byte) (string, []byte, error) {
	separatorIndex := bytes.IndexByte(data, '|')
	if separatorIndex == -1 {
		return "", nil, fmt.Errorf("invalid message format")
	}

	prefix := string(data[:separatorIndex+1])
	remainingData := data[separatorIndex+1:]

	return prefix, remainingData, nil
}

// unpack 处理解压缩和解密
func unpack(data []byte, compressed, encrypted bool) ([]byte, error) {
	var err error
	if encrypted {
		decryptedData, err := utils.Decrypt(string(data))
		if err != nil {
			return nil, fmt.Errorf("decryption error: %w", err)
		}
		data = []byte(decryptedData)
	}
	if compressed {
		data, err = utils.Decompress(data)
		if err != nil {
			return nil, fmt.Errorf("decompression error: %w", err)
		}
	}
	return data, nil
}

// UnpackMessage 解包消息，包括解密和解压缩
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
		// 未压缩未加密
	default:
		return Message{}, fmt.Errorf("unknown prefix: %s", prefix)
	}

	unpackedData, err := unpack(remainingData, compressed, encrypted)
	if err != nil {
		return Message{}, fmt.Errorf("unpack error: %w", err)
	}

	return decodeMessage(unpackedData)
}
