package main

import (
	"encoding/json"
	"testing"
)

func TestCompressMsg(t *testing.T) {
	// 测试不压缩的消息
	msg := Message{
		Type:   MessageTypePing,
		Data:   "ping",
		Sender: "node1",
		ID:     "123",
	}

	data, err := CompressMsg(msg)
	if err != nil {
		t.Fatalf("CompressMsg failed: %v", err)
	}

	var decodedMsg Message
	if err := json.Unmarshal(data, &decodedMsg); err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if decodedMsg.Compressed {
		t.Error("Expected message to be uncompressed")
	}

	// 测试压缩的消息
	msg.Type = MessageTypeChat
	data, err = CompressMsg(msg)
	if err != nil {
		t.Fatalf("CompressMsg failed: %v", err)
	}

	decodedMsg, err = DecompressMsg(data)
	if err != nil {
		t.Fatalf("DecompressMsg failed: %v", err)
	}

	if decodedMsg.Compressed {
		t.Error("Expected message to be uncompressed after decompression")
	}

	// 验证解压后的消息内容是否正确
	if decodedMsg.Type != MessageTypeChat || decodedMsg.Data != "ping" || decodedMsg.Sender != "node1" {
		t.Errorf("Decoded message does not match original: %+v", decodedMsg)
	}
}
