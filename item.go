package secs4go

// ============================================================
// Item Factory Functions - 极简API
// ============================================================

// L 创建List项
func L(items ...*Item) *Item {
	return &Item{
		Type:  TypeList,
		Value: items,
	}
}

// B 创建Binary项
func B(data ...byte) *Item {
	return &Item{
		Type:  TypeBinary,
		Value: data,
	}
}

// A 创建ASCII字符串项
func A(data string) *Item {
	return &Item{
		Type:  TypeASCII,
		Value: []byte(data),
	}
}

// J 创建JIS8字符串项
func J(data string) *Item {
	return &Item{
		Type:  TypeJIS8,
		Value: []byte(data),
	}
}

// ===== 有符号整数 =====

// I1 创建int8数组项
func I1(data ...int8) *Item {
	return &Item{
		Type:  TypeInt8,
		Value: data,
	}
}

// I2 创建int16数组项
func I2(data ...int16) *Item {
	return &Item{
		Type:  TypeInt16,
		Value: data,
	}
}

// I4 创建int32数组项
func I4(data ...int32) *Item {
	return &Item{
		Type:  TypeInt32,
		Value: data,
	}
}

// I8 创建int64数组项
func I8(data ...int64) *Item {
	return &Item{
		Type:  TypeInt64,
		Value: data,
	}
}

// ===== 无符号整数 =====

// U1 创建uint8数组项
func U1(data ...uint8) *Item {
	return &Item{
		Type:  TypeUInt8,
		Value: data,
	}
}

// U2 创建uint16数组项
func U2(data ...uint16) *Item {
	return &Item{
		Type:  TypeUInt16,
		Value: data,
	}
}

// U4 创建uint32数组项
func U4(data ...uint32) *Item {
	return &Item{
		Type:  TypeUInt32,
		Value: data,
	}
}

// U8 创建uint64数组项
func U8(data ...uint64) *Item {
	return &Item{
		Type:  TypeUInt64,
		Value: data,
	}
}

// ===== 浮点数 =====

// F4 创建float32数组项
func F4(data ...float32) *Item {
	return &Item{
		Type:  TypeFloat32,
		Value: data,
	}
}

// F8 创建float64数组项
func F8(data ...float64) *Item {
	return &Item{
		Type:  TypeFloat64,
		Value: data,
	}
}

// ===== 长名称别名(可选) =====

// List 创建List项(长名称)
func List(items ...*Item) *Item {
	return L(items...)
}

// Binary 创建Binary项(长名称)
func Binary(data []byte) *Item {
	return B(data...)
}

// ASCII 创建ASCII项(长名称)
func ASCII(data string) *Item {
	return A(data)
}

// JIS8 创建JIS8项(长名称)
func JIS8(data string) *Item {
	return J(data)
}

// ============================================================
// Item Helper Methods
// ============================================================

// IsList 判断是否为List
func (i *Item) IsList() bool {
	return i != nil && i.Type == TypeList
}

// IsEmpty 判断是否为空
func (i *Item) IsEmpty() bool {
	if i == nil {
		return true
	}

	switch v := i.Value.(type) {
	case []*Item:
		return len(v) == 0
	case []byte: // []byte 等同于 []uint8
		return len(v) == 0
	case []int8:
		return len(v) == 0
	case []int16:
		return len(v) == 0
	case []int32:
		return len(v) == 0
	case []int64:
		return len(v) == 0
	case []uint16:
		return len(v) == 0
	case []uint32:
		return len(v) == 0
	case []uint64:
		return len(v) == 0
	case []float32:
		return len(v) == 0
	case []float64:
		return len(v) == 0
	default:
		return false
	}
}

// GetLength 获取长度
func (i *Item) GetLength() int {
	if i == nil {
		return 0
	}

	switch v := i.Value.(type) {
	case []*Item:
		return len(v)
	case []byte: // []byte 等同于 []uint8
		return len(v)
	case []int8:
		return len(v)
	case []int16:
		return len(v)
	case []int32:
		return len(v)
	case []int64:
		return len(v)
	case []uint16:
		return len(v)
	case []uint32:
		return len(v)
	case []uint64:
		return len(v)
	case []float32:
		return len(v)
	case []float64:
		return len(v)
	default:
		return 0
	}
}

// GetItem 获取子项(仅List)
func (i *Item) GetItem(index int) *Item {
	if i == nil || i.Type != TypeList {
		return nil
	}

	children, ok := i.Value.([]*Item)
	if !ok || index < 0 || index >= len(children) {
		return nil
	}

	return children[index]
}

// Append 添加子项(仅List,链式调用)
func (i *Item) Append(child *Item) *Item {
	if i == nil || i.Type != TypeList {
		return i
	}

	children, ok := i.Value.([]*Item)
	if !ok {
		children = make([]*Item, 0)
	}

	children = append(children, child)
	i.Value = children

	return i
}
