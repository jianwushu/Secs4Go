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
	header HSMSHeader
	data   []byte
	err    error
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
	closed    bool

	// 回调
	msgHandler func(*Message) // 数据消息处理回调

	itemCodec *ItemCodec // 编解码器

	// 独立收发机制：使用 SystemBytes 关联请求和回复
	pendingReplies sync.Map // map[uint32]chan replyResult
}

// NewSecsGem 创建会话
func NewSecsGem(deviceName string, config *Config, codec *ItemCodec) *SecsGem {
	if codec == nil {
		codec = DefaultItemCodec
	}
	return &SecsGem{
		deviceName: deviceName,
		config:     config,
		logger:     NewSilentLogger(),
		itemCodec:  codec,
		done:       make(chan struct{}),
	}
}

// BindTransport 显式绑定 transport、logger 与消息回调
func (s *SecsGem) BindTransport(transport *HSMSTransport, logger Logger) {
	if s == nil {
		return
	}
	if logger == nil {
		logger = NewSilentLogger()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}
	if s.transport != nil && transport != nil && s.transport != transport {
		return
	}
	if s.transport != nil && s.transport != transport {
		s.transport.OnMessage(nil)
	}
	if transport != nil {
		transport.logger = logger
		transport.OnMessage(s.handleDataMessage)
	}

	s.transport = transport
	s.logger = logger
}

// UnbindTransport 显式解绑 transport 与消息回调
func (s *SecsGem) UnbindTransport() {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.transport != nil {
		s.transport.OnMessage(nil)
	}
	s.transport = nil
}

// OnMessage 设置数据消息处理回调（收到数据消息时调用）
func (s *SecsGem) OnMessage(handler func(*Message)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.msgHandler = handler
}

// Close 关闭会话：终止所有等待回复的任务（T3）
// 注意：Close 不会自动停止底层 transport；如需断开连接，请调用 transport.Stop()
//
// 语义约定：Close/断线导致的“等待回复取消”被视为“无回复”，因此不会返回错误。
// 这会吞掉断线错误（可能导致丢消息）；如果上层需要重试/告警，请不要使用该语义。
func (s *SecsGem) Close() {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		if s.transport != nil {
			s.transport.OnMessage(nil)
			s.transport = nil
		}
		s.mu.Unlock()
		close(s.done)
	})
	// 唤醒所有等待 reply 的 goroutine：投递一个空结果（err=nil, data=nil），使其按“无回复”返回。
	s.cancelPendingReplies(nil)
}

