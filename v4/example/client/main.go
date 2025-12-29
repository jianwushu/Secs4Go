package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/your-org/secs4go_v4"
)

// SecsClient SECS-I/GEM客户端示例
// 职责: 创建会话、连接、发送/接收消息

var client *secs4go_v4.SecsGem

const (
	ServerAddress = "127.0.0.1:5000"
	ClientDevice  = 0 // 客户端设备ID
)

func main() {
	// 1. 创建配置（客户端模式）
	config := secs4go_v4.DefaultConfig(ServerAddress)
	config.DeviceID = ClientDevice
	config.IsActive = true // 客户端模式
	config.EnableHeartbeat = true

	hsmsConnection := secs4go_v4.NewHSMSTransport(config)

	// 2. 创建会话
	client = secs4go_v4.NewSecsGem("TEST", config, hsmsConnection, nil)

	// 3. 设置消息处理回调（接收服务端主动发送的消息）
	client.OnMessage(handleMessage)

	// 4. 启动会话（连接并Select）
	if err := hsmsConnection.Start(); err != nil {
		log.Fatalf("连接失败: %v", err)
	}

	// 4. 发送消息测试
	testMessages()

	// 5. 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// 6. 停止会话
	hsmsConnection.Stop()
	log.Printf("已断开连接")
}

// testMessages 测试发送各种SECS消息
func testMessages() {
	// 并发发送需要回复的消息
	go func() {
		log.Println("--- [goroutine1] 发送 S1F3 (需要回复) ---")
		msg := secs4go_v4.NewMessage(1, 3).WithWBit(true)
		reply := &secs4go_v4.Message{}
		if err := client.Send(msg, reply); err != nil {
			log.Printf("[goroutine1] S1F3 失败: %v", err)
		} else {
			log.Printf("[goroutine1] 收到 S1F4: %v", reply.Item)
		}
	}()

	go func() {
		log.Println("--- [goroutine2] 发送 S2F33 (需要回复) ---")
		msg := secs4go_v4.NewMessage(2, 33).WithWBit(true)
		reply := &secs4go_v4.Message{}
		if err := client.Send(msg, reply); err != nil {
			log.Printf("[goroutine2] S2F33 失败: %v", err)
		} else {
			log.Printf("[goroutine2] 收到 S2F34: %v", reply.Item)
		}
	}()

	// 无需回复的消息
	go func() {
		log.Println("--- 发送 S1F1 (无需回复) ---")
		msg := secs4go_v4.NewMessage(1, 1).WithWBit(true)
		reply := &secs4go_v4.Message{}
		if err := client.Send(msg, reply); err != nil {
			log.Printf("S1F1 失败: %v", err)
		} else {
			log.Printf("S1F1 发送成功")
		}
	}()

	// 等待 goroutine 完成
	time.Sleep(10 * time.Second)
}

// handleMessage 处理服务端主动发送的消息
func handleMessage(msg *secs4go_v4.Message) {
	log.Printf("收到服务端消息 S%dF%d", msg.Stream, msg.Function)
}
