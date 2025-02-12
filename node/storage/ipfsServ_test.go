// storage/ipfs_service_test.go
package storage

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/ipfs/kubo/repo/fsrepo"
)

// storage/ipfs_service_test.go
func TestIPFSServiceLifecycle(t *testing.T) {
	t.Run("完整生命周期", func(t *testing.T) {
		tmpDir := t.TempDir()

		// 创建服务
		serv, err := NewIPFSService(tmpDir)
		if err != nil {
			t.Fatal("创建服务失败:", err)
		}

		// 首次启动
		if err := serv.Start(); err != nil {
			t.Fatal("首次启动失败:", err)
		}

		// 重复启动检查
		if err := serv.Start(); err == nil {
			t.Error("应返回重复启动错误")
		}

		// 正常停止
		if err := serv.Stop(); err != nil {
			t.Error("正常停止失败:", err)
		}

		// 重复停止
		if err := serv.Stop(); err != nil {
			t.Error("重复停止应返回nil:", err)
		}

		// 重启检查
		if err := serv.Start(); err != nil {
			t.Error("重启失败:", err)
		}
	})

	t.Run("配置验证", func(t *testing.T) {
		tmpDir := t.TempDir()
		serv, _ := NewIPFSService(tmpDir)
		_ = serv
		// 验证Sync配置
		repo, _ := fsrepo.Open(tmpDir)
		cfg, _ := repo.Config()
		if syncVal := cfg.Datastore.Spec["sync"]; syncVal != true {
			t.Errorf("Sync配置未正确设置，期望true，实际%v", syncVal)
		}
	})
}
func TestRepoOperations(t *testing.T) {
	repoPath, cleanup := createTestRepo(t) // 实际使用函数返回值
	defer cleanup()
	_ = repoPath
	// 后续使用 repoPath 进行测试...
}

func createTestRepo(t *testing.T) (string, func()) {
	dir, err := ioutil.TempDir("", "ipfs-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}

	return dir, func() {
		os.RemoveAll(dir)
	}
}
