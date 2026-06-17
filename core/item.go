package core

import "fmt"

// ============================================================
// Item 数据模型
// ============================================================

// Item SECS数据项
type Item struct {
	Type  ItemType
	Value interface{}
}

// ============================================================
// Item 工厂函数
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
		Value: data,
	}
}

// J 创建JIS8字符串项
func J(data string) *Item {
	return &Item{
		Type:  TypeJIS8,
		Value: []byte(data),
	}
}

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

// Bool 创建Boolean数组项
func Bool(data ...bool) *Item {
	return &Item{
		Type:  TypeBoolean,
		Value: data,
	}
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
	case []byte:
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
	case []bool:
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
	case []byte:
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
	case []bool:
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

// ============================================================
// 内部辅助函数（跨类型转换）
// ============================================================

// firstByte 从切片取第一个字节值
func firstByte(value interface{}) (byte, bool) {
	switch v := value.(type) {
	case []byte:
		if len(v) > 0 {
			return v[0], true
		}
	}
	return 0, false
}

// firstBool 从切片取第一个布尔值
func firstBool(value interface{}) (bool, bool) {
	switch v := value.(type) {
	case []bool:
		if len(v) > 0 {
			return v[0], true
		}
	}
	return false, false
}

// firstInt 从切片取第一个值并跨类型转换为 int
func firstInt(value interface{}) (int, bool) {
	switch v := value.(type) {
	case []byte:
		if len(v) > 0 {
			return int(v[0]), true
		}
	case []int8:
		if len(v) > 0 {
			return int(v[0]), true
		}
	case []int16:
		if len(v) > 0 {
			return int(v[0]), true
		}
	case []int32:
		if len(v) > 0 {
			return int(v[0]), true
		}
	case []int64:
		if len(v) > 0 {
			return int(v[0]), true
		}
	case []uint16:
		if len(v) > 0 {
			return int(v[0]), true
		}
	case []uint32:
		if len(v) > 0 {
			return int(v[0]), true
		}
	case []uint64:
		if len(v) > 0 {
			return int(v[0]), true
		}
	case []float32:
		if len(v) > 0 {
			return int(v[0]), true
		}
	case []float64:
		if len(v) > 0 {
			return int(v[0]), true
		}
	}
	return 0, false
}

// firstUint 从切片取第一个值并跨类型转换为 uint64（不含浮点）
func firstUint(value interface{}) (uint64, bool) {
	switch v := value.(type) {
	case []byte:
		if len(v) > 0 {
			return uint64(v[0]), true
		}
	case []int8:
		if len(v) > 0 {
			return uint64(v[0]), true
		}
	case []int16:
		if len(v) > 0 {
			return uint64(v[0]), true
		}
	case []int32:
		if len(v) > 0 {
			return uint64(v[0]), true
		}
	case []int64:
		if len(v) > 0 {
			return uint64(v[0]), true
		}
	case []uint16:
		if len(v) > 0 {
			return uint64(v[0]), true
		}
	case []uint32:
		if len(v) > 0 {
			return uint64(v[0]), true
		}
	case []uint64:
		if len(v) > 0 {
			return v[0], true
		}
	}
	return 0, false
}

// firstFloat 从切片取第一个值并跨类型转换为 float64
func firstFloat(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case []byte:
		if len(v) > 0 {
			return float64(v[0]), true
		}
	case []int8:
		if len(v) > 0 {
			return float64(v[0]), true
		}
	case []int16:
		if len(v) > 0 {
			return float64(v[0]), true
		}
	case []int32:
		if len(v) > 0 {
			return float64(v[0]), true
		}
	case []int64:
		if len(v) > 0 {
			return float64(v[0]), true
		}
	case []uint16:
		if len(v) > 0 {
			return float64(v[0]), true
		}
	case []uint32:
		if len(v) > 0 {
			return float64(v[0]), true
		}
	case []uint64:
		if len(v) > 0 {
			return float64(v[0]), true
		}
	case []float32:
		if len(v) > 0 {
			return float64(v[0]), true
		}
	case []float64:
		if len(v) > 0 {
			return v[0], true
		}
	}
	return 0, false
}

