package core

import (
	"encoding/binary"
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

// ParseMessage HSMSHeader + []byte → Message
// 从HSMSHeader提取S/F/WBit，解析Item
func ParseMessage(header HSMSHeader, data []byte, codec *ItemCodec) (*Message, error) {
	msg := &Message{
		Stream:      header.Stream(),
		WBit:        header.WBit(),
		Function:    header.Function(),
		SystemBytes: header.SystemBytes,
		RawFrame:    BuildCompleteFrame(header, data),
		Timestamp:   time.Now(),
	}

	// 解析Item数据
	if len(data) > 0 {
		var item *Item
		var err error

		item, _, err = codec.DecodeItem(data)

		if err != nil {
			return nil, err
		}
		msg.Item = item
	}

	return msg, nil
}

// BuildCompleteFrame 格式化完整帧数据 (4B长度 + 10B头部 + 数据)
func BuildCompleteFrame(header HSMSHeader, itemData []byte) []byte {
	headerBytes := header.Encode()
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(len(headerBytes)+len(itemData)))
	frameBytes := append(lengthBuf, headerBytes...)
	frameBytes = append(frameBytes, itemData...)
	return frameBytes
}
