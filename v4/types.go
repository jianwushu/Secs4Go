package secs4go_v4

import "fmt"

// ============================================================
// 基础类型定义
// ============================================================

// ItemType SECS-II格式码 (6位值)
type ItemType uint8

// SECS-II格式码定义
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

// PType HSMS Presentation Type
type PType uint8

const (
	PTypeSECSII PType = 0x00 // SECS-II消息
)

// SType HSMS Session Type
type SType uint8

const (
	STypeDataMessage SType = 0x00 // 数据消息
	STypeSelectReq   SType = 0x01 // Select.req
	STypeSelectRsp   SType = 0x02 // Select.rsp
	STypeDeselectReq SType = 0x03 // Deselect.req
	STypeDeselectRsp SType = 0x04 // Deselect.rsp
	STypeLinktestReq SType = 0x05 // Linktest.req
	STypeLinktestRsp SType = 0x06 // Linktest.rsp
	STypeRejectReq   SType = 0x07 // Reject.req
	STypeSeparateReq SType = 0x09 // Separate.req
)

func (s SType) String() string {
	switch s {
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
		return fmt.Sprintf("Unknown(0x%02X)", s)
	}
}

// ============================================================
// 连接状态
// ============================================================

// ConnectionState 连接状态
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota // TCP未连接
	StateConnecting                          // 连接中
	StateConnected                           // TCP已连接 (Not Selected)
	StateSelected                            // 已完成Select
	StateReconnecting                        // 重连中
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
	case StateReconnecting:
		return "Reconnecting"
	default:
		return fmt.Sprintf("Unknown(%d)", s)
	}
}

// StateChangeHandler 状态变更事件处理器
type StateChangeHandler func(oldState, newState ConnectionState)
