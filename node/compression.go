// compression.go
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
)

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
