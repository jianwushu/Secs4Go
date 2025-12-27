package secs4go

import (
	"fmt"
	"strings"
)

// formatHexData 格式化16进制数据(每个字节用空格隔开)
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

// formatSML 格式化Item为SML格式
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
		childIndentStr := strings.Repeat("  ", indent+1)
		return fmt.Sprintf("%s<L[%d]\n%s\n%s>", indentStr, len(children), childIndentStr+strings.Join(childParts, "\n"+childIndentStr), indentStr)

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

// formatSML 格式化Item为SML格式 (兼容原有接口)
func formatSML(item *Item) string {
	return formatSMLWithIndent(item, 0)
}