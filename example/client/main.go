package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/jianwushu/Secs4go/secs4go"
)

// SecsClient SECS-I/GEM客户端示例
// 职责: 创建会话、连接、发送/接收消息

var client *secs4go.SecsGem

const (
	ServerAddress = "127.0.0.1:5000"
	ClientDevice  = 0 // 客户端设备ID
)

func main() {
	// 1. 创建配置（客户端模式）
	config := secs4go.DefaultConfig(ServerAddress)
	config.DeviceID = ClientDevice
	config.IsActive = true // 客户端模式
	config.EnableHeartbeat = true

	hsmsConnection := secs4go.NewHSMSTransport(config)

	// 2. 创建会话
	client = secs4go.NewSecsGem("TEST", config, hsmsConnection, nil)

	// 3. 设置消息处理回调（接收服务端主动发送的消息）
	client.OnMessage(handleMessage)

	// 4. 启动会话（连接并Select）
	if err := hsmsConnection.Start(); err != nil {
		log.Fatalf("连接失败: %v", err)
	}

	// 6. 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// 7. 停止会话
	hsmsConnection.Stop()
	log.Printf("已断开连接")
}

// handleMessage 处理服务端主动发送的消息
func handleMessage(msg *secs4go.Message) {
	sf := fmt.Sprintf("S%vF%v", msg.Stream, msg.Function)
	switch sf {
	case "S1F1":
		log.Printf("收到 S1F1, 发送 S1F2 回复")
		reply := secs4go.NewMessage(1, 2).
			WithItem(secs4go.L(
				secs4go.A("HOST"),
				secs4go.A("1.0"),
			))
		if err := client.SendReply(msg, reply); err != nil {
			log.Printf("发送S1F2失败: %v", err)
		}
	case "S6F11": // S1F3 (Selected Equipment Status Request)
		log.Printf("收到 S6F11, 发送 S6F12 回复")
		reply := secs4go.NewMessage(6, 12).
			WithItem(secs4go.B(0))
		if err := client.SendReply(msg, reply); err != nil {
			log.Printf("发送S1F4失败: %v", err)
		}
	default:
		client.SendDefaultReply(msg)
	}
}
