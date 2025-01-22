package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config represents the configuration for the node.
type Config struct {
	Port          string `json:"port"`
	BootstrapNode string `json:"bootstrap_node"`
	MaxConns      int    `json:"max_conns"`
	LogLevel      string `json:"log_level"`
}

// NameEntry represents a name with its description and dialogues.
type NameEntry struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	SpecialAbility string   `json:"special_ability"`
	Tone           string   `json:"tone"`
	Dialogues      []string `json:"dialogues"`
}

// LoadConfig loads the configuration from a file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return &config, nil
}

// LoadNames loads the names and descriptions from a file.
func LoadNames(path string) ([]NameEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read names file: %w", err)
	}

	var names []NameEntry
	if err := json.Unmarshal(data, &names); err != nil {
		return nil, fmt.Errorf("failed to parse names: %w", err)
	}
	return names, nil
}
