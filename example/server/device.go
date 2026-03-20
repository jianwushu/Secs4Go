package main

import (
	"fmt"
	"sync"

	"github.com/jianwushu/Secs4go/secs4go"
)

// deviceMu 保护共享设备状态 map 的并发读写
var deviceMu sync.RWMutex

// SVID 状态变量
type SVID struct {
	name  string
	value interface{} // string / int / float64 / bool
	unit  string
}

func (v *SVID) ValueToString() string {
	// 注意：多类型 case 中 val 类型为 interface{}，直接 fmt.Sprint 最安全
	switch val := v.value.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%g", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(val)
	}
}

// 常量声明
type ECID struct {
	name  string
	value interface{}
	min   interface{}
	max   interface{}
	def   interface{}
	unit  string
}

// 数据量声明
type DVID struct {
	name  string
	value interface{}
}

// 事件声明
type EventLink struct {
	enable bool
	links  []string
}

// 报告声明
type ReportLink struct {
	links []string
}

var (
	// SvMap 状态变量表 (Status Variables)
	SvMap = map[string]SVID{
		"2001": {"ControlState", 0, ""},
		"2002": {"RunState", 0, ""},
		"2003": {"PrevState", 0, ""},
		"2004": {"SubState", 0, ""},
		"2005": {"RecipeID", "test", ""},
		"2006": {"OPID", "70225490", ""},
		"2007": {"MC Name", "HH", ""},
		"2008": {"MC Version", "v1.0.0", ""},
	}
	// DvMap 数据变量表 (Data Variables)
	DvMap = map[string]DVID{
		"1001": {"CRR_START_ID", ""},
		"1002": {"CRR_START_TIME", ""},
		"1003": {"CRR_END_ID", ""},
		"1004": {"CRR_END_TIME", ""},
		"1005": {"CRR_IN_MAP", ""},
		"1006": {"CRR_OUT_MAP", ""},
	}
	// EvMap 设备常量表 (Equipment Constants)，min/max/def 使用 float32
	EvMap = map[string]ECID{
		"3001": {"T3", float32(45), float32(45), float32(45), float32(45), "s"},
		"3002": {"T5", float32(10), float32(10), float32(10), float32(10), "s"},
	}
	// EventLinks 事件-报告关联表
	EventLinks = map[string]EventLink{
		"10020": {true, []string{"10020"}},
	}
	// ReportLinks 报告-变量关联表
	ReportLinks = map[string]ReportLink{
		"10020": {[]string{"1001", "1002"}},
	}
)

// parseUintIDs 从 List 类型 Item 中安全解析整数 ID 列表（支持 U2/U4/I2）
func parseUintIDs(item *secs4go.Item) []string {
	if !item.IsList() {
		return nil
	}
	ids := make([]string, 0, item.GetLength())
	for i := 0; i < item.GetLength(); i++ {
		child := item.GetItem(i)
		if child == nil {
			continue
		}
		switch child.Type {
		case secs4go.TypeUInt16:
			if v, ok := child.Value.([]uint16); ok && len(v) > 0 {
				ids = append(ids, fmt.Sprint(v[0]))
			}
		case secs4go.TypeUInt32:
			if v, ok := child.Value.([]uint32); ok && len(v) > 0 {
				ids = append(ids, fmt.Sprint(v[0]))
			}
		case secs4go.TypeInt16:
			if v, ok := child.Value.([]int16); ok && len(v) > 0 {
				ids = append(ids, fmt.Sprint(v[0]))
			}
		default:
			ids = append(ids, fmt.Sprint(child.Value))
		}
	}
	return ids
}

