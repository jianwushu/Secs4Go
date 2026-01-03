package main

import "strconv"

func String2Int8(str string) int8 {
	num, err := strconv.ParseInt(str, 10, 8)
	if err != nil {
		return 0
	}
	return int8(num)
}

func String2Int16(str string) int16 {
	num, err := strconv.ParseInt(str, 10, 16)
	if err != nil {
		return 0
	}
	return int16(num)
}

func String2Int32(str string) int32 {
	num, err := strconv.ParseInt(str, 10, 32)
	if err != nil {
		return 0
	}
	return int32(num)
}

func String2Int64(str string) int64 {
	num, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return 0
	}
	return num
}

func String2UInt8(str string) uint8 {
	num, err := strconv.ParseUint(str, 10, 8)
	if err != nil {
		return 0
	}
	return uint8(num)
}

func String2UInt16(str string) uint16 {
	num, err := strconv.ParseUint(str, 10, 16)
	if err != nil {
		return 0
	}
	return uint16(num)
}

func String2UInt32(str string) uint32 {
	num, err := strconv.ParseUint(str, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(num)
}

func String2UInt64(str string) uint64 {
	num, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		return 0
	}
	return num
}

func String2Float32(str string) float32 {
	num, err := strconv.ParseFloat(str, 32)
	if err != nil {
		return 0
	}
	return float32(num)
}

func String2Float64(str string) float64 {
	num, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return 0
	}
	return num
}
