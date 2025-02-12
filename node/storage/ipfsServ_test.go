package storage

import (
	"testing"
)

func TestIPFSService(t *testing.T) {
	// 使用一个临时目录作为 repoPath
	//repoPath := t.TempDir()
	//t.Logf("repoPath: %s", repoPath)
	// 创建 IPFS 服务
	repoPath := "../ipfs-repo" // 使用测试用的仓库路径

	ipfsService, err := NewIPFSService(repoPath)
	if err != nil {
		t.Fatalf("创建 IPFS 服务失败: %v", err)
	}
	defer ipfsService.Stop() // 确保在测试结束时关闭 IPFS 节点

	// 启动 IPFS 服务 (实际上不需要启动)
	if err := ipfsService.Start(); err != nil {
		t.Fatalf("启动 IPFS 服务失败: %v", err)
	}

	// 在这里添加与 IPFS 守护节点交互的测试代码
	// 例如：
	// - 添加文件
	// - 获取文件
	// - 查询网络
	// ...

	// 示例：检查 IPFS 节点是否已启动 (无法直接检查，需要通过其他方式验证)
	// 可以尝试添加一个文件，然后尝试获取它，如果成功则认为节点已启动
	// ...
}