// parseFirstUint 从单个数值 Item 中安全解析第一个整数，返回字符串形式
func parseFirstUint(item *secs4go.Item) string {
	if item == nil {
		return ""
	}
	switch item.Type {
	case secs4go.TypeUInt16:
		if v, ok := item.Value.([]uint16); ok && len(v) > 0 {
			return fmt.Sprint(v[0])
		}
	case secs4go.TypeUInt32:
		if v, ok := item.Value.([]uint32); ok && len(v) > 0 {
			return fmt.Sprint(v[0])
		}
	case secs4go.TypeInt16:
		if v, ok := item.Value.([]int16); ok && len(v) > 0 {
			return fmt.Sprint(v[0])
		}
	}
	return fmt.Sprint(item.Value)
}

func parseBoolFlag(item *secs4go.Item, fieldName string) (bool, error) {
	if item == nil {
		return false, fmt.Errorf("%s 缺失", fieldName)
	}
	boolVals, ok := item.Value.([]bool)
	if !ok {
		return false, fmt.Errorf("%s 类型错误: 期望 []bool, 实际 ItemType=%v ValueType=%T", fieldName, item.Type, item.Value)
	}
	if len(boolVals) == 0 {
		return false, fmt.Errorf("%s 类型错误: []bool 为空", fieldName)
	}
	return boolVals[0], nil
}

func parseIDListFromListItem(item *secs4go.Item, fieldName string) ([]string, error) {
	if err := requireListWithMinLength(item, 0, fieldName); err != nil {
		return nil, err
	}
	return parseUintIDs(item), nil
}

func requireListWithMinLength(item *secs4go.Item, minLength int, name string) error {
	if item == nil {
		return fmt.Errorf("%s 缺失", name)
	}
	if !item.IsList() {
		return fmt.Errorf("%s 不是 List 格式", name)
	}
	if item.GetLength() < minLength {
		return fmt.Errorf("%s 长度不足: 期望至少 %d, 实际 %d", name, minLength, item.GetLength())
	}
	return nil
}

func requireListLength(item *secs4go.Item, expectedLength int, name string) error {
	if err := requireListWithMinLength(item, expectedLength, name); err != nil {
		return err
	}
	if item.GetLength() != expectedLength {
		return fmt.Errorf("%s 长度错误: 期望 %d, 实际 %d", name, expectedLength, item.GetLength())
	}
	return nil
}

func getRequiredListItem(parent *secs4go.Item, index int, parentName string) (*secs4go.Item, error) {
	if parent == nil {
		return nil, fmt.Errorf("%s 缺失", parentName)
	}
	if !parent.IsList() {
		return nil, fmt.Errorf("%s 不是 List 格式", parentName)
	}
	if index < 0 || index >= parent.GetLength() {
		return nil, fmt.Errorf("%s 缺少索引 %d: 实际长度 %d", parentName, index, parent.GetLength())
	}
	item := parent.GetItem(index)
	if item == nil {
		return nil, fmt.Errorf("%s[%d] 为空", parentName, index)
	}
	return item, nil
}

// svName 安全获取 SvMap 中的字符串字段（避免直接类型断言 panic）
func svName(key string) string {
	deviceMu.RLock()
	sv, ok := SvMap[key]
	deviceMu.RUnlock()
	if !ok {
		return ""
	}
	if s, ok := sv.value.(string); ok {
		return s
	}
	return fmt.Sprint(sv.value)
}

// Are You There Request (R)  →  S1F2
func HandleS1F1(item *secs4go.Item) *secs4go.Message {
	return secs4go.NewMessage(1, 2).WithItem(secs4go.L(
		secs4go.A(svName("2007")),
		secs4go.A(svName("2008")),
	))
}

// Selected Equipment Status Request (SSR)  →  S1F4
func HandleS1F3(item *secs4go.Item) *secs4go.Message {
	svList := parseUintIDs(item)

	deviceMu.RLock()
	defer deviceMu.RUnlock()

	var vals []*secs4go.Item
	if len(svList) == 0 {
		for _, sv := range SvMap {
			vals = append(vals, secs4go.A(sv.ValueToString()))
		}
	} else {
		for _, id := range svList {
			if sv, ok := SvMap[id]; ok {
				vals = append(vals, secs4go.A(sv.ValueToString()))
			} else {
				vals = append(vals, secs4go.L()) // 未知 SVID 返回空列表
			}
		}
	}
	return secs4go.NewMessage(1, 4).WithItem(secs4go.L(vals...))
}

