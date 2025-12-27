# Secs4Go_V3 架构优化方案

## 1. 核心原则

### 1.1 层次划分
```
┌─────────────────────────────────────────────────────────────────┐
│                         TCP/IP                                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│              HSMS-SS 传输层 (纯粹数据传输)                        │
│  - TCP连接、帧收发、超时管理                                      │
│  - Stream/Function 在 HSMS 头部                                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│              SECS-II 消息层 (GEM数据编解码)                       │
│  - 只处理 Item 类型序列化/反序列化                                │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    业务逻辑层 (用户代码)                          │
└─────────────────────────────────────────────────────────────────┘
```

### 1.2 当前帧结构（代码实际实现）
```
┌─────────────────────────────────────────────────────────────────────────────┐
│  4字节      │                       10字节 HSMS 头部                          │
│  Length     │ SessionID │  S+W  │  F   │ PType │ SType │    SystemBytes      │
│  (4 bytes)  │  (2)      │ (1)   │ (1)  │  (1)  │  (1)  │      (4)            │
└─────────────────────────────────────────────────────────────────────────────┘
     ▲                           ↑           ↑
     │                           │           └── Function (Header Byte 3)
     │                           └───────────── Stream + WBit (Header Byte 2)
     └────────────────────────────────────── 4字节长度前缀 (不含自身)

                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         可变长 SECS-II 数据                                  │
│                         (纯 Item 数据: Format + Length + Data)               │
└─────────────────────────────────────────────────────────────────────────────┘

最小帧: 4 + 10 = 14 字节 (无SECS-II数据时)
```

**关键点：Stream 和 Function 在 HSMS 头部，不在 SECS-II 数据里！**

### 1.3 HSMS头部结构

```go
// HSMSHeader HSMS消息头 (10字节)
type HSMSHeader struct {
    SessionID   uint16  // 字节0-1: 设备ID
    StreamWBit  uint8   // 字节2: Stream + WBit (bit7)
    Function    uint8   // 字节3: Function
    PType       byte    // 字节4: 0x00=SECS-II
    SType       byte    // 字节5: 消息类型
    SystemBytes uint32  // 字节6-9: 事务ID
}
```

---

## 2. 问题定义

### 2.1 当前问题

| 问题 | 位置 | 影响 |
|------|------|------|
| 编码函数过多 | `encode.go` | 复杂、维护难 |
| HSMS层过度封装 | `hsms.go` | 不必要的抽象 |
| Session职责混合 | `session.go` | 难以测试 |

### 2.2 当前代码的问题

```go
// encode.go - 过度复杂
func encodeItem(item *Item) ([]byte, error)      // Item编码
func encodeItemData(item *Item, buf []byte)      // Item数据编码
func calculateItemDataLength(item *Item)         // 长度计算
func calculateEncodedLength(item *Item)          // 编码长度计算
func encodeMessage(msg *Message) ([]byte, error) // 消息编码

// 10+个函数处理"简单"的编码工作
```

---

## 3. 优化方案：只需两套编解码

### 3.1 HSMS帧编解码 (传输层)

**文件**: `hsms_frame.go`

