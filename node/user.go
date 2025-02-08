package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/uuid"
)

// User contains information about the user.
type User struct {
	UUID           string   `json:"uuid"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	SpecialAbility string   `json:"special_ability"`
	Tone           string   `json:"tone"`
	Dialogues      []string `json:"dialogues"`
}

// NewUser creates a new User instance with a unique UUID.
func NewUser(name, description, specialAbility, tone string, dialogues []string) *User {
	uuid, _ := uuid.NewRandom()
	return &User{
		UUID:           uuid.String(),
		Name:           name,
		Description:    description,
		SpecialAbility: specialAbility,
		Tone:           tone,
		Dialogues:      dialogues,
	}
}

// SaveUser saves the user information to a file.
func SaveUser(user *User, filename string) error {
	data, err := json.MarshalIndent(user, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal user: %v", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write user to file: %v", err)
	}

	return nil
}

// LoadUser loads user information from a file.
func LoadUser(filename string) (*User, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file does not exist: %v", err)
		}
		return nil, fmt.Errorf("failed to read user file: %v", err)
	}

	var user User
	if err := json.Unmarshal(data, &user); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user: %v", err)
	}

	return &user, nil
}
