package secs4go

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// ============================================================
// 错误定义
// ============================================================

var ErrTimeoutT3 = errors.New("T3 reply timeout")

type replyResult struct {
	data []byte
	err  error
}

// ============================================================
// SecsGem - SECS/GEM 应用层封装
// 职责: 封装 HSMSTransport，提供简洁的应用层 API 仅处理数据消息（Message）的发送
// ============================================================

// SecsGem SECS/GEM 会话（应用层使用）
type SecsGem struct {
	deviceName string
	config     *Config
	transport  *HSMSTransport
	logger     Logger
	mu         sync.RWMutex

	done      chan struct{}
	closeOnce sync.Once

	// 回调
	msgHandler func(*Message) // 数据消息处理回调

	itemCodec *ItemCodec // 编解码器

	// 独立收发机制：使用 SystemBytes 关联请求和回复
	pendingReplies sync.Map // map[uint32]chan replyResult
}

// NewSecsGem 创建会话
func NewSecsGem(deviceName string, config *Config, hsmsConnection *HSMSTransport, logger Logger, codec *ItemCodec) *SecsGem {
	if logger == nil {
		logger = NewFileLogger(deviceName)
	}
	if codec == nil {
		codec = DefaultItemCodec
	}
	hsmsConnection.logger = logger
	secsGem := &SecsGem{
		deviceName: deviceName,
		config:     config,
		transport:  hsmsConnection,
		logger:     logger,
		itemCodec:  codec,
		done:       make(chan struct{}),
	}
	// 设置数据消息回调（所有数据会话由 secsgem 统一处理）
	hsmsConnection.OnMessage(secsGem.handleDataMessage)
	return secsGem
}

// OnMessage 设置数据消息处理回调（收到数据消息时调用）
func (s *SecsGem) OnMessage(handler func(*Message)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.msgHandler = handler
}

// Close 关闭会话：终止所有等待回复的任务（T3）
// 注意：Close 不会自动停止底层 transport；如需断开连接，请调用 transport.Stop()
func (s *SecsGem) Close() {
	s.closeOnce.Do(func() {
		close(s.done)
	})
	s.cancelPendingReplies(ErrNotConnected)
}

func (s *SecsGem) cancelPendingReplies(err error) {
	s.pendingReplies.Range(func(_, v any) bool {
		ch := v.(chan replyResult)
		select {
		case ch <- replyResult{err: err}:
		default:
		}
		return true
	})
}

// ============================================================
// 消息发送 (应用层API)
// ============================================================

// Send 发送消息并返回回复
// 无需回复的消息返回 (nil, nil)
func (s *SecsGem) Send(msg *Message) (*Message, error) {
	// 编码 Item
	var itemData []byte
	if msg.Item != nil {
		var err error
		itemData, err = s.itemCodec.EncodeItem(msg.Item)
		if err != nil {
			return nil, fmt.Errorf("failed to encode item: %v", err)
		}
	}

	// 构建 HSMSHeader
	header := BuildDataHeader(s.config.DeviceID, msg.Stream, msg.Function, msg.WBit, s.transport.NextSystemBytes())
	frameData := BuildCompleteFrame(header, itemData)

	// 日志
	s.logger.Info(">>> Send S%dF%d(W=%v, SysBytes=%d)\n%s\n%s",
		msg.Stream, msg.Function, msg.WBit, header.SystemBytes,
		FormatHexData(frameData), FormatSML(msg.Item))

	// 无需回复的消息
	if !msg.WBit {
		if err := s.transport.Send(frameData); err != nil {
			return nil, fmt.Errorf("send failed: %w", err)
		}
		return nil, nil
	}

	// 需要回复的消息
	replyData, err := s.sendAndWait(frameData, header)
	if err != nil {
		return nil, fmt.Errorf("send failed: %w", err)
	}

	// 解析回复
	if len(replyData) == 0 {
		return nil, nil
	}

	parsed, err := ParseMessage(header, replyData, s.transport, s.itemCodec)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reply: %v", err)
	}
	return parsed, nil
}

