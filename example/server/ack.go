package main

import (
	secs4go_v4 "github.com/jianwushu/secs4go/v4"
)

const (
	DACK0  = 0
	DACK1  = 1
	DACK2  = 2
	DACK3  = 3
	DACK4  = 4
	LRACK0 = 0
	LRACK1 = 1
	LRACK2 = 2
	LRACK3 = 3
	LRACK4 = 4
	LRACK5 = 5
	ERACK0 = 0
	ERACK1 = 1
	ERACK2 = 2
	ERACK3 = 3
	ERACK4 = 4
)

func DACKMessage(ack byte) *secs4go_v4.Message {
	return secs4go_v4.NewMessage(2, 34).WithItem(secs4go_v4.B(ack))
}

func LRACKMessage(ack byte) *secs4go_v4.Message {
	return secs4go_v4.NewMessage(2, 36).WithItem(secs4go_v4.B(ack))
}