```go
package secs4go

import (
	"encoding/binary"
	"io"
)

// ============================================================
// HSMS帧编解码 - 简单纯粹
// ============================================================

// WriteHSMSFrame 写入HSMS帧
// 格式: [4字节长度][10字节头部][数据...]
// 数据部分只包含 SECS-II Item
func WriteHSMSFrame(writer io.Writer, header HSMSHeader, itemData []byte) error {
	// 计算总长度 (头部10字节 + 数据)
	totalLen := 10 + len(itemData)

	// 写入4字节长度前缀
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(totalLen))
	if _, err := writer.Write(lengthBuf); err != nil {
		return err
	}

	// 写入10字节头部
	headerBuf := make([]byte, 10)
	binary.BigEndian.PutUint16(headerBuf[0:2], header.SessionID)
	headerBuf[2] = header.StreamWBit
	headerBuf[3] = header.Function
	headerBuf[4] = header.PType
	headerBuf[5] = header.SType
	binary.BigEndian.PutUint32(headerBuf[6:10], header.SystemBytes)
	if _, err := writer.Write(headerBuf); err != nil {
		return err
	}

	// 写入数据 (SECS-II Item)
	if len(itemData) > 0 {
		if _, err := writer.Write(itemData); err != nil {
			return err
		}
	}

	return nil
}

// ReadHSMSFrame 读取HSMS帧
// 返回: 头部(10字节), SECS-II数据(Item), 错误
func ReadHSMSFrame(reader io.Reader) (HSMSHeader, []byte, error) {
	var header HSMSHeader

	// 读取4字节长度
	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(reader, lengthBuf); err != nil {
		return header, nil, err
	}

	frameLen := binary.BigEndian.Uint32(lengthBuf)
	if frameLen < 10 {
		return header, nil, ErrInvalidFrame
	}

	// 读取头部 + 数据
	dataLen := int(frameLen) - 10
	frameData := make([]byte, frameLen)
	if _, err := io.ReadFull(reader, frameData); err != nil {
		return header, nil, err
	}

	// 解析头部
	header.SessionID = binary.BigEndian.Uint16(frameData[0:2])
	header.StreamWBit = frameData[2]
	header.Function = frameData[3]
	header.PType = frameData[4]
	header.SType = frameData[5]
	header.SystemBytes = binary.BigEndian.Uint32(frameData[6:10])

	// 提取SECS-II数据 (Item)
	var itemData []byte
	if dataLen > 0 {
		itemData = frameData[10:]
	}

	return header, itemData, nil
}
```

### 3.2 SECS-II Item 编解码 (消息层)

**文件**: `secs_item_codec.go`

