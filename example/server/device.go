package main

import (
	"fmt"
	"strconv"

	secs4go_v4 "github.com/jianwushu/secs4go/v4"
)

// 变量声明
type SVID struct {
	name  string
	value interface{} // A Ux Ix
	unit  string
}

func (v *SVID) ValueToString() string {
	switch val := v.value.(type) {
	case string:
		return val
	case int:
		return strconv.Itoa(val)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
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
	DvMap = map[string]DVID{
		"1001": {"CRR_START_ID", ""},
		"1002": {"CRR_START_TIME", ""},
		"1003": {"CRR_END_ID", ""},
		"1004": {"CRR_END_TIME", ""},
		"1005": {"CRR_IN_MAP", ""},
		"1006": {"CRR_OUT_MAP", ""},
	}
	EvMap = map[string]ECID{
		"3001": {"T3", 45, 45, 45, 45, "s"},
		"3002": {"T5", 10, 10, 10, 10, "s"},
	}

	EventLinks = map[string]EventLink{
		"10020": {true, []string{"10020"}},
	}

	ReportLinks = map[string]ReportLink{
		"10020": {[]string{"1001", "1002"}},
	}
)

// Selected Equipment Status Request (SSR)
func HandleS1F3(item *secs4go_v4.Item) *secs4go_v4.Message {

	// 解析S1F3数据
	var svList []string
	if item.IsList() {
		for i := 0; i < item.GetLength(); i++ {
			child := item.GetItem(i)
			switch child.Type {
			case secs4go_v4.TypeUInt16:
				svList = append(svList, fmt.Sprint(child.Value.([]uint16)[0]))
			case secs4go_v4.TypeUInt32:
				svList = append(svList, fmt.Sprint(child.Value.([]uint32)[0]))
			default:
				svList = append(svList, fmt.Sprint(child.Value.([]int16)[0]))
			}
		}
	}

	// 构建S1F4返回数据
	var replyVidVals []*secs4go_v4.Item
	if len(svList) == 0 {
		for _, vid := range SvMap {
			replyVidVals = append(replyVidVals, secs4go_v4.A(vid.ValueToString()))
		}
	} else {
		for _, sv := range svList {
			if vid, ok := SvMap[sv]; ok {
				replyVidVals = append(replyVidVals, secs4go_v4.A(vid.ValueToString()))
			} else {
				replyVidVals = append(replyVidVals, secs4go_v4.L())
			}
		}
	}

	if len(replyVidVals) != 0 {
		return secs4go_v4.NewMessage(1, 4).WithItem(secs4go_v4.L(replyVidVals...))
	}

	return secs4go_v4.NewMessage(1, 4).WithItem(secs4go_v4.L())
}

// Status Variable Namelist Request (SVNR)
func HandleS1F11(item *secs4go_v4.Item) *secs4go_v4.Message {

	// 解析S1F11数据
	var svList []string
	if item.IsList() {
		for i := 0; i < item.GetLength(); i++ {
			child := item.GetItem(i)
			switch child.Type {
			case secs4go_v4.TypeUInt16:
				svList = append(svList, fmt.Sprint(child.Value.([]uint16)[0]))
			case secs4go_v4.TypeUInt32:
				svList = append(svList, fmt.Sprint(child.Value.([]uint32)[0]))
			default:
				svList = append(svList, fmt.Sprint(child.Value.([]int16)[0]))
			}
		}
	}

	// 构建S1F12返回数据
	var replyVidVals []*secs4go_v4.Item
	if len(svList) == 0 {
		for key, vid := range SvMap {
			replyVidVals = append(replyVidVals, secs4go_v4.L(
				secs4go_v4.U4(String2UInt32(key)),
				secs4go_v4.A(vid.name),
				secs4go_v4.A(vid.unit),
			))
		}
	} else {
		for _, sv := range svList {
			if vid, ok := SvMap[sv]; ok {
				replyVidVals = append(replyVidVals, secs4go_v4.L(
					secs4go_v4.U4(String2UInt32(sv)),
					secs4go_v4.A(vid.name),
					secs4go_v4.A(vid.unit),
				))
			} else {
				replyVidVals = append(replyVidVals, secs4go_v4.L(
					secs4go_v4.U4(String2UInt32(sv)),
					secs4go_v4.A(""),
					secs4go_v4.A(""),
				))
			}
		}
	}

	if len(replyVidVals) != 0 {
		return secs4go_v4.NewMessage(1, 4).WithItem(secs4go_v4.L(replyVidVals...))
	}

	return secs4go_v4.NewMessage(1, 12).WithItem(secs4go_v4.L())
}

// Equipment Constant Request (ECR)
func HandleS2F13(item *secs4go_v4.Item) *secs4go_v4.Message {
	// 解析S2F13数据
	var evList []string
	if item.IsList() {
		for i := 0; i < item.GetLength(); i++ {
			child := item.GetItem(i)
			switch child.Type {
			case secs4go_v4.TypeUInt16:
				evList = append(evList, fmt.Sprint(child.Value.([]uint16)[0]))
			case secs4go_v4.TypeUInt32:
				evList = append(evList, fmt.Sprint(child.Value.([]uint32)[0]))
			default:
				evList = append(evList, fmt.Sprint(child.Value.([]int16)[0]))
			}
		}
	}

	// 构建S2F14返回数据
	var replyVidVals []*secs4go_v4.Item
	if len(evList) == 0 {
		for _, vid := range SvMap {
			replyVidVals = append(replyVidVals, secs4go_v4.A(vid.ValueToString()))
		}
	} else {
		for _, sv := range evList {
			if vid, ok := SvMap[sv]; ok {
				replyVidVals = append(replyVidVals, secs4go_v4.A(vid.ValueToString()))
			} else {
				replyVidVals = append(replyVidVals, secs4go_v4.L())
			}
		}
	}

	if len(replyVidVals) != 0 {
		return secs4go_v4.NewMessage(2, 14).WithItem(secs4go_v4.L(replyVidVals...))
	}

	return secs4go_v4.NewMessage(2, 14).WithItem(secs4go_v4.L())
}

// Equipment Constant Namelist Request (ECNR)
func HandleS2F29(item *secs4go_v4.Item) *secs4go_v4.Message {
	// 解析S2F29数据
	var evList []string
	if item.IsList() {
		for i := 0; i < item.GetLength(); i++ {
			child := item.GetItem(i)
			switch child.Type {
			case secs4go_v4.TypeUInt16:
				evList = append(evList, fmt.Sprint(child.Value.([]uint16)[0]))
			case secs4go_v4.TypeUInt32:
				evList = append(evList, fmt.Sprint(child.Value.([]uint32)[0]))
			default:
				evList = append(evList, fmt.Sprint(child.Value.([]int16)[0]))
			}
		}
	}

	// 构建S1F12返回数据
	var replyVidVals []*secs4go_v4.Item
	if len(evList) == 0 {
		for key, vid := range EvMap {
			replyVidVals = append(replyVidVals, secs4go_v4.L(
				secs4go_v4.U4(String2UInt32(key)),
				secs4go_v4.A(vid.name),
				secs4go_v4.F4(vid.min.(float32)),
				secs4go_v4.F4(vid.max.(float32)),
				secs4go_v4.F4(vid.def.(float32)),
				secs4go_v4.A(vid.unit),
			))
		}
	} else {
		for _, sv := range evList {
			if vid, ok := EvMap[sv]; ok {
				replyVidVals = append(replyVidVals, secs4go_v4.L(
					secs4go_v4.U4(String2UInt32(sv)),
					secs4go_v4.A(vid.name),
					secs4go_v4.F4(vid.min.(float32)),
					secs4go_v4.F4(vid.max.(float32)),
					secs4go_v4.F4(vid.def.(float32)),
					secs4go_v4.A(vid.unit),
				))
			} else {
				replyVidVals = append(replyVidVals, secs4go_v4.L(
					secs4go_v4.U4(String2UInt32(sv)),
					secs4go_v4.A(""),
					secs4go_v4.A(""),
					secs4go_v4.A(""),
					secs4go_v4.A(""),
					secs4go_v4.A(""),
				))
			}
		}
	}

	if len(replyVidVals) != 0 {
		return secs4go_v4.NewMessage(2, 30).WithItem(secs4go_v4.L(replyVidVals...))
	}

	return secs4go_v4.NewMessage(2, 30).WithItem(secs4go_v4.L())
}

// Define Report (DR)
func HandleS2F33(item *secs4go_v4.Item) (*secs4go_v4.Message, error) {

	var reportListMap = map[string]ReportLink{}

	if item.IsList() {
		dataItem := item.GetItem(1)
		for i := 0; i < dataItem.GetLength(); i++ {
			child := dataItem.GetItem(i)
			if child.IsList() && child.GetLength() == 2 {
				reportID := fmt.Sprint(child.GetItem(0).Value.([]uint16)[0])

				if IsReportDefined(reportID) {
					return DACKMessage(DACK3), fmt.Errorf("report %s already defined", reportID)
				}

				vidItem := child.GetItem(1)

				vids := []string{}
				for j := 0; j < vidItem.GetLength(); j++ {
					vid := fmt.Sprint(vidItem.GetItem(j).Value.([]uint16)[0])
					if !IsVidDefined(vid) {
						return DACKMessage(DACK4), fmt.Errorf("vid %s not defined", vid)
					}
					vids = append(vids, vid)
				}
				reportListMap[reportID] = ReportLink{vids}
			} else {
				return DACKMessage(DACK1), fmt.Errorf("report format error")
			}
		}
		//  if empty clear all
		if len(reportListMap) == 0 {
			ReportLinks = map[string]ReportLink{}
		} else {
			for reportID, reportLink := range reportListMap {
				ReportLinks[reportID] = reportLink
			}
		}
		return DACKMessage(DACK0), nil
	}
	return DACKMessage(DACK2), fmt.Errorf("report format error")
}

// Link Event Report (LER)
func HandleS2F35(item *secs4go_v4.Item) (*secs4go_v4.Message, error) {
	var eventListMap = map[string]EventLink{}

	if item.IsList() {
		dataItem := item.GetItem(1)
		for i := 0; i < dataItem.GetLength(); i++ {
			child := dataItem.GetItem(i)
			if child.IsList() && child.GetLength() == 2 {
				eventID := fmt.Sprint(child.GetItem(0).Value.([]uint16)[0])

				if IsEventDefined(eventID) {
					return LRACKMessage(DACK3), fmt.Errorf("event %s already defined", eventID)
				}

				vidItem := child.GetItem(1)

				reportIDs := []string{}
				for j := 0; j < vidItem.GetLength(); j++ {
					reportID := fmt.Sprint(vidItem.GetItem(j).Value.([]uint16)[0])
					if !IsReportDefined(reportID) {
						return LRACKMessage(DACK4), fmt.Errorf("report %s not defined", reportID)
					}
					reportIDs = append(reportIDs, reportID)
				}
				eventListMap[eventID] = EventLink{false, reportIDs}
			} else {
				return LRACKMessage(DACK1), fmt.Errorf("event format error")
			}
		}
		//  if empty clear all
		if len(eventListMap) == 0 {
			EventLinks = map[string]EventLink{}
		} else {
			for eventID, eventLink := range eventListMap {
				EventLinks[eventID] = eventLink
			}
		}
		return LRACKMessage(DACK0), nil
	}
	return LRACKMessage(DACK2), fmt.Errorf("event format error")
}

// 检查vid是否定义
func IsVidDefined(vid string) bool {
	_, ok := SvMap[vid]
	_, ok2 := DvMap[vid]
	_, ok3 := EvMap[vid]
	return ok || ok2 || ok3
}

// 检查report是否定义
func IsReportDefined(reportID string) bool {
	_, ok := ReportLinks[reportID]
	return ok
}

// 检查event是否定义
func IsEventDefined(eventID string) bool {
	_, ok := EventLinks[eventID]
	return ok
}
