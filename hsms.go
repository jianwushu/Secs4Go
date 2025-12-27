package secs4go

import (
	"encoding/binary"
	"fmt"
	"io"
)

// ============================================================
// HSMS Packet Structure
// ============================================================

// HSMS消息头长度(10字节)
const hsmsHeaderLength = 10

// hsmsPacket HSMS数据包
type hsmsPacket struct {
	// Header (10 bytes)
	SessionID   uint16 // Session ID (2 bytes)
	HeaderByte2 byte   // Header Byte 2
	HeaderByte3 byte   // Header Byte 3
	PType       PType  // Presentation Type (1 byte)
	SType       SType  // Session Type (1 byte)
	SystemBytes uint32 // System Bytes (4 bytes)

	// Data (variable length)
	Data []byte
}

// ============================================================
// HSMS Encoding/Decoding
// ============================================================

// encodeHSMSPacket 编码HSMS数据包
func encodeHSMSPacket(packet *hsmsPacket) ([]byte, error) {
	dataLen := len(packet.Data)
	totalLen := hsmsHeaderLength + dataLen

	buf := make([]byte, totalLen+4) // +4 for length prefix

	// Length (4 bytes, big-endian)
	binary.BigEndian.PutUint32(buf[0:4], uint32(totalLen))

	// Session ID (2 bytes)
	binary.BigEndian.PutUint16(buf[4:6], packet.SessionID)

	// Header Byte 2 & 3
	buf[6] = packet.HeaderByte2
	buf[7] = packet.HeaderByte3

	// PType
	buf[8] = byte(packet.PType)

	// SType
	buf[9] = byte(packet.SType)

	// System Bytes (4 bytes)
	binary.BigEndian.PutUint32(buf[10:14], packet.SystemBytes)

	// Data
	if dataLen > 0 {
		copy(buf[14:], packet.Data)
	}

	return buf, nil
}

// decodeHSMSPacket 解码HSMS数据包
func decodeHSMSPacket(reader io.Reader) (*hsmsPacket, error) {
	// Read length (4 bytes)
	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(reader, lengthBuf); err != nil {
		return nil, fmt.Errorf("failed to read length: %w", err)
	}

	length := binary.BigEndian.Uint32(lengthBuf)
	if length < hsmsHeaderLength {
		return nil, fmt.Errorf("invalid packet length: %d", length)
	}

	// Read header + data
	packetBuf := make([]byte, length)
	if _, err := io.ReadFull(reader, packetBuf); err != nil {
		return nil, fmt.Errorf("failed to read packet: %w", err)
	}

	packet := &hsmsPacket{
		SessionID:   binary.BigEndian.Uint16(packetBuf[0:2]),
		HeaderByte2: packetBuf[2],
		HeaderByte3: packetBuf[3],
		PType:       PType(packetBuf[4]),
		SType:       SType(packetBuf[5]),
		SystemBytes: binary.BigEndian.Uint32(packetBuf[6:10]),
	}

	// Data (if any)
	if length > hsmsHeaderLength {
		packet.Data = packetBuf[10:]
	}

	return packet, nil
}

// ============================================================
// HSMS Control Messages
// ============================================================

// createSelectRsp 创建Select.rsp消息
func createSelectRsp(sessionID uint16, systemBytes uint32, status byte) *hsmsPacket {
	return &hsmsPacket{
		SessionID:   0xFFFF, // 控制会话固定使用0xFFFF
		HeaderByte2: 0,
		HeaderByte3: status,
		PType:       PTypeSECSII,
		SType:       STypeSelectRsp,
		SystemBytes: systemBytes,
		Data:        nil,
	}
}

// createControlMessage 创建控制消息
func createControlMessage(stype SType, systemBytes uint32) *hsmsPacket {
	return &hsmsPacket{
		SessionID:   0xFFFF, // 控制会话固定使用0xFFFF
		HeaderByte2: 0,
		HeaderByte3: 0,
		PType:       PTypeSECSII,
		SType:       stype,
		SystemBytes: systemBytes,
		Data:        nil,
	}
}
