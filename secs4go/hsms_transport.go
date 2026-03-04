package secs4go

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// ============================================================
// 错误定义
// ============================================================

var (
	ErrNotConnected = errors.New("not connected")
	ErrTimeoutT6    = errors.New("T6 control transaction timeout")
)

// ============================================================
// HSMSTransport 传输层
// 职责: TCP连接管理、T5-T8超时控制、自动重连、心跳检测、控制会话处理
// ============================================================

type HSMSTransport struct {
	config *Config
	conn   net.Conn
	mu     sync.RWMutex
	wg     sync.WaitGroup
	state  ConnectionState

	logger Logger

	// 服务端模式
	listener net.Listener

	// 超时控制
	t6Timer *time.Timer // Control transaction (控制会话)
	t7Timer *time.Timer // Not selected

	// 消息信道
	controlReplyChan chan struct{} // 控制回复信道 (T6)

	// 生命周期控制
	stopChan      chan struct{} // 停止信号通道（每次 Start/Stop 生命周期都会重建）
	stopOnce      sync.Once
	stopping      bool
	reconnectChan chan struct{} // 重连信号通道
	readyChan     chan struct{} // 就绪信号通道 (Select完成)

	// 回调
	ctrlHandler  func(HSMSHeader)         // 控制消息处理回调 (服务端)
	stateHandler StateChangeHandler       // 状态变更回调
	dataHandler  func(HSMSHeader, []byte) // 数据消息处理回调 (所有数据会话)

	// SystemBytes 生成器
	systemByte uint32
}

// NewHSMSTransport 创建传输层
func NewHSMSTransport(config *Config) *HSMSTransport {
	return &HSMSTransport{
		config:           config,
		state:            StateDisconnected,
		t6Timer:          time.NewTimer(config.T6),
		t7Timer:          time.NewTimer(config.T7),
		controlReplyChan: make(chan struct{}, 1),
		stopChan:         make(chan struct{}),
		reconnectChan:    make(chan struct{}, 1),
		readyChan:        make(chan struct{}),
	}
}

// ============================================================
// 生命周期管理
// ============================================================

// Start 启动传输层 (根据 IsActive 自动选择客户端或服务端模式)
func (t *HSMSTransport) Start() error {
	// 新一轮生命周期初始化（支持 Stop() 后再次 Start()）
	t.mu.Lock()
	if t.state == StateDisconnected {
		t.stopChan = make(chan struct{})
		t.stopOnce = sync.Once{}
		t.readyChan = make(chan struct{})
		t.stopping = false
	}
	t.mu.Unlock()

	if t.config.IsActive {
		// 客户端模式: 连接并启动后台协程
		t.logger.Info("Starting transport (active mode)...")

		// 先启动自动重连（确保在 Connect 失败前已就绪）
		if t.config.AutoReconnect {
			t.wg.Add(1)
			go t.autoReconnectLoop()
		}

		// 连接 (Connect 内部已启动 receiveLoop)
		if err := t.Connect(t.config.Address); err != nil {
			if !t.config.AutoReconnect {
				return fmt.Errorf("failed to connect: %v", err)
			}
			t.logger.Warn("Initial connection failed, will retry: %v", err)
			// 触发重连
			select {
			case t.reconnectChan <- struct{}{}:
			default:
			}
		}

		// 启动心跳检测
		if t.config.EnableHeartbeat {
			t.wg.Add(1)
			go t.heartbeatLoop()
		}
		return nil
	}

	// 服务端模式: 监听
	t.logger.Info("Starting transport (passive mode)...")

	if err := t.Listen(t.config.Address); err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}
	t.logger.Info("Listening on %s", t.LocalAddr())

	// 启动连接处理协程 (Accept 后每个连接会启动 receiveLoop)
	t.wg.Add(1)
	go t.handleConnections()

	// 启动心跳检测（服务端模式也启用）
	if t.config.EnableHeartbeat {
		t.wg.Add(1)
		go t.heartbeatLoop()
	}

	return nil
}

