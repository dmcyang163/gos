package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

//	func generateBase62UUID() string {
//		uuidBytes := uuid.New()
//		encoded := base62.Encode(uuidBytes[:])
//		return string(encoded)
//	}
func generateUUID() string {
	return uuid.New().String()
}

// generateTraceID 生成唯一的跟踪 ID。
func GenerateTraceID() string {
	return "trace-" + generateUUID()
}

// generateMessageID 生成唯一的 message ID。
func GenerateMessageID() string {
	return "msg-" + generateUUID()
}

// collectFiles 收集目录下的所有文件信息
func CollectFiles(dirPath string) ([]struct {
	FilePath    string
	FullRelPath string
}, error) {
	// 检查目录是否存在
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory does not exist: %s", dirPath)
	}

	// 获取目录名字
	dirName := filepath.Base(dirPath)

	// 收集所有文件的路径和相对路径
	var files []struct {
		FilePath    string
		FullRelPath string
	}

	err := filepath.Walk(dirPath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 忽略目录，只发送文件
		if info.IsDir() {
			return nil
		}

		// 计算相对路径
		relPath, err := filepath.Rel(dirPath, filePath)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// 将 dirName 作为 relPath 的父目录
		fullRelPath := filepath.Join(dirName, relPath)

		// 收集文件信息
		files = append(files, struct {
			FilePath    string
			FullRelPath string
		}{filePath, fullRelPath})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return files, nil
}
