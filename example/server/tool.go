package main

import (
	"log"
	"strconv"
)

// String2UInt32 将字符串转为 uint32。
// 转换失败时返回 0 并打印告警日志，避免静默错误。
func String2UInt32(str string) uint32 {
	num, err := strconv.ParseUint(str, 10, 32)
	if err != nil {
		log.Printf("[WARN] String2UInt32: 无法解析 %q: %v，已回退为 0", str, err)
		return 0
	}
	return uint32(num)
}