// Stop 停止传输层（幂等；支持 Stop() 后再次 Start()）
func (t *HSMSTransport) Stop() {
	// 标记 stopping（避免 Stop 过程中触发 passive 重开监听 / active 自动重连）
	t.mu.Lock()
	if !t.stopping {
		t.stopping = true
	}
	currentState := t.state
	isSelected := currentState == StateSelected
	t.mu.Unlock()

	// 优先发送 Separate.req 让对方知晓断开（失败忽略）
	if isSelected {
		_ = t.SendSeparateReq()
	}

	// 发出停止信号（幂等）
	t.stopOnce.Do(func() {
		close(t.stopChan)
	})

	// 先关闭底层资源以打断 Accept/read
	t.closeResources()

	// 等待所有协程退出
	t.wg.Wait()

	// 收敛状态并仅通知一次
	t.mu.Lock()
	prevState := t.state
	t.state = StateDisconnected
	t.mu.Unlock()

	if prevState != StateDisconnected {
		t.notifyStateChange(prevState, StateDisconnected)
	}

	t.logger.Info("Transport stopped")
}

// ReadyChan 返回就绪通道 (Select完成时发送信号)
func (t *HSMSTransport) ReadyChan() <-chan struct{} {
	return t.readyChan
}

// ReconnectChan 返回重连通道 (供外部触发重连)
func (t *HSMSTransport) ReconnectChan() chan<- struct{} {
	return t.reconnectChan
}

// autoReconnectLoop 自动重连循环
func (t *HSMSTransport) autoReconnectLoop() {
	defer t.wg.Done()

	for {
		select {
		case <-t.reconnectChan:
			t.logger.Info("Reconnecting...")

			retries := 0
			for {
				// 检查是否停止
				select {
				case <-t.stopChan:
					return
				default:
				}

				// 检查重试次数
				if t.config.MaxReconnectTries > 0 && retries >= t.config.MaxReconnectTries {
					t.logger.Error("Max reconnect tries reached")
					return
				}

				// 等待重连延迟 (使用T5)
				select {
				case <-t.stopChan:
					return
				case <-time.After(t.config.T5):
				}

				// 尝试重连
				if err := t.reconnect(); err == nil {
					t.logger.Info("Reconnected successfully")
					break
				} else {
					retries++
					t.logger.Warn("Reconnect attempt %d failed: %v", retries, err)
				}
			}

		case <-t.stopChan:
			return
		}
	}
}

// reconnect 内部重连方法
func (t *HSMSTransport) reconnect() error {
	// 统一使用 handleDisconnect 处理断开和触发重连
	t.handleDisconnect()

	// 重新连接 (Connect 内部已包含 Select)
	if err := t.Connect(t.config.Address); err != nil {
		return err
	}

	return nil
}

// heartbeatLoop 心跳检测循环
func (t *HSMSTransport) heartbeatLoop() {
	defer t.wg.Done()

	for {
		// 每次心跳后重新计算间隔（从心跳完成开始计算）
		heartbeatTimer := time.NewTimer(t.config.HeartbeatInterval)
		select {
		case <-heartbeatTimer.C:
			// 检查是否已停止
			select {
			case <-t.stopChan:
				return
			default:
			}

			// 检查是否已选择
			if t.IsSelected() {
				if err := t.LinkTestReq(); err != nil {
					t.logger.Error("Heartbeat failed: %v", err)
					// 心跳失败时调用 handleDisconnect 触发重连
					t.handleDisconnect()
				} else {
					t.logger.Debug("Heartbeat OK")
				}
			}

		case <-t.stopChan:
			return
		}
	}
}

// notifySelected 通知 Select 完成 (内部调用)
func (t *HSMSTransport) notifySelected() {
	select {
	case t.readyChan <- struct{}{}:
	default:
	}
}

// ============================================================
// 客户端方法
// ============================================================

