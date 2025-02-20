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
	illegalChars := "/\\:*?\"<>|" // 定义非法字符
	var builder strings.Builder
	for _, char := range part {
		if !strings.ContainsRune(illegalChars, char) {
			builder.WriteRune(char) // 保留合法字符
		}
	}
	return builder.String()
}

// 处理part，去掉_和-，并将它们后面的单词首字母大写
func processPart(part string) string {
	// 使用正则表达式匹配_或-后面跟随的单词
	re := regexp.MustCompile(`[_-]([^\s_-])`)
	// 替换匹配到的部分，使找到的字母变为大写
	return re.ReplaceAllStringFunc(part, func(match string) string {
		if len(match) > 1 {
			return strings.ToUpper(match[1:2]) + match[2:]
		}
		return match
	})
}

func getProgressFilePath(path string) string {
	// 清理路径以去除任何多余的分隔符或相对路径元素
	cleanedPath := filepath.Clean(path)

	// 计算路径的SHA256哈希值
	hashBytes := sha256.Sum256([]byte(cleanedPath))
	// 将哈希值转换为十六进制字符串，并截取前3个字符
	hashStr := hex.EncodeToString(hashBytes[:])[:5]

	// 分割清理过的路径为各个部分，并清理每个部分的非法字符
	parts := strings.Split(cleanedPath, string(filepath.Separator))
	for i, part := range parts {
		// 首先清理非法字符
		part = sanitizeFileName(part)
		// 然后处理连接符和大小写
		parts[i] = processPart(part)
	}

	// 将路径的各部分用下划线连接起来作为文件名的基础
	baseFileName := strings.Join(parts, "_")

	// 构建最终的文件名，将 "tpg" + hashStr 放在最前面
	fileName := fmt.Sprintf("tpg%s_%s.json", hashStr, baseFileName)

	// 确保 fileName 不超过255个字符
	const maxFileNameLength = 255
	if len(fileName) > maxFileNameLength {
		// 直接从 .json 前的部分进行截断
		fileNameWithoutExt := fileName[:len(fileName)-len(".json")] // 获取去掉 .json 后缀的文件名
		// 截断文件名，确保总长度不超过255个字符，并加上 .json 后缀
		fileName = fileNameWithoutExt[:maxFileNameLength-len(".json")] + ".json"
	}

	// 拼接成最终的进度文件路径
	progressFilePath := filepath.Join(progressDir, fileName)

	return progressFilePath
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

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(progress); err != nil {
		return fmt.Errorf("failed to encode progress entry: %w", err)
	}

	return nil
}

// loadProgress 从文件中加载传输进度
// loadProgress 自动推断类型
func loadProgress(path string) (Progress, error) {
	// 尝试读取现有进度文件
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

	// 解码现有进度
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
