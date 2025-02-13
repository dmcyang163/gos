package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

// User contains information about the user.
type User struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`

	Password    string               `json:"password"`     // 密码字段
	IsOnline    bool                 `json:"is_online"`    // 是否在线
	IsInvisible bool                 `json:"is_invisible"` // 是否隐身
	LastSeen    int64                `json:"last_seen"`    // 最后活跃时间
	namesMap    map[string]NameEntry // 存储名称和 NameEntry 的映射
}

// NameEntry represents a name with its description and dialogues.
type NameEntry struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	SpecialAbility string   `json:"special_ability"`
	Tone           string   `json:"tone"`
	Dialogues      []string `json:"dialogues"`
}

// NewUser creates a new User instance with a unique UUID.
func NewUser(name string) *User {
	uuid, _ := uuid.NewRandom()
	namesMap := loadNamesMap()
	nameEntry, _ := GetRandomNameEntry(namesMap)
	name_ := nameEntry.Name
	return &User{
		UUID:        uuid.String(),
		Name:        name_,
		IsOnline:    false,
		IsInvisible: false,
		LastSeen:    time.Now().Unix(),
		namesMap:    namesMap, // 从 names.json 加载 namesMap
	}
}

// loadNamesMap 从 names.json 文件中加载 namesMap
func loadNamesMap() map[string]NameEntry {
	data, err := os.ReadFile("names.json")
	if err != nil {
		fmt.Println("Failed to read names.json:", err)
		return make(map[string]NameEntry)
	}

	var entries []NameEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		fmt.Println("Failed to parse names.json:", err)
		return make(map[string]NameEntry)
	}

	// 将数组转换为 map
	namesMap := make(map[string]NameEntry)
	for _, entry := range entries {
		namesMap[entry.Name] = entry
	}

	return namesMap
}

// GetRandomNameEntry 随机读取一个 NameEntry
func GetRandomNameEntry(namesMap map[string]NameEntry) (NameEntry, bool) {
	if len(namesMap) == 0 {
		fmt.Println("namesMap is empty: no entries found")
		return NameEntry{}, false
	}

	// 获取所有键
	keys := make([]string, 0, len(namesMap))
	for key := range namesMap {
		keys = append(keys, key)
	}

	// 返回随机值
	randomKey := keys[rand.Intn(len(keys))]
	return namesMap[randomKey], true
}

// FindDialogueForSender 根据发送者名称查找对话
func (u *User) FindDialogueForSender(senderName string) string {
	for name, entry := range u.namesMap {
		if strings.Contains(senderName, name) {
			if len(entry.Dialogues) > 0 {
				return entry.Dialogues[rand.Intn(len(entry.Dialogues))]
			}
		}
	}
	return "你好，我是" + senderName + "。"
}

// Login 用户登录
func (u *User) Login(password string) error {
	if u.Password != password {
		return fmt.Errorf("invalid password")
	}

	u.IsOnline = true
	u.LastSeen = time.Now().Unix()
	return nil
}

// Logout 用户登出
func (u *User) Logout() {
	u.IsOnline = false
	u.LastSeen = time.Now().Unix()
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