// Connect 客户端: 连接到服务端并发起 Select
func (t *HSMSTransport) Connect(address string) error {
	t.mu.Lock()

	// T5: 使用 T5 作为连接超时
	conn, err := net.DialTimeout("tcp", address, t.config.T5)
	if err != nil {
		t.mu.Unlock()
		return err
	}

	t.conn = conn

	// T7: 启动 Not Selected 超时
	t.resetT7Locked()

	// 启动读取协程
	go t.receiveLoop()

	t.mu.Unlock()

	// 发起 Select 请求 (使用 T6)
	header := BuildControlHeader(STypeSelectReq, t.NextSystemBytes(), 0)
	if err := t.SendControlAndWait(header); err != nil {
		return fmt.Errorf("select failed: %v", err)
	}

	// Select 成功后通知就绪
	if t.IsSelected() {
		t.notifySelected()
	}

	return nil
}

// ============================================================
// 服务端方法
// ============================================================

// Listen 服务端: 监听连接（进入 Listening 状态）
func (t *HSMSTransport) Listen(address string) error {
	// 如果已停止，直接返回错误（避免 Stop() 后又把端口 listen 起来）
	select {
	case <-t.stopChan:
		return net.ErrClosed
	default:
	}

	t.mu.Lock()
	// 关闭旧监听器
	if t.listener != nil {
		_ = t.listener.Close()
		t.listener = nil
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.mu.Unlock()
		return err
	}

	prevState := t.state
	t.listener = listener
	t.conn = nil // 清空旧连接
	t.state = StateListening
	t.mu.Unlock()

	if prevState != StateListening {
		t.notifyStateChange(prevState, StateListening)
	}
	return nil
}

// Accept 服务端: 接受连接 (单连接模式：关闭监听器，将连接设置到自身)
func (t *HSMSTransport) Accept() error {
	t.mu.RLock()
	listener := t.listener
	t.mu.RUnlock()
	if listener == nil {
		return net.ErrClosed
	}

	conn, err := listener.Accept()
	if err != nil {
		return err
	}

	// 关闭监听器（单连接模式）并设置连接
	t.mu.Lock()
	prevState := t.state
	if t.listener != nil {
		_ = t.listener.Close()
		t.listener = nil
	}
	t.conn = conn
	t.state = StateConnected
	t.mu.Unlock()

	if prevState != StateConnected {
		t.notifyStateChange(prevState, StateConnected)
	}

	// T7: 启动 Not Selected 超时
	t.resetT7()

	// 启动读取协程
	go t.receiveLoop()

	return nil
}

// handleConnections 服务端连接处理协程 - 单连接模式：Accept 后退出，断开后重新启动
func (t *HSMSTransport) handleConnections() {
	defer t.wg.Done()

	for {
		select {
		case <-t.stopChan:
			return
		default:
		}

		// 若尚未处于监听状态，则打开监听器
		t.mu.RLock()
		listener := t.listener
		state := t.state
		t.mu.RUnlock()
		if listener == nil || state != StateListening {
			if err := t.Listen(t.config.Address); err != nil {
				select {
				case <-t.stopChan:
					return
				default:
					t.logger.Error("Failed to listen: %v", err)
					return
				}
			}
			t.logger.Info("Listening on %s", t.LocalAddr())
		}

		// 等待 Accept（阻塞）
		if err := t.Accept(); err != nil {
			select {
			case <-t.stopChan:
				return
			default:
				// listener 被 Stop() 关闭时，Accept 会报错；这里不当作业务错误
				if errors.Is(err, net.ErrClosed) || strings.Contains(err.Error(), "use of closed") {
					continue
				}
				t.logger.Error("Failed to accept connection: %v", err)
				continue
			}
		}

		// Accept 成功，协程退出
		t.logger.Info("Accepted connection from %s", t.RemoteAddr())
		return
	}
}

// ============================================================
// 数据发送
// ============================================================

// Send 发送原始字节数据 (用于已组装的完整帧)
func (t *HSMSTransport) Send(data []byte) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.conn == nil {
		return ErrNotConnected
	}
	t.conn.SetWriteDeadline(time.Now().Add(t.config.T8))
	_, err := t.conn.Write(data)
	return err
}

// NextSystemBytes 生成下一个 SystemBytes (导出供 SecsGem 使用)
func (t *HSMSTransport) NextSystemBytes() uint32 {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.systemByte++
	return t.systemByte
}

// ============================================================
// 控制会话发送 (T6)
// ============================================================

