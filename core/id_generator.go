package core

import "sync/atomic"

// ============================================================
// MessageIdGenerator 消息ID生成器
// 用于生成 HSMS SystemBytes，支持自定义策略
// ============================================================

// MessageIdGenerator 消息ID生成器接口
// 生成 HSMS 协议中的 SystemBytes，用于关联请求和回复
type MessageIdGenerator interface {
	// Next 生成下一个唯一的消息ID
	Next() uint32
}

// DefaultIdGenerator 默认消息ID生成器（原子递增计数器）
// 从 0 开始，每次调用 Next() 递增 1，溢出后自然回绕到 0
type DefaultIdGenerator struct {
	counter atomic.Uint32
}

// NewDefaultIdGenerator 创建默认消息ID生成器
func NewDefaultIdGenerator() *DefaultIdGenerator {
	return &DefaultIdGenerator{}
}

// Next 生成下一个消息ID（线程安全）
func (g *DefaultIdGenerator) Next() uint32 {
	return g.counter.Add(1)
}

type IdGeneratorFunc func() uint32

// Next 调用函数生成消息ID
func (f IdGeneratorFunc) Next() uint32 {
	return f()
}
