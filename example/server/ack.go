package main

import secs4go "github.com/jianwushu/secs4go/core"

// DACK 常量：S2F34 Define Report Acknowledge
const (
	DACK0 = byte(0) // 成功
	DACK1 = byte(1) // 格式错误
	DACK2 = byte(2) // 报文不是 List
	DACK3 = byte(3) // 报告 ID 已存在
	DACK4 = byte(4) // VID 未定义
)

// LRACK 常量：S2F36 Link Event Report Acknowledge
const (
	LRACK0 = byte(0) // 成功
	LRACK1 = byte(1) // 格式错误
	LRACK2 = byte(2) // 报文不是 List
	LRACK3 = byte(3) // 事件 ID 已存在
	LRACK4 = byte(4) // 报告 ID 未定义
	LRACK5 = byte(5) // 事件 ID 未定义
)

// ERACK 常量：S2F38 Enable/Disable Collection Event Report Acknowledge
const (
	ERACK0 = byte(0) // 成功
	ERACK1 = byte(1) // 事件 ID 未定义
	ERACK2 = byte(2) // 格式错误
)

// DACKMessage 构建 S2F34 回复
func DACKMessage(ack byte) *secs4go.Message {
	return secs4go.NewMessage(2, 34).WithItem(secs4go.B(ack))
}

// LRACKMessage 构建 S2F36 回复
func LRACKMessage(ack byte) *secs4go.Message {
	return secs4go.NewMessage(2, 36).WithItem(secs4go.B(ack))
}

// ERACKMessage 构建 S2F38 回复
func ERACKMessage(ack byte) *secs4go.Message {
	return secs4go.NewMessage(2, 38).WithItem(secs4go.B(ack))
}
