package main

import (
	"bufio"
	"fmt"
	"math/rand"
	_ "net/http/pprof"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/pflag"
)

func main() {
	// 启动 pprof 性能分析服务器
	go func() {
		//http.ListenAndServe(":6060", nil)
	}()

	// 初始化随机数种子
	rand.Seed(time.Now().UnixNano())

	// 定义命令行参数
	var configFile string
	pflag.StringVarP(&configFile, "config", "c", "", "Path to the configuration file")
	pflag.Parse()

	// 检查命令行参数
	if configFile == "" {
		fmt.Println("Usage: go run main.go -c <config_file>")
		return
	}

	// 加载配置文件
	configLoader := NewJSONConfigLoader()
	config, err := configLoader.LoadConfig(configFile)
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
		if err := node.connectToPeer(config.BootstrapNode); err != nil {
			logger.Errorf("Failed to connect to bootstrap node: %v", err)
		} else {
			// 连接成功后，等待 5 秒再发送文件
			time.Sleep(5 * time.Second)

			// 发送文件
			filePath := "D:/young/gos/node/123/222222.dll" // 要发送的文件路径
			if err := node.SendDir("127.0.0.1:1234", filePath); err != nil {
				logger.Errorf("Failed to send file: %v", err)
			} else {
				logger.Infof("File %s sent successfully to %s", filePath, config.BootstrapNode)
			}

		}
	}

	// 读取用户输入并发送消息
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		message := scanner.Text()
		color.Green("You: %s\n", message)
		node.BroadcastMessage(message)

	}
}
