package main

import (
	secs4go "github.com/jianwushu/secs4go/core"
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
)

func DACKMessage(ack byte) *secs4go.Message {
	return secs4go.NewMessage(2, 34).WithItem(secs4go.B(ack))
}

func LRACKMessage(ack byte) *secs4go.Message {
	return secs4go.NewMessage(2, 36).WithItem(secs4go.B(ack))
}

func ERACKMessage(ack byte) *secs4go.Message {
	return secs4go.NewMessage(2, 38).WithItem(secs4go.B(ack))
}
