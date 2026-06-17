package sml

import (
	"fmt"
	"strings"

	"github.com/jianwushu/secs4go/core"
	secs4go "github.com/jianwushu/secs4go/core"
)

// ToSML 将 SECS 消息转换为 SML 格式字符串
func ToSML(msg *secs4go.Message) string {
	msg1 := fmt.Sprintf("S%dF%d(W=%v, SysBytes=%d)", msg.Stream, msg.Function, msg.WBit, msg.SystemBytes)
	if msg.Item == nil {
		return msg1 + "\n."
	}
	msg2 := formatItem(msg.Item, 0)
	return fmt.Sprintf("%s\n%s\n.", msg1, msg2)
}

// ToSMLWithHex 将 SECS 消息转换为 SML 格式字符串，并包含原始帧的16进制表示
func ToSMLWithHex(msg *secs4go.Message) string {
	msg1 := fmt.Sprintf("S%dF%d(W=%v, SysBytes=%d)", msg.Stream, msg.Function, msg.WBit, msg.SystemBytes)
	msg2 := fmt.Sprintf("%s", formatHexData(msg.RawFrame))
	if msg.Item == nil {
		return fmt.Sprintf("%s\n%s\n.", msg1, msg2)
	}
	msg3 := formatItem(msg.Item, 0)
	return fmt.Sprintf("%s\n%s\n%s\n.", msg1, msg2, msg3)
}

func formatHexData(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	hex := make([]string, len(data))
	for i, b := range data {
		hex[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(hex, " ")
}

// formatNumericSlice 将数值切片格式化为 SML 标签字符串，泛型统一处理所有数值类型。
func formatNumericSlice[T ~int8 | ~int16 | ~int32 | ~int64 | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~float32 | ~float64](
	indent, tag string, values []T, fmtStr string,
) string {
	strs := make([]string, len(values))
	for i, v := range values {
		strs[i] = fmt.Sprintf(fmtStr, v)
	}
	return fmt.Sprintf("%s<%s[%d] %s>", indent, tag, len(values), strings.Join(strs, " "))
}

func formatItem(item *secs4go.Item, indent int) string {
	if item == nil {
		return "."
	}

	indentStr := strings.Repeat("  ", indent)

	switch item.Type {
	case core.TypeList:
		children, ok := item.Value.([]*secs4go.Item)
		if !ok || len(children) == 0 {
			return fmt.Sprintf("%s<L[0]>", indentStr)
		}
		childParts := make([]string, len(children))
		for i, child := range children {
			childParts[i] = formatItem(child, indent+1)
		}
		return fmt.Sprintf("%s<L[%d]\n%s\n%s>", indentStr, len(children), strings.Join(childParts, "\n"), indentStr)
	case core.TypeBinary:
		return fmt.Sprintf("%s<B[%d] %s>", indentStr, len(item.Value.([]byte)), formatHexData(item.Value.([]byte)))
	case core.TypeBoolean:
		bools, ok := item.Value.([]bool)
		if !ok || len(bools) == 0 {
			return fmt.Sprintf("%s<BOOLEAN[0]>", indentStr)
		}
		parts := make([]string, len(bools))
		for idx, b := range bools {
			if b {
				parts[idx] = "TRUE"
			} else {
				parts[idx] = "FALSE"
			}
		}
		return fmt.Sprintf("%s<BOOLEAN[%d] %s>", indentStr, len(bools), strings.Join(parts, " "))
	case secs4go.TypeASCII:
		return fmt.Sprintf("%s<A[%d] \"%s\">", indentStr, len(item.Value.(string)), item.Value.(string))
	case secs4go.TypeJIS8:
		return fmt.Sprintf("%s<J[%d] \"%s\">", indentStr, len(item.Value.([]byte)), string(item.Value.([]byte)))
	case secs4go.TypeInt8:
		return formatNumericSlice(indentStr, "I1", item.Value.([]int8), "%d")
	case secs4go.TypeInt16:
		return formatNumericSlice(indentStr, "I2", item.Value.([]int16), "%d")
	case secs4go.TypeInt32:
		return formatNumericSlice(indentStr, "I4", item.Value.([]int32), "%d")
	case secs4go.TypeInt64:
		return formatNumericSlice(indentStr, "I8", item.Value.([]int64), "%d")
	case secs4go.TypeUInt8:
		return formatNumericSlice(indentStr, "U1", item.Value.([]uint8), "%d")
	case secs4go.TypeUInt16:
		return formatNumericSlice(indentStr, "U2", item.Value.([]uint16), "%d")
	case secs4go.TypeUInt32:
		return formatNumericSlice(indentStr, "U4", item.Value.([]uint32), "%d")
	case secs4go.TypeUInt64:
		return formatNumericSlice(indentStr, "U8", item.Value.([]uint64), "%d")
	case secs4go.TypeFloat32:
		return formatNumericSlice(indentStr, "F4", item.Value.([]float32), "%f")
	case secs4go.TypeFloat64:
		return formatNumericSlice(indentStr, "F8", item.Value.([]float64), "%f")
	}
	return fmt.Sprintf("%s<unknown type %d>", indentStr, item.Type)
}
