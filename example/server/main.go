package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jianwushu/Secs4go/secs4go"
)

// SecsServer SECS-I/GEM服务端示例
// 职责: 创建会话、设置消息处理回调、处理业务消息

const (
	ListenAddress = ":5000"
	DeviceID      = 0 // 设备ID
)

var server *secs4go.SecsGem

func main() {
	// 1. 创建配置（服务端模式）
	config := secs4go.DefaultConfig(ListenAddress)
	config.T3 = 10 * time.Second
	config.DeviceID = DeviceID
	config.IsActive = false
	config.EnableHeartbeat = false

	hsmsConnection := secs4go.NewHSMSTransport(config)

	hsmsConnection.OnStateChange(handleStateChange)

	codec, _ := secs4go.NewItemCodec(config.ItemAEncoding)

	// 2. 创建会话
	server = secs4go.NewSecsGem("Host", config, hsmsConnection, nil, codec)

	// 3. 设置消息处理回调
	server.OnMessage(handleMessage)

	// 4. 启动会话（开始监听）
	if err := hsmsConnection.Start(); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
	log.Printf("服务端已启动，监听: %s", ListenAddress)

	go testSendMessage()

	// 6. 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// 7. 停止会话
	hsmsConnection.Stop()
	log.Printf("服务端已停止")
}

// handleMessage 处理数据消息
func handleMessage(msg *secs4go.Message) {
	sf := fmt.Sprintf("S%vF%v", msg.Stream, msg.Function)
	switch sf {
	case "S1F1":
		log.Printf("收到 S1F1, 发送 S1F2 回复")
		reply := HandleS1F1(msg.Item)
		if err := server.SendReply(msg, reply); err != nil {
			log.Printf("发送S1F2失败: %v", err)
		}
	case "S1F3": // S1F3 (Selected Equipment Status Request)
		log.Printf("收到 S1F3, 发送 S1F4 回复")
		reply := HandleS1F3(msg.Item)
		if err := server.SendReply(msg, reply); err != nil {
			log.Printf("发送S1F4失败: %v", err)
		}
	case "S1F11": // S1F11 (Status Variable Namelist Request)
		log.Printf("收到 S1F11, 发送 S1F12 回复")
		reply := HandleS1F11(msg.Item)
		if err := server.SendReply(msg, reply); err != nil {
			log.Printf("发送S1F12失败: %v", err)
		}
	case "S1F13":
		log.Printf("收到 S1F13, 发送 S1F14 回复")
		reply := HandleS1F13(msg.Item)
		if err := server.SendReply(msg, reply); err != nil {
			log.Printf("发送S1F14失败: %v", err)
		}
	case "S2F33":
		log.Printf("收到 S2F33, 发送 S2F34 回复")
		reply, err := HandleS2F33(msg.Item)
		if err != nil {
			log.Printf("处理S2F33失败: %v", err)
		}
		if err := server.SendReply(msg, reply); err != nil {
			log.Printf("发送S2F34失败: %v", err)
		}
	case "S2F35":
		log.Printf("收到 S2F35, 发送 S2F36 回复")
		reply, err := HandleS2F35(msg.Item)
		if err != nil {
			log.Printf("处理S2F35失败: %v", err)
		}
		if err := server.SendReply(msg, reply); err != nil {
			log.Printf("发送S2F36失败: %v", err)
		}
	default:
		server.SendDefaultReply(msg)
	}
}

func handleStateChange(oldState, newState secs4go.ConnectionState) {
	log.Printf("状态变更: %s -> %s", oldState, newState)
}

func testSendMessage() {

	i := 10

	for {
		if server.IsSelected() {

			i = i + 1
			UpdateDv("1001", fmt.Sprintf("CRR_好_%d", i))

			time.Sleep(1 * time.Second)

			go func() {
				msg, err := Trigger10020()
				if err != nil {
					log.Printf("触发10020失败: %v", err)
					return
				}
				_, err = server.Send(msg)
				if err != nil {
					log.Printf("S6F11 失败: %v", err)
				}

			}()

			time.Sleep(45 * time.Second)
		}
	}

}
