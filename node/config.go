package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config represents the configuration for the node.
type Config struct {
	Port              string `json:"port"`
	BootstrapNode     string `json:"bootstrap_node"`
	MaxConns          int    `json:"max_conns"`
	LogLevel          string `json:"log_level"`
	LogMaxSize        int    `json:"log_max_size"`       // 新增
	LogMaxBackups     int    `json:"log_max_backups"`    // 新增
	LogMaxAge         int    `json:"log_max_age"`        // 新增
	LogCompress       bool   `json:"log_compress"`       // 新增
	DiscoveryInterval int    `json:"discovery_interval"` // 新增
	HeartbeatInterval int    `json:"heartbeat_interval"` // 新增
	LogAPI            string `json:"log_api"`            // 新增
}

// NameEntry represents a name with its description and dialogues.
type NameEntry struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	SpecialAbility string   `json:"special_ability"`
	Tone           string   `json:"tone"`
	Dialogues      []string `json:"dialogues"`
}

// loadJSONFile 是一个通用函数，用于读取文件内容并解析 JSON 数据
func loadJSONFile(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}
	return nil
}

// LoadConfig loads the configuration from a file.
func LoadConfig(path string) (*Config, error) {
	var config Config
	if err := loadJSONFile(path, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// LoadNames loads the names and descriptions from a file.
func LoadNames(path string) ([]NameEntry, error) {
	var names []NameEntry
	if err := loadJSONFile(path, &names); err != nil {
		return nil, err
	}
	return names, nil
}