```go
package secs4go

import (
	"encoding/binary"
	"fmt"
)

// ============================================================
// SECS-II Item 编解码 - 核心 GEM 数据处理
// ============================================================

// ItemType SECS-II数据类型
type ItemType byte

// SECS-II 类型码
const (
	TypeList    ItemType = 0x00
	TypeBinary  ItemType = 0x20
	TypeASCII   ItemType = 0x40
	TypeBool    ItemType = 0x28
	TypeInt8    ItemType = 0x30
	TypeInt16   ItemType = 0x31
	TypeInt32   ItemType = 0x32
	TypeInt64   ItemType = 0x33
	TypeUInt8   ItemType = 0x50
	TypeUInt16  ItemType = 0x51
	TypeUInt32  ItemType = 0x52
	TypeUInt64  ItemType = 0x53
	TypeFloat32 ItemType = 0x44
	TypeFloat64 ItemType = 0x45
)

// Item SECS-II数据项
type Item struct {
	Type  ItemType
	Value interface{} // []byte, []int32, etc.
}

// EncodeItem 编码Item
// 格式: [1字节格式码][1-3字节长度][数据...]
func EncodeItem(item *Item) ([]byte, error) {
	if item == nil {
		return nil, nil
	}

	// 获取数据字节
	data, err := itemValueToBytes(item.Type, item.Value)
	if err != nil {
		return nil, err
	}

	// 计算长度字段字节数
	var lenBytes int
	dataLen := len(data)

	if dataLen <= 0xFF {
		lenBytes = 1
	} else if dataLen <= 0xFFFF {
		lenBytes = 2
	} else {
		lenBytes = 3
	}

	// 格式字节: 高6位类型, 低2位长度字节数
	formatByte := byte(item.Type) << 2
	if lenBytes == 1 {
		formatByte |= 0x01
	} else if lenBytes == 2 {
		formatByte |= 0x02
	} else {
		formatByte |= 0x03
	}

	// 构建结果
	result := make([]byte, 1+lenBytes+dataLen)
	result[0] = formatByte

	// 写入长度
	if lenBytes == 1 {
		result[1] = byte(dataLen)
	} else if lenBytes == 2 {
		binary.BigEndian.PutUint16(result[1:3], uint16(dataLen))
	} else {
		result[1] = byte(dataLen >> 16)
		result[2] = byte(dataLen >> 8)
		result[3] = byte(dataLen)
	}

	// 复制数据
	copy(result[1+lenBytes:], data)

	return result, nil
}

// DecodeItem 解码Item
// 返回: Item, 消耗的字节数, 错误
func DecodeItem(data []byte) (*Item, int, error) {
	if len(data) < 2 {
		return nil, 0, ErrInvalidItem
	}

	formatByte := data[0]
	itemType := ItemType(formatByte >> 2)
	lengthBytes := int(formatByte & 0x03)

	// 解析长度
	var dataLen int
	headerLen := 1 + lengthBytes

	if lengthBytes == 1 {
		dataLen = int(data[1])
	} else if lengthBytes == 2 {
		dataLen = int(binary.BigEndian.Uint16(data[1:3]))
	} else if lengthBytes == 3 {
		dataLen = int(data[1])<<16 | int(data[2])<<8 | int(data[3])
	}

	// 提取数据
	itemData := data[headerLen : headerLen+dataLen]

	// 解码值
	value, err := itemBytesToValue(itemType, itemData)
	if err != nil {
		return nil, 0, err
	}

	return &Item{Type: itemType, Value: value}, headerLen + dataLen, nil
}

// ============================================================
// 值转换辅助函数
// ============================================================

func itemValueToBytes(t ItemType, v interface{}) ([]byte, error) {
	switch t {
	case TypeList:
		items, ok := v.([]*Item)
		if !ok {
			return nil, ErrInvalidList
		}
		var result []byte
		for _, item := range items {
			b, err := EncodeItem(item)
			if err != nil {
				return nil, err
			}
			result = append(result, b...)
		}
		return result, nil

	case TypeBinary, TypeASCII:
		b, ok := v.([]byte)
		if !ok {
			return nil, ErrInvalidValue
		}
		return b, nil

	case TypeBool:
		b, ok := v.([]bool)
		if !ok {
			return nil, ErrInvalidValue
		}
		result := make([]byte, len(b))
		for i, v := range b {
			if v {
				result[i] = 1
			}
		}
		return result, nil

	case TypeInt8:
		return intSliceToBytes(v, 1), nil
	case TypeInt16:
		return intSliceToBytes(v, 2), nil
	case TypeInt32:
		return intSliceToBytes(v, 4), nil
	case TypeInt64:
		return intSliceToBytes(v, 8), nil

	case TypeUInt8:
		b, ok := v.([]byte)
		if !ok {
			return nil, ErrInvalidValue
		}
		return b, nil
	case TypeUInt16:
		return uintSliceToBytes(v, 2), nil
	case TypeUInt32:
		return uintSliceToBytes(v, 4), nil
	case TypeUInt64:
		return uintSliceToBytes(v, 8), nil

	case TypeFloat32:
		return floatSliceToBytes(v, 32), nil
	case TypeFloat64:
		return floatSliceToBytes(v, 64), nil

	default:
		return nil, ErrUnknownType
	}
}

func itemBytesToValue(t ItemType, data []byte) (interface{}, error) {
	switch t {
	case TypeList:
		items := make([]*Item, 0)
		offset := 0
		for offset < len(data) {
			item, consumed, err := DecodeItem(data[offset:])
			if err != nil {
				break
			}
			items = append(items, item)
			offset += consumed
		}
		return items, nil

	case TypeBinary, TypeASCII, TypeUInt8:
		return data, nil

	case TypeBool:
		result := make([]bool, len(data))
		for i, b := range data {
			result[i] = b != 0
		}
		return result, nil

	case TypeInt8:
		return intSliceFromBytes(data, 1), nil
	case TypeInt16:
		return intSliceFromBytes(data, 2), nil
	case TypeInt32:
		return intSliceFromBytes(data, 4), nil
	case TypeInt64:
		return intSliceFromBytes(data, 8), nil

	case TypeUInt16:
		return uintSliceFromBytes(data, 2), nil
	case TypeUInt32:
		return uintSliceFromBytes(data, 4), nil
	case TypeUInt64:
		return uintSliceFromBytes(data, 8), nil

	case TypeFloat32:
		return floatSliceFromBytes(data, 32), nil
	case TypeFloat64:
		return floatSliceFromBytes(data, 64), nil

	default:
		return nil, ErrUnknownType
	}
}

// 辅助函数实现...
func intSliceToBytes(v interface{}, elemSize int) []byte {
	// ... 实现
	return nil
}
func intSliceFromBytes(data []byte, elemSize int) interface{} {
	// ... 实现
	return nil
}
func uintSliceToBytes(v interface{}, elemSize int) []byte {
	// ... 实现
	return nil
}
func uintSliceFromBytes(data []byte, elemSize int) interface{} {
	// ... 实现
	return nil
}
func floatSliceToBytes(v interface{}, bits int) []byte {
	// ... 实现
	return nil
}
func floatSliceFromBytes(data []byte, bits int) interface{} {
	// ... 实现
	return nil
}
```

---

