package secs4go

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ============================================================
// 错误定义
// ============================================================

var ErrInvalidFrame = errors.New("invalid HSMS frame")

// ReadHSMSFrame 读取HSMS帧
// 返回: 头部(10字节), SECS-II数据(Item), 错误
func ReadHSMSFrame(reader io.Reader) (HSMSHeader, []byte, error) {
	// 读取4字节长度
	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(reader, lengthBuf); err != nil {
		return HSMSHeader{}, nil, err
	}

	frameLen := binary.BigEndian.Uint32(lengthBuf)
	if frameLen < HSMSHeaderLength {
		return HSMSHeader{}, nil, ErrInvalidFrame
	}

	// 读取头部 + 数据
	dataLen := int(frameLen) - HSMSHeaderLength
	frameData := make([]byte, frameLen)
	if _, err := io.ReadFull(reader, frameData); err != nil {
		return HSMSHeader{}, nil, err
	}

	// 解析头部
	header := DecodeHeader(frameData[:HSMSHeaderLength])

	// 提取SECS-II数据 (Item)
	var itemData []byte
	if dataLen > 0 {
		itemData = frameData[HSMSHeaderLength:]
	}

	return header, itemData, nil
}

// ============================================================
// 格式化工具
// ============================================================

// FormatHexData 格式化16进制数据(每个字节用空格隔开)
func FormatHexData(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	hex := make([]string, len(data))
	for i, b := range data {
		hex[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(hex, " ")
}

// BuildCompleteFrame 格式化完整帧数据 (4B长度 + 10B头部 + 数据)
func BuildCompleteFrame(header HSMSHeader, itemData []byte) []byte {
	headerBytes := header.Encode()
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(len(headerBytes)+len(itemData)))
	frameBytes := append(lengthBuf, headerBytes...)
	frameBytes = append(frameBytes, itemData...)
	return frameBytes
}
