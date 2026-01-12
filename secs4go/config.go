package secs4go

import (
	"fmt"
	"time"
)

// Config 配置结构 - 所有参数都有SEMI标准默认值
type Config struct {
	// 必填参数
	Address string // "127.0.0.1:5000" 或 ":5000"

	// 可选参数(都有默认值)
	DeviceID uint16 // 默认: 0

	// 连接模式
	IsActive bool // 默认: true (客户端模式)

	// SEMI标准超时(用户99%情况不需要改)
	T3 time.Duration // Reply timeout (默认: 45s)
	T5 time.Duration // Connect separation (默认: 10s)
	T6 time.Duration // Control transaction (默认: 5s)
	T7 time.Duration // Not selected (默认: 10s)
	T8 time.Duration // Network intercharacter (默认: 5s)

	// 自动重连配置(Active模式自动重连, 使用T5作为重连间隔)
	AutoReconnect     bool // 默认: true
	MaxReconnectTries int  // 默认: -1 (无限重试)

	// 心跳配置(自动LinkTest)
	EnableHeartbeat   bool          // 默认: true
	HeartbeatInterval time.Duration // 默认: 60s

	// 编码配置
	ItemAEncoding string // 默认: "ASCII" (支持 "ASCII", "GBK", "UTF-8", "GB2312")
}

// DefaultConfig 返回默认配置
func DefaultConfig(address string) *Config {
	return &Config{
		Address:           address,
		DeviceID:          0,
		IsActive:          true,
		T3:                45 * time.Second,
		T5:                10 * time.Second,
		T6:                5 * time.Second,
		T7:                10 * time.Second,
		T8:                5 * time.Second,
		AutoReconnect:     true,
		MaxReconnectTries: -1, // 无限重试
		EnableHeartbeat:   true,
		HeartbeatInterval: 60 * time.Second,
		ItemAEncoding:     "ASCII",
	}
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Address == "" {
		return fmt.Errorf("address is required")
	}

	if c.T3 <= 0 {
		return fmt.Errorf("T3 must be positive")
	}

	if c.T5 <= 0 {
		return fmt.Errorf("T5 must be positive")
	}

	if c.T6 <= 0 {
		return fmt.Errorf("T6 must be positive")
	}

	if c.T7 <= 0 {
		return fmt.Errorf("T7 must be positive")
	}

	if c.T8 <= 0 {
		return fmt.Errorf("T8 must be positive")
	}

	if c.EnableHeartbeat && c.HeartbeatInterval <= 0 {
		return fmt.Errorf("heartbeat interval must be positive when heartbeat is enabled")
	}

	return nil
}

// Clone 克隆配置
func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}

	clone := *c
	return &clone
}
