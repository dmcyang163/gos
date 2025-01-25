package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"

	"github.com/fatih/color"
)

func main() {
	// 启动 pprof 性能分析服务器
	go func() {
		http.ListenAndServe(":6060", nil)
	}()

	// 初始化随机数种子
	rand.Seed(time.Now().UnixNano())

	// 检查命令行参数
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <config_file> [sendfile <file_path> <peer_addr>]")
		return
	}

	// 加载配置文件
	configLoader := NewJSONConfigLoader()
	config, err := configLoader.LoadConfig(os.Args[1])
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	// 加载名字列表
	names, err := configLoader.LoadNames("names.json")
	if err != nil {
		fmt.Printf("Error loading names: %v\n", err)
		return
	}

	// 初始化日志模块
	logger := NewLogrusLogger(config)

	// 初始化 Goroutine 池
	executor, err := NewAntsExecutor(100, logger)
	if err != nil {
		fmt.Printf("Error creating executor: %v\n", err)
		return
	}
	defer executor.Release()

	// 创建节点并注入依赖
	node := NewNode(config, names, logger, executor)

	// 启动日志级别 API
	StartLogLevelAPI(logger, config)

	// 启动服务器和其他协程
	go node.startServer()
	go node.startDiscovery()
	go node.startHeartbeat()

	// 连接到引导节点
	if config.BootstrapNode != "" {
		node.connectToPeer(config.BootstrapNode)
	}

	// 检查是否需要发送文件
	if len(os.Args) > 2 && os.Args[2] == "sendfile" {
		if len(os.Args) < 5 {
			fmt.Println("Usage: go run main.go <config_file> sendfile <file_path> <peer_addr>")
			return
		}

		filePath := os.Args[3]
		peerAddr := os.Args[4]
		if err := node.SendFile(peerAddr, filePath); err != nil {
			fmt.Printf("Error sending file: %v\n", err)
		}
		return
	}

	// 读取用户输入并发送消息
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		message := scanner.Text()
		color.Green("You: %s\n", message)
		node.BroadcastMessage(message)
	}
}