func (s *SecsGem) cancelPendingReplies(err error) {
	s.pendingReplies.Range(func(_, v any) bool {
		ch := v.(chan replyResult)
		select {
		case ch <- replyResult{data: nil, err: err}:
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
	if msg == nil {
		return nil, fmt.Errorf("message is nil")
	}
	s.mu.RLock()
	closed := s.closed
	transport := s.transport
	s.mu.RUnlock()
	if closed {
		return nil, fmt.Errorf("secsgem is closed")
	}
	if transport == nil {
		return nil, fmt.Errorf("transport is not bound")
	}

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
	header := BuildDataHeader(s.config.DeviceID, msg.Stream, msg.Function, msg.WBit, transport.NextSystemBytes())
	frameData := BuildCompleteFrame(header, itemData)
	msg.applyProtocolSnapshot(header, itemData, frameData, time.Now())

	// 日志
	s.logger.Info(">>> Send S%dF%d(W=%v, SysBytes=%d)\n%s\n%s",
		msg.Stream, msg.Function, msg.WBit, header.SystemBytes,
		FormatHexData(msg.RawFrame), FormatSMLWithCodec(msg.Item, s.itemCodec))

	// 无需回复的消息
	if !msg.WBit {
		if err := transport.Send(msg.RawFrame); err != nil {
			return nil, fmt.Errorf("send failed: %w", err)
		}
		return nil, nil
	}

	// 需要回复的消息
	reply, err := s.sendAndWait(msg.RawFrame, header)
	if err != nil {
		return nil, fmt.Errorf("send failed: %w", err)
	}

	// 解析回复
	if len(reply.data) == 0 {
		return nil, nil
	}

	parsed, err := ParseMessage(reply.header, reply.data, s.itemCodec)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reply: %v", err)
	}
	return parsed, nil
}

// SendReply 发送回复消息（使用原消息的 SystemBytes）
func (s *SecsGem) SendReply(origMsg *Message, reply *Message) error {
	s.mu.RLock()
	closed := s.closed
	transport := s.transport
	s.mu.RUnlock()
	if closed {
		return fmt.Errorf("secsgem is closed")
	}
	if transport == nil {
		return fmt.Errorf("transport is not bound")
	}
	if origMsg == nil {
		return fmt.Errorf("original message is nil")
	}
	if reply == nil {
		return fmt.Errorf("reply message is nil")
	}

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
	reply.applyProtocolSnapshot(header, itemData, frameData, time.Now())

	// 日志
	s.logger.Info(">>> Send S%dF%d(W=false, SysBytes=%d)\n%s\n%s",
		reply.Stream, reply.Function, header.SystemBytes,
		FormatHexData(reply.RawFrame), FormatSMLWithCodec(reply.Item, s.itemCodec))

	// 发送
	return transport.Send(reply.RawFrame)
}

// sendAndWait 发送并等待回复（独立收发机制）
func (s *SecsGem) sendAndWait(frameData []byte, header HSMSHeader) (replyResult, error) {
	s.mu.RLock()
	closed := s.closed
	transport := s.transport
	s.mu.RUnlock()
	if closed {
		return replyResult{}, fmt.Errorf("secsgem is closed")
	}
	if transport == nil {
		return replyResult{}, fmt.Errorf("transport is not bound")
	}

	systemBytes := header.SystemBytes

	// 创建回复通道
	replyChan := make(chan replyResult, 1)
	s.pendingReplies.Store(systemBytes, replyChan)

	// 确保清理
	defer func() {
		s.pendingReplies.Delete(systemBytes)
	}()

	// 发送
	if err := transport.Send(frameData); err != nil {
		s.logger.Error("[%v] send failed: %v", header.SystemBytes, err)
		return replyResult{}, err
	}

	timer := time.NewTimer(s.config.T3)
	defer timer.Stop()

	// 等待回复或超时 (T3)
	select {
	case res := <-replyChan:
		if res.err != nil {
			return replyResult{}, res.err
		}
		return res, nil
	case <-s.done:
		// 会话关闭：视为无回复
		return replyResult{}, nil
	case <-transport.ConnDone():
		// 连接断开：视为无回复
		return replyResult{}, nil
	case <-timer.C:
		s.logger.Error("Timeout waiting for reply (T3=%v) (SysBytes=%d)", s.config.T3, systemBytes)
		return replyResult{}, ErrTimeoutT3
	}
}

// handleDataMessage 处理收到的数据消息（所有数据会话统一处理）
func (s *SecsGem) handleDataMessage(header HSMSHeader, itemData []byte) {
	s.mu.RLock()
	closed := s.closed
	s.mu.RUnlock()
	if closed {
		return
	}

	msg, err := ParseMessage(header, itemData, s.itemCodec)
	if err != nil {
		s.logger.Error("Failed to parse message: %v", err)
		return
	}
	// 记录接收日志
	s.logReceivedData(msg)

	// 优先按 pending request 关联 reply；未命中时按普通上行消息处理。
	if s.handleReply(header, itemData) {
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
func (s *SecsGem) handleReply(header HSMSHeader, itemData []byte) bool {
	if ch, ok := s.pendingReplies.Load(header.SystemBytes); ok {
		select {
		case ch.(chan replyResult) <- replyResult{header: header, data: itemData}:
		default:
		}
		return true
	}
	return false
}

// logReceivedData 记录数据消息接收日志
func (s *SecsGem) logReceivedData(msg *Message) {
	rawFrame := msg.RawFrame
	s.logger.Info("<<< Recv S%dF%d(W=%v, SysBytes=%d)\n%s\n%s", msg.Stream, msg.Function, msg.WBit, msg.SystemBytes, FormatHexData(rawFrame), FormatSMLWithCodec(msg.Item, s.itemCodec))
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
	if s.transport == nil {
		return false
	}
	return s.transport.IsSelected()
}
