package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/jianwushu/Secs4go/example/sharedcfg"
	"github.com/jianwushu/Secs4go/secs4go"
)

// SecsServer SECS-I/GEM服务端示例
// 职责: main 只负责参数解析与启动；ServerApp 负责 wiring、生命周期与消息分发。

type ServerApp struct {
	opts      serverOptions
	config    *secs4go.Config
	transport *secs4go.HSMSTransport
	server    *secs4go.SecsGem
	logger    secs4go.Logger
}

func NewServerApp(opts serverOptions) (*ServerApp, error) {
	config := buildServerConfig(opts)
	transport := secs4go.NewHSMSTransport(config)
	transport.OnStateChange(func(oldState, newState secs4go.ConnectionState) {
		log.Printf("状态变更: %s -> %s", oldState, newState)
	})

	codec, err := secs4go.NewItemCodec(config.ItemAEncoding)
	if err != nil {
		return nil, fmt.Errorf("创建编解码器失败: %w", err)
	}

	logger := secs4go.NewFileLoggerWithLevel("Host", sharedcfg.ParseLogLevel(opts.LogLevel))
	server := secs4go.NewSecsGem("Host", config, codec)
	server.BindTransport(transport, logger)

	app := &ServerApp{
		opts:      opts,
		config:    config,
		transport: transport,
		server:    server,
		logger:    logger,
	}
	app.server.OnMessage(app.handleMessage)
	return app, nil
}

func (app *ServerApp) Run(ctx context.Context) error {
	if err := app.transport.Start(); err != nil {
		return fmt.Errorf("启动失败: %w", err)
	}
	defer func() {
		app.transport.Stop()
		log.Printf("服务端已停止")
	}()

	if app.opts.EnablePublisher {
		publisher := newEventPublisher(app.opts, app.server)
		go publisher.run(ctx)
		log.Printf("服务端已启动，监听: %s, 事件周期: %s, 事件发布器: enabled", app.config.Address, app.opts.EventInterval)
	} else {
		log.Printf("服务端已启动，监听: %s, 事件周期: %s, 事件发布器: disabled", app.config.Address, app.opts.EventInterval)
	}

	<-ctx.Done()
	return nil
}

func (app *ServerApp) handleMessage(msg *secs4go.Message) {
	sf := fmt.Sprintf("S%vF%v", msg.Stream, msg.Function)
	switch sf {
	case "S1F1":
		log.Printf("收到 S1F1, 发送 S1F2 回复")
		reply := HandleS1F1(msg.Item)
		if err := app.server.SendReply(msg, reply); err != nil {
			log.Printf("发送S1F2失败: %v", err)
		}
	case "S1F3":
		log.Printf("收到 S1F3, 发送 S1F4 回复")
		reply := HandleS1F3(msg.Item)
		if err := app.server.SendReply(msg, reply); err != nil {
			log.Printf("发送S1F4失败: %v", err)
		}
	case "S1F11":
		log.Printf("收到 S1F11, 发送 S1F12 回复")
		reply := HandleS1F11(msg.Item)
		if err := app.server.SendReply(msg, reply); err != nil {
			log.Printf("发送S1F12失败: %v", err)
		}
	case "S1F13":
		log.Printf("收到 S1F13, 发送 S1F14 回复")
		reply := HandleS1F13(msg.Item)
		if err := app.server.SendReply(msg, reply); err != nil {
			log.Printf("发送S1F14失败: %v", err)
		}
	case "S2F33":
		log.Printf("收到 S2F33, 发送 S2F34 回复")
		reply, err := HandleS2F33(msg.Item)
		if err != nil {
			log.Printf("处理S2F33失败: %v", err)
		}
		if err := app.server.SendReply(msg, reply); err != nil {
			log.Printf("发送S2F34失败: %v", err)
		}
	case "S2F35":
		log.Printf("收到 S2F35, 发送 S2F36 回复")
		reply, err := HandleS2F35(msg.Item)
		if err != nil {
			log.Printf("处理S2F35失败: %v", err)
		}
		if err := app.server.SendReply(msg, reply); err != nil {
			log.Printf("发送S2F36失败: %v", err)
		}
	case "S2F37":
		log.Printf("收到 S2F37, 发送 S2F38 回复")
		reply, err := HandleS2F37(msg.Item)
		if err != nil {
			log.Printf("处理S2F37失败: %v", err)
		}
		if err := app.server.SendReply(msg, reply); err != nil {
			log.Printf("发送S2F38失败: %v", err)
		}
	default:
		app.server.SendDefaultReply(msg)
	}
}

func main() {
	opts, err := parseServerOptions(os.Args[1:])
	if err != nil {
		log.Fatalf("解析服务端参数失败: %v", err)
	}

	app, err := NewServerApp(opts)
	if err != nil {
		log.Fatalf("初始化服务端失败: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx); err != nil {
		log.Fatalf("服务端运行失败: %v", err)
	}
}
