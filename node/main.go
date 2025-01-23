package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"time"
)

// 全局名字列表
var names []NameEntry

func main() {
	rand.Seed(time.Now().UnixNano())

	// 加载名字列表
	var err error
	names, err = LoadNames("names.json")
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
		node.send(message)
	}
}
