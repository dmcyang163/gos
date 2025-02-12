package storage

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	config "github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/repo/fsrepo"
)

// IPFS 服务结构体
type IPFSService struct {
	node *core.IpfsNode
	repo *fsrepo.FSRepo
}

// GetExecutablePath 获取GOPATH/bin目录下指定可执行文件的完整路径
func GetExecutablePath(exeName string) (string, error) {
	// 获取GOPATH环境变量
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		return "", fmt.Errorf("GOPATH environment variable is not set")
	}
	// 构建可执行文件的完整路径
	binPath := filepath.Join(gopath, "bin", exeName)
	return binPath, nil
}

// 创建 IPFS 服务
func NewIPFSService(repoPath string) (*IPFSService, error) {
	// 检查 repoPath 是否有效
	if repoPath == "" {
		return nil, fmt.Errorf("repoPath 不能为空")
	}
	// 迁移 IPFS 仓库
	exe, _ := GetExecutablePath("fs-repo-migrations.exe")
	fmt.Printf("exe: %s\n", exe)
	if err := exec.Command(exe, "--repo=ipfs-repo", "run").Run(); err != nil {
		return nil, fmt.Errorf("迁移 IPFS 仓库失败: %v", err)
	}

	// 初始化 IPFS 仓库
	if !fsrepo.IsInitialized(repoPath) {
		// 初始化配置
		config, err := config.Init(os.Stdout, 2048)
		if err != nil {
			return nil, fmt.Errorf("初始化配置失败: %w", err)
		}
		// 初始化仓库
		if err := fsrepo.Init(repoPath, config); err != nil {
			return nil, fmt.Errorf("初始化 IPFS 仓库失败: %w", err)
		}
	}

	// 打开 IPFS 仓库
	repo, err := fsrepo.Open(repoPath)
	if err != nil {
		return nil, fmt.Errorf("打开 IPFS 仓库失败: %w", err)
	}
	defer repo.Close() // Close the repository when done

	// 创建 IPFS 节点
	ctx := context.Background()
	node, err := core.NewNode(ctx, &core.BuildCfg{
		Repo: repo,
		Host: libp2p.DefaultHostOption,
	})
	if err != nil {
		return nil, fmt.Errorf("创建 IPFS 节点失败: %w", err)
	}

	return &IPFSService{
		node: node,
		repo: repo.(*fsrepo.FSRepo),
	}, nil
}

// 启动 IPFS 服务
func (s *IPFSService) Start() error {
	// IPFS 节点在创建时已经启动
	return nil
}

// 停止 IPFS 服务
func (s *IPFSService) Stop() error {
	if err := s.node.Close(); err != nil {
		return fmt.Errorf("停止 IPFS 节点失败: %w", err)
	}
	return nil
}

/*
func main() {
	// 创建 IPFS 服务
	repoPath := "ipfs-repo"
	ipfsService, err := NewIPFSService(repoPath)
	if err != nil {
		fmt.Println("创建 IPFS 服务失败:", err)
		return
	}

	// 启动 IPFS 服务
	if err := ipfsService.Start(); err != nil {
		fmt.Println("启动 IPFS 服务失败:", err)
		return
	}

	// 与 IPFS 守护节点交互
	// ...

	// 停止 IPFS 服务
	defer ipfsService.Stop()
}
*/
