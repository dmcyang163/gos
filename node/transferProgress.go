package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// ProgressEntry 表示一个文件的传输进度
type ProgressEntry struct {
	RelPath   string `json:"relPath"`   // 文件的相对路径
	Offset    int64  `json:"offset"`    // 已传输的字节数
	Completed bool   `json:"completed"` // 是否传输完成
}

// Progress 表示文件或目录的传输进度
type Progress struct {
	Type    string          `json:"type"`    // "file" 或 "dir"
	Path    string          `json:"path"`    // 文件或目录的路径
	Entries []ProgressEntry `json:"entries"` // 传输进度条目
}

var progressDir = "progress" // 进度文件存储目录
var progressMutex sync.Mutex // 用于保护进度文件的并发访问

// ensureProgressDir 确保进度文件目录存在
func ensureProgressDir() error {
	if _, err := os.Stat(progressDir); os.IsNotExist(err) {
		return os.MkdirAll(progressDir, os.ModePerm)
	}
	return nil
}

// 删除字符串中的非法字符
func sanitizeFileName(part string) string {
	illegalChars := "/\\:*?\"<>|"
	for _, char := range illegalChars {
		part = strings.ReplaceAll(part, string(char), "")
	}
	return part
}

// 处理part，去掉_和-，并将它们后面的单词首字母大写
func processPart(part string) string {
	re := regexp.MustCompile(`[_-]([^\s_-])`)
	return re.ReplaceAllStringFunc(part, func(match string) string {
		if len(match) > 1 {
			return strings.ToUpper(match[1:2]) + match[2:]
		}
		return match
	})
}

func getProgressFilePath(path string) string {
	cleanedPath := filepath.Clean(path)

	// 计算路径的 SHA256 哈希值，并取前 5 个字符
	hashBytes := sha256.Sum256([]byte(cleanedPath))
	hashStr := hex.EncodeToString(hashBytes[:])[:5]

	// 清理路径的每个部分
	parts := strings.Split(cleanedPath, string(filepath.Separator))
	for i, part := range parts {
		parts[i] = processPart(sanitizeFileName(part))
	}

	// 构建基础文件名
	baseFileName := strings.Join(parts, "_")

	// 构建最终文件名
	fileName := fmt.Sprintf("tpg%s_%s.json", hashStr, baseFileName)

	// 确保文件名长度不超过 255 个字符
	const maxFileNameLength = 255
	if len(fileName) > maxFileNameLength {
		// 直接在 .json 前截断
		fileName = fileName[:maxFileNameLength-len(".json")] + ".json"
	}

	// 拼接进度文件路径
	return filepath.Join(progressDir, fileName)
}

// saveProgress 保存传输进度到文件
func saveProgress(progress Progress) error {
	progressMutex.Lock()
	defer progressMutex.Unlock()

	if err := ensureProgressDir(); err != nil {
		return fmt.Errorf("failed to create progress directory: %w", err)
	}

	filePath := getProgressFilePath(progress.Path)
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create progress file: %w", err)
	}
	defer file.Close()

	if err := json.NewEncoder(file).Encode(progress); err != nil {
		return fmt.Errorf("failed to encode progress entry: %w", err)
	}

	return nil
}

// loadProgress 从文件中加载传输进度
// loadProgress 自动推断类型
func loadProgress(path string) (Progress, error) {
	filePath := getProgressFilePath(path)
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// 新任务：根据路径类型初始化
			if fi, err := os.Stat(path); err == nil && fi.IsDir() {
				return Progress{Type: "dir", Path: path}, nil
			}
			return Progress{Type: "file", Path: path}, nil
		}
		return Progress{}, err
	}
	defer file.Close()

	var progress Progress
	if err := json.NewDecoder(file).Decode(&progress); err != nil {
		return Progress{}, err
	}
	return progress, nil
}

// deleteProgress 删除进度文件
func deleteProgress(path string) error {
	progressMutex.Lock()
	defer progressMutex.Unlock()

	filePath := getProgressFilePath(path)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete progress file: %w", err)
	}
	return nil
}