// Status Variable Namelist Request (SVNR)  →  S1F12
func HandleS1F11(item *secs4go.Item) *secs4go.Message {
	svList := parseUintIDs(item)

	deviceMu.RLock()
	defer deviceMu.RUnlock()

	var entries []*secs4go.Item
	if len(svList) == 0 {
		for key, sv := range SvMap {
			entries = append(entries, secs4go.L(
				secs4go.U4(String2UInt32(key)),
				secs4go.A(sv.name),
				secs4go.A(sv.unit),
			))
		}
	} else {
		for _, id := range svList {
			if sv, ok := SvMap[id]; ok {
				entries = append(entries, secs4go.L(
					secs4go.U4(String2UInt32(id)),
					secs4go.A(sv.name),
					secs4go.A(sv.unit),
				))
			} else {
				entries = append(entries, secs4go.L( // 未知 SVID：ID 填充，名称/单位为空
					secs4go.U4(String2UInt32(id)),
					secs4go.A(""),
					secs4go.A(""),
				))
			}
		}
	}
	return secs4go.NewMessage(1, 12).WithItem(secs4go.L(entries...))
}

// Establish Communications Request (CR)  →  S1F14
func HandleS1F13(item *secs4go.Item) *secs4go.Message {
	return secs4go.NewMessage(1, 14).WithItem(secs4go.L(
		secs4go.B(0),
		secs4go.L(
			secs4go.A(svName("2007")),
			secs4go.A(svName("2008")),
		),
	))
}

// Equipment Constant Request (ECR)  →  S2F14
// 注意：EC 来自 EvMap（设备常量），而非 SvMap（状态变量）
func HandleS2F13(item *secs4go.Item) *secs4go.Message {
	ecList := parseUintIDs(item)

	deviceMu.RLock()
	defer deviceMu.RUnlock()

	var vals []*secs4go.Item
	if len(ecList) == 0 {
		for _, ec := range EvMap {
			vals = append(vals, secs4go.A(fmt.Sprint(ec.value)))
		}
	} else {
		for _, id := range ecList {
			if ec, ok := EvMap[id]; ok {
				vals = append(vals, secs4go.A(fmt.Sprint(ec.value)))
			} else {
				vals = append(vals, secs4go.L())
			}
		}
	}
	return secs4go.NewMessage(2, 14).WithItem(secs4go.L(vals...))
}

// Equipment Constant Namelist Request (ECNR)  →  S2F30
func HandleS2F29(item *secs4go.Item) *secs4go.Message {
	ecList := parseUintIDs(item)

	deviceMu.RLock()
	defer deviceMu.RUnlock()

	ecEntry := func(id string, ec ECID) *secs4go.Item {
		return secs4go.L(
			secs4go.U4(String2UInt32(id)),
			secs4go.A(ec.name),
			secs4go.F4(ec.min.(float32)),
			secs4go.F4(ec.max.(float32)),
			secs4go.F4(ec.def.(float32)),
			secs4go.A(ec.unit),
		)
	}
	ecEmpty := func(id string) *secs4go.Item {
		return secs4go.L(secs4go.U4(String2UInt32(id)),
			secs4go.A(""), secs4go.A(""), secs4go.A(""), secs4go.A(""), secs4go.A(""))
	}

	var entries []*secs4go.Item
	if len(ecList) == 0 {
		for key, ec := range EvMap {
			entries = append(entries, ecEntry(key, ec))
		}
	} else {
		for _, id := range ecList {
			if ec, ok := EvMap[id]; ok {
				entries = append(entries, ecEntry(id, ec))
			} else {
				entries = append(entries, ecEmpty(id))
			}
		}
	}
	return secs4go.NewMessage(2, 30).WithItem(secs4go.L(entries...))
}

