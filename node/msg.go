package main

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
