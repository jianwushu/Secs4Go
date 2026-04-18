package secs4go

import (
	"fmt"
	"time"
)

// ============================================================
// Message 消息定义
// ============================================================

// Message SECS消息
type Message struct {
	Stream      uint8     // 消息流 (1-127)
	Function    uint8     // 消息功能
	WBit        bool      // 等待位(需要回复)
	SystemBytes uint32    // 系统字节(消息跟踪)
	Item        *Item     // 消息体
	RawFrame    []byte    // 原始完整帧数据(4B长度 + 10B头 + 数据)，发送/接收链路必须保证有值
	Timestamp   time.Time // 时间戳
}

// NewMessage 创建新消息
func NewMessage(stream, function uint8) *Message {
	return &Message{
		Stream:    stream,
		Function:  function,
		WBit:      false,
		Timestamp: time.Now(),
	}
}

// WithWBit 设置等待位
func (m *Message) WithWBit(wbit bool) *Message {
	m.WBit = wbit
	return m
}

// WithItem 设置消息体
func (m *Message) WithItem(item *Item) *Message {
	m.Item = item
	return m
}

// WithSystemBytes 设置SystemBytes
func (m *Message) WithSystemBytes(sb uint32) *Message {
	m.SystemBytes = sb
	return m
}

// ============================================================
// 消息格式化 (用于日志)
// ============================================================

// FormatMessage 格式化消息(用于日志)
func FormatMessage(msg *Message) string {
	if msg == nil {
		return "nil"
	}
	return fmt.Sprintf("S%dF%d(W=%v, SysBytes=%d)", msg.Stream, msg.Function, msg.WBit, msg.SystemBytes)
}

func cloneBytes(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	cloned := make([]byte, len(data))
	copy(cloned, data)
	return cloned
}

func (m *Message) applyProtocolSnapshot(header HSMSHeader, itemData []byte, frameData []byte, timestamp time.Time) {
	if m == nil {
		return
	}
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	m.Stream = header.Stream()
	m.WBit = header.WBit()
	m.Function = header.Function()
	m.SystemBytes = header.SystemBytes
	m.RawFrame = cloneBytes(frameData)
	m.Timestamp = timestamp
}

// ParseMessage HSMSHeader + []byte → Message
// 从HSMSHeader提取S/F/WBit，解析Item
func ParseMessage(header HSMSHeader, data []byte, codec *ItemCodec) (*Message, error) {
	msg := &Message{}
	msg.applyProtocolSnapshot(header, data, BuildCompleteFrame(header, data), time.Now())

	// 解析Item数据
	if len(data) > 0 {
		var item *Item
		var err error

		if codec != nil {
			item, _, err = codec.DecodeItem(data)
		} else {
			item, _, err = DecodeItem(data)
		}

		if err != nil {
			return nil, err
		}
		msg.Item = item
	}

	return msg, nil
}
