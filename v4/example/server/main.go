package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/your-org/secs4go_v4"
)

// SecsServer SECS-I/GEM服务端示例
// 职责: 创建会话、设置消息处理回调、处理业务消息

const (
	ListenAddress = ":5000"
	ServerDevice  = 0 // 设备ID
)

var server *secs4go_v4.SecsGem

func main() {
	// 1. 创建配置（服务端模式）
	config := secs4go_v4.DefaultConfig(ListenAddress)
	config.DeviceID = ServerDevice
	config.IsActive = false // 服务端模式
	config.EnableHeartbeat = true

	hsmsConnection := secs4go_v4.NewHSMSTransport(config)

	// 2. 创建会话
	server = secs4go_v4.NewSecsGem("Host", config, hsmsConnection, nil)

	// 3. 设置消息处理回调 (会同步设置到 transport)
	server.OnMessage(handleMessage)

	// 4. 启动会话（开始监听）
	if err := hsmsConnection.Start(); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
	log.Printf("服务端已启动，监听: %s", ListenAddress)

	// 6. 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// 7. 停止会话
	hsmsConnection.Stop()
	log.Printf("服务端已停止")
}

// handleMessage 处理数据消息
func handleMessage(msg *secs4go_v4.Message) {
	sf := fmt.Sprintf("S%vF%v", msg.Stream, msg.Function)
	switch sf {
	case "S1F1":
		log.Printf("收到 S1F1, 发送 S1F2 回复")
		reply := secs4go_v4.NewMessage(1, 2).
			WithItem(secs4go_v4.L(
				secs4go_v4.A("HOST"),
				secs4go_v4.A("1.0"),
			))
		if err := server.SendReply(msg, reply); err != nil {
			log.Printf("发送S1F2失败: %v", err)
		}
	case "S1F3": // S1F3 (Selected Equipment Status Request)
		log.Printf("收到 S1F3, 发送 S1F4 回复")
		reply := secs4go_v4.NewMessage(1, 4).
			WithItem(secs4go_v4.L(
				secs4go_v4.L(
					secs4go_v4.A("RUN"),
					secs4go_v4.A("1"),
				),
				secs4go_v4.L(
					secs4go_v4.A("TEMP"),
					secs4go_v4.F4(25.5),
				),
			))
		if err := server.SendReply(msg, reply); err != nil {
			log.Printf("发送S1F4失败: %v", err)
		}
	case "S2F33":
		log.Printf("收到 S2F33, 发送 S2F34 回复")
		reply := secs4go_v4.NewMessage(2, 34).
			WithItem(secs4go_v4.A("20241227120000"))
		if err := server.SendReply(msg, reply); err != nil {
			log.Printf("发送S2F34失败: %v", err)
		}
	default:
		server.SendDefaultReply(msg)
	}
}
