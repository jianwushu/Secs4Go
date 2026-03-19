package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/jianwushu/Secs4go/secs4go"
)

// main 是 SECS/GEM 客户端（Host）示例入口
// 职责: 建立连接 → 注册消息回调 → 等待退出信号
func main() {
	opts, err := parseClientOptions(os.Args[1:])
	if err != nil {
		log.Fatalf("解析客户端参数失败: %v", err)
	}

	// 1. 创建配置（客户端/主动模式）
	config := buildClientConfig(opts)

	// 2. 创建传输层与编解码器
	hsmsConnection := secs4go.NewHSMSTransport(config)
	codec, err := secs4go.NewItemCodec(config.ItemAEncoding)
	if err != nil {
		log.Fatalf("创建编解码器失败: %v", err)
	}

	// 3. 创建 logger（按日志级别写入文件）
	logger := secs4go.NewFileLoggerWithLevel("TEST", parseLogLevel(opts.LogLevel))

	// 4. 创建会话，注册消息处理回调
	client := secs4go.NewSecsGem("TEST", config, hsmsConnection, logger, codec)
	// 消息回调事件
	client.OnMessage(makeMessageHandler(client))
	// 状态回调事件
	hsmsConnection.OnStateChange(makeStateChangeHandler(client, hsmsConnection))

	// 4. 启动连接（发起 TCP 连接并完成 Select）
	if err := hsmsConnection.Start(); err != nil {
		log.Fatalf("连接失败: %v", err)
	}
	log.Printf("客户端已连接到: %s", config.Address)

	// 5. 等待退出信号（Ctrl+C / SIGTERM）
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// 6. 优雅停止
	hsmsConnection.Stop()
	log.Printf("已断开连接")
}

// makeMessageHandler 返回消息处理闭包，通过闭包持有 client 引用，避免全局变量
func makeMessageHandler(client *secs4go.SecsGem) func(*secs4go.Message) {
	return func(msg *secs4go.Message) {
		sf := fmt.Sprintf("S%dF%d", msg.Stream, msg.Function)
		switch sf {
		case "S1F1": // Are You There Request
			log.Printf("收到 S1F1, 发送 S1F2 回复")
			reply := secs4go.NewMessage(1, 2).WithItem(secs4go.L(
				secs4go.A("你好"),
				secs4go.A("1.0"),
			))
			if err := client.SendReply(msg, reply); err != nil {
				log.Printf("发送 S1F2 失败: %v", err)
			}
		case "S6F11": // Collection Event Report
			log.Printf("收到 S6F11, 发送 S6F12 回复")
			// reply := secs4go.NewMessage(6, 12).WithItem(secs4go.B(0))
			// if err := client.SendReply(msg, reply); err != nil {
			// 	log.Printf("发送 S6F12 失败: %v", err)
			// }
		default:
			client.SendDefaultReply(msg)
		}
	}
}

func makeStateChangeHandler(client *secs4go.SecsGem, hsmsConnection *secs4go.HSMSTransport) func(oldState, newState secs4go.ConnectionState) {
	return func(oldState, newState secs4go.ConnectionState) {
		log.Printf("状态变更: %s -> %s", oldState, newState)
		switch newState {
		case secs4go.StateSelected:
			// 1.握手
			_, err := client.Send(secs4go.NewMessage(1, 13).WithWBit(true).WithItem(
				secs4go.L(),
			))
			if err != nil {
				log.Printf("发送 S1F13 失败: %v", err)
				hsmsConnection.Reconnect() // 发 Separate.req 后立即触发重连
				return
			}

			// 2. 请求设备上线
			_, err = client.Send(secs4go.NewMessage(1, 17).WithWBit(true))
			if err != nil {
				log.Printf("发送 S1F17 失败: %v", err)
				// TODO 暂不处理
				return
			}
		}
	}
}

// parseLogLevel 将字符串日志级别转换为 secs4go.LogLevel
func parseLogLevel(level string) secs4go.LogLevel {
	switch level {
	case "debug":
		return secs4go.LogLevelDebug
	case "warn":
		return secs4go.LogLevelWarn
	case "error":
		return secs4go.LogLevelError
	default:
		return secs4go.LogLevelInfo
	}
}