// asIntSlice 跨类型切片转换为 []int
func asIntSlice(value interface{}) ([]int, bool) {
	switch v := value.(type) {
	case []byte:
		r := make([]int, len(v))
		for i, x := range v {
			r[i] = int(x)
		}
		return r, true
	case []int8:
		r := make([]int, len(v))
		for i, x := range v {
			r[i] = int(x)
		}
		return r, true
	case []int16:
		r := make([]int, len(v))
		for i, x := range v {
			r[i] = int(x)
		}
		return r, true
	case []int32:
		r := make([]int, len(v))
		for i, x := range v {
			r[i] = int(x)
		}
		return r, true
	case []int64:
		r := make([]int, len(v))
		for i, x := range v {
			r[i] = int(x)
		}
		return r, true
	case []uint16:
		r := make([]int, len(v))
		for i, x := range v {
			r[i] = int(x)
		}
		return r, true
	case []uint32:
		r := make([]int, len(v))
		for i, x := range v {
			r[i] = int(x)
		}
		return r, true
	case []uint64:
		r := make([]int, len(v))
		for i, x := range v {
			r[i] = int(x)
		}
		return r, true
	case []float32:
		r := make([]int, len(v))
		for i, x := range v {
			r[i] = int(x)
		}
		return r, true
	case []float64:
		r := make([]int, len(v))
		for i, x := range v {
			r[i] = int(x)
		}
		return r, true
	}
	return nil, false
}

// asInt64Slice 跨类型切片转换为 []int64
func asInt64Slice(value interface{}) ([]int64, bool) {
	switch v := value.(type) {
	case []byte:
		r := make([]int64, len(v))
		for i, x := range v {
			r[i] = int64(x)
		}
		return r, true
	case []int8:
		r := make([]int64, len(v))
		for i, x := range v {
			r[i] = int64(x)
		}
		return r, true
	case []int16:
		r := make([]int64, len(v))
		for i, x := range v {
			r[i] = int64(x)
		}
		return r, true
	case []int32:
		r := make([]int64, len(v))
		for i, x := range v {
			r[i] = int64(x)
		}
		return r, true
	case []int64:
		r := make([]int64, len(v))
		copy(r, v)
		return r, true
	case []uint16:
		r := make([]int64, len(v))
		for i, x := range v {
			r[i] = int64(x)
		}
		return r, true
	case []uint32:
		r := make([]int64, len(v))
		for i, x := range v {
			r[i] = int64(x)
		}
		return r, true
	case []uint64:
		r := make([]int64, len(v))
		for i, x := range v {
			r[i] = int64(x)
		}
		return r, true
	case []float32:
		r := make([]int64, len(v))
		for i, x := range v {
			r[i] = int64(x)
		}
		return r, true
	case []float64:
		r := make([]int64, len(v))
		for i, x := range v {
			r[i] = int64(x)
		}
		return r, true
	}
	return nil, false
}

// asUint64Slice 跨类型切片转换为 []uint64（不含浮点）
func asUint64Slice(value interface{}) ([]uint64, bool) {
	switch v := value.(type) {
	case []byte:
		r := make([]uint64, len(v))
		for i, x := range v {
			r[i] = uint64(x)
		}
		return r, true
	case []int8:
		r := make([]uint64, len(v))
		for i, x := range v {
			r[i] = uint64(x)
		}
		return r, true
	case []int16:
		r := make([]uint64, len(v))
		for i, x := range v {
			r[i] = uint64(x)
		}
		return r, true
	case []int32:
		r := make([]uint64, len(v))
		for i, x := range v {
			r[i] = uint64(x)
		}
		return r, true
	case []int64:
		r := make([]uint64, len(v))
		for i, x := range v {
			r[i] = uint64(x)
		}
		return r, true
	case []uint16:
		r := make([]uint64, len(v))
		for i, x := range v {
			r[i] = uint64(x)
		}
		return r, true
	case []uint32:
		r := make([]uint64, len(v))
		for i, x := range v {
			r[i] = uint64(x)
		}
		return r, true
	case []uint64:
		r := make([]uint64, len(v))
		copy(r, v)
		return r, true
	}
	return nil, false
}

