package secs4go

import (
	"encoding/binary"
	"fmt"
	"math"
)

// ============================================================
// SECS-II Item Encoding
// ============================================================

// encodeItem 编码Item为SECS-II格式
func encodeItem(item *Item) ([]byte, error) {
	if item == nil {
		return nil, fmt.Errorf("item is nil")
	}

	// 计算数据长度
	var dataLen int
	var err error
	if item.Type == TypeList {
		// 对于List，数据长度是子项数量（按用户要求）
		dataLen, err = calculateItemDataLength(item)
		if err != nil {
			return nil, err
		}
	} else {
		// 对于其他类型，数据长度是实际字节数
		dataLen, err = calculateItemDataLength(item)
		if err != nil {
			return nil, err
		}
	}

	// 格式字节: 6位格式码 + 2位长度字节数
	formatByte := byte(item.Type) << 2

	// 确定长度字节数并设置格式字节的低2位
	var lengthBytes int
	if dataLen <= 0xFF {
		lengthBytes = 1
		formatByte |= 0x01 // 低2位为 01, 表示1字节长度字段
	} else if dataLen <= 0xFFFF {
		lengthBytes = 2
		formatByte |= 0x02 // 低2位为 10, 表示2字节长度字段
	} else if dataLen <= 0xFFFFFF {
		lengthBytes = 3
		formatByte |= 0x03 // 低2位为 11, 表示3字节长度字段
	} else {
		return nil, fmt.Errorf("data length too large: %d", dataLen)
	}

	// 分配缓冲区
	var totalLen int
	if item.Type == TypeList {
		// 对于List，总长度 = 格式字节 + 长度字节 + 所有子项编码后的总长度
		children, ok := item.Value.([]*Item)
		if !ok {
			return nil, fmt.Errorf("invalid list value")
		}

		actualDataLen := 0
		for _, child := range children {
			childLen, err := calculateEncodedLength(child)
			if err != nil {
				return nil, err
			}
			actualDataLen += childLen
		}
		totalLen = 1 + lengthBytes + actualDataLen
	} else {
		totalLen = 1 + lengthBytes + dataLen
	}
	buf := make([]byte, totalLen)

	// 写入格式字节
	buf[0] = formatByte

	// 写入长度
	switch lengthBytes {
	case 1:
		buf[1] = byte(dataLen)
	case 2:
		binary.BigEndian.PutUint16(buf[1:3], uint16(dataLen))
	case 3:
		buf[1] = byte(dataLen >> 16)
		binary.BigEndian.PutUint16(buf[2:4], uint16(dataLen))
	}

	// 写入数据
	if item.Type == TypeList {
		// 对于List，写入所有子项的编码数据
		children, ok := item.Value.([]*Item)
		if !ok {
			return nil, fmt.Errorf("invalid list value")
		}

		offset := 0
		for _, child := range children {
			childData, err := encodeItem(child)
			if err != nil {
				return nil, err
			}
			copy(buf[1+lengthBytes+offset:], childData)
			offset += len(childData)
		}
	} else if dataLen > 0 {
		if err := encodeItemData(item, buf[1+lengthBytes:]); err != nil {
			return nil, err
		}
	}

	return buf, nil
}

// calculateItemDataLength 计算Item数据长度
func calculateItemDataLength(item *Item) (int, error) {
	switch item.Type {
	case TypeList:
		// List的数据长度是子项数量，而不是编码后的总字节数
		children, ok := item.Value.([]*Item)
		if !ok {
			return 0, fmt.Errorf("invalid list value")
		}

		return len(children), nil

	case TypeBinary, TypeBoolean, TypeASCII, TypeJIS8:
		// 字节数组
		data, ok := item.Value.([]byte)
		if !ok {
			return 0, fmt.Errorf("invalid byte array value")
		}
		return len(data), nil

	case TypeInt8:
		data, ok := item.Value.([]int8)
		if !ok {
			return 0, fmt.Errorf("invalid int8 array value")
		}
		return len(data), nil

	case TypeInt16:
		data, ok := item.Value.([]int16)
		if !ok {
			return 0, fmt.Errorf("invalid int16 array value")
		}
		return len(data) * 2, nil

	case TypeInt32:
		data, ok := item.Value.([]int32)
		if !ok {
			return 0, fmt.Errorf("invalid int32 array value")
		}
		return len(data) * 4, nil

	case TypeInt64:
		data, ok := item.Value.([]int64)
		if !ok {
			return 0, fmt.Errorf("invalid int64 array value")
		}
		return len(data) * 8, nil

	case TypeUInt8:
		data, ok := item.Value.([]uint8)
		if !ok {
			return 0, fmt.Errorf("invalid uint8 array value")
		}
		return len(data), nil

	case TypeUInt16:
		data, ok := item.Value.([]uint16)
		if !ok {
			return 0, fmt.Errorf("invalid uint16 array value")
		}
		return len(data) * 2, nil

	case TypeUInt32:
		data, ok := item.Value.([]uint32)
		if !ok {
			return 0, fmt.Errorf("invalid uint32 array value")
		}
		return len(data) * 4, nil

	case TypeUInt64:
		data, ok := item.Value.([]uint64)
		if !ok {
			return 0, fmt.Errorf("invalid uint64 array value")
		}
		return len(data) * 8, nil

	case TypeFloat32:
		data, ok := item.Value.([]float32)
		if !ok {
			return 0, fmt.Errorf("invalid float32 array value")
		}
		return len(data) * 4, nil

	case TypeFloat64:
		data, ok := item.Value.([]float64)
		if !ok {
			return 0, fmt.Errorf("invalid float64 array value")
		}
		return len(data) * 8, nil

	default:
		return 0, fmt.Errorf("unknown item type: %d", item.Type)
	}
}

