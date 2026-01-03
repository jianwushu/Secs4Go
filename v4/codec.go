package secs4go_v4

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

// ============================================================
// 错误定义
// ============================================================

var ErrInvalidFrame = errors.New("invalid HSMS frame")

// ============================================================
// 字节序转换工具
// ============================================================

// EncodeUint32 uint32 转大端字节数组
func EncodeUint32(v uint32) []byte {
	b := make([]byte, 4)
	b[0] = byte(v >> 24)
	b[1] = byte(v >> 16)
	b[2] = byte(v >> 8)
	b[3] = byte(v)
	return b
}

// EncodeUint16 uint16 转大端字节数组
func EncodeUint16(v uint16) []byte {
	b := make([]byte, 2)
	b[0] = byte(v >> 8)
	b[1] = byte(v)
	return b
}

// DecodeUint32 大端字节数组转 uint32
func DecodeUint32(b []byte) uint32 {
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// DecodeUint16 大端字节数组转 uint16
func DecodeUint16(b []byte) uint16 {
	return uint16(b[0])<<8 | uint16(b[1])
}

// ReadHSMSFrame 读取HSMS帧
// 返回: 头部(10字节), SECS-II数据(Item), 错误
func ReadHSMSFrame(reader io.Reader) (HSMSHeader, []byte, error) {
	// 读取4字节长度
	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(reader, lengthBuf); err != nil {
		return HSMSHeader{}, nil, err
	}

	frameLen := DecodeUint32(lengthBuf)
	if frameLen < HSMSHeaderLength {
		return HSMSHeader{}, nil, ErrInvalidFrame
	}

	// 读取头部 + 数据
	dataLen := int(frameLen) - HSMSHeaderLength
	frameData := make([]byte, frameLen)
	if _, err := io.ReadFull(reader, frameData); err != nil {
		return HSMSHeader{}, nil, err
	}

	// 解析头部
	header := DecodeHeader(frameData[:HSMSHeaderLength])

	// 提取SECS-II数据 (Item)
	var itemData []byte
	if dataLen > 0 {
		itemData = frameData[HSMSHeaderLength:]
	}

	return header, itemData, nil
}

// ============================================================
// 格式化工具
// ============================================================

// FormatHexData 格式化16进制数据(每个字节用空格隔开)
func FormatHexData(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	hex := make([]string, len(data))
	for i, b := range data {
		hex[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(hex, " ")
}

// BuildCompleteFrame 格式化完整帧数据 (4B长度 + 10B头部 + 数据)
func BuildCompleteFrame(header HSMSHeader, itemData []byte) []byte {
	headerBytes := header.Encode()
	frameBytes := append(EncodeUint32(uint32(len(headerBytes)+len(itemData))), headerBytes...)
	frameBytes = append(frameBytes, itemData...)
	return frameBytes
}

// ============================================================
// SML 格式化
// ============================================================

// formatSMLWithIndent 格式化Item为SML格式 (带缩进)
func formatSMLWithIndent(item *Item, indent int) string {
	if item == nil {
		return "."
	}

	indentStr := strings.Repeat("  ", indent)

	switch item.Type {
	case TypeList:
		children, ok := item.Value.([]*Item)
		if !ok {
			return "<invalid list>"
		}
		if len(children) == 0 {
			return fmt.Sprintf("%s<L[0]>", indentStr)
		}

		childParts := make([]string, len(children))
		for i, child := range children {
			childParts[i] = formatSMLWithIndent(child, indent+1)
		}
		return fmt.Sprintf("%s<L[%d]\n%s\n%s>", indentStr, len(children), strings.Join(childParts, "\n"), indentStr)

	case TypeBinary, TypeBoolean:
		data, ok := item.Value.([]byte)
		if !ok {
			return fmt.Sprintf("%s<invalid binary>", indentStr)
		}
		hex := make([]string, len(data))
		for i, b := range data {
			hex[i] = fmt.Sprintf("0x%02X", b)
		}
		return fmt.Sprintf("%s<B[%d] %s>", indentStr, len(data), strings.Join(hex, " "))

	case TypeASCII:
		data, ok := item.Value.([]byte)
		if !ok {
			return fmt.Sprintf("%s<invalid ascii>", indentStr)
		}
		return fmt.Sprintf("%s<A[%d] \"%s\">", indentStr, len(data), string(data))

	case TypeJIS8:
		data, ok := item.Value.([]byte)
		if !ok {
			return fmt.Sprintf("%s<invalid jis8>", indentStr)
		}
		return fmt.Sprintf("%s<J[%d] \"%s\">", indentStr, len(data), string(data))

	case TypeInt8:
		values, ok := item.Value.([]int8)
		if !ok {
			return fmt.Sprintf("%s<invalid i1>", indentStr)
		}
		strs := make([]string, len(values))
		for i, v := range values {
			strs[i] = fmt.Sprintf("%d", v)
		}
		return fmt.Sprintf("%s<I1[%d] %s>", indentStr, len(values), strings.Join(strs, " "))

	case TypeInt16:
		values, ok := item.Value.([]int16)
		if !ok {
			return fmt.Sprintf("%s<invalid i2>", indentStr)
		}
		strs := make([]string, len(values))
		for i, v := range values {
			strs[i] = fmt.Sprintf("%d", v)
		}
		return fmt.Sprintf("%s<I2[%d] %s>", indentStr, len(values), strings.Join(strs, " "))

	case TypeInt32:
		values, ok := item.Value.([]int32)
		if !ok {
			return fmt.Sprintf("%s<invalid i4>", indentStr)
		}
		strs := make([]string, len(values))
		for i, v := range values {
			strs[i] = fmt.Sprintf("%d", v)
		}
		return fmt.Sprintf("%s<I4[%d] %s>", indentStr, len(values), strings.Join(strs, " "))

	case TypeInt64:
		values, ok := item.Value.([]int64)
		if !ok {
			return fmt.Sprintf("%s<invalid i8>", indentStr)
		}
		strs := make([]string, len(values))
		for i, v := range values {
			strs[i] = fmt.Sprintf("%d", v)
		}
		return fmt.Sprintf("%s<I8[%d] %s>", indentStr, len(values), strings.Join(strs, " "))

	case TypeUInt8:
		values, ok := item.Value.([]byte)
		if !ok {
			return fmt.Sprintf("%s<invalid u1>", indentStr)
		}
		strs := make([]string, len(values))
		for i, v := range values {
			strs[i] = fmt.Sprintf("%d", v)
		}
		return fmt.Sprintf("%s<U1[%d] %s>", indentStr, len(values), strings.Join(strs, " "))

	case TypeUInt16:
		values, ok := item.Value.([]uint16)
		if !ok {
			return fmt.Sprintf("%s<invalid u2>", indentStr)
		}
		strs := make([]string, len(values))
		for i, v := range values {
			strs[i] = fmt.Sprintf("%d", v)
		}
		return fmt.Sprintf("%s<U2[%d] %s>", indentStr, len(values), strings.Join(strs, " "))

	case TypeUInt32:
		values, ok := item.Value.([]uint32)
		if !ok {
			return fmt.Sprintf("%s<invalid u4>", indentStr)
		}
		strs := make([]string, len(values))
		for i, v := range values {
			strs[i] = fmt.Sprintf("%d", v)
		}
		return fmt.Sprintf("%s<U4[%d] %s>", indentStr, len(values), strings.Join(strs, " "))

	case TypeUInt64:
		values, ok := item.Value.([]uint64)
		if !ok {
			return fmt.Sprintf("%s<invalid u8>", indentStr)
		}
		strs := make([]string, len(values))
		for i, v := range values {
			strs[i] = fmt.Sprintf("%d", v)
		}
		return fmt.Sprintf("%s<U8[%d] %s>", indentStr, len(values), strings.Join(strs, " "))

	case TypeFloat32:
		values, ok := item.Value.([]float32)
		if !ok {
			return fmt.Sprintf("%s<invalid f4>", indentStr)
		}
		strs := make([]string, len(values))
		for i, v := range values {
			strs[i] = fmt.Sprintf("%g", v)
		}
		return fmt.Sprintf("%s<F4[%d] %s>", indentStr, len(values), strings.Join(strs, " "))

	case TypeFloat64:
		values, ok := item.Value.([]float64)
		if !ok {
			return fmt.Sprintf("%s<invalid f8>", indentStr)
		}
		strs := make([]string, len(values))
		for i, v := range values {
			strs[i] = fmt.Sprintf("%g", v)
		}
		return fmt.Sprintf("%s<F8[%d] %s>", indentStr, len(values), strings.Join(strs, " "))

	default:
		return fmt.Sprintf("%s<Unknown type %d>", indentStr, item.Type)
	}
}

// FormatSML 格式化Item为SML格式
func FormatSML(item *Item) string {
	return formatSMLWithIndent(item, 0)
}
