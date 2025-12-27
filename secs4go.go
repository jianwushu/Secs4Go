// Package secs4go 提供SECS/GEM + HSMS协议的Go语言实现
//
// 这是一个极简化的驱动库,参考Secs4Net的成熟设计:
//   - 单包导入
//   - 内置SEMI标准默认值
//   - 自动重连、自动心跳
//   - 极简API
//
// 快速开始:
//
//	// 客户端
//	client := secs4go.NewClient("127.0.0.1:5000")
//	client.Start()
//	defer client.Stop()
//
//	client.SendAndWait(
//	    secs4go.NewMessage(1, 13).WithItem(secs4go.L()),
//	)
//
//	// 服务器
//	server := secs4go.NewServer(":5000")
//	server.OnMessage(func(msg *secs4go.Message) error {
//	    // 处理消息
//	    return nil
//	})
//	server.Start()
//	defer server.Stop()
package secs4go

import "fmt"

// Version 版本号
const Version = "3.0.0"

// ============================================================
// 客户端/服务器创建 - 极简API
// ============================================================

// NewClient 创建客户端(Active模式)
func NewClient(address string) *Session {
	config := DefaultConfig(address)
	config.IsActive = true

	session, err := newSession(config)
	if err != nil {
		panic(fmt.Sprintf("failed to create client: %v", err))
	}

	return session
}

// NewClientWithConfig 使用自定义配置创建客户端
func NewClientWithConfig(config *Config) *Session {
	config.IsActive = true

	session, err := newSession(config)
	if err != nil {
		panic(fmt.Sprintf("failed to create client: %v", err))
	}

	return session
}

// NewServer 创建服务器(Passive模式)
func NewServer(address string) *Session {
	config := DefaultConfig(address)
	config.IsActive = false

	session, err := newSession(config)
	if err != nil {
		panic(fmt.Sprintf("failed to create server: %v", err))
	}

	return session
}

// NewServerWithConfig 使用自定义配置创建服务器
func NewServerWithConfig(config *Config) *Session {
	config.IsActive = false

	session, err := newSession(config)
	if err != nil {
		panic(fmt.Sprintf("failed to create server: %v", err))
	}

	return session
}

// ============================================================
// 链式配置API(可选)
// ============================================================

// WithDeviceID 设置设备编号
func (s *Session) WithDeviceName(deviceName string) *Session {
	s.config.DeviceName = deviceName
	return s
}

// WithDeviceID 设置设备ID
func (s *Session) WithDeviceID(deviceID uint16) *Session {
	s.config.DeviceID = deviceID
	return s
}

// WithAutoReconnect 设置自动重连
func (s *Session) WithAutoReconnect(enable bool) *Session {
	s.config.AutoReconnect = enable
	return s
}

// WithHeartbeat 设置心跳
func (s *Session) WithHeartbeat(enable bool) *Session {
	s.config.EnableHeartbeat = enable
	return s
}

// WithLogger 设置日志器
func (s *Session) WithLogger(logger Logger) *Session {
	s.config.Logger = logger
	return s
}

// WithStateChangeHandler 设置状态变更事件回调
func (s *Session) WithStateChangeHandler(handler StateChangeHandler) *Session {
	s.OnStateChange(handler)
	return s
}

// WithMessageHandler 设置消息处理器
func (s *Session) WithMessageHandler(handler MessageHandler) *Session {
	s.OnMessage(handler)
	return s
}
