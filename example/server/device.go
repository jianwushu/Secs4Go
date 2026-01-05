package main

import (
	"fmt"
	"strconv"

	secs4go "github.com/jianwushu/secs4go/core"
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
	case int, int16, int32, int64:
		return fmt.Sprint(val.(int64))
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
		"1001": {"CRR_START_ID", "t"},
		"1002": {"CRR_START_TIME", "tt"},
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

// Are You There Request (R)
func HandleS1F1(item *secs4go.Item) *secs4go.Message {
	return secs4go.NewMessage(1, 2).WithItem(secs4go.L(
		secs4go.A(SvMap["2007"].value.(string)),
		secs4go.A(SvMap["2008"].value.(string)),
	))
}

// Selected Equipment Status Request (SSR)
func HandleS1F3(item *secs4go.Item) *secs4go.Message {

	// 解析S1F3数据
	var svList []string
	if item.IsList() {
		for i := 0; i < item.GetLength(); i++ {
			child := item.GetItem(i)
			switch child.Type {
			case secs4go.TypeUInt16:
				svList = append(svList, fmt.Sprint(child.Value.([]uint16)[0]))
			case secs4go.TypeUInt32:
				svList = append(svList, fmt.Sprint(child.Value.([]uint32)[0]))
			default:
				svList = append(svList, fmt.Sprint(child.Value.([]int16)[0]))
			}
		}
	}

	// 构建S1F4返回数据
	var replyVidVals []*secs4go.Item
	if len(svList) == 0 {
		for _, vid := range SvMap {
			replyVidVals = append(replyVidVals, secs4go.A(vid.ValueToString()))
		}
	} else {
		for _, sv := range svList {
			if vid, ok := SvMap[sv]; ok {
				replyVidVals = append(replyVidVals, secs4go.A(vid.ValueToString()))
			} else {
				replyVidVals = append(replyVidVals, secs4go.L())
			}
		}
	}

	if len(replyVidVals) != 0 {
		return secs4go.NewMessage(1, 4).WithItem(secs4go.L(replyVidVals...))
	}

	return secs4go.NewMessage(1, 4).WithItem(secs4go.L())
}

// Status Variable Namelist Request (SVNR)
func HandleS1F11(item *secs4go.Item) *secs4go.Message {

	// 解析S1F11数据
	var svList []string
	if item.IsList() {
		for i := 0; i < item.GetLength(); i++ {
			child := item.GetItem(i)
			switch child.Type {
			case secs4go.TypeUInt16:
				svList = append(svList, fmt.Sprint(child.Value.([]uint16)[0]))
			case secs4go.TypeUInt32:
				svList = append(svList, fmt.Sprint(child.Value.([]uint32)[0]))
			default:
				svList = append(svList, fmt.Sprint(child.Value.([]int16)[0]))
			}
		}
	}

	// 构建S1F12返回数据
	var replyVidVals []*secs4go.Item
	if len(svList) == 0 {
		for key, vid := range SvMap {
			replyVidVals = append(replyVidVals, secs4go.L(
				secs4go.U4(String2UInt32(key)),
				secs4go.A(vid.name),
				secs4go.A(vid.unit),
			))
		}
	} else {
		for _, sv := range svList {
			if vid, ok := SvMap[sv]; ok {
				replyVidVals = append(replyVidVals, secs4go.L(
					secs4go.U4(String2UInt32(sv)),
					secs4go.A(vid.name),
					secs4go.A(vid.unit),
				))
			} else {
				replyVidVals = append(replyVidVals, secs4go.L(
					secs4go.U4(String2UInt32(sv)),
					secs4go.A(""),
					secs4go.A(""),
				))
			}
		}
	}

	if len(replyVidVals) != 0 {
		return secs4go.NewMessage(1, 4).WithItem(secs4go.L(replyVidVals...))
	}

	return secs4go.NewMessage(1, 12).WithItem(secs4go.L())
}

// Establish Communications Request (CR)
func HandleS1F13(item *secs4go.Item) *secs4go.Message {
	return secs4go.NewMessage(1, 14).WithItem(secs4go.L(
		secs4go.B(0),
		secs4go.L(
			secs4go.A(SvMap["2007"].value.(string)),
			secs4go.A(SvMap["2008"].value.(string)),
		),
	))
}

// Equipment Constant Request (ECR)
func HandleS2F13(item *secs4go.Item) *secs4go.Message {
	// 解析S2F13数据
	var evList []string
	if item.IsList() {
		for i := 0; i < item.GetLength(); i++ {
			child := item.GetItem(i)
			switch child.Type {
			case secs4go.TypeUInt16:
				evList = append(evList, fmt.Sprint(child.Value.([]uint16)[0]))
			case secs4go.TypeUInt32:
				evList = append(evList, fmt.Sprint(child.Value.([]uint32)[0]))
			default:
				evList = append(evList, fmt.Sprint(child.Value.([]int16)[0]))
			}
		}
	}

	// 构建S2F14返回数据
	var replyVidVals []*secs4go.Item
	if len(evList) == 0 {
		for _, vid := range SvMap {
			replyVidVals = append(replyVidVals, secs4go.A(vid.ValueToString()))
		}
	} else {
		for _, sv := range evList {
			if vid, ok := SvMap[sv]; ok {
				replyVidVals = append(replyVidVals, secs4go.A(vid.ValueToString()))
			} else {
				replyVidVals = append(replyVidVals, secs4go.L())
			}
		}
	}

	if len(replyVidVals) != 0 {
		return secs4go.NewMessage(2, 14).WithItem(secs4go.L(replyVidVals...))
	}

	return secs4go.NewMessage(2, 14).WithItem(secs4go.L())
}

// Equipment Constant Namelist Request (ECNR)
func HandleS2F29(item *secs4go.Item) *secs4go.Message {
	// 解析S2F29数据
	var evList []string
	if item.IsList() {
		for i := 0; i < item.GetLength(); i++ {
			child := item.GetItem(i)
			switch child.Type {
			case secs4go.TypeUInt16:
				evList = append(evList, fmt.Sprint(child.Value.([]uint16)[0]))
			case secs4go.TypeUInt32:
				evList = append(evList, fmt.Sprint(child.Value.([]uint32)[0]))
			default:
				evList = append(evList, fmt.Sprint(child.Value.([]int16)[0]))
			}
		}
	}

	// 构建S1F12返回数据
	var replyVidVals []*secs4go.Item
	if len(evList) == 0 {
		for key, vid := range EvMap {
			replyVidVals = append(replyVidVals, secs4go.L(
				secs4go.U4(String2UInt32(key)),
				secs4go.A(vid.name),
				secs4go.F4(vid.min.(float32)),
				secs4go.F4(vid.max.(float32)),
				secs4go.F4(vid.def.(float32)),
				secs4go.A(vid.unit),
			))
		}
	} else {
		for _, sv := range evList {
			if vid, ok := EvMap[sv]; ok {
				replyVidVals = append(replyVidVals, secs4go.L(
					secs4go.U4(String2UInt32(sv)),
					secs4go.A(vid.name),
					secs4go.F4(vid.min.(float32)),
					secs4go.F4(vid.max.(float32)),
					secs4go.F4(vid.def.(float32)),
					secs4go.A(vid.unit),
				))
			} else {
				replyVidVals = append(replyVidVals, secs4go.L(
					secs4go.U4(String2UInt32(sv)),
					secs4go.A(""),
					secs4go.A(""),
					secs4go.A(""),
					secs4go.A(""),
					secs4go.A(""),
				))
			}
		}
	}

	if len(replyVidVals) != 0 {
		return secs4go.NewMessage(2, 30).WithItem(secs4go.L(replyVidVals...))
	}

	return secs4go.NewMessage(2, 30).WithItem(secs4go.L())
}

// Define Report (DR)
func HandleS2F33(item *secs4go.Item) (*secs4go.Message, error) {

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
func HandleS2F35(item *secs4go.Item) (*secs4go.Message, error) {
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

func HandleS2F37(item *secs4go.Item) (*secs4go.Message, error) {

	ceidList := []string{}

	if item.IsList() {
		enable := item.GetItem(0).Value.([]bool)[0]
		dataItem := item.GetItem(1)
		for i := 0; i < dataItem.GetLength(); i++ {
			child := dataItem.GetItem(i)
			ceid := fmt.Sprint(child.Value.([]uint16)[0])
			if !IsEventDefined(ceid) {
				return ERACKMessage(ERACK1), fmt.Errorf("event %s not defined", ceid)
			}
			ceidList = append(ceidList, ceid)
		}

		for _, ceid := range ceidList {
			targetEvent := EventLinks[ceid]
			targetEvent.enable = enable
			EventLinks[ceid] = targetEvent
		}
		return LRACKMessage(ERACK0), nil
	}
	return ERACKMessage(ERACK2), fmt.Errorf("invalid format")
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

// 检查event是否有效
func IsEventValid(eventID string) bool {
	_, ok := EventLinks[eventID]
	return ok && EventLinks[eventID].enable
}

func Trigger10020() (*secs4go.Message, error) {

	if !IsEventValid("10020") {
		return nil, fmt.Errorf("event 10020 not enabled")
	}

	return buildEvent("10020")
}

func Trigger10021() (*secs4go.Message, error) {

	return nil, nil
}

func Trigger10050() (*secs4go.Message, error) {

	return nil, nil
}

func buildEvent(ceid string) (*secs4go.Message, error) {

	eventLink, ok := EventLinks[ceid]
	if !ok {
		return nil, fmt.Errorf("event %s not defined", ceid)
	}

	reportList := []*secs4go.Item{}
	for _, reportID := range eventLink.links {
		reportLink, ok := ReportLinks[reportID]
		if !ok {
			// return nil, fmt.Errorf("report %s not defined", reportID)
			continue
		}

		report, _ := buildReport(reportID, reportLink)
		reportList = append(reportList, report)
	}

	s6f11 := secs4go.NewMessage(6, 11).WithItem(secs4go.L(
		secs4go.U2(0),
		secs4go.U4(String2UInt32(ceid)),
		secs4go.L(reportList...),
	))

	return s6f11, nil
}

func buildReport(reportID string, reportLink ReportLink) (*secs4go.Item, error) {

	vids := []*secs4go.Item{}

	for _, vid := range reportLink.links {
		dv, ok := DvMap[vid]
		if !ok {
			vids = append(vids, secs4go.A(""))
		} else {
			vids = append(vids, secs4go.A(dv.value.(string)))
		}
	}

	report := secs4go.L(
		secs4go.U4(String2UInt32(reportID)),
		secs4go.L(vids...),
	)

	return report, nil
}

func UpdateDv(vid string, value interface{}) error {
	dv, ok := DvMap[vid]
	if !ok {
		return fmt.Errorf("vid %s not defined", vid)
	}
	dv.value = value
	DvMap[vid] = dv
	return nil
}
