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
	Stream      uint8          // 消息流 (1-127)
	Function    uint8          // 消息功能
	WBit        bool           // 等待位(需要回复)
	SystemBytes uint32         // 系统字节(消息跟踪)
	Item        *Item          // 消息体
	Timestamp   time.Time      // 时间戳
	sender      *HSMSTransport // 发送方 transport (用于回复)
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

// ============================================================
// Message <-> HSMSHeader 转换
// ============================================================

// BuildHSMSHeader Message → HSMSHeader
// 将Message的S/F/WBit提取到HSMSHeader
func BuildHSMSHeader(deviceID uint16, msg *Message, sType SType, systemBytes uint32) HSMSHeader {
	headerByte2 := msg.Stream & 0x7F
	if msg.WBit {
		headerByte2 |= 0x80
	}

	return HSMSHeader{
		SessionID:   deviceID,
		HeaderByte2: headerByte2,
		HeaderByte3: msg.Function,
		PType:       PTypeSECSII,
		SType:       sType,
		SystemBytes: systemBytes,
	}
}

// ParseMessage HSMSHeader + []byte → Message
// 从HSMSHeader提取S/F/WBit，解析Item
func ParseMessage(header HSMSHeader, data []byte, sender *HSMSTransport) (*Message, error) {
	msg := &Message{
		Stream:      header.Stream(),
		WBit:        header.WBit(),
		Function:    header.Function(),
		SystemBytes: header.SystemBytes,
		Timestamp:   time.Now(),
		sender:      sender,
	}

	// 解析Item数据
	if len(data) > 0 {
		item, _, err := DecodeItem(data)
		if err != nil {
			return nil, err
		}
		msg.Item = item
	}

	return msg, nil
}
