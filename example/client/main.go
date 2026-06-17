package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	secs4go "github.com/jianwushu/secs4go/core"
	"github.com/jianwushu/secs4go/example/sharedcfg"
	"github.com/jianwushu/secs4go/extension/sml"
)

// main 是 SECS/GEM 客户端（Host）示例入口
// 职责: main 只负责参数解析与启动；ClientApp 负责 wiring、生命周期与消息分发。

type ClientApp struct {
	opts      clientOptions
	config    *secs4go.Config
	transport *secs4go.HSMSTransport
	client    *secs4go.SecsGem
	logger    secs4go.Logger
}

func NewClientApp(opts clientOptions) (*ClientApp, error) {
	config := buildClientConfig(opts)
	transport := secs4go.NewHSMSTransport(config)

	codec, err := secs4go.NewItemCodec(config.ItemAEncoding)
	if err != nil {
		return nil, fmt.Errorf("创建编解码器失败: %w", err)
	}

	logger := secs4go.NewFileLoggerWithLevel("TEST", sharedcfg.ParseLogLevel(opts.LogLevel))
	client := secs4go.NewSecsGem("TEST", config, codec, logger)
	client.WithMessageFormatter(sml.ToSMLWithHex)
	client.BindTransport(transport)

	app := &ClientApp{
		opts:      opts,
		config:    config,
		transport: transport,
		client:    client,
		logger:    logger,
	}
	app.client.OnMessage(app.handleMessage)
	app.transport.OnStateChange(app.handleStateChange)
	return app, nil
}

func (app *ClientApp) Run(ctx context.Context) error {
	if err := app.transport.Start(); err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	defer func() {
		app.transport.Stop()
		log.Printf("已断开连接")
	}()

	log.Printf("客户端已连接到: %s", app.config.Address)
	<-ctx.Done()
	return nil
}

func (app *ClientApp) handleMessage(msg *secs4go.Message) {
	sf := fmt.Sprintf("S%dF%d", msg.Stream, msg.Function)
	switch sf {
	case "S1F1":
		log.Printf("收到 S1F1, 发送 S1F2 回复")
		reply := secs4go.NewMessage(1, 2).WithItem(secs4go.L(
			secs4go.A("你好"),
			secs4go.A("1.0"),
		))
		if err := app.client.SendReply(msg, reply); err != nil {
			log.Printf("发送 S1F2 失败: %v", err)
		}
	case "S6F11":
		// 模拟随机延时 (0-2000ms)
		delay := time.Duration(rand.Intn(2001)) * time.Millisecond
		app.logger.Warn("收到 S6F11, 延时 %v 后发送 S6F12 回复", delay)
		time.Sleep(delay)

		reply := secs4go.NewMessage(6, 12).WithItem(secs4go.B(0))
		if err := app.client.SendReply(msg, reply); err != nil {
			log.Printf("发送 S6F12 失败: %v", err)
		}

		delay = time.Duration(rand.Intn(2001)) * time.Millisecond
		app.logger.Warn("任务延时 %v ", delay)
		time.Sleep(delay)

		item := msg.Item.GetItem(2).GetItem(0).GetItem(1).GetItem(0)
		val, _ := item.FirstBool()
		app.logger.Warn("%v item: %v", msg.SystemBytes, val)

	default:
		app.client.SendDefaultReply(msg)
	}
}

func (app *ClientApp) handleStateChange(oldState, newState secs4go.ConnectionState) {
	log.Printf("状态变更: %s -> %s", oldState, newState)
	switch newState {
	case secs4go.StateSelected:
		_, err := app.client.Send(secs4go.NewMessage(1, 13).WithWBit(true).WithItem(
			secs4go.L(),
		))
		if err != nil {
			log.Printf("发送 S1F13 失败: %v", err)
			app.transport.Reconnect()
			return
		}

		_, err = app.client.Send(secs4go.NewMessage(1, 17).WithWBit(true))
		if err != nil {
			log.Printf("发送 S1F17 失败: %v", err)
			return
		}
	}
}

func main() {
	opts, err := parseClientOptions(os.Args[1:])
	if err != nil {
		log.Fatalf("解析客户端参数失败: %v", err)
	}

	app, err := NewClientApp(opts)
	if err != nil {
		log.Fatalf("初始化客户端失败: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx); err != nil {
		log.Fatalf("客户端运行失败: %v", err)
	}
}
