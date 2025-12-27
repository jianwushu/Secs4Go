package secs4go

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// ============================================================
// Session - 核心会话管理
// ============================================================

// Session SECS/HSMS会话
type Session struct {
	config *Config
	conn   net.Conn
	state  ConnectionState

	// Passive模式监听器
	listener net.Listener

	// 消息处理
	messageHandler MessageHandler
	pendingReplies sync.Map // map[uint32]chan *Message

	// 状态变更回调
	stateChangeHandler StateChangeHandler

	// 自动化
	reconnectChan chan struct{}
	stopChan      chan struct{}
	readyChan     chan struct{} // 连接就绪通知
	wg            sync.WaitGroup

	// 同步
	mu         sync.RWMutex
	systemByte uint32

	// 上下文
	ctx    context.Context
	cancel context.CancelFunc
}

// MessageHandler 消息处理器
type MessageHandler func(msg *Message) error

// NewSession 创建新会话(内部使用)
func newSession(config *Config) (*Session, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	s := &Session{
		config:        config,
		state:         StateDisconnected,
		reconnectChan: make(chan struct{}, 1),
		stopChan:      make(chan struct{}),
		readyChan:     make(chan struct{}),
		ctx:           ctx,
		cancel:        cancel,
	}

	return s, nil
}

// Start 启动会话
func (s *Session) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != StateDisconnected {
		return fmt.Errorf("session already started")
	}

	if s.config.IsActive {
		// Active模式:主动连接
		connectFailed := false
		if err := s.connect(); err != nil {
			if !s.config.AutoReconnect {
				return err
			}
			// 如果启用自动重连,不返回错误
			s.config.Logger.Warn("Initial connection failed, will retry: %v", err)
			connectFailed = true
		}

		// 启动自动重连
		if s.config.AutoReconnect {
			s.wg.Add(1)
			go s.autoReconnectLoop()

			// 如果初始连接失败,触发重连
			if connectFailed {
				select {
				case s.reconnectChan <- struct{}{}:
				default:
				}
			}
		}

		// 启动自动心跳
		if s.config.EnableHeartbeat {
			s.wg.Add(1)
			go s.heartbeatLoop()
		}

		// 启动消息接收
		s.wg.Add(1)
		go s.receiveLoop()
	} else {
		// Passive模式:监听连接
		listener, err := net.Listen("tcp", s.config.Address)
		if err != nil {
			return fmt.Errorf("failed to listen: %w", err)
		}

		s.listener = listener
		s.state = StateConnected
		s.config.Logger.Info("Listening on %s", s.config.Address)

		// 启动接受连接的goroutine
		s.wg.Add(1)
		go s.acceptLoop()
	}

	return nil
}

// WaitReady 等待连接就绪(Selected状态)
func (s *Session) WaitReady() error {
	// 先检查当前状态
	if s.IsReady() {
		return nil
	}

	// 等待就绪信号
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(s.config.T7)

	for {
		select {
		case <-s.readyChan:
			return nil
		case <-ticker.C:
			// 定期检查状态(防止信号丢失)
			if s.IsReady() {
				return nil
			}
		case <-s.ctx.Done():
			return fmt.Errorf("session stopped")
		case <-timeout:
			// 最后再检查一次
			if s.IsReady() {
				return nil
			}
			return fmt.Errorf("timeout waiting for ready (T7=%v)", s.config.T7)
		}
	}
}

// IsReady 检查是否就绪
func (s *Session) IsReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state == StateSelected
}

// Stop 停止会话
func (s *Session) Stop() error {
	s.mu.Lock()
	if s.state == StateDisconnected {
		s.mu.Unlock()
		return nil
	}

	oldState := s.state
	conn := s.conn
	s.mu.Unlock()

	// 程序退出前发送Separate断开请求(仅在Selected状态)
	if oldState == StateSelected && conn != nil {
		// 同步发送Separate请求（不等待回复）
		s.sendSeparateNoLock(conn)
	}

	// 关闭监听器(Passive模式)
	if s.listener != nil {
		s.listener.Close()
	}

	// 停止所有goroutine
	close(s.stopChan)
	s.cancel()

	// 等待所有goroutine结束
	s.wg.Wait()

	// 最后关闭连接
	s.mu.Lock()
	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}
	s.state = StateDisconnected
	s.mu.Unlock()

	if s.stateChangeHandler != nil && oldState != StateDisconnected {
		s.stateChangeHandler(oldState, StateDisconnected)
	}

	s.config.Logger.Info("Session stopped")

	return nil
}