// SendReply 发送回复消息（使用原消息的 SystemBytes）
func (s *SecsGem) SendReply(origMsg *Message, reply *Message) error {
	// 编码 Item
	var itemData []byte
	if reply.Item != nil {
		var err error
		itemData, err = s.itemCodec.EncodeItem(reply.Item)
		if err != nil {
			return fmt.Errorf("failed to encode item: %v", err)
		}
	}

	// 使用原消息的 SystemBytes
	header := BuildDataHeader(s.config.DeviceID, reply.Stream, reply.Function, false, origMsg.SystemBytes)
	frameData := BuildCompleteFrame(header, itemData)

	// 日志
	s.logger.Info(">>> Send S%dF%d(W=false, SysBytes=%d)\n%s\n%s",
		reply.Stream, reply.Function, header.SystemBytes,
		FormatHexData(frameData), FormatSML(reply.Item))

	// 发送
	transport := origMsg.sender
	if transport == nil {
		transport = s.transport
	}
	return transport.Send(frameData)
}

// sendAndWait 发送并等待回复（独立收发机制）
func (s *SecsGem) sendAndWait(frameData []byte, header HSMSHeader) ([]byte, error) {
	systemBytes := header.SystemBytes

	// 创建回复通道
	replyChan := make(chan replyResult, 1)
	s.pendingReplies.Store(systemBytes, replyChan)

	// 确保清理
	defer func() {
		s.pendingReplies.Delete(systemBytes)
	}()

	// 发送
	if err := s.transport.Send(frameData); err != nil {
		s.logger.Error("[%v] send failed: %v", header.SystemBytes, err)
		return nil, err
	}

	timer := time.NewTimer(s.config.T3)
	defer timer.Stop()

	// 等待回复或超时 (T3)
	select {
	case res := <-replyChan:
		if res.err != nil {
			return nil, res.err
		}
		return res.data, nil
	case <-s.done:
		return nil, ErrNotConnected
	case <-s.transport.ConnDone():
		return nil, ErrNotConnected
	case <-timer.C:
		s.logger.Error("Timeout waiting for reply (T3=%v) (SysBytes=%d)", s.config.T3, systemBytes)
		return nil, ErrTimeoutT3
	}
}

// handleDataMessage 处理收到的数据消息（所有数据会话统一处理）
func (s *SecsGem) handleDataMessage(header HSMSHeader, itemData []byte) {

	msg, err := ParseMessage(header, itemData, s.transport, s.itemCodec)
	if err != nil {
		s.logger.Error("Failed to parse message: %v", err)
		return
	}
	// 记录接收日志
	s.logReceivedData(msg)

	// 如果是回复消息（WBit=false），查找等待的请求并发送回复数据
	if !msg.WBit {
		s.handleReply(msg.SystemBytes, itemData)
		return
	}

	// 主消息：向上层回调（复制handler引用，避免竞态）
	s.mu.RLock()
	handler := s.msgHandler
	s.mu.RUnlock()

	// 使用复制后的handler（解锁后可能已被修改，但复制值不变）
	if handler != nil {
		handler(msg)
	}
}

// handleReply 处理收到的回复消息（由 handleDataMessage 调用）
func (s *SecsGem) handleReply(systemBytes uint32, itemData []byte) {
	if ch, ok := s.pendingReplies.Load(systemBytes); ok {
		select {
		case ch.(chan replyResult) <- replyResult{data: itemData}:
		default:
		}
	}
}

// logReceivedData 记录数据消息接收日志
func (s *SecsGem) logReceivedData(msg *Message) {
	header := BuildDataHeader(s.config.DeviceID, msg.Stream, msg.Function, msg.WBit, msg.SystemBytes)
	var itemData []byte
	if msg.Item != nil {
		var err error
		itemData, err = s.itemCodec.EncodeItem(msg.Item)
		if err != nil {
			s.logger.Error("Failed to encode item for log: %v", err)
			return
		}
	}
	frameData := BuildCompleteFrame(header, itemData)
	s.logger.Info("<<< Recv S%dF%d(W=%v, SysBytes=%d)\n%s\n%s", msg.Stream, msg.Function, msg.WBit, msg.SystemBytes, FormatHexData(frameData), FormatSML(msg.Item))
}

// sendDefaultReply 发送默认回复 (上层未处理时)
func (s *SecsGem) SendDefaultReply(msg *Message) {
	if msg.WBit {
		reply := NewMessage(msg.Stream, msg.Function+1).WithItem(B(0))
		s.SendReply(msg, reply)
	}

}

// ============================================================
// 辅助方法
// ============================================================

// IsActive 检查是否客户端模式
func (s *SecsGem) IsActive() bool {
	return s.config.IsActive
}

// IsSelected 检查是否已 Select
func (s *SecsGem) IsSelected() bool {
	return s.transport.IsSelected()
}