## 4. 传输层 (HSMSTransport)

**文件**: `hsms_transport.go`

### 4.1 设计原则

- 直接使用 `Config` 中的所有 SEMI 超时参数 (T3-T8)
- 客户端/服务端统一接口
- 职责纯粹：只负责 TCP 连接和帧收发

### 4.2 超时参数映射

| SEMI参数 | 用途 | HSMSTransport使用 |
|----------|------|-------------------|
| T3 | Reply timeout | `Send()` 写入超时 |
| T5 | Connect timeout | `Connect()` 连接超时 |
| T6 | Control transaction | 控制消息超时 (待实现) |
| T7 | Not selected | `WaitReady()` 超时 |
| T8 | Network intercharacter | `Receive()` 读取超时 |

### 4.3 完整实现

```go
package secs4go

import (
	"net"
	"sync"
)

// ============================================================
// HSMS-SS 传输层 - 客户端/服务端统一实现
// ============================================================

// HSMSTransport HSMS传输层
type HSMSTransport struct {
	conn   net.Conn
	config *Config
	mu     sync.RWMutex
	state  TransportState

	// 服务端模式
	listener net.Listener
}

// TransportState 传输状态
type TransportState int

const (
	StateDisconnected TransportState = iota
	StateConnecting
	StateConnected
	StateSelected
)

// NewHSMSTransport 创建传输层
func NewHSMSTransport(config *Config) *HSMSTransport {
	return &HSMSTransport{
		config: config,
		state:  StateDisconnected,
	}
}

// ============================================================
// 客户端方法
// ============================================================

// Connect 客户端: 连接到服务端
func (t *HSMSTransport) Connect(address string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.state == StateConnected {
		return nil
	}

	// 使用 T5 作为连接超时
	conn, err := net.DialTimeout("tcp", address, t.config.T5)
	if err != nil {
		return err
	}

	t.conn = conn
	t.state = StateConnected
	return nil
}

// ============================================================
// 服务端方法
// ============================================================

// Listen 服务端: 监听连接
func (t *HSMSTransport) Listen(address string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	t.listener = listener
	t.state = StateConnected
	return nil
}

// Accept 服务端: 接受连接 (返回新的 HSMSTransport)
func (t *HSMSTransport) Accept() (*HSMSTransport, error) {
	conn, err := t.listener.Accept()
	if err != nil {
		return nil, err
	}

	return &HSMSTransport{
		conn:   conn,
		config: t.config,
		state:  StateConnected,
	}, nil
}

// ============================================================
// 公共方法
// ============================================================

// Close 关闭连接
func (t *HSMSTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.listener != nil {
		t.listener.Close()
		t.listener = nil
	}

	if t.conn != nil {
		err := t.conn.Close()
		t.conn = nil
		t.state = StateDisconnected
		return err
	}
	return nil
}

// Send 发送数据（使用 T3 作为写入超时）
func (t *HSMSTransport) Send(header HSMSHeader, itemData []byte) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.conn == nil {
		return ErrNotConnected
	}

	// T3: Reply timeout 作为发送超时
	t.conn.SetWriteDeadline(time.Now().Add(t.config.T3))
	return WriteHSMSFrame(t.conn, header, itemData)
}

// Receive 接收数据（使用 T8 作为读取超时）
func (t *HSMSTransport) Receive() (HSMSHeader, []byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.conn == nil {
		return HSMSHeader{}, nil, ErrNotConnected
	}

	// T8: Network intercharacter timeout 作为读取超时
	t.conn.SetReadDeadline(time.Now().Add(t.config.T8))
	return ReadHSMSFrame(t.conn)
}

// GetState 获取状态
func (t *HSMSTransport) GetState() TransportState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

// RemoteAddr 获取远端地址
func (t *HSMSTransport) RemoteAddr() net.Addr {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.conn != nil {
		return t.conn.RemoteAddr()
	}
	return nil
}

// LocalAddr 获取本地地址
func (t *HSMSTransport) LocalAddr() net.Addr {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.conn != nil {
		return t.conn.LocalAddr()
	}
	if t.listener != nil {
		return t.listener.Addr()
	}
	return nil
}
```

### 4.4 使用示例

**客户端：**
```go
transport := NewHSMSTransport(config)
transport.Connect("127.0.0.1:5000")
header := BuildDataHeader(deviceID, stream, function, true, systemBytes)
transport.Send(header, itemData)
```