// connect 建立连接(内部使用,仅用于Active模式)
func (s *Session) connect() error {
	// Active模式:连接到服务器
	conn, err := net.DialTimeout("tcp", s.config.Address, s.config.T5)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	s.conn = conn
	oldState := s.state
	s.state = StateConnected
	s.mu.Unlock()

	if s.stateChangeHandler != nil && oldState != StateConnected {
		s.stateChangeHandler(oldState, StateConnected)
	}

	s.config.Logger.Info("Connected to %s", s.config.Address)

	// 发送Select.req
	if err := s.sendControlMessage(STypeSelectReq, s.systemByte+1); err != nil {
		s.conn.Close()
		s.conn = nil
		s.state = StateDisconnected
		return fmt.Errorf("failed to send select: %w", err)
	}
	s.mu.Lock()

	return nil
}

// acceptLoop 接受连接循环(Passive模式)
func (s *Session) acceptLoop() {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopChan:
			return
		default:
		}

		// 接受连接
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stopChan:
				return
			default:
				s.config.Logger.Error("Failed to accept connection: %v", err)
				continue
			}
		}

		s.config.Logger.Info("Accepted connection from %s", conn.RemoteAddr())

		// 设置连接
		s.mu.Lock()
		if s.conn != nil {
			// 已有连接,关闭旧连接
			s.conn.Close()
		}
		s.conn = conn
		oldState := s.state
		s.state = StateConnected
		s.mu.Unlock()
		if s.stateChangeHandler != nil && oldState != StateConnected {
			s.stateChangeHandler(oldState, StateConnected)
		}
		// s.mu.Lock()

		// 启动消息接收
		s.wg.Add(1)
		go s.receiveLoop()

		// 启动心跳(如果启用)
		if s.config.EnableHeartbeat {
			s.wg.Add(1)
			go s.heartbeatLoop()
		}
	}
}

// ============================================================
// 自动化功能
// ============================================================

// autoReconnectLoop 自动重连循环
func (s *Session) autoReconnectLoop() {
	defer s.wg.Done()

	for {
		select {
		case <-s.reconnectChan:
			s.config.Logger.Info("Reconnecting...")

			retries := 0
			for {
				// 检查是否停止
				select {
				case <-s.stopChan:
					return
				default:
				}

				// 检查重试次数
				if s.config.MaxReconnectTries > 0 && retries >= s.config.MaxReconnectTries {
					s.config.Logger.Error("Max reconnect tries reached")
					return
				}

				// 等待重连延迟
				time.Sleep(s.config.ReconnectDelay)

				// 尝试重连
				s.mu.Lock()
				err := s.connect()
				s.mu.Unlock()

				if err == nil {
					s.config.Logger.Info("Reconnected successfully")
					break
				}

				retries++
				s.config.Logger.Warn("Reconnect attempt %d failed: %v", retries, err)
			}

		case <-s.stopChan:
			return
		}
	}
}

// heartbeatLoop 自动心跳循环
func (s *Session) heartbeatLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.RLock()
			state := s.state
			s.mu.RUnlock()

			if state == StateSelected {
				// 发送Linktest.req并等待回复
				if err := s.sendLinktestWithTimeout(); err != nil {
					s.config.Logger.Error("Heartbeat failed: %v", err)
					// 触发重连
					select {
					case s.reconnectChan <- struct{}{}:
					default:
					}
				} else {
					s.config.Logger.Debug("Heartbeat successful")
				}
			}

		case <-s.stopChan:
			return
		}
	}
}

// receiveLoop 消息接收循环
func (s *Session) receiveLoop() {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopChan:
			return
		default:
		}

		// 接收HSMS数据包
		s.mu.RLock()
		conn := s.conn
		s.mu.RUnlock()

		if conn == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// 设置读取超时
		conn.SetReadDeadline(time.Now().Add(s.config.T8))

		packet, err := decodeHSMSPacket(conn)
		if err != nil {
			// 超时或连接关闭
			netErr, ok := err.(net.Error)

			// s.config.Logger.Error("err typr: %v，ok: %v", netErr, ok)

			if !ok || ok && netErr.Timeout() {
				continue
			}
			s.config.Logger.Error("Failed to receive packet: %v", err)

			// Active模式触发重连
			if s.config.AutoReconnect && s.config.IsActive {
				select {
				case s.reconnectChan <- struct{}{}:
				default:
				}
			}
			return
		}

		// 处理数据包
		if err := s.handlePacket(packet); err != nil {
			s.config.Logger.Error("Failed to handle packet: %v", err)
		}
	}
}

