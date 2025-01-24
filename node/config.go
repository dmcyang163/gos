// config_loader.go
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
	LogMaxSize        int    `json:"log_max_size"`
	LogMaxBackups     int    `json:"log_max_backups"`
	LogMaxAge         int    `json:"log_max_age"`
	LogCompress       bool   `json:"log_compress"`
	DiscoveryInterval int    `json:"discovery_interval"`
	HeartbeatInterval int    `json:"heartbeat_interval"`
	LogAPI            string `json:"log_api"`
}

// NameEntry represents a name with its description and dialogues.
type NameEntry struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	SpecialAbility string   `json:"special_ability"`
	Tone           string   `json:"tone"`
	Dialogues      []string `json:"dialogues"`
}

// ConfigLoader 是配置文件加载器的接口
type ConfigLoader interface {
	LoadConfig(path string) (*Config, error)
	LoadNames(path string) ([]NameEntry, error)
}

// JSONConfigLoader 是 JSON 配置文件加载器的实现
type JSONConfigLoader struct{}

// NewJSONConfigLoader 创建一个新的 JSONConfigLoader
func NewJSONConfigLoader() *JSONConfigLoader {
	return &JSONConfigLoader{}
}

// LoadConfig 加载配置文件
func (l *JSONConfigLoader) LoadConfig(path string) (*Config, error) {
	var config Config
	if err := loadJSONFile(path, &config); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return &config, nil
}

// LoadNames 加载名字列表
func (l *JSONConfigLoader) LoadNames(path string) ([]NameEntry, error) {
	var names []NameEntry
	if err := loadJSONFile(path, &names); err != nil {
		return nil, fmt.Errorf("failed to load names: %w", err)
	}
	return names, nil
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
