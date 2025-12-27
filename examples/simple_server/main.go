package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	secs4go "github.com/your-org/secs4go_v3"
)

var server *secs4go.Session

func main() {
	// ============================================================
	// 极简服务器 - 只需3行代码!
	// ============================================================

	// 1. 创建服务器
	server = secs4go.NewServer("127.0.0.1:5000")

	// 2. 设置消息处理器
	server.OnMessage(handleMessage)
	server.OnStateChange(handleState)

	// 3. 启动(自动监听、自动心跳)
	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	log.Println("Server started on:5000")
	log.Println("Waiting for connections...")

	// 创建信号通道，捕获 Ctrl+C (SIGINT) 和 SIGTERM
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 等待退出信号
	sig := <-sigChan
	log.Printf("Received signal: %v, shutting down gracefully...", sig)

	// 优雅关闭
	server.Stop()
	log.Println("Server stopped successfully")
}

// handleMessage 消息处理器
func handleMessage(msg *secs4go.Message) error {
	log.Printf("Received message: S%dF%d (WBit=%v)", msg.Stream, msg.Function, msg.WBit)

	// 处理特定消息
	switch {
	case msg.Stream == 1 && msg.Function == 13:
		// S1F13 - Establish Communications Request
		log.Println("Received Establish Communications Request")

		// 回复 S1F14
		return msg.Reply(
			secs4go.NewMessage(1, 14).WithItem(
				secs4go.L(
					secs4go.B(0), // COMMACK (0 = Accepted)
					secs4go.L(
						secs4go.A("Equipment"),
						secs4go.A("1.0.0"),
					),
				),
			),
		)

	case msg.Stream == 1 && msg.Function == 1:
		// S1F1 - Are You There Request
		log.Println("Received Are You There Request")

		// 回复 S1F2
		return msg.Reply(
			secs4go.NewMessage(1, 2).WithItem(
				secs4go.L(
					secs4go.A("Equipment"),
					secs4go.A("Rev 1.0"),
				),
			),
		)

	// case msg.Stream == 5 && msg.Function == 1:
	// 	// S5F1 - Alarm Report
	// 	log.Println("Received Alarm Report")

	// 	// 回复 S5F2
	// 	return msg.Reply(
	// 		secs4go.NewMessage(5, 2).WithItem(secs4go.B(0)),
	// 	)

	// case msg.Stream == 6 && msg.Function == 11:
	// 	// S6F11 - Event Report
	// 	log.Println("Received Event Report")

	// 	// 回复 S6F12
	// 	return msg.Reply(
	// 		secs4go.NewMessage(6, 12).WithItem(secs4go.B(0)),
	// 	)

	// case msg.Stream == 2 && msg.Function == 41:
	// 	// S2F41 - Host Command Send
	// 	log.Println("Received Host Command")

	// 	// 回复 S2F42
	// 	return msg.Reply(
	// 		secs4go.NewMessage(2, 42).WithItem(
	// 			secs4go.L(
	// 				secs4go.B(0), // HCACK (0 = Command Accepted)
	// 				secs4go.L(),  // Parameters
	// 			),
	// 		),
	// 	)

	default:
		log.Printf("Unhandled message: S%dF%d", msg.Stream, msg.Function)
	}

	return nil
}

func handleState(oldState, newState secs4go.ConnectionState) {
	if newState == secs4go.StateSelected {
		mockMessage()
	}
}

func mockMessage() {

	go func() {
		for {
			_, err := server.SendAndWait(
				secs4go.NewMessage(6, 11).
					WithWBit(true).
					WithItem(secs4go.L(
						secs4go.U4(0),
						secs4go.U4(10020),
						secs4go.L(
							secs4go.U4(10020),
							secs4go.L(
								secs4go.A("TEST"),
								secs4go.U4(10020),
							),
						),
					)),
			)
			if err != nil {
				log.Printf("Error: %v", err)
			}

			if !server.IsReady() {
				break
			}
			time.Sleep(2 * time.Second)
		}

	}()

	// go func() {
	// 	for {
	// 		_, err := server.SendAndWait(
	// 			secs4go.NewMessage(6, 11).
	// 				WithWBit(true).
	// 				WithItem(secs4go.L(
	// 					secs4go.U4(0),
	// 					secs4go.U4(10021),
	// 					secs4go.L(
	// 						secs4go.U4(10021),
	// 						secs4go.L(
	// 							secs4go.A("TEST"),
	// 							secs4go.U4(10021),
	// 						),
	// 					),
	// 				)),
	// 		)
	// 		if err != nil {
	// 			log.Printf("Error: %v", err)
	// 		}
	// 		time.Sleep(200 * time.Microsecond)
	// 	}

	// }()
}