// ============================================================
// HSMS控制消息
// ============================================================

// sendLinktestWithTimeout 发送Linktest.req并等待回复
func (s *Session) sendLinktestWithTimeout() error {
	s.mu.Lock()
	s.systemByte++
	systemByte := s.systemByte
	s.mu.Unlock()

	// 创建回复通道
	replyChan := make(chan *Message, 1)
	s.pendingReplies.Store(systemByte, replyChan)
	defer s.pendingReplies.Delete(systemByte)

	s.sendControlMessage(STypeLinktestReq, systemByte)

	// 等待回复或超时
	select {
	case <-replyChan:
		return nil
	case <-time.After(s.config.T3):
		return fmt.Errorf("linktest timeout: no response within %v", s.config.T3)
	case <-s.ctx.Done():
		return fmt.Errorf("session stopped during linktest")
	}
}

// sendSeparateNoLock 发送Separate.req (不获取锁，用于Stop时调用)
func (s *Session) sendSeparateNoLock(conn net.Conn) error {
	s.mu.Lock()
	s.systemByte++
	systemByte := s.systemByte
	s.mu.Unlock()

	packet := createControlMessage(STypeSeparateReq, systemByte)

	data, err := encodeHSMSPacket(packet)
	if err != nil {
		return fmt.Errorf("failed to encode separate.req: %w", err)
	}

	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("failed to send separate.req: %w", err)
	}

	s.config.Logger.Info(">>> Send Separate.req (SystemBytes=%d) HEX: %s", systemByte, formatHexData(data))
	s.config.Logger.Info("Initiating disconnect")

	return nil
}

func (s *Session) handlePacket(packet *hsmsPacket) error {
	switch packet.SType {
	case STypeDataMessage:
		// 数据消息
		return s.handleDataMessage(packet)

	case STypeSelectReq:
		// Select.req (Passive模式)
		return s.handleSelectReq(packet)

	case STypeSelectRsp:
		// Select.rsp (Active模式)
		return s.handleSelectRsp(packet)

	case STypeLinktestReq:
		// Linktest.req
		return s.handleLinktestReq(packet)

	case STypeLinktestRsp:
		// Linktest.rsp
		data, _ := encodeHSMSPacket(packet)
		s.config.Logger.Info("<<< Recv Linktest.rsp (SystemBytes=%d) HEX: %s", packet.SystemBytes, formatHexData(data))

		// 检查是否有等待的心跳回复
		if ch, ok := s.pendingReplies.Load(packet.SystemBytes); ok {
			replyChan := ch.(chan *Message)
			// 创建虚拟消息对象表示心跳回复
			heartbeatReply := &Message{
				SystemBytes: packet.SystemBytes,
				Timestamp:   time.Now(),
			}
			select {
			case replyChan <- heartbeatReply:
			default:
			}
		}
		return nil

	case STypeDeselectReq:
		// Deselect.req
		data, _ := encodeHSMSPacket(packet)
		s.config.Logger.Info("<<< Recv Deselect.req (SystemBytes=%d) HEX: %s", packet.SystemBytes, formatHexData(data))
		s.mu.Lock()
		s.state = StateConnected
		s.mu.Unlock()
		return s.sendControlMessage(STypeDeselectRsp, packet.SystemBytes)

	case STypeSeparateReq:
		// Separate.req - 对方主动断开连接
		return s.handleSeparateReq(packet)

	default:
		s.config.Logger.Warn("Unknown SType: %d", packet.SType)
		return nil
	}
}