// SendControl 发送控制消息
func (t *HSMSTransport) SendControl(header HSMSHeader) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.conn == nil {
		return ErrNotConnected
	}

	// 控制消息日志: 一行格式 (消息头 + 完整帧HEX)
	frameData := BuildCompleteFrame(header, nil)
	t.logger.Info(">>> Send %s (SystemBytes=%d) HEX: %s", header.SType, header.SystemBytes, FormatHexData(frameData))

	t.conn.SetWriteDeadline(time.Now().Add(t.config.T8))
	_, err := t.conn.Write(frameData)
	return err
}

// SendControlAndWait 发送控制消息并等待回复 (使用 T6)
func (t *HSMSTransport) SendControlAndWait(header HSMSHeader) error {
	t.mu.RLock()
	if t.conn == nil {
		t.mu.RUnlock()
		return ErrNotConnected
	}

	// T6: 启动 Control transaction 超时
	if !t.t6Timer.Stop() {
		select {
		case <-t.t6Timer.C:
		default:
		}
	}
	t.t6Timer.Reset(t.config.T6)
	t.mu.RUnlock()

	// 发送控制消息
	if err := t.SendControl(header); err != nil {
		return err
	}

	// 等待 T6 超时或回复
	select {
	case <-t.t6Timer.C:
		return ErrTimeoutT6
	case <-t.controlReplyChan:
		return nil
	}
}

// SendSelectRsp 服务端: 发送 Select.rsp 响应
func (t *HSMSTransport) SendSelectRsp(systemBytes uint32, status byte) {
	header := BuildSelectRspHeader(systemBytes, status)
	t.SendControl(header)
}

// LinkTest 发送 LinkTest 请求 (使用 T6)
func (t *HSMSTransport) LinkTestReq() error {
	header := BuildControlHeader(STypeLinktestReq, t.NextSystemBytes(), 0)
	return t.SendControlAndWait(header)
}

// SendDeselectRsp 服务端: 发送 Deselect.rsp 响应
func (t *HSMSTransport) SendDeselectRsp(systemBytes uint32, status byte) {
	header := BuildDeselectRspHeader(systemBytes, status)
	t.SendControl(header)
}

// SendRejectRsp 服务端: 发送 Reject.rsp 响应
func (t *HSMSTransport) SendRejectRsp(systemBytes uint32, reason byte) {
	header := BuildRejectReqHeader(systemBytes, reason)
	t.SendControl(header)
}

// SendSeparateReq 发送 Separate.req (断开连接)
func (t *HSMSTransport) SendSeparateReq() error {
	header := BuildControlHeader(STypeSeparateReq, t.NextSystemBytes(), 0)
	return t.SendControl(header)
}

// ============================================================
// 公共方法
// ============================================================

// IsSelected 检查是否已 Select
func (t *HSMSTransport) IsSelected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state == StateSelected
}

// GetState 获取状态
func (t *HSMSTransport) GetState() ConnectionState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

// RemoteAddr 获取远端地址
func (t *HSMSTransport) RemoteAddr() net.Addr {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.conn != nil {
		return t.conn.RemoteAddr()
	}
	return nil
}

// LocalAddr 获取本地地址
func (t *HSMSTransport) LocalAddr() net.Addr {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.conn != nil {
		return t.conn.LocalAddr()
	}
	if t.listener != nil {
		return t.listener.Addr()
	}
	return nil
}

// closeResources 关闭底层资源（listener/conn/timers），不修改 state、不触发回调
func (t *HSMSTransport) closeResources() {
	// 停止所有定时器
	t.t6Timer.Stop()
	t.t7Timer.Stop()

	// 取出资源引用，避免在持锁状态下做 Close() 阻塞
	t.mu.Lock()
	conn := t.conn
	listener := t.listener
	t.conn = nil
	t.listener = nil
	t.mu.Unlock()

	if conn != nil {
		_ = conn.Close()
	}
	if listener != nil {
		_ = listener.Close()
	}
}

// Close 关闭连接（资源关闭 + 状态置为 Disconnected；不触发回调）
func (t *HSMSTransport) Close() {
	t.mu.Lock()
	t.state = StateDisconnected
	t.mu.Unlock()
	t.closeResources()
}

