package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/fatih/color"
)

// 定义常量用于配置文件和名称文件路径
const (
	namesFile = "names.json"
)

func main() {
	// 初始化随机数种子
	rand.Seed(time.Now().UnixNano())

	// 加载名字列表
	names, err := LoadNames(namesFile)
	if err != nil {
		fmt.Printf("Error loading names: %v\n", err)
		return
	}

	// 检查命令行参数
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <config_file>")
		return
	}

	// 加载配置文件
	config, err := LoadConfig(os.Args[1])
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	// 创建节点
	node := NewNode(config, names)

	// 启动日志级别 API
	node.startLogLevelAPI("8080")

	// 启动服务器和其他协程
	go node.startServer()
	go node.startDiscovery()
	go node.startHeartbeat()

	// 连接到引导节点
	if config.BootstrapNode != "" {
		node.connectToPeer(config.BootstrapNode)
	}

	// 读取用户输入并发送消息
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		message := scanner.Text()
		color.Green("You: %s\n", message) // 彩色显示用户输入
		node.send(message)
	}
}