// handleDataMessage 处理数据消息
func (s *Session) handleDataMessage(packet *hsmsPacket) error {
	// 重新编码以获取原始数据用于日志
	data, _ := encodeHSMSPacket(packet)
	// 解析消息
	msg := &Message{
		Stream:      packet.HeaderByte2 & 0x7F,
		Function:    packet.HeaderByte3,
		WBit:        (packet.HeaderByte2 & 0x80) != 0,
		SystemBytes: packet.SystemBytes,
		Timestamp:   time.Now(),
		session:     s,
	}

	// 解析Item (如果有数据)
	if len(packet.Data) > 0 {
		item, err := decodeItem(packet.Data)
		if err != nil {
			s.config.Logger.Error("Failed to decode item: %v", err)
			return err
		}
		msg.Item = item
	}

	var preMsg string
	// 打印接收日志
	if msg.WBit {
		preMsg = fmt.Sprintf("%s (Primary)", formatMessage(msg))
	} else {
		preMsg = fmt.Sprintf("%s (Reply)", formatMessage(msg))
	}

	s.logReciveMessage(preMsg, data, msg.Item)

	// 如果是回复消息(WBit=false),查找等待的请求
	if !msg.WBit {
		if ch, ok := s.pendingReplies.Load(msg.SystemBytes); ok {
			replyChan := ch.(chan *Message)
			select {
			case replyChan <- msg:
			default:
			}
			return nil
		}
	}

	// 调用消息处理器
	s.mu.RLock()
	handler := s.messageHandler
	s.mu.RUnlock()

	if handler != nil {
		if err := handler(msg); err != nil {
			s.config.Logger.Error("Message handler error: %v", err)
			return err
		}
	}

	return nil
}

// handleSelectReq 处理Select.req (Passive模式)
func (s *Session) handleSelectReq(packet *hsmsPacket) error {
	// 重新编码以获取原始数据用于日志
	data, _ := encodeHSMSPacket(packet)

	s.config.Logger.Info("<<< Recv Select.req (SystemBytes=%d) HEX: %s", packet.SystemBytes, formatHexData(data))

	s.mu.Lock()
	oldState := s.state
	s.state = StateSelected
	s.mu.Unlock()
	s.config.Logger.Info("Session selected")

	if s.stateChangeHandler != nil && oldState != StateSelected {
		s.stateChangeHandler(oldState, StateSelected)
	}
	// 通知连接就绪
	select {
	case s.readyChan <- struct{}{}:
	default:
	}

	// 发送Select.rsp
	return s.sendSelectRsp(packet.SystemBytes, 0)
}

// handleSelectRsp 处理Select.rsp (Active模式)
func (s *Session) handleSelectRsp(packet *hsmsPacket) error {
	// 重新编码以获取原始数据用于日志
	data, _ := encodeHSMSPacket(packet)

	status := packet.HeaderByte3
	s.config.Logger.Info("<<< Recv Select.rsp (SystemBytes=%d, Status=%d) HEX: %s", packet.SystemBytes, status, formatHexData(data))

	if status == 0 {
		s.mu.Lock()
		oldState := s.state
		s.state = StateSelected
		s.mu.Unlock()

		s.config.Logger.Info("Session selected")

		if s.stateChangeHandler != nil && oldState != StateSelected {
			s.stateChangeHandler(oldState, StateSelected)
		}

		// 通知连接就绪
		select {
		case s.readyChan <- struct{}{}:
		default:
		}
	} else {
		s.config.Logger.Error("Select rejected with status: %d", status)
	}

	return nil
}

// handleLinktestReq 处理Linktest.req
func (s *Session) handleLinktestReq(packet *hsmsPacket) error {
	// 重新编码以获取原始数据用于日志
	data, _ := encodeHSMSPacket(packet)

	s.config.Logger.Info("<<< Recv Linktest.req (SystemBytes=%d) HEX: %s", packet.SystemBytes, formatHexData(data))

	return s.sendControlMessage(STypeLinktestRsp, packet.SystemBytes)
}

// handleSeparateReq 处理Separate.req (对方主动断开连接)
func (s *Session) handleSeparateReq(packet *hsmsPacket) error {
	// 重新编码以获取原始数据用于日志
	data, _ := encodeHSMSPacket(packet)

	s.config.Logger.Info("<<< Recv Separate.req (SystemBytes=%d) HEX: %s", packet.SystemBytes, formatHexData(data))
	s.config.Logger.Info("Remote peer initiated disconnect")

	// 清理所有等待的回复
	s.pendingReplies.Range(func(key, value interface{}) bool {
		s.pendingReplies.Delete(key)
		return true
	})

	// 关闭连接
	s.mu.Lock()
	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}
	oldState := s.state
	s.state = StateDisconnected
	s.mu.Unlock()
	if s.stateChangeHandler != nil && oldState != StateDisconnected {
		s.stateChangeHandler(oldState, StateDisconnected)
	}

	// 如果是Active模式且启用自动重连,触发重连
	if s.config.AutoReconnect && s.config.IsActive {
		s.config.Logger.Info("Triggering auto-reconnect due to remote disconnect")
		select {
		case s.reconnectChan <- struct{}{}:
		default:
		}
	}

	// 停止会话
	s.Stop()

	return nil
}