// encodeItemData 编码Item数据部分
func encodeItemData(item *Item, buf []byte) error {
	switch item.Type {
	case TypeList:
		// List: 递归编码每个子项
		children, ok := item.Value.([]*Item)
		if !ok {
			return fmt.Errorf("invalid list value")
		}

		offset := 0
		for _, child := range children {
			childData, err := encodeItem(child)
			if err != nil {
				return fmt.Errorf("failed to encode child item: %w", err)
			}
			copy(buf[offset:], childData)
			offset += len(childData)
		}

	case TypeBinary, TypeBoolean, TypeASCII, TypeJIS8:
		// 字节数组: 直接复制
		data, ok := item.Value.([]byte)
		if !ok {
			return fmt.Errorf("invalid byte array value")
		}
		copy(buf, data)

	case TypeInt8:
		data, ok := item.Value.([]int8)
		if !ok {
			return fmt.Errorf("invalid int8 array value")
		}
		for i, v := range data {
			buf[i] = byte(v)
		}

	case TypeInt16:
		data, ok := item.Value.([]int16)
		if !ok {
			return fmt.Errorf("invalid int16 array value")
		}
		for i, v := range data {
			binary.BigEndian.PutUint16(buf[i*2:], uint16(v))
		}

	case TypeInt32:
		data, ok := item.Value.([]int32)
		if !ok {
			return fmt.Errorf("invalid int32 array value")
		}
		for i, v := range data {
			binary.BigEndian.PutUint32(buf[i*4:], uint32(v))
		}

	case TypeInt64:
		data, ok := item.Value.([]int64)
		if !ok {
			return fmt.Errorf("invalid int64 array value")
		}
		for i, v := range data {
			binary.BigEndian.PutUint64(buf[i*8:], uint64(v))
		}

	case TypeUInt8:
		data, ok := item.Value.([]uint8)
		if !ok {
			return fmt.Errorf("invalid uint8 array value")
		}
		copy(buf, data)

	case TypeUInt16:
		data, ok := item.Value.([]uint16)
		if !ok {
			return fmt.Errorf("invalid uint16 array value")
		}
		for i, v := range data {
			binary.BigEndian.PutUint16(buf[i*2:], v)
		}

	case TypeUInt32:
		data, ok := item.Value.([]uint32)
		if !ok {
			return fmt.Errorf("invalid uint32 array value")
		}
		for i, v := range data {
			binary.BigEndian.PutUint32(buf[i*4:], v)
		}

	case TypeUInt64:
		data, ok := item.Value.([]uint64)
		if !ok {
			return fmt.Errorf("invalid uint64 array value")
		}
		for i, v := range data {
			binary.BigEndian.PutUint64(buf[i*8:], v)
		}

	case TypeFloat32:
		data, ok := item.Value.([]float32)
		if !ok {
			return fmt.Errorf("invalid float32 array value")
		}
		for i, v := range data {
			binary.BigEndian.PutUint32(buf[i*4:], math.Float32bits(v))
		}

	case TypeFloat64:
		data, ok := item.Value.([]float64)
		if !ok {
			return fmt.Errorf("invalid float64 array value")
		}
		for i, v := range data {
			binary.BigEndian.PutUint64(buf[i*8:], math.Float64bits(v))
		}

	default:
		return fmt.Errorf("unknown item type: %d", item.Type)
	}

	return nil
}

