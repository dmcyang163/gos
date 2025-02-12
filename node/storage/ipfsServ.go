// storage/ipfs_service.go
package storage

import (
	"context"
	"fmt"
	"os"
	"sync"

	config "github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/coreapi"
	"github.com/ipfs/kubo/plugin/loader"
	"github.com/ipfs/kubo/repo/fsrepo"
)

type IPFSService struct {
	node    *core.IpfsNode
	cfgPath string

	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	running bool
}

func NewIPFSService(repoPath string) (*IPFSService, error) {
	expandedPath := os.ExpandEnv(repoPath)

	// 初始化插件系统
	if err := loadPlugins(expandedPath); err != nil {
		return nil, fmt.Errorf("加载插件失败: %w", err)
	}

	// 确保仓库初始化
	if !fsrepo.IsInitialized(expandedPath) {
		// 修正后（显式设置Sync字段）
		cfg, err := config.Init(os.Stdout, 2048)
		if err != nil {
			return nil, fmt.Errorf("生成配置失败: %w", err)
		}

		// 显式配置flatfs存储
		// 在NewIPFSService的配置部分修改
		cfg.Datastore.Spec = map[string]interface{}{
			"type":      "flatfs",
			"path":      "blocks",
			"shardFunc": "/repo/flatfs/shard/v1/next-to-last/2",
			// V2.1 添加同步配置（增量修改）
			"sync": true,
		}

		// 配置其他必要参数
		cfg.Addresses.Swarm = []string{"/ip4/0.0.0.0/tcp/4001"}
		cfg.Bootstrap = config.DefaultBootstrapAddresses

		if err := fsrepo.Init(expandedPath, cfg); err != nil {
			return nil, fmt.Errorf("初始化仓库失败: %w", err)
		}
	}

	return &IPFSService{
		cfgPath: expandedPath,
	}, nil
}

func (s *IPFSService) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.node != nil {
		return fmt.Errorf("service already started")
	}

	// 新增上下文检查
	if s.ctx == nil {
		s.ctx, s.cancel = context.WithCancel(context.Background())
	}

	if s.running {
		return fmt.Errorf("服务已在运行")
	}

	// 打开仓库
	repo, err := fsrepo.Open(s.cfgPath)
	if err != nil {
		return fmt.Errorf("打开仓库失败: %w", err)
	}

	// 修改Start()方法中的上下文创建逻辑
	if s.ctx == nil { // 保持原有检查
		// V3.1 统一上下文创建点
		s.ctx, s.cancel = context.WithCancel(context.Background())
	} else {
		// V3.2 复用现有上下文
		select {
		case <-s.ctx.Done():
			s.ctx, s.cancel = context.WithCancel(context.Background())
		default:
		}
	}

	// 删除旧的重建上下文的代码
	// s.ctx, s.cancel = context.WithCancel(context.Background()) // 删除此行

	// 创建节点
	node, err := core.NewNode(s.ctx, &core.BuildCfg{
		Repo:   repo,
		Online: true,
	})
	if err != nil {
		return fmt.Errorf("创建节点失败: %w", err)
	}

	s.node = node
	s.running = true

	// 初始化核心API
	if _, err = coreapi.NewCoreAPI(s.node); err != nil {
		return fmt.Errorf("初始化API失败: %w", err)
	}

	return nil
}

func (s *IPFSService) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// V3.3 状态优先检查
	if !s.running {
		return nil
	}

	// V3.4 安全调用顺序
	if s.cancel != nil {
		s.cancel() // 先取消上下文
	}

	if s.node != nil {
		if err := s.node.Close(); err != nil { // 再关闭节点
			return fmt.Errorf("关闭节点失败: %w", err)
		}
	}

	// V3.5 状态原子更新
	s.running = false
	return nil
}

func loadPlugins(repoPath string) error {
	plugins, err := loader.NewPluginLoader(repoPath)
	if err != nil {
		return fmt.Errorf("初始化插件加载器失败: %w", err)
	}

	if err := plugins.Initialize(); err != nil {
		return fmt.Errorf("初始化插件失败: %w", err)
	}

	if err := plugins.Inject(); err != nil {
		return fmt.Errorf("注入插件失败: %w", err)
	}
	return nil
}