// asFloat64Slice 跨类型切片转换为 []float64
func asFloat64Slice(value interface{}) ([]float64, bool) {
	switch v := value.(type) {
	case []byte:
		r := make([]float64, len(v))
		for i, x := range v {
			r[i] = float64(x)
		}
		return r, true
	case []int8:
		r := make([]float64, len(v))
		for i, x := range v {
			r[i] = float64(x)
		}
		return r, true
	case []int16:
		r := make([]float64, len(v))
		for i, x := range v {
			r[i] = float64(x)
		}
		return r, true
	case []int32:
		r := make([]float64, len(v))
		for i, x := range v {
			r[i] = float64(x)
		}
		return r, true
	case []int64:
		r := make([]float64, len(v))
		for i, x := range v {
			r[i] = float64(x)
		}
		return r, true
	case []uint16:
		r := make([]float64, len(v))
		for i, x := range v {
			r[i] = float64(x)
		}
		return r, true
	case []uint32:
		r := make([]float64, len(v))
		for i, x := range v {
			r[i] = float64(x)
		}
		return r, true
	case []uint64:
		r := make([]float64, len(v))
		for i, x := range v {
			r[i] = float64(x)
		}
		return r, true
	case []float32:
		r := make([]float64, len(v))
		for i, x := range v {
			r[i] = float64(x)
		}
		return r, true
	case []float64:
		r := make([]float64, len(v))
		copy(r, v)
		return r, true
	}
	return nil, false
}

// ============================================================
// Item Accessor Methods
// ============================================================

// AsList 读取List子项
func (i *Item) AsList() ([]*Item, bool) {
	if i == nil || i.Type != TypeList {
		return nil, false
	}
	children, ok := i.Value.([]*Item)
	if !ok {
		return nil, false
	}
	return children, true
}

// AsBytes 读取字节数据
func (i *Item) AsBytes() ([]byte, bool) {
	if i == nil {
		return nil, false
	}
	if i.Type != TypeBinary && i.Type != TypeJIS8 && i.Type != TypeASCII {
		return nil, false
	}
	switch v := i.Value.(type) {
	case []byte:
		return v, true
	case string:
		return []byte(v), true
	default:
		return nil, false
	}
}

// AsString 读取字符串数据
func (i *Item) AsString() (string, bool) {
	if i == nil || (i.Type != TypeASCII && i.Type != TypeJIS8) {
		return "", false
	}
	switch v := i.Value.(type) {
	case string:
		return v, true
	case []byte:
		return string(v), true
	default:
		return "", false
	}
}

// AsBoolSlice 读取 bool 切片
func (i *Item) AsBoolSlice() ([]bool, bool) {
	if i == nil || i.Type != TypeBoolean {
		return nil, false
	}
	vals, ok := i.Value.([]bool)
	if !ok {
		return nil, false
	}
	return vals, true
}

// AsIntSlice 跨类型读取 int 切片
// 支持源类型: B, I1, I2, I4, I8, U1, U2, U4, U8, F4, F8
func (i *Item) AsIntSlice() ([]int, bool) {
	if i == nil {
		return nil, false
	}
	return asIntSlice(i.Value)
}

// AsInt64Slice 跨类型读取 int64 切片
// 支持源类型: B, I1, I2, I4, I8, U1, U2, U4, U8, F4, F8
func (i *Item) AsInt64Slice() ([]int64, bool) {
	if i == nil {
		return nil, false
	}
	return asInt64Slice(i.Value)
}

// AsUint64Slice 跨类型读取 uint64 切片（不含浮点）
// 支持源类型: B, I1, I2, I4, I8, U1, U2, U4, U8
func (i *Item) AsUint64Slice() ([]uint64, bool) {
	if i == nil {
		return nil, false
	}
	return asUint64Slice(i.Value)
}

// AsFloat64Slice 跨类型读取 float64 切片
// 支持源类型: B, I1, I2, I4, I8, U1, U2, U4, U8, F4, F8
func (i *Item) AsFloat64Slice() ([]float64, bool) {
	if i == nil {
		return nil, false
	}
	return asFloat64Slice(i.Value)
}

// ============================================================
// First* 系列 — 跨类型单值访问
// ============================================================

// FirstByte 获取单个字节值
// 支持源类型: B
func (i *Item) FirstByte() (byte, bool) {
	if i == nil || i.Type != TypeBinary {
		return 0, false
	}
	return firstByte(i.Value)
}

// FirstBool 获取布尔值
// 支持源类型: BOOLEAN
func (i *Item) FirstBool() (bool, bool) {
	if i == nil || i.Type != TypeBoolean {
		return false, false
	}
	return firstBool(i.Value)
}

