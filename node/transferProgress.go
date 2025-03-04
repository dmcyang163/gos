package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"node/utils"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ProgressEntry 表示一个文件的传输进度
type ProgressEntry struct {
	RelPath   string `json:"relPath"`   // 文件的相对路径
	FileSize  int64  `json:"fileSize"`  // 文件的总大小
	Checksum  string `json:"checksum"`  // 文件的校验码（哈希码）
	Offset    int64  `json:"offset"`    // 已传输的字节数
	Completed bool   `json:"completed"` // 是否传输完成
	StartTime string `json:"startTime"` // 传输开始时间（格式：2006-01-02 15:04:05.000）
	EndTime   string `json:"endTime"`   // 传输结束时间（格式：2006-01-02 15:04:05.000）
}

// Progress 表示文件或目录的传输进度
type Progress struct {
	Type    string          `json:"type"`    // "file" 或 "dir"
	Path    string          `json:"path"`    // 文件或目录的路径
	Entries []ProgressEntry `json:"entries"` // 传输进度条目
}

const (
	progressDir       = "progress" // 进度文件存储目录
	maxFileNameLength = 255        // 最大文件名长度
)

var (
	progressMutex sync.Mutex                            // 用于保护进度文件的并发访问
	illegalChars  = regexp.MustCompile(`[/\\:*?"<>|]`)  // 定义非法字符的正则表达式
	re            = regexp.MustCompile(`[_-]([^\s_-])`) // 匹配_或-后面跟随的单词
)

// ensureProgressDir 确保进度文件目录存在
func ensureProgressDir() error {
	return os.MkdirAll(progressDir, os.ModePerm)
}

// sanitizeFileName 删除字符串中的非法字符
func sanitizeFileName(part string) string {
	return illegalChars.ReplaceAllString(part, "")
}

// processPart 处理part，去掉_和-，并将它们后面的单词首字母大写
func processPart(part string) string {
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
		part = sanitizeFileName(part)
		parts[i] = processPart(part)
	}

	// 构建基础文件名
	baseFileName := strings.Join(parts, "_")

	// 构建最终文件名
	fileName := fmt.Sprintf("tpg%s_%s.json", hashStr, baseFileName)

	// 确保文件名长度不超过 255 个字符
	if len(fileName) > maxFileNameLength {
		// 直接在 .json 前截断
		fileName = fileName[:maxFileNameLength-len(".json")] + ".json"
	}

	// 拼接进度文件路径
	return filepath.Join(progressDir, fileName)
}

// withMutex 使用互斥锁执行操作
func withMutex(fn func() error) error {
	progressMutex.Lock()
	defer progressMutex.Unlock()
	return fn()
}

// saveProgress 保存传输进度到文件
func saveProgress(progress Progress) error {
	return withMutex(func() error {
		if err := ensureProgressDir(); err != nil {
			return fmt.Errorf("failed to create progress directory: %w", err)
		}

		filePath := getProgressFilePath(progress.Path)
		file, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed to create progress file '%s': %w", filePath, err)
		}
		defer file.Close()

		if err := json.NewEncoder(file).Encode(progress); err != nil {
			return fmt.Errorf("failed to encode progress entry: %w", err)
		}

		return nil
	})
}

// loadProgress 从文件中加载传输进度
func loadProgress(path string) (Progress, error) {
	var progress Progress
	err := withMutex(func() error {
		filePath := getProgressFilePath(path)
		file, err := os.Open(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				// 新任务：根据路径类型初始化
				if fi, err := os.Stat(path); err == nil && fi.IsDir() {
					progress = Progress{Type: "dir", Path: path}
				} else {
					progress = Progress{Type: "file", Path: path}
				}
				return nil
			}
			return err
		}
		defer file.Close()

		if err := json.NewDecoder(file).Decode(&progress); err != nil {
			return fmt.Errorf("failed to decode progress from file '%s': %w", filePath, err)
		}

		// 确保每个条目的 StartTime、EndTime、FileSize 和 Checksum 有默认值
		for i := range progress.Entries {
			if progress.Entries[i].StartTime == "" {
				progress.Entries[i].StartTime = time.Now().Format("2006-01-02 15:04:05.000")
			}
			if progress.Entries[i].EndTime == "" {
				progress.Entries[i].EndTime = ""
			}
			if progress.Entries[i].FileSize == 0 {
				// 如果 FileSize 为 0，尝试从文件系统中获取文件大小
				fullPath := filepath.Join(path, progress.Entries[i].RelPath)
				if fi, err := os.Stat(fullPath); err == nil {
					progress.Entries[i].FileSize = fi.Size()
				}
			}
			if progress.Entries[i].Checksum == "" {
				// 如果 Checksum 为空，尝试计算文件的哈希码
				fullPath := filepath.Join(path, progress.Entries[i].RelPath)
				if checksum, err := utils.CalculateFileChecksum(fullPath); err == nil {
					progress.Entries[i].Checksum = checksum
				}
			}
		}
		return nil
	})

	return progress, err
}

// deleteProgress 删除进度文件
func deleteProgress(path string) error {
	return withMutex(func() error {
		filePath := getProgressFilePath(path)
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete progress file '%s': %w", filePath, err)
		}
		return nil
	})
}