// Define Report (DR)  →  S2F34
func HandleS2F33(item *secs4go.Item) (*secs4go.Message, error) {
	if err := requireListWithMinLength(item, 2, "S2F33 报文"); err != nil {
		return DACKMessage(DACK2), fmt.Errorf("S2F33: %w", err)
	}

	dataItem, err := getRequiredListItem(item, 1, "S2F33 报文")
	if err != nil {
		return DACKMessage(DACK2), fmt.Errorf("S2F33: %w", err)
	}
	if err := requireListWithMinLength(dataItem, 0, "S2F33 报文[1] 报告列表"); err != nil {
		return DACKMessage(DACK2), fmt.Errorf("S2F33: %w", err)
	}

	pending := map[string]ReportLink{}

	for i := 0; i < dataItem.GetLength(); i++ {
		child, err := getRequiredListItem(dataItem, i, "S2F33 报文[1] 报告列表")
		if err != nil {
			return DACKMessage(DACK1), fmt.Errorf("S2F33: %w", err)
		}
		if err := requireListLength(child, 2, fmt.Sprintf("S2F33 报文[1][%d] 报告项", i)); err != nil {
			return DACKMessage(DACK1), fmt.Errorf("S2F33: %w", err)
		}
		reportIDItem, err := getRequiredListItem(child, 0, fmt.Sprintf("S2F33 报文[1][%d] 报告项", i))
		if err != nil {
			return DACKMessage(DACK1), fmt.Errorf("S2F33: %w", err)
		}
		reportIDsItem, err := getRequiredListItem(child, 1, fmt.Sprintf("S2F33 报文[1][%d] 报告项", i))
		if err != nil {
			return DACKMessage(DACK1), fmt.Errorf("S2F33: %w", err)
		}

		reportID := parseFirstUint(reportIDItem)
		if IsReportDefined(reportID) {
			return DACKMessage(DACK3), fmt.Errorf("S2F33: 报告 %s 已存在", reportID)
		}
		vids, err := parseIDListFromListItem(reportIDsItem, fmt.Sprintf("S2F33 报文[1][%d] 报告项[1] VID 列表", i))
		if err != nil {
			return DACKMessage(DACK1), fmt.Errorf("S2F33: %w", err)
		}
		for _, vid := range vids {
			if !IsVidDefined(vid) {
				return DACKMessage(DACK4), fmt.Errorf("S2F33: VID %s 未定义", vid)
			}
		}
		pending[reportID] = ReportLink{vids}
	}

	deviceMu.Lock()
	if len(pending) == 0 {
		ReportLinks = map[string]ReportLink{} // 空列表 = 清除所有报告
	} else {
		for id, link := range pending {
			ReportLinks[id] = link
		}
	}
	deviceMu.Unlock()
	return DACKMessage(DACK0), nil
}