// encodeMessage 编码Message为SECS-II格式
func encodeMessage(msg *Message) ([]byte, error) {
	// 编码Item
	var itemData []byte
	var err error
	if msg.Item != nil {
		itemData, err = encodeItem(msg.Item)
		if err != nil {
			return nil, fmt.Errorf("failed to encode item: %w", err)
		}
	}

	// 计算总长度: 10字节头部 + Item数据
	totalLen := 10 + len(itemData)
	buf := make([]byte, totalLen)

	// Header Byte 0-1: Stream和Function
	// Bit 7 of byte 0: WBit
	// Bit 6-0 of byte 0: Stream (1-127)
	// Byte 1: Function (0-255)
	buf[0] = msg.Stream & 0x7F
	if msg.WBit {
		buf[0] |= 0x80
	}
	buf[1] = msg.Function

	// Header Byte 2-9: 保留(0)
	// 注意: 这里简化处理,实际SECS-II头部只有2字节
	// 剩余8字节由HSMS使用

	// 复制Item数据
	if len(itemData) > 0 {
		copy(buf[10:], itemData)
	}

	return buf, nil
}

// ============================================================
// SECS-II Item Decoding
// ============================================================

// decodeItem 解码SECS-II格式的Item
func decodeItem(data []byte) (*Item, error) {
	if len(data) < 2 {
		return nil, fmt.Errorf("data too short")
	}

	// 格式字节
	formatByte := data[0]
	itemType := ItemType((formatByte >> 2) & 0x3F)
	lengthBytes := int(formatByte & 0x03)

	if len(data) < 1+lengthBytes {
		return nil, fmt.Errorf("data too short for length bytes")
	}

	// 读取长度
	var dataLen int
	switch lengthBytes {
	case 1:
		dataLen = int(data[1])
	case 2:
		dataLen = int(binary.BigEndian.Uint16(data[1:3]))
	case 3:
		dataLen = int(data[1])<<16 | int(data[2])<<8 | int(data[3])
	}

	headerLen := 1 + lengthBytes
	if len(data) < headerLen {
		return nil, fmt.Errorf("data too short for item data")
	}

	// 对于List，dataLen是子项数量，itemData应该是剩余的所有数据
	var itemData []byte
	if itemType == TypeList {
		// 对于List，itemData是剩余的所有字节
		itemData = data[headerLen:]
	} else {
		// 对于其他类型，dataLen是实际的字节数
		if len(data) < headerLen+dataLen {
			return nil, fmt.Errorf("data too short for item data")
		}
		itemData = data[headerLen : headerLen+dataLen]
	}

	// 根据类型解码
	item := &Item{Type: itemType}

	switch itemType {
	case TypeList:
		// List: 根据子项数量解码子项
		children := make([]*Item, 0)
		offset := 0
		remainingItems := dataLen // dataLen现在是子项数量，而不是字节数

		for i := 0; i < remainingItems && offset < len(itemData); i++ {
			remaining := itemData[offset:]
			if len(remaining) < 1 {
				break // 数据不足，无法解码
			}

			// 检查是否有足够的数据进行基本的格式检查
			formatByte := remaining[0]
			lengthBytes := int(formatByte & 0x03)

			if len(remaining) < 1+lengthBytes {
				// 如果连长度字段都不完整，记录错误但尝试继续
				fmt.Printf("警告: List子项数据不完整，偏移 %d: 需要 %d 字节，实际 %d 字节\n",
					offset, 1+lengthBytes, len(remaining))
				break
			}

			// 检查是否有足够的数据读取子项
			headerLen := 1 + lengthBytes

			// 读取子项数据长度
			var subDataLen int
			switch lengthBytes {
			case 1:
				subDataLen = int(remaining[1])
			case 2:
				subDataLen = int(binary.BigEndian.Uint16(remaining[1:3]))
			case 3:
				subDataLen = int(remaining[1])<<16 | int(remaining[2])<<8 | int(remaining[3])
			}

			totalNeeded := headerLen + subDataLen
			if len(remaining) < totalNeeded {
				// 数据不完整，尝试解码已存在的数据
				fmt.Printf("警告: List子项数据不完整，偏移 %d: 需要 %d 字节，实际 %d 字节\n",
					offset, totalNeeded, len(remaining))

				// 尝试解码仅能解码的部分
				if len(remaining) > headerLen {
					subItemData := remaining[headerLen:]
					itemType := ItemType((formatByte >> 2) & 0x3F)
					incompleteItem := &Item{Type: itemType}

					// 对于字节数组类型，可以存储部分数据
					if itemType == TypeBinary || itemType == TypeBoolean ||
						itemType == TypeASCII || itemType == TypeJIS8 || itemType == TypeUInt8 {
						incompleteItem.Value = subItemData
					}

					children = append(children, incompleteItem)
				}
				break
			}

			// 数据完整，尝试正常解码
			child, err := decodeItem(remaining)
			if err != nil {
				fmt.Printf("警告: List子项解码失败: %v\n", err)
				break
			}
			children = append(children, child)

			// 计算子项长度
			childLen, err := calculateEncodedLength(child)
			if err != nil {
				return nil, err
			}
			offset += childLen
		}
		item.Value = children

	case TypeBinary, TypeBoolean, TypeASCII, TypeJIS8:
		// 字节数组
		item.Value = append([]byte(nil), itemData...)

	case TypeInt8:
		values := make([]int8, len(itemData))
		for i, b := range itemData {
			values[i] = int8(b)
		}
		item.Value = values

	case TypeInt16:
		count := len(itemData) / 2
		values := make([]int16, count)
		for i := 0; i < count; i++ {
			values[i] = int16(binary.BigEndian.Uint16(itemData[i*2:]))
		}
		item.Value = values

	case TypeInt32:
		count := len(itemData) / 4
		values := make([]int32, count)
		for i := 0; i < count; i++ {
			values[i] = int32(binary.BigEndian.Uint32(itemData[i*4:]))
		}
		item.Value = values

	case TypeInt64:
		count := len(itemData) / 8
		values := make([]int64, count)
		for i := 0; i < count; i++ {
			values[i] = int64(binary.BigEndian.Uint64(itemData[i*8:]))
		}
		item.Value = values

	case TypeUInt8:
		item.Value = append([]byte(nil), itemData...)

	case TypeUInt16:
		count := len(itemData) / 2
		values := make([]uint16, count)
		for i := 0; i < count; i++ {
			values[i] = binary.BigEndian.Uint16(itemData[i*2:])
		}
		item.Value = values

	case TypeUInt32:
		count := len(itemData) / 4
		values := make([]uint32, count)
		for i := 0; i < count; i++ {
			values[i] = binary.BigEndian.Uint32(itemData[i*4:])
		}
		item.Value = values

	case TypeUInt64:
		count := len(itemData) / 8
		values := make([]uint64, count)
		for i := 0; i < count; i++ {
			values[i] = binary.BigEndian.Uint64(itemData[i*8:])
		}
		item.Value = values

	case TypeFloat32:
		count := len(itemData) / 4
		values := make([]float32, count)
		for i := 0; i < count; i++ {
			bits := binary.BigEndian.Uint32(itemData[i*4:])
			values[i] = math.Float32frombits(bits)
		}
		item.Value = values

	case TypeFloat64:
		count := len(itemData) / 8
		values := make([]float64, count)
		for i := 0; i < count; i++ {
			bits := binary.BigEndian.Uint64(itemData[i*8:])
			values[i] = math.Float64frombits(bits)
		}
		item.Value = values

	default:
		return nil, fmt.Errorf("unknown item type: %d", itemType)
	}

	return item, nil
}