// FirstInt 获取 int 值（跨类型）
// 支持源类型: B, I1, I2, I4, I8, U1, U2, U4, U8, F4, F8
// 浮点类型会截断小数部分
func (i *Item) FirstInt() (int, bool) {
	if i == nil {
		return 0, false
	}
	return firstInt(i.Value)
}

// FirstUint 获取 uint64 值（跨类型，不含浮点）
// 支持源类型: B, I1, I2, I4, I8, U1, U2, U4, U8
func (i *Item) FirstUint() (uint64, bool) {
	if i == nil {
		return 0, false
	}
	return firstUint(i.Value)
}

// FirstFloat 获取 float64 值（跨类型）
// 支持源类型: B, I1, I2, I4, I8, U1, U2, U4, U8, F4, F8
func (i *Item) FirstFloat() (float64, bool) {
	if i == nil {
		return 0, false
	}
	return firstFloat(i.Value)
}

// ============================================================
// Format — 通用字符串转换
// ============================================================

// Format 将任意 Item 转换为人类可读的字符串表示
// 所有类型统一转换，不返回 error，不 panic
func (i *Item) Format() string {
	if i == nil {
		return ""
	}

	switch i.Type {
	case TypeList:
		children, ok := i.Value.([]*Item)
		if !ok {
			return "<invalid list>"
		}
		return fmt.Sprintf("L[%d]", len(children))
	case TypeBinary:
		data, ok := i.Value.([]byte)
		if !ok {
			return "<invalid binary>"
		}
		if len(data) == 0 {
			return ""
		}
		hex := make([]string, len(data))
		for idx, b := range data {
			hex[idx] = fmt.Sprintf("%02X", b)
		}
		return joinStrings(hex, " ")
	case TypeBoolean:
		bools, ok := i.Value.([]bool)
		if !ok {
			return "<invalid boolean>"
		}
		if len(bools) == 0 {
			return ""
		}
		parts := make([]string, len(bools))
		for idx, b := range bools {
			if b {
				parts[idx] = "true"
			} else {
				parts[idx] = "false"
			}
		}
		return joinStrings(parts, ", ")
	case TypeASCII:
		s, ok := i.Value.(string)
		if !ok {
			return "<invalid ascii>"
		}
		return s
	case TypeJIS8:
		data, ok := i.Value.([]byte)
		if !ok {
			return "<invalid jis8>"
		}
		return string(data)
	case TypeInt8:
		return formatNumericSlice(i.Value.([]int8), "%d")
	case TypeInt16:
		return formatNumericSlice(i.Value.([]int16), "%d")
	case TypeInt32:
		return formatNumericSlice(i.Value.([]int32), "%d")
	case TypeInt64:
		return formatNumericSlice(i.Value.([]int64), "%d")
	case TypeUInt8:
		return formatNumericSlice(i.Value.([]byte), "%d")
	case TypeUInt16:
		return formatNumericSlice(i.Value.([]uint16), "%d")
	case TypeUInt32:
		return formatNumericSlice(i.Value.([]uint32), "%d")
	case TypeUInt64:
		return formatNumericSlice(i.Value.([]uint64), "%d")
	case TypeFloat32:
		return formatNumericSlice(i.Value.([]float32), "%g")
	case TypeFloat64:
		return formatNumericSlice(i.Value.([]float64), "%g")
	default:
		return "<unknown type>"
	}
}

// formatNumericSlice 格式化数值切片为逗号分隔的字符串
func formatNumericSlice[T ~int8 | ~int16 | ~int32 | ~int64 | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~float32 | ~float64](
	values []T, fmtStr string,
) string {
	if len(values) == 0 {
		return ""
	}
	if len(values) == 1 {
		return fmt.Sprintf(fmtStr, values[0])
	}
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = fmt.Sprintf(fmtStr, v)
	}
	return joinStrings(parts, ", ")
}

// joinStrings 使用分隔符连接字符串切片
func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	n := len(sep) * (len(parts) - 1)
	for _, s := range parts {
		n += len(s)
	}
	b := make([]byte, 0, n)
	b = append(b, parts[0]...)
	for _, s := range parts[1:] {
		b = append(b, sep...)
		b = append(b, s...)
	}
	return string(b)
}
