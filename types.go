package secs4go

import (
	"fmt"
	"time"
)

// ============================================================
// Message Types
// ============================================================

// Message SECS消息
type Message struct {
	Stream      uint8     // 消息流 (1-127)
	Function    uint8     // 消息功能
	WBit        bool      // 等待位(需要回复)
	SystemBytes uint32    // 系统字节(消息跟踪)
	Item        *Item     // 消息体
	Timestamp   time.Time // 时间戳
	session     *Session  // 所属会话(用于回复)
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

// Reply 回复消息
func (m *Message) Reply(reply *Message) error {
	if m.session == nil {
		return fmt.Errorf("message has no session")
	}
	return m.session.Reply(m, reply)
}

// ============================================================
// Item Types
// ============================================================

// ItemType SECS-II格式码(6位值)
type ItemType uint8

// SECS-II格式码
const (
	TypeList    ItemType = 0o00 // 000000 - List
	TypeBinary  ItemType = 0o10 // 001000 - Binary
	TypeBoolean ItemType = 0o11 // 001001 - Boolean
	TypeASCII   ItemType = 0o20 // 010000 - ASCII
	TypeJIS8    ItemType = 0o21 // 010001 - JIS-8
	TypeInt64   ItemType = 0o30 // 011000 - 8-byte signed integer
	TypeInt8    ItemType = 0o31 // 011001 - 1-byte signed integer
	TypeInt16   ItemType = 0o32 // 011010 - 2-byte signed integer
	TypeInt32   ItemType = 0o34 // 011100 - 4-byte signed integer
	TypeFloat64 ItemType = 0o40 // 100000 - 8-byte floating point
	TypeFloat32 ItemType = 0o44 // 100100 - 4-byte floating point
	TypeUInt64  ItemType = 0o50 // 101000 - 8-byte unsigned integer
	TypeUInt8   ItemType = 0o51 // 101001 - 1-byte unsigned integer
	TypeUInt16  ItemType = 0o52 // 101010 - 2-byte unsigned integer
	TypeUInt32  ItemType = 0o54 // 101100 - 4-byte unsigned integer
	TypeUnknown ItemType = 0o77 // Unknown
)

// Item SECS数据项
type Item struct {
	Type  ItemType
	Value interface{}
	// Value可以是:
	//   - []*Item for LIST
	//   - []byte for BINARY, BOOLEAN, ASCII, JIS8
	//   - []int8, []int16, []int32, []int64 for signed integers
	//   - []uint8, []uint16, []uint32, []uint64 for unsigned integers
	//   - []float32, []float64 for floating point numbers
}

// ============================================================
// HSMS Types
// ============================================================

// PType HSMS PType
type PType uint8

const (
	PTypeSECSII PType = 0 // SECS-II消息
)

// SType HSMS SType
type SType uint8

const (
	STypeDataMessage SType = 0 // 数据消息
	STypeSelectReq   SType = 1 // Select.req
	STypeSelectRsp   SType = 2 // Select.rsp
	STypeDeselectReq SType = 3 // Deselect.req
	STypeDeselectRsp SType = 4 // Deselect.rsp
	STypeLinktestReq SType = 5 // Linktest.req
	STypeLinktestRsp SType = 6 // Linktest.rsp
	STypeRejectReq   SType = 7 // Reject.req
	STypeSeparateReq SType = 9 // Separate.req
)

func (sType SType) String() string {
	switch sType {
	case STypeDataMessage:
		return "DataMessage"
	case STypeSelectReq:
		return "Select.req"
	case STypeSelectRsp:
		return "Select.rsp"
	case STypeDeselectReq:
		return "Deselect.req"
	case STypeDeselectRsp:
		return "Deselect.rsp"
	case STypeLinktestReq:
		return "Linktest.req"
	case STypeLinktestRsp:
		return "Linktest.rsp"
	case STypeRejectReq:
		return "Reject.req"
	case STypeSeparateReq:
		return "Separate.req"
	default:
		return fmt.Sprintf("Unknown(%d)", sType)
	}
}

// StateChangeHandler 状态变更事件处理器
type StateChangeHandler func(oldState, newState ConnectionState)

// ConnectionState 连接状态
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateSelected
)

func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "Disconnected"
	case StateConnecting:
		return "Connecting"
	case StateConnected:
		return "Connected"
	case StateSelected:
		return "Selected"
	default:
		return fmt.Sprintf("Unknown(%d)", s)
	}
}
