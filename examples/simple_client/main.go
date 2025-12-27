package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	secs4go "github.com/your-org/secs4go_v3"
)

var client *secs4go.Session

func main() {
	// ============================================================
	// 极简客户端 - 只需3行代码!
	// ============================================================

	// 1. 创建客户端(自动使用SEMI标准默认值)
	client = secs4go.NewClient("127.0.0.1:5000")

	// 2. 设置消息处理器
	client.OnMessage(handleMessage)
	client.OnStateChange(handleState)

	// 3. 启动(自动连接、自动重连、自动心跳)
	if err := client.Start(); err != nil {
		log.Printf("Warning: Initial connection failed, will auto-retry: %v", err)
	}

	for {
		if err := client.WaitReady(); err != nil {
			log.Printf("Waiting for connection: %v, retrying...", err)
			continue
		}
		break
	}

	// 保持运行，监听退出信号
	log.Println("Client running, press Ctrl+C to exit...")

	// 创建信号通道，捕获 Ctrl+C (SIGINT) 和 SIGTERM
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 等待退出信号
	sig := <-sigChan
	log.Printf("Received signal: %v, shutting down gracefully...", sig)

	// 优雅关闭
	client.Stop()
	log.Println("Client stopped successfully")
}

// handleMessage 消息处理器
func handleMessage(msg *secs4go.Message) error {

	// 处理特定消息
	switch {
	case msg.Stream == 6 && msg.Function == 11:

		// 自动回复 S6F12
		return msg.Reply(
			secs4go.NewMessage(6, 12).WithItem(secs4go.B(0)),
		)
	default:
		if msg.WBit {
			return client.SendDefaultReply(msg)
		}
	}

	return nil
}

func handleState(oldState, newState secs4go.ConnectionState) {
	if newState == secs4go.StateSelected {

		// 异步发送S1F13以避免阻塞状态回调
		go func() {
			_, err := client.SendAndWait(
				secs4go.NewMessage(1, 13).
					WithWBit(true).
					WithItem(secs4go.L()),
			)
			if err != nil {
				log.Printf("Error: %v", err)
			}
		}()
	}
}