// ============================================================
// 内部方法
// ============================================================

// receiveLoop 读取协程 - TCP读取 + 消息路由
func (t *HSMSTransport) receiveLoop() {
	for {
		t.mu.RLock()
		conn := t.conn
		t.mu.RUnlock()

		if conn == nil {
			return
		}

		// 检查是否已停止（避免读到已关闭的连接）
		select {
		case <-t.stopChan:
			return
		default:
		}

		// T8: 设置读取超时
		// conn.SetReadDeadline(time.Now().Add(t.config.T8))

		header, itemData, err := ReadHSMSFrame(conn)
		if err != nil {
			// 检查是否是 "use of closed network connection" 错误
			// 这种错误发生在 Stop() 关闭连接后，receiveLoop 仍在读取的情况
			if strings.Contains(err.Error(), "use of closed") {
				return // 静默返回，避免不必要的错误日志
			}
			t.logger.Error(fmt.Sprintf("read error: %s", err))
			t.handleDisconnect()
			return
		}

		// 路由到处理方法
		if header.SType == STypeDataMessage {
			t.processDataMessage(header, itemData)
		} else {
			t.processControlMessage(header)
		}
	}
}

// ============================================================
// 消息处理 - 统一入口
// ============================================================

// processDataMessage 处理数据消息 - 只做解析，路由到 secsgem 的回调
func (t *HSMSTransport) processDataMessage(header HSMSHeader, itemData []byte) {
	// 解析消息 (传入 self transport 用于回复)
	// msg, err := ParseMessage(header, itemData, t, t.itemCodec)
	// if err != nil {
	// 	t.logger.Error("Failed to parse message: %v", err)
	// 	return
	// }

	// 路由到数据消息回调（所有数据会话由 secsgem 统一处理）
	t.mu.RLock()
	handler := t.dataHandler
	t.mu.RUnlock()
	if handler != nil {
		handler(header, itemData)
	}
}

// processControlMessage 处理控制消息 - 内部处理 + T6超时控制
func (t *HSMSTransport) processControlMessage(header HSMSHeader) {
	// 记录接收日志 (一行格式)
	t.logReceivedControl(header)

	// T6: 停止 Control transaction 超时
	t.t6Timer.Stop()

	// 内部处理控制消息
	t.handleControlInternal(header)

	// 通知控制回复信道
	select {
	case t.controlReplyChan <- struct{}{}:
	default:
	}
}

// ============================================================
// 辅助方法 - 日志与默认回复
// ============================================================

// logReceivedControl 记录控制消息接收日志 (一行格式)
func (t *HSMSTransport) logReceivedControl(header HSMSHeader) {
	frameData := BuildCompleteFrame(header, nil)
	t.logger.Info("<<< Recv %s (SystemBytes=%d) HEX: %s", header.SType, header.SystemBytes, FormatHexData(frameData))
}

// handleControlInternal 内部处理控制消息
func (t *HSMSTransport) handleControlInternal(header HSMSHeader) {
	switch header.SType {
	case STypeSelectRsp:
		t.t7Timer.Stop()
		t.mu.Lock()
		prevState := t.state
		if header.HeaderByte3 == 0x00 {
			t.state = StateSelected
		}
		t.mu.Unlock()
		if header.HeaderByte3 == 0x00 {
			t.notifyStateChange(prevState, StateSelected)
			t.notifySelected()
		}

	case STypeSelectReq: // 服务端收到 Select.req
		t.sendSelectRsp(header.SystemBytes, 0x00)
		t.mu.Lock()
		prevState := t.state
		t.state = StateSelected
		t.mu.Unlock()
		t.notifyStateChange(prevState, StateSelected)
		t.notifySelected()

	case STypeDeselectRsp:
		t.mu.Lock()
		prevState := t.state
		t.state = StateConnected
		t.mu.Unlock()
		t.notifyStateChange(prevState, StateConnected)

	case STypeDeselectReq: // 服务端收到 Deselect.req
		t.sendDeselectRsp(header.SystemBytes, 0x00)
		t.mu.Lock()
		prevState := t.state
		t.state = StateConnected
		t.mu.Unlock()
		t.notifyStateChange(prevState, StateConnected)

	case STypeLinktestReq: // 自动回复 LinkTest.rsp
		t.sendLinkTestRsp(header.SystemBytes)

	case STypeLinktestRsp:
		// LinkTest 响应，心跳检测使用

	case STypeRejectReq:
		// Reject 请求，日志已记录

	case STypeSeparateReq:
		t.handleDisconnect()
	}

	// 调用应用层回调 (如果设置了)
	t.mu.RLock()
	handler := t.ctrlHandler
	t.mu.RUnlock()
	if handler != nil {
		handler(header)
	}
}