// Link Event Report (LER)  →  S2F36
func HandleS2F35(item *secs4go.Item) (*secs4go.Message, error) {
	if err := requireListWithMinLength(item, 2, "S2F35 报文"); err != nil {
		return LRACKMessage(LRACK2), fmt.Errorf("S2F35: %w", err)
	}

	dataItem, err := getRequiredListItem(item, 1, "S2F35 报文")
	if err != nil {
		return LRACKMessage(LRACK2), fmt.Errorf("S2F35: %w", err)
	}
	if err := requireListWithMinLength(dataItem, 0, "S2F35 报文[1] 事件列表"); err != nil {
		return LRACKMessage(LRACK2), fmt.Errorf("S2F35: %w", err)
	}

	pending := map[string]EventLink{}

	for i := 0; i < dataItem.GetLength(); i++ {
		child, err := getRequiredListItem(dataItem, i, "S2F35 报文[1] 事件列表")
		if err != nil {
			return LRACKMessage(LRACK1), fmt.Errorf("S2F35: %w", err)
		}
		if err := requireListLength(child, 2, fmt.Sprintf("S2F35 报文[1][%d] 事件项", i)); err != nil {
			return LRACKMessage(LRACK1), fmt.Errorf("S2F35: %w", err)
		}
		eventIDItem, err := getRequiredListItem(child, 0, fmt.Sprintf("S2F35 报文[1][%d] 事件项", i))
		if err != nil {
			return LRACKMessage(LRACK1), fmt.Errorf("S2F35: %w", err)
		}
		reportIDsItem, err := getRequiredListItem(child, 1, fmt.Sprintf("S2F35 报文[1][%d] 事件项", i))
		if err != nil {
			return LRACKMessage(LRACK1), fmt.Errorf("S2F35: %w", err)
		}

		eventID := parseFirstUint(eventIDItem)
		if IsEventDefined(eventID) {
			return LRACKMessage(LRACK3), fmt.Errorf("S2F35: 事件 %s 已存在", eventID)
		}
		reportIDs, err := parseIDListFromListItem(reportIDsItem, fmt.Sprintf("S2F35 报文[1][%d] 事件项[1] 报告列表", i))
		if err != nil {
			return LRACKMessage(LRACK1), fmt.Errorf("S2F35: %w", err)
		}
		for _, rid := range reportIDs {
			if !IsReportDefined(rid) {
				return LRACKMessage(LRACK4), fmt.Errorf("S2F35: 报告 %s 未定义", rid)
			}
		}
		pending[eventID] = EventLink{enable: false, links: reportIDs}
	}

	deviceMu.Lock()
	if len(pending) == 0 {
		EventLinks = map[string]EventLink{} // 空列表 = 清除所有事件关联
	} else {
		for id, link := range pending {
			EventLinks[id] = link
		}
	}
	deviceMu.Unlock()
	return LRACKMessage(LRACK0), nil
}

// Enable/Disable Collection Event Report  →  S2F38
func HandleS2F37(item *secs4go.Item) (*secs4go.Message, error) {
	if err := requireListWithMinLength(item, 2, "S2F37 报文"); err != nil {
		return ERACKMessage(ERACK2), fmt.Errorf("S2F37: %w", err)
	}

	enableItem, err := getRequiredListItem(item, 0, "S2F37 报文")
	if err != nil {
		return ERACKMessage(ERACK2), fmt.Errorf("S2F37: %w", err)
	}
	enable, err := parseBoolFlag(enableItem, "S2F37 enable 字段")
	if err != nil {
		return ERACKMessage(ERACK2), fmt.Errorf("S2F37: %w", err)
	}

	ceidsItem, err := getRequiredListItem(item, 1, "S2F37 报文")
	if err != nil {
		return ERACKMessage(ERACK2), fmt.Errorf("S2F37: %w", err)
	}
	ceids, err := parseIDListFromListItem(ceidsItem, "S2F37 CEID 列表")
	if err != nil {
		return ERACKMessage(ERACK2), fmt.Errorf("S2F37: %w", err)
	}
	for _, ceid := range ceids {
		if !IsEventDefined(ceid) {
			return ERACKMessage(ERACK1), fmt.Errorf("S2F37: 事件 %s 未定义", ceid)
		}
	}

	deviceMu.Lock()
	for _, ceid := range ceids {
		ev := EventLinks[ceid]
		ev.enable = enable
		EventLinks[ceid] = ev
	}
	deviceMu.Unlock()
	return ERACKMessage(ERACK0), nil
}

// IsVidDefined 检查 VID 是否在任意变量表中定义（使用读锁）
func IsVidDefined(vid string) bool {
	deviceMu.RLock()
	defer deviceMu.RUnlock()
	_, ok1 := SvMap[vid]
	_, ok2 := DvMap[vid]
	_, ok3 := EvMap[vid]
	return ok1 || ok2 || ok3
}