// calculateEncodedLength 计算编码后的Item总长度
func calculateEncodedLength(item *Item) (int, error) {
	dataLen, err := calculateItemDataLength(item)
	if err != nil {
		return 0, err
	}

	// 确定长度字节数
	var lengthBytes int
	if dataLen <= 0xFF {
		lengthBytes = 1
	} else if dataLen <= 0xFFFF {
		lengthBytes = 2
	} else if dataLen <= 0xFFFFFF {
		lengthBytes = 3
	} else {
		return 0, fmt.Errorf("data length too large: %d", dataLen)
	}

	// 对于List,需要递归计算子项长度
	if item.Type == TypeList {
		children, ok := item.Value.([]*Item)
		if !ok {
			return 0, fmt.Errorf("invalid list value")
		}

		totalDataLen := 0
		for _, child := range children {
			childLen, err := calculateEncodedLength(child)
			if err != nil {
				return 0, err
			}
			totalDataLen += childLen
		}
		return 1 + lengthBytes + totalDataLen, nil
	}

	return 1 + lengthBytes + dataLen, nil
}

// ============================================================
// Public API for Item Encoding/Decoding
// ============================================================

// EncodeItem 公开API: 编码Item为SECS-II格式
func EncodeItem(item *Item) ([]byte, error) {
	return encodeItem(item)
}

// DecodeItem 公开API: 解码SECS-II格式的Item
func DecodeItem(data []byte) (*Item, error) {
	return decodeItem(data)
}