// sendLinkTestRsp 发送 LinkTest.rsp
func (t *HSMSTransport) sendLinkTestRsp(systemBytes uint32) {
	header := BuildControlHeader(STypeLinktestRsp, systemBytes, 0)
	t.SendControl(header)
}

// sendSelectRsp 发送 Select.rsp
func (t *HSMSTransport) sendSelectRsp(systemBytes uint32, status byte) {
	t.logger.Debug("sendSelectRsp INPUT: systemBytes=%d", systemBytes)
	header := BuildSelectRspHeader(systemBytes, status)
	t.logger.Debug("sendSelectRsp after build: SType=%s SystemBytes=%d", header.SType, header.SystemBytes)
	t.SendControl(header)
}

// sendDeselectRsp 发送 Deselect.rsp
func (t *HSMSTransport) sendDeselectRsp(systemBytes uint32, status byte) {
	header := BuildDeselectRspHeader(systemBytes, status)
	t.SendControl(header)
}

// resetT7 重置 T7 Not Selected 超时
func (t *HSMSTransport) resetT7() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.resetT7Locked()
}

func (t *HSMSTransport) resetT7Locked() {
	if !t.t7Timer.Stop() {
		select {
		case <-t.t7Timer.C:
		default:
		}
	}
	t.t7Timer.Reset(t.config.T7)
}

// handleDisconnect 处理断开连接
func (t *HSMSTransport) handleDisconnect() {
	// Stop() 过程中不做断开处理，避免重开监听/触发重连
	t.mu.RLock()
	stopping := t.stopping
	t.mu.RUnlock()
	if stopping {
		return
	}
	select {
	case <-t.stopChan:
		return
	default:
	}

	t.mu.Lock()
	if t.state == StateDisconnected {
		t.mu.Unlock()
		return
	}
	prevState := t.state
	t.state = StateDisconnected
	t.mu.Unlock()

	// 停止所有定时器
	t.t6Timer.Stop()
	t.t7Timer.Stop()

	// 关闭连接
	t.mu.Lock()
	if t.conn != nil {
		_ = t.conn.Close()
		t.conn = nil
	}
	t.mu.Unlock()

	// 通知状态变更（断开连接）
	t.notifyStateChange(prevState, StateDisconnected)

	// 服务端模式：重新打开监听器
	if !t.config.IsActive {
		// 重新启动 handleConnections 协程
		t.wg.Add(1)
		go t.handleConnections()
	}

	// 客户端模式：触发自动重连
	if t.config.IsActive && t.config.AutoReconnect {
		select {
		case t.reconnectChan <- struct{}{}:
		default:
		}
	}
}

// ============================================================
// 服务端模式 - 控制会话处理
// ============================================================

// OnControl 设置控制消息处理回调（服务端用）
func (t *HSMSTransport) OnControl(handler func(HSMSHeader)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ctrlHandler = handler
}

// OnStateChange 设置状态变更回调
func (t *HSMSTransport) OnStateChange(handler StateChangeHandler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stateHandler = handler
}

// notifyStateChange 通知状态变更
func (t *HSMSTransport) notifyStateChange(oldState, newState ConnectionState) {
	t.mu.RLock()
	handler := t.stateHandler
	t.mu.RUnlock()

	if handler != nil {
		handler(oldState, newState)
	}
}

// OnMessage 设置消息处理回调（收到数据消息时调用）
func (t *HSMSTransport) OnMessage(handler func(HSMSHeader, []byte)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.dataHandler = handler
}
