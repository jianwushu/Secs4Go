package secs4go_v4

import "encoding/binary"

// ============================================================
// HSMSHeader HSMS消息头 (10字节)
// ============================================================

// HSMSHeaderLength HSMS头长度
const HSMSHeaderLength = 10

// HSMSHeader HSMS消息头
type HSMSHeader struct {
	SessionID   uint16 // 2 bytes
	HeaderByte2 uint8  // 1 byte
	HeaderByte3 uint8  // 1 byte
	PType       PType  // 1 byte
	SType       SType  // 1 byte
	SystemBytes uint32 // 4 bytes
}

// ============================================================
// Header 构建函数
// ============================================================

// BuildDataHeader 构建数据消息头
func BuildDataHeader(sessionID uint16, stream, function uint8, wbit bool, systemBytes uint32) HSMSHeader {
	stream = stream & 0x7F
	if wbit {
		stream |= 0x80
	}

	return HSMSHeader{
		SessionID:   sessionID,
		HeaderByte2: stream,
		HeaderByte3: function,
		PType:       PTypeSECSII,
		SType:       STypeDataMessage,
		SystemBytes: systemBytes,
	}
}

// BuildControlHeader 构建控制消息头
func BuildControlHeader(sType SType, systemBytes uint32, status byte) HSMSHeader {
	return HSMSHeader{
		SessionID:   0xFFFF, // 控制会话固定使用0xFFFF
		HeaderByte2: 0,
		HeaderByte3: status,
		PType:       PTypeSECSII,
		SType:       sType,
		SystemBytes: systemBytes,
	}
}

// BuildSelectRspHeader 构建Select.rsp头
func BuildSelectRspHeader(systemBytes uint32, status byte) HSMSHeader {
	return HSMSHeader{
		SessionID:   0xFFFF,
		HeaderByte2: 0,
		HeaderByte3: status,
		PType:       PTypeSECSII,
		SType:       STypeSelectRsp,
		SystemBytes: systemBytes,
	}
}

// BuildDeselectRspHeader 构建Deselect.rsp头
func BuildDeselectRspHeader(systemBytes uint32, status byte) HSMSHeader {
	return HSMSHeader{
		SessionID:   0xFFFF,
		HeaderByte2: 0,
		HeaderByte3: status,
		PType:       PTypeSECSII,
		SType:       STypeDeselectRsp,
		SystemBytes: systemBytes,
	}
}

// BuildRejectReqHeader 构建Reject.req头
func BuildRejectReqHeader(systemBytes uint32, reason byte) HSMSHeader {
	return HSMSHeader{
		SessionID:   0xFFFF,
		HeaderByte2: 0,
		HeaderByte3: reason,
		PType:       PTypeSECSII,
		SType:       STypeRejectReq,
		SystemBytes: systemBytes,
	}
}

// ============================================================
// Header 编码/解码
// ============================================================

// Encode 编码Header为10字节
func (h *HSMSHeader) Encode() []byte {
	buf := make([]byte, HSMSHeaderLength)

	binary.BigEndian.PutUint16(buf[0:2], h.SessionID)
	buf[2] = h.HeaderByte2
	buf[3] = h.HeaderByte3
	buf[4] = uint8(h.PType)
	buf[5] = uint8(h.SType)
	binary.BigEndian.PutUint32(buf[6:10], h.SystemBytes)

	return buf
}

// DecodeHeader 解码10字节为Header
func DecodeHeader(data []byte) HSMSHeader {
	return HSMSHeader{
		SessionID:   binary.BigEndian.Uint16(data[0:2]),
		HeaderByte2: data[2],
		HeaderByte3: data[3],
		PType:       PType(data[4]),
		SType:       SType(data[5]),
		SystemBytes: binary.BigEndian.Uint32(data[6:10]),
	}
}

// ============================================================
// Header 辅助方法
// ============================================================

// Stream 获取Stream
func (h *HSMSHeader) Stream() uint8 {
	return h.HeaderByte2 & 0x7F
}

// WBit 获取WBit
func (h *HSMSHeader) WBit() bool {
	return (h.HeaderByte2 & 0x80) != 0
}

// Function 获取Function
func (h *HSMSHeader) Function() uint8 {
	return h.HeaderByte3
}

// IsDataMessage 判断是否为数据消息
func (h *HSMSHeader) IsDataMessage() bool {
	return h.SType == STypeDataMessage
}

// IsControlMessage 判断是否为控制消息
func (h *HSMSHeader) IsControlMessage() bool {
	return !h.IsDataMessage()
}

// FormatHeader 格式化Header用于日志
func FormatHeader(h *HSMSHeader) string {
	if h.IsDataMessage() {
		return FormatMessage(&Message{
			Stream:      h.Stream(),
			Function:    h.Function(),
			WBit:        h.WBit(),
			SystemBytes: h.SystemBytes,
		})
	}
	return h.SType.String()
}
