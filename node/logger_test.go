package main

import (
	"testing"
)

func TestLogger(t *testing.T) {
	config := &Config{
		Log: LogConfig{
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     28,
			Compress:   true,
			Level:      "debug",
		},
	}
	logger := NewLogrusLogger(config)

	logger.WithFields(map[string]interface{}{
		"port":  "1234",
		"error": "some error",
	}).Info("Test log message")

	// 显式输出日志到控制台
	t.Log("Test log message with port and error fields")
}