// sendSelectRsp 发送Select.rsp
func (s *Session) sendSelectRsp(systemBytes uint32, status byte) error {
	packet := createSelectRsp(s.config.DeviceID, systemBytes, status)

	data, err := encodeHSMSPacket(packet)
	if err != nil {
		return fmt.Errorf("failed to encode select.rsp: %w", err)
	}

	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("connection is nil")
	}

	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("failed to send select.rsp: %w", err)
	}

	s.config.Logger.Info(">>> Send Select.rsp (SystemBytes=%d, Status=%d) HEX: %s", systemBytes, status, formatHexData(data))
	return nil
}

// sendDeselectRsp 发送Deselect.rsp
func (s *Session) sendControlMessage(sType SType, systemBytes uint32) error {
	packet := createControlMessage(sType, systemBytes)

	data, err := encodeHSMSPacket(packet)
	if err != nil {
		return fmt.Errorf("failed to encode %v: %w", sType.String(), err)
	}

	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("connection is nil")
	}

	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("failed to send %v: %w", sType.String(), err)
	}

	s.config.Logger.Info(">>> Send %v (SystemBytes=%d) HEX: %s", sType.String(), systemBytes, formatHexData(data))
	return nil
}

// ============================================================
// 消息发送/接收
// ============================================================

// Send 发送消息(不等待回复)
func (s *Session) Send(msg *Message) error {
	s.mu.Lock()
	if s.state != StateSelected {
		s.mu.Unlock()
		return fmt.Errorf("session not selected")
	}

	// 分配SystemBytes
	if msg.SystemBytes == 0 {
		s.systemByte++
		msg.SystemBytes = s.systemByte
	}

	conn := s.conn
	s.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("connection is nil")
	}

	// 编码消息
	msgData, err := encodeMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}

	// 创建HSMS数据包
	packet := &hsmsPacket{
		SessionID:   s.config.DeviceID,
		HeaderByte2: msgData[0], // Stream + WBit
		HeaderByte3: msgData[1], // Function
		PType:       PTypeSECSII,
		SType:       STypeDataMessage,
		SystemBytes: msg.SystemBytes,
		Data:        msgData[10:], // Item数据
	}

	// 编码HSMS包
	data, err := encodeHSMSPacket(packet)
	if err != nil {
		return fmt.Errorf("failed to encode HSMS packet: %w", err)
	}

	// 发送
	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// 记录发送日志
	s.logSendMessage(formatMessage(msg), data, msg.Item)
	return nil
}

// SendAndWait 发送消息并等待回复
func (s *Session) SendAndWait(msg *Message) (*Message, error) {
	s.mu.Lock()
	if s.state != StateSelected {
		s.mu.Unlock()
		return nil, fmt.Errorf("session not selected")
	}

	if !msg.WBit {
		s.mu.Unlock()
		return nil, fmt.Errorf("message must have WBit set")
	}

	// 分配SystemBytes
	if msg.SystemBytes == 0 {
		s.systemByte++
		msg.SystemBytes = s.systemByte
	}

	// 创建回复通道
	replyChan := make(chan *Message, 1)
	s.pendingReplies.Store(msg.SystemBytes, replyChan)
	defer s.pendingReplies.Delete(msg.SystemBytes)

	conn := s.conn
	s.mu.Unlock()

	if conn == nil {
		return nil, fmt.Errorf("connection is nil")
	}

	// 编码消息
	msgData, err := encodeMessage(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}

	// 创建HSMS数据包
	packet := &hsmsPacket{
		SessionID:   s.config.DeviceID,
		HeaderByte2: msgData[0], // Stream + WBit
		HeaderByte3: msgData[1], // Function
		PType:       PTypeSECSII,
		SType:       STypeDataMessage,
		SystemBytes: msg.SystemBytes,
		Data:        msgData[10:], // Item数据
	}

	// 编码HSMS包
	data, err := encodeHSMSPacket(packet)
	if err != nil {
		return nil, fmt.Errorf("failed to encode HSMS packet: %w", err)
	}

	// 发送
	if _, err := conn.Write(data); err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// 记录发送日志
	s.logSendMessage(formatMessage(msg)+" (Wait for reply)", data, msg.Item)

	// 等待回复
	select {
	case reply := <-replyChan:
		return reply, nil
	case <-time.After(s.config.T3):
		s.config.Logger.Warn("Timeout waiting for reply (T3=%v): S%dF%d (SysBytes=%d)", s.config.T3, msg.Stream, msg.Function, msg.SystemBytes)
		return nil, fmt.Errorf("timeout waiting for reply")
	case <-s.ctx.Done():
		return nil, fmt.Errorf("session stopped")
	}
}

