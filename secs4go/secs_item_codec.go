package secs4go

import (
	"encoding/binary"
	"errors"
	"math"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// ============================================================
// 错误定义
// ============================================================

var (
	ErrInvalidItem  = errors.New("invalid SECS item")
	ErrInvalidList  = errors.New("invalid list value")
	ErrInvalidValue = errors.New("invalid value type")
	ErrUnknownType  = errors.New("unknown item type")
)

// ============================================================
// ItemCodec 编解码器
// ============================================================

// ItemCodec 负责 Item 的编解码，支持配置字符串编码
type ItemCodec struct {
	encoder *encoding.Encoder
	decoder *encoding.Decoder
}

// NewItemCodec 创建新的编解码器
func NewItemCodec(encodingName string) (*ItemCodec, error) {
	var enc encoding.Encoding

	switch strings.ToUpper(encodingName) {
	case "GBK":
		enc = simplifiedchinese.GBK
	case "GB2312":
		enc = simplifiedchinese.GB18030 // GB18030 兼容 GB2312
	case "UTF-8", "UTF8":
		enc = nil // UTF-8 是默认值，不需要转换
	case "ASCII":
		enc = nil // ASCII 也是默认处理（直接转换）
	default:
		// 默认为 ASCII/UTF-8 (不转换)
		enc = nil
	}

	codec := &ItemCodec{}
	if enc != nil {
		codec.encoder = enc.NewEncoder()
		codec.decoder = enc.NewDecoder()
	}

	return codec, nil
}

// DefaultItemCodec 默认编解码器 (UTF-8/ASCII)
var DefaultItemCodec, _ = NewItemCodec("ASCII")

// EncodeItem 编码Item
func (c *ItemCodec) EncodeItem(item *Item) ([]byte, error) {
	if item == nil {
		return nil, nil
	}

	// 特殊处理 List
	if item.Type == TypeList {
		return c.encodeListItem(item.Value)
	}

	// 其他类型
	data, err := c.itemValueToBytes(item.Type, item.Value)
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

	// 格式字节
	formatByte := byte(item.Type) << 2
	switch lenBytes {
	case 1:
		formatByte |= 0x01
	case 2:
		formatByte |= 0x02
	default:
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

// encodeListItem 编码List Item
func (c *ItemCodec) encodeListItem(value interface{}) ([]byte, error) {
	items, ok := value.([]*Item)
	if !ok {
		return nil, ErrInvalidList
	}

	count := len(items)
	var lenBytes int
	if count <= 0xFF {
		lenBytes = 1
	} else if count <= 0xFFFF {
		lenBytes = 2
	} else {
		lenBytes = 3
	}

	formatByte := byte(TypeList) << 2
	if lenBytes == 1 {
		formatByte |= 0x01
	} else if lenBytes == 2 {
		formatByte |= 0x02
	} else {
		formatByte |= 0x03
	}

	var itemData []byte
	for _, item := range items {
		data, err := c.EncodeItem(item)
		if err != nil {
			return nil, err
		}
		itemData = append(itemData, data...)
	}

	result := make([]byte, 1+lenBytes+len(itemData))
	result[0] = formatByte

	if lenBytes == 1 {
		result[1] = byte(count)
	} else if lenBytes == 2 {
		binary.BigEndian.PutUint16(result[1:3], uint16(count))
	} else {
		result[1] = byte(count >> 16)
		result[2] = byte(count >> 8)
		result[3] = byte(count)
	}

	copy(result[1+lenBytes:], itemData)

	return result, nil
}

// itemValueToBytes 将Item值转换为字节数组
func (c *ItemCodec) itemValueToBytes(itemType ItemType, value interface{}) ([]byte, error) {
	switch itemType {
	case TypeBinary, TypeJIS8:
		return encodeBinary(value)
	case TypeASCII:
		return c.encodeString(value) // 使用配置的编码
	case TypeBoolean:
		return encodeBoolean(value)
	case TypeInt8:
		return encodeInt8(value)
	case TypeInt16:
		return encodeInt16(value)
	case TypeInt32:
		return encodeInt32(value)
	case TypeInt64:
		return encodeInt64(value)
	case TypeUInt8:
		return encodeUInt8(value)
	case TypeUInt16:
		return encodeUInt16(value)
	case TypeUInt32:
		return encodeUInt32(value)
	case TypeUInt64:
		return encodeUInt64(value)
	case TypeFloat32:
		return encodeFloat32(value)
	case TypeFloat64:
		return encodeFloat64(value)
	default:
		return nil, ErrUnknownType
	}
}

// DecodeItem 解码Item
func (c *ItemCodec) DecodeItem(data []byte) (*Item, int, error) {
	if len(data) < 2 {
		return nil, 0, ErrInvalidItem
	}

	formatByte := data[0]
	itemType := ItemType(formatByte >> 2)
	lengthBytes := int(formatByte & 0x03)

	var length int
	headerLen := 1 + lengthBytes

	if lengthBytes == 1 {
		length = int(data[1])
	} else if lengthBytes == 2 {
		length = int(binary.BigEndian.Uint16(data[1:3]))
	} else if lengthBytes == 3 {
		length = int(data[1])<<16 | int(data[2])<<8 | int(data[3])
	}

	if itemType == TypeList {
		return c.decodeListItem(data, headerLen, length)
	}

	if len(data) < headerLen+length {
		return nil, 0, ErrInvalidItem
	}

	itemData := data[headerLen : headerLen+length]

	value, err := c.itemBytesToValue(itemType, itemData)
	if err != nil {
		return nil, 0, err
	}

	return &Item{Type: itemType, Value: value}, headerLen + length, nil
}

// decodeListItem 解码List Item
func (c *ItemCodec) decodeListItem(data []byte, headerLen, itemCount int) (*Item, int, error) {
	items := make([]*Item, 0)
	offset := headerLen

	for i := 0; i < itemCount; i++ {
		if offset >= len(data) {
			return nil, 0, ErrInvalidItem
		}
		item, consumed, err := c.DecodeItem(data[offset:])
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
		offset += consumed
	}

	return &Item{Type: TypeList, Value: items}, offset, nil
}

// itemBytesToValue 将字节数组转换为Item值
func (c *ItemCodec) itemBytesToValue(itemType ItemType, data []byte) (interface{}, error) {
	switch itemType {
	case TypeList:
		return nil, nil
	case TypeBinary, TypeJIS8:
		return data, nil
	case TypeASCII:
		return c.decodeString(data) // 使用配置的编码
	case TypeBoolean:
		return decodeBoolean(data)
	case TypeInt8:
		return decodeInt8(data)
	case TypeInt16:
		return decodeInt16(data)
	case TypeInt32:
		return decodeInt32(data)
	case TypeInt64:
		return decodeInt64(data)
	case TypeUInt8:
		return decodeUInt8(data)
	case TypeUInt16:
		return decodeUInt16(data)
	case TypeUInt32:
		return decodeUInt32(data)
	case TypeUInt64:
		return decodeUInt64(data)
	case TypeFloat32:
		return decodeFloat32(data)
	case TypeFloat64:
		return decodeFloat64(data)
	default:
		return nil, ErrUnknownType
	}
}

// ============================================================
// 字符串编码/解码 (支持配置)
// ============================================================

func (c *ItemCodec) encodeString(value interface{}) ([]byte, error) {
	str, ok := value.(string)
	if !ok {
		// 尝试转换 []byte
		if b, ok := value.([]byte); ok {
			str = string(b)
		} else {
			return nil, ErrInvalidValue
		}
	}

	if c.encoder == nil {
		return []byte(str), nil
	}

	data, _, err := transform.Bytes(c.encoder, []byte(str))
	return data, err
}

func (c *ItemCodec) decodeString(data []byte) (interface{}, error) {
	if c.decoder == nil {
		return data, nil // 保持原始字节，由上层转换为string
	}

	decoded, _, err := transform.Bytes(c.decoder, data)
	if err != nil {
		return data, nil // 解码失败返回原始数据
	}
	return decoded, nil
}

// ============================================================
// 兼容性接口 (使用默认编解码器)
// ============================================================

// EncodeItem 编码Item (使用默认UTF-8/ASCII)
func EncodeItem(item *Item) ([]byte, error) {
	return DefaultItemCodec.EncodeItem(item)
}

// DecodeItem 解码Item (使用默认UTF-8/ASCII)
func DecodeItem(data []byte) (*Item, int, error) {
	return DefaultItemCodec.DecodeItem(data)
}

// ============================================================
// Binary/ASCII/JIS8 编码/解码 (基础实现)
// ============================================================

func encodeBinary(value interface{}) ([]byte, error) {
	switch v := value.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	default:
		return nil, ErrInvalidValue
	}
}

// ============================================================
// Boolean 编码/解码
// ============================================================

func encodeBoolean(value interface{}) ([]byte, error) {
	bools, ok := value.([]bool)
	if !ok {
		return nil, ErrInvalidValue
	}

	// 每字节存储8个布尔值
	result := make([]byte, (len(bools)+7)/8)
	for i, b := range bools {
		if b {
			result[i/8] |= 1 << (i % 8)
		}
	}
	return result, nil
}

func decodeBoolean(data []byte) ([]bool, error) {
	bools := make([]bool, len(data)*8)
	for i, b := range data {
		for j := 0; j < 8 && i*8+j < len(bools); j++ {
			bools[i*8+j] = (b & (1 << j)) != 0
		}
	}
	return bools, nil
}

// ============================================================
// Int8 编码/解码
// ============================================================

func encodeInt8(value interface{}) ([]byte, error) {
	vals, ok := value.([]int8)
	if !ok {
		return nil, ErrInvalidValue
	}
	result := make([]byte, len(vals))
	for i, v := range vals {
		result[i] = byte(v)
	}
	return result, nil
}

func decodeInt8(data []byte) ([]int8, error) {
	vals := make([]int8, len(data))
	for i, b := range data {
		vals[i] = int8(b)
	}
	return vals, nil
}

// ============================================================
// Int16 编码/解码
// ============================================================

func encodeInt16(value interface{}) ([]byte, error) {
	vals, ok := value.([]int16)
	if !ok {
		return nil, ErrInvalidValue
	}
	result := make([]byte, len(vals)*2)
	for i, v := range vals {
		binary.BigEndian.PutUint16(result[i*2:], uint16(v))
	}
	return result, nil
}

func decodeInt16(data []byte) ([]int16, error) {
	vals := make([]int16, len(data)/2)
	for i := 0; i < len(vals); i++ {
		vals[i] = int16(binary.BigEndian.Uint16(data[i*2:]))
	}
	return vals, nil
}

// ============================================================
// Int32 编码/解码
// ============================================================

func encodeInt32(value interface{}) ([]byte, error) {
	vals, ok := value.([]int32)
	if !ok {
		return nil, ErrInvalidValue
	}
	result := make([]byte, len(vals)*4)
	for i, v := range vals {
		binary.BigEndian.PutUint32(result[i*4:], uint32(v))
	}
	return result, nil
}

func decodeInt32(data []byte) ([]int32, error) {
	vals := make([]int32, len(data)/4)
	for i := 0; i < len(vals); i++ {
		vals[i] = int32(binary.BigEndian.Uint32(data[i*4:]))
	}
	return vals, nil
}

// ============================================================
// Int64 编码/解码
// ============================================================

func encodeInt64(value interface{}) ([]byte, error) {
	vals, ok := value.([]int64)
	if !ok {
		return nil, ErrInvalidValue
	}
	result := make([]byte, len(vals)*8)
	for i, v := range vals {
		binary.BigEndian.PutUint64(result[i*8:], uint64(v))
	}
	return result, nil
}

func decodeInt64(data []byte) ([]int64, error) {
	vals := make([]int64, len(data)/8)
	for i := 0; i < len(vals); i++ {
		vals[i] = int64(binary.BigEndian.Uint64(data[i*8:]))
	}
	return vals, nil
}

// ============================================================
// UInt8 编码/解码
// ============================================================

func encodeUInt8(value interface{}) ([]byte, error) {
	switch v := value.(type) {
	case []uint8:
		return v, nil
	default:
		return nil, ErrInvalidValue
	}
}

func decodeUInt8(data []byte) ([]uint8, error) {
	return data, nil
}

// ============================================================
// UInt16 编码/解码
// ============================================================

func encodeUInt16(value interface{}) ([]byte, error) {
	vals, ok := value.([]uint16)
	if !ok {
		return nil, ErrInvalidValue
	}
	result := make([]byte, len(vals)*2)
	for i, v := range vals {
		binary.BigEndian.PutUint16(result[i*2:], v)
	}
	return result, nil
}

func decodeUInt16(data []byte) ([]uint16, error) {
	vals := make([]uint16, len(data)/2)
	for i := 0; i < len(vals); i++ {
		vals[i] = binary.BigEndian.Uint16(data[i*2:])
	}
	return vals, nil
}

// ============================================================
// UInt32 编码/解码
// ============================================================

func encodeUInt32(value interface{}) ([]byte, error) {
	vals, ok := value.([]uint32)
	if !ok {
		return nil, ErrInvalidValue
	}
	result := make([]byte, len(vals)*4)
	for i, v := range vals {
		binary.BigEndian.PutUint32(result[i*4:], v)
	}
	return result, nil
}

func decodeUInt32(data []byte) ([]uint32, error) {
	vals := make([]uint32, len(data)/4)
	for i := 0; i < len(vals); i++ {
		vals[i] = binary.BigEndian.Uint32(data[i*4:])
	}
	return vals, nil
}

// ============================================================
// UInt64 编码/解码
// ============================================================

func encodeUInt64(value interface{}) ([]byte, error) {
	vals, ok := value.([]uint64)
	if !ok {
		return nil, ErrInvalidValue
	}
	result := make([]byte, len(vals)*8)
	for i, v := range vals {
		binary.BigEndian.PutUint64(result[i*8:], v)
	}
	return result, nil
}

func decodeUInt64(data []byte) ([]uint64, error) {
	vals := make([]uint64, len(data)/8)
	for i := 0; i < len(vals); i++ {
		vals[i] = binary.BigEndian.Uint64(data[i*8:])
	}
	return vals, nil
}

// ============================================================
// Float32 编码/解码
// ============================================================

func encodeFloat32(value interface{}) ([]byte, error) {
	vals, ok := value.([]float32)
	if !ok {
		return nil, ErrInvalidValue
	}
	result := make([]byte, len(vals)*4)
	for i, v := range vals {
		binary.BigEndian.PutUint32(result[i*4:], math.Float32bits(v))
	}
	return result, nil
}

func decodeFloat32(data []byte) ([]float32, error) {
	vals := make([]float32, len(data)/4)
	for i := 0; i < len(vals); i++ {
		vals[i] = math.Float32frombits(binary.BigEndian.Uint32(data[i*4:]))
	}
	return vals, nil
}

// ============================================================
// Float64 编码/解码
// ============================================================

func encodeFloat64(value interface{}) ([]byte, error) {
	vals, ok := value.([]float64)
	if !ok {
		return nil, ErrInvalidValue
	}
	result := make([]byte, len(vals)*8)
	for i, v := range vals {
		binary.BigEndian.PutUint64(result[i*8:], math.Float64bits(v))
	}
	return result, nil
}

func decodeFloat64(data []byte) ([]float64, error) {
	vals := make([]float64, len(data)/8)
	for i := 0; i < len(vals); i++ {
		vals[i] = math.Float64frombits(binary.BigEndian.Uint64(data[i*8:]))
	}
	return vals, nil
}
