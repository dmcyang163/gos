package storage

import (
	"bufio"
	"fmt"
	"os"

	shell "github.com/ipfs/go-ipfs-api"
)

// Octopus 接口定义了与 IPFS 相关的操作
type Octopus interface {
	AddFile(filePath string) (string, error)
	GetFile(cid string) (string, error)
	UpdateFile(cid, newFilePath string) (string, error)
	DeleteFile(cid string) error
}

// octopusImpl 是 Octopus 接口的一个实现
type octopusImpl struct {
	shell *shell.Shell
}

// NewOctopus 创建一个新的 Octopus 实例
func NewOctopus() Octopus {
	return &octopusImpl{
		shell: shell.NewShell("localhost:5001"),
	}
}

// AddFile 将文件添加到 IPFS 网络并返回其 CID
func (o *octopusImpl) AddFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	cid, err := o.shell.Add(file)
	if err != nil {
		return "", fmt.Errorf("failed to add file to IPFS: %v", err)
	}
	return cid, nil
}

// GetFile 根据 CID 从 IPFS 网络获取文件
func (o *octopusImpl) GetFile(cid string) (string, error) {
	file, err := o.shell.Cat(cid)
	if err != nil {
		return "", fmt.Errorf("failed to get file from IPFS: %v", err)
	}
	defer file.Close()

	// 读取文件内容到缓冲区
	scanner := bufio.NewScanner(file)
	content := ""
	for scanner.Scan() {
		content += scanner.Text() + "\n"
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read file content: %v", err)
	}

	return content, nil
}

// UpdateFile 更新 IPFS 网络中的文件内容
func (o *octopusImpl) UpdateFile(cid, newFilePath string) (string, error) {
	// 首先删除旧文件
	err := o.shell.Unpin(cid)
	if err != nil {
		return "", fmt.Errorf("failed to remove old file from IPFS: %v", err)
	}

	// 添加新文件
	file, err := os.Open(newFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to open new file: %v", err)
	}
	defer file.Close()

	newCid, err := o.shell.Add(file)
	if err != nil {
		return "", fmt.Errorf("failed to add new file to IPFS: %v", err)
	}

	return newCid, nil
}

// DeleteFile 从 IPFS 网络中删除文件
func (o *octopusImpl) DeleteFile(cid string) error {
	err := o.shell.Unpin(cid)
	if err != nil {
		return fmt.Errorf("failed to delete file from IPFS: %v", err)
	}
	return nil
}