// Reply 回复消息
func (s *Session) Reply(primary *Message, reply *Message) error {
	s.mu.RLock()
	if s.state != StateSelected {
		s.mu.RUnlock()
		return fmt.Errorf("session not selected")
	}

	conn := s.conn
	s.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("connection is nil")
	}

	// 设置SystemBytes为主消息的SystemBytes
	reply.SystemBytes = primary.SystemBytes
	reply.WBit = false // 回复消息不需要WBit

	// 编码消息
	msgData, err := encodeMessage(reply)
	if err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}

	// 创建HSMS数据包
	packet := &hsmsPacket{
		SessionID:   s.config.DeviceID,
		HeaderByte2: msgData[0], // Stream + WBit
		HeaderByte3: msgData[1], // Function
		PType:       PTypeSECSII,
		SType:       STypeDataMessage,
		SystemBytes: reply.SystemBytes,
		Data:        msgData[10:], // Item数据
	}

	// 编码HSMS包
	data, err := encodeHSMSPacket(packet)
	if err != nil {
		return fmt.Errorf("failed to encode HSMS packet: %w", err)
	}

	// 发送
	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("failed to send reply: %w", err)
	}

	// 记录发送日志
	s.logSendMessage(fmt.Sprintf("%s to %s", formatMessage(reply), formatMessage(primary)), data, reply.Item)
	return nil
}

// OnMessage 设置消息处理器
func (s *Session) OnMessage(handler MessageHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messageHandler = handler
}

// OnStateChange 设置状态变更事件回调
func (s *Session) OnStateChange(handler StateChangeHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stateChangeHandler = handler
}

// ============================================================
// 日志辅助函数
// ============================================================

// logSendMessage 记录发送消息日志(公共方法)
func (s *Session) logSendMessage(message string, data []byte, item *Item) {
	msg := fmt.Sprintf(">>> Send %s\n%s", message, formatHexData(data))
	if item != nil {
		msg += "\n" + formatSML(item)
	}
	s.config.Logger.Info("%s", msg)
}

// logSendMessage 记录发送消息日志(公共方法)
func (s *Session) logReciveMessage(message string, data []byte, item *Item) {

	msg := fmt.Sprintf("<<< Recv %s\n%s", message, formatHexData(data))
	if item != nil {
		msg += "\n" + formatSML(item)
	}
	s.config.Logger.Info("%s", msg)
}

// ============================================================
// 默认回复处理
// ============================================================

// sendDefaultReply 发送默认回复消息
func (s *Session) SendDefaultReply(primary *Message) error {
	// 根据SECS协议标准，创建默认回复消息
	// 规则：SXF(X+1)，即流相同，功能号+1
	reply := &Message{
		Stream:      primary.Stream,
		Function:    primary.Function + 1,
		WBit:        false,
		SystemBytes: primary.SystemBytes,
		Timestamp:   time.Now(),
		session:     s,
	}

	// 对于特殊情况提供默认的Item数据
	switch primary.Stream {
	case 1:
		// Stream 1 默认回复通常为空或简单的确认
		reply.Item = L()
	case 6:
		// Stream 6 默认回复通常是确认消息
		reply.Item = B(0) // 0表示成功
	case 2:
		// Stream 2 默认回复
		reply.Item = L()
	case 3:
		// Stream 3 默认回复
		reply.Item = L()
	case 5:
		// Stream 5 默认回复
		reply.Item = L()
	case 7:
		// Stream 7 默认回复
		reply.Item = L()
	default:
		// 其他流使用空列表作为默认回复
		reply.Item = L()
	}

	// 发送回复消息
	return s.Reply(primary, reply)
}