**服务端：**
```go
transport := NewHSMSTransport(config)
transport.Listen(":5000")

for {
    client, _ := transport.Accept()
    go handleClient(client)
}

func handleClient(t *HSMSTransport) {
    header, itemData, _ := t.Receive()
    // 处理消息...
}
```

---

## 5. 消息构建辅助函数

**文件**: `hsms_message.go`

```go
package secs4go

import "time"

// ============================================================
// 消息构建辅助函数
// ============================================================

// BuildDataHeader 构建数据消息的HSMS头部
func BuildDataHeader(deviceID uint16, stream, function uint8, wbit bool, systemBytes uint32) HSMSHeader {
	streamWBit := stream & 0x7F
	if wbit {
		streamWBit |= 0x80
	}
	return HSMSHeader{
		SessionID:   deviceID,
		StreamWBit:  streamWBit,
		Function:    function,
		PType:       0x00, // SECS-II
		SType:       0x01, // Data
		SystemBytes: systemBytes,
	}
}

// BuildSelectReqHeader 构建Select.req头部
func BuildSelectReqHeader(systemBytes uint32) HSMSHeader {
	return HSMSHeader{
		SessionID:   0xFFFF,
		StreamWBit:  0,
		Function:    0,
		PType:       0x00,
		SType:       0x09, // Select.req
		SystemBytes: systemBytes,
	}
}

// ParseHeader 从HSMS头部解析S/F
func ParseHeader(h HSMSHeader) (stream, function uint8, wbit bool) {
	stream = h.StreamWBit & 0x7F
	wbit = (h.StreamWBit & 0x80) != 0
	function = h.Function
	return
}
```

---

## 6. 文件结构

优化后的项目结构：

```
secs4go/
├── secs4go.go              # API入口 (3行启动客户端/服务器)
├── hsms_transport.go       # HSMS传输层 (客户端/服务端 + 帧收发 + T3-T8超时)
├── hsms_frame.go           # HSMS帧读写 (4字节长度 + 10字节头)
├── hsms_message.go         # 消息头部构建/解析
├── secs_item_codec.go      # SECS-II Item编解码 (核心)
├── item.go                 # Item工厂函数
├── types.go                # 类型定义
├── config.go               # 配置 (T3-T8超时参数)
├── logger.go               # 日志
└── utils.go                # 工具函数
```

---

## 7. 对比

| 项目 | 优化前 | 优化后 |
|------|--------|--------|
| HSMS编解码 | `encodeHSMSPacket` + `decodeHSMSPacket` | `WriteHSMSFrame` + `ReadHSMSFrame` |
| Item编解码 | 8+ 函数 | `EncodeItem` + `DecodeItem` |
| 消息头处理 | 分散在session.go | `BuildDataHeader` + `ParseHeader` |
| 超时参数 | 部分使用 T3/T5/T7/T8，**T6未使用** | 全部使用 `*Config` |
| 客户端/服务端 | 混合在Session | 统一 `HSMSTransport` |
| 总函数数 | 20+ | 10 |
| 职责清晰度 | 混合 | 分离 |

---

## 8. 错误定义

```go
var (
	ErrNotConnected   = errors.New("not connected")
	ErrInvalidFrame   = errors.New("invalid HSMS frame")
	ErrInvalidItem    = errors.New("invalid SECS item")
	ErrInvalidList    = errors.New("invalid list value")
	ErrInvalidValue   = errors.New("invalid value type")
	ErrUnknownType    = errors.New("unknown item type")
)
```

---

## 9. 使用示例

```go
// 发送消息
header := BuildDataHeader(deviceID, stream, function, true, systemBytes)
itemData, _ := EncodeItem(item)
transport.Send(header, itemData)

// 接收消息
header, itemData, _ := transport.Receive()
stream, function, wbit := ParseHeader(header)
item, _ := DecodeItem(itemData)
```

---

## 10. 迁移步骤

1. **创建新文件**: `hsms_frame.go` + `hsms_message.go` + `secs_item_codec.go`
2. **重写HSMSTransport**: 添加客户端/服务端方法，使用完整 `*Config`
3. **移植功能**: 将现有编解码逻辑迁移到新文件
4. **简化Session**: 使用新的传输层和编解码器
5. **测试验证**: 确保功能不变
6. **删除旧文件**: `hsms.go` + `encode.go`