// IsReportDefined 检查报告 ID 是否已定义（使用读锁）
func IsReportDefined(reportID string) bool {
	deviceMu.RLock()
	defer deviceMu.RUnlock()
	_, ok := ReportLinks[reportID]
	return ok
}

// IsEventDefined 检查事件 ID 是否已定义（使用读锁）
func IsEventDefined(eventID string) bool {
	deviceMu.RLock()
	defer deviceMu.RUnlock()
	_, ok := EventLinks[eventID]
	return ok
}

// IsEventValid 检查事件是否已定义且处于 enabled 状态（使用读锁）
func IsEventValid(eventID string) bool {
	deviceMu.RLock()
	defer deviceMu.RUnlock()
	ev, ok := EventLinks[eventID]
	return ok && ev.enable
}

// TriggerEvent 触发指定 CEID 的 S6F11 Collection Event Report
// 若事件未定义或未 enable，返回 error
func TriggerEvent(ceid string) (*secs4go.Message, error) {
	if !IsEventValid(ceid) {
		return nil, fmt.Errorf("事件 %s 未定义或未启用", ceid)
	}
	return buildEvent(ceid)
}

// buildEvent 构建 S6F11 消息（调用前须保证事件已 enabled）
func buildEvent(ceid string) (*secs4go.Message, error) {
	deviceMu.RLock()
	eventLink, ok := EventLinks[ceid]
	if !ok {
		deviceMu.RUnlock()
		return nil, fmt.Errorf("事件 %s 未定义", ceid)
	}
	// 快照 report ID 列表，减少持锁时间
	reportIDs := append([]string(nil), eventLink.links...)
	deviceMu.RUnlock()

	var reportList []*secs4go.Item
	for _, reportID := range reportIDs {
		deviceMu.RLock()
		reportLink, exists := ReportLinks[reportID]
		deviceMu.RUnlock()
		if !exists {
			continue // 报告未定义时跳过，不中断整个事件
		}
		reportList = append(reportList, buildReport(reportID, reportLink))
	}

	return secs4go.NewMessage(6, 11).WithWBit(true).WithItem(secs4go.L(
		secs4go.U2(0),
		secs4go.U4(String2UInt32(ceid)),
		secs4go.L(reportList...),
	)), nil
}

// buildReport 构建单条报告 Item（调用前须自行管理锁）
func buildReport(reportID string, reportLink ReportLink) *secs4go.Item {
	deviceMu.RLock()
	defer deviceMu.RUnlock()

	vals := make([]*secs4go.Item, 0, len(reportLink.links))
	for _, vid := range reportLink.links {
		if dv, ok := DvMap[vid]; ok {
			vals = append(vals, secs4go.A(fmt.Sprint(dv.value))) // 安全：fmt.Sprint 处理任意类型
		} else {
			vals = append(vals, secs4go.A(""))
		}
	}
	return secs4go.L(
		secs4go.U4(String2UInt32(reportID)),
		secs4go.L(vals...),
	)
}

// UpdateDv 更新数据变量值（使用写锁）
func UpdateDv(vid string, value interface{}) error {
	deviceMu.Lock()
	defer deviceMu.Unlock()
	dv, ok := DvMap[vid]
	if !ok {
		return fmt.Errorf("DV %s 未定义", vid)
	}
	dv.value = value
	DvMap[vid] = dv
	return nil
}

// UpdateSv 更新状态变量值（使用写锁）
func UpdateSv(vid string, value interface{}) error {
	deviceMu.Lock()
	defer deviceMu.Unlock()
	sv, ok := SvMap[vid]
	if !ok {
		return fmt.Errorf("SV %s 未定义", vid)
	}
	sv.value = value
	SvMap[vid] = sv
	return nil
}
