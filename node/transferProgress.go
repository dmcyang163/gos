package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// getProgressFilePath 获取进度文件的路径
func getProgressFilePath(path string) string {
	// 清理路径
	cleanedPath := filepath.Clean(path)

	// 处理盘符 (如果存在)
	if filepath.IsAbs(cleanedPath) {
		volumeName := filepath.VolumeName(cleanedPath)
		// 移除盘符后面的冒号
		volumeName = strings.TrimSuffix(volumeName, ":")
		cleanedPath = volumeName + cleanedPath[len(filepath.VolumeName(cleanedPath)):]
	}

	// 分割路径
	parts := strings.Split(cleanedPath, string(filepath.Separator))

	// 拼接路径的各个部分
	fileName := strings.Join(parts, "_")

	// 添加 .json 后缀
	fileName += ".json"

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
