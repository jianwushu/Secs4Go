package core

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
	ErrNotConnected     = errors.New("not connected")
	ErrTimeoutT6        = errors.New("T6 control transaction timeout")
	ErrTransportStopped = errors.New("transport stopped")
)

// ============================================================
// HSMSTransport 传输层
// 职责: TCP连接管理、T5-T8超时控制、自动重连、心跳检测、控制会话处理
// ============================================================

// writeRequest 写请求（用于单写者模式）
type writeRequest struct {
	data  []byte     // 完整帧数据
	errCh chan error // 结果回传通道
}

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
	t7Start  chan struct{}
	t7Cancel chan struct{}

	pendingControl sync.Map // map[uint32]chan HSMSHeader

	// 连接生命周期（每次建立/断开连接都会轮转）
	connDone     chan struct{}
	connDoneOnce sync.Once

	// 生命周期控制
	stopChan      chan struct{} // 停止信号通道
	stopOnce      sync.Once
	stopping      bool
	reconnectChan chan struct{} // 重连信号通道
	// 消息分发 Channel（控制/数据分离）
	ctrlChan  chan HSMSHeader   // 控制消息通道
	dataChan  chan DataMessage  // 数据消息通道
	writeChan chan writeRequest // 写请求通道（单写者模式）

	// 回调
	stateHandler StateChangeHandler       // 状态变更回调
	dataHandler  func(HSMSHeader, []byte) // 数据消息处理回调 (所有数据会话)

	// 消息ID生成器（可自定义）
	idGen MessageIdGenerator
}

// NewHSMSTransport 创建传输层
func NewHSMSTransport(config *Config) *HSMSTransport {
	return &HSMSTransport{
		config:        config,
		state:         StateDisconnected,
		logger:        NewSilentLogger(),
		idGen:         NewDefaultIdGenerator(),
		connDone:      nil,
		stopChan:      make(chan struct{}),
		reconnectChan: make(chan struct{}, 1),
		writeChan:     make(chan writeRequest, 16),
	}
}

// ============================================================
// 生命周期管理
// ============================================================

// Start 启动传输层 (根据 IsActive 自动选择客户端或服务端模式)
func (t *HSMSTransport) Start() error {
	if t == nil {
		return fmt.Errorf("transport is nil")
	}
	if t.logger == nil {
		t.logger = NewSilentLogger()
	}
	if err := t.config.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// 新一轮生命周期初始化（支持 Stop() 后再次 Start()）
	needStart := false
	t.mu.Lock()
	if t.state == StateDisconnected {
		t.stopChan = make(chan struct{})
		t.stopOnce = sync.Once{}
		t.stopping = false
		// 初始化消息分发 Channel
		t.ctrlChan = make(chan HSMSHeader, 8)
		t.dataChan = make(chan DataMessage, 64)
		t.writeChan = make(chan writeRequest, 16)
		// T7 信号通道（每次生命周期重建）
		t.t7Start = make(chan struct{}, 1)
		t.t7Cancel = make(chan struct{}, 1)
		needStart = true
	}
	t.mu.Unlock()

	// 仅在首次初始化时启动消费协程，防止重复 Start 导致 goroutine 泄漏
	if !needStart {
		return nil
	}

	// 启动消息消费协程（控制/数据分离处理 + 单写者）+ T7 监控
	t.wg.Add(4)
	go t.t7MonitorLoop()
	go t.ctrlMessageConsumer()
	go t.dataMessageConsumer()
	go t.writeLoop()

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

	// 排空消息通道，确保消费协程不被阻塞
	t.drainChannels()

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

// ConnDone 返回连接断开信号（连接断开 / Stop / Close 时关闭）
func (t *HSMSTransport) ConnDone() <-chan struct{} {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.connDone == nil {
		done := make(chan struct{})
		close(done)
		return done
	}
	return t.connDone
}

// resetConnDoneLocked 重建连接生命周期通道（必须在持锁状态下调用）
func (t *HSMSTransport) resetConnDoneLocked() {
	if t.connDone != nil {
		t.closeConnDoneLocked()
	}
	t.connDone = make(chan struct{})
	t.connDoneOnce = sync.Once{}
}

// closeConnDoneLocked 关闭连接生命周期通道（必须在持锁状态下调用）
func (t *HSMSTransport) closeConnDoneLocked() {
	if t.connDone == nil {
		return
	}
	t.connDoneOnce.Do(func() {
		close(t.connDone)
	})
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
				t.handleDisconnect()
				if err := t.Connect(t.config.Address); err == nil {
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
				heartbeatTimer.Stop()
				return
			default:
			}

			// 检查是否已选择
			if t.IsSelected() {
				if err := t.LinkTestReq(); err != nil {
					t.logger.Error("Heartbeat failed: %v", err)
					// 心跳失败时调用 handleDisconnect 触发重连
					t.handleDisconnect()
				}
			}

		case <-t.stopChan:
			heartbeatTimer.Stop()
			return
		}
	}
}

// ============================================================
// T7 (Not Selected) 超时监控
// ============================================================

func (t *HSMSTransport) t7MonitorLoop() {
	defer t.wg.Done()

	timer := time.NewTimer(t.config.T7)
	timer.Stop() // 初始未激活
	active := false

	for {
		select {
		case <-t.stopChan:
			timer.Stop()
			return

		case <-t.t7Start:
			// (重新)启动 T7：先 drain 旧触发，再 Reset
			if active && !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(t.config.T7)
			active = true

		case <-t.t7Cancel:
			// 取消 T7（Select 成功）
			if active && !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			active = false

		case <-timer.C:
			// T7 超时：仅当处于「已连接未 Selected」时才断开
			active = false
			if t.GetState() == StateConnected {
				t.logger.Warn("T7 timeout: connection not selected within %v", t.config.T7)
				t.handleDisconnect()
			}
		}
	}
}

// signalT7Start 非阻塞通知 T7 monitor (重新)启动计时
func (t *HSMSTransport) signalT7Start() {
	select {
	case t.t7Start <- struct{}{}:
	default:
	}
}

// signalT7Cancel 非阻塞通知 T7 monitor 取消计时
func (t *HSMSTransport) signalT7Cancel() {
	select {
	case t.t7Cancel <- struct{}{}:
	default:
	}
}

// ============================================================
// 客户端方法
// ============================================================

// Connect 客户端: 连接到服务端并发起 Select
func (t *HSMSTransport) Connect(address string) error {
	// 进入前先检查是否已停止（与 Stop 竞态防护 #1）
	select {
	case <-t.stopChan:
		return ErrTransportStopped
	default:
	}

	conn, err := net.DialTimeout("tcp", address, t.config.T5)
	if err != nil {
		return err
	}

	select {
	case <-t.stopChan:
		_ = conn.Close()
		return ErrTransportStopped
	default:
	}

	t.mu.Lock()
	t.conn = conn
	// 新连接生命周期：重建 connDone（保持打开状态，断连时再关闭）
	t.resetConnDoneLocked()
	// 显式进入 Connected：保证后续 Select 失败时 handleDisconnect 的早退检查能通过并清理
	prevState := t.state
	t.state = StateConnected
	t.mu.Unlock()

	if prevState != StateConnected {
		t.notifyStateChange(prevState, StateConnected)
	}

	// T7: 启动 Not Selected 超时
	t.signalT7Start()

	// 启动读取协程（纳入 WaitGroup，Stop 可正确等待其退出）
	t.wg.Add(1)
	go t.receiveLoop()

	// 发起 Select 请求 (使用 T6)
	header := BuildControlHeader(STypeSelectReq, t.NextSystemBytes(), 0)
	if err := t.SendControlAndWait(header); err != nil {
		// Select 失败：清理 conn 并让 receiveLoop 退出，避免旧协程与新连接并存
		t.handleDisconnect()
		return fmt.Errorf("select failed: %v", err)
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
	t.conn = nil
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
	// 新连接生命周期：重建 connDone（保持打开状态，断连时再关闭）
	t.resetConnDoneLocked()
	t.state = StateConnected
	t.mu.Unlock()

	if prevState != StateConnected {
		t.notifyStateChange(prevState, StateConnected)
	}

	// T7: 启动 Not Selected 超时
	t.signalT7Start()

	// 启动读取协程（纳入 WaitGroup）
	t.wg.Add(1)
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

// enqueueWrite 将写请求入队并同步等待结果
func (t *HSMSTransport) enqueueWrite(data []byte) error {
	errCh := make(chan error, 1)
	req := writeRequest{data: data, errCh: errCh}
	select {
	case t.writeChan <- req:
		return <-errCh
	case <-t.stopChan:
		return ErrTransportStopped
	case <-t.ConnDone():
		return ErrNotConnected
	}
}

// Send 发送原始字节数据 (用于已组装的完整帧)
func (t *HSMSTransport) Send(data []byte) error {
	return t.enqueueWrite(data)
}

// NextSystemBytes 生成下一个 SystemBytes
func (t *HSMSTransport) NextSystemBytes() uint32 {
	return t.idGen.Next()
}

func (t *HSMSTransport) SetIdGenerator(gen MessageIdGenerator) {
	if gen == nil {
		return
	}
	t.idGen = gen
}

// ============================================================
// 控制会话发送 (T6)
// ============================================================

// SendControl 发送控制消息
func (t *HSMSTransport) SendControl(header HSMSHeader) error {
	// 控制消息日志: 一行格式 (消息头 + 完整帧HEX)
	frameData := BuildCompleteFrame(header, nil)
	t.logSendControl(header, frameData)
	return t.enqueueWrite(frameData)
}

// SendControlAndWait 发送控制消息并等待回复 (使用 T6)
func (t *HSMSTransport) SendControlAndWait(header HSMSHeader) error {
	t.mu.RLock()
	if t.conn == nil {
		t.mu.RUnlock()
		return ErrNotConnected
	}
	t.mu.RUnlock()

	// T6: 创建事务级局部 Timer
	timer := time.NewTimer(t.config.T6)
	defer timer.Stop()

	// 注册到 pendingControl（按 SystemBytes 关联回复）
	replyCh := make(chan HSMSHeader, 1)
	t.pendingControl.Store(header.SystemBytes, replyCh)
	defer t.pendingControl.Delete(header.SystemBytes)

	// 发送控制消息（通过 writeChan 串行化）
	if err := t.SendControl(header); err != nil {
		return err
	}

	// 等待 T6 超时或回复
	select {
	case <-t.stopChan:
		return ErrTransportStopped
	case <-t.ConnDone():
		return ErrNotConnected
	case <-timer.C:
		return ErrTimeoutT6
	case <-replyCh:
		return nil
	}
}

// LinkTest 发送 LinkTest 请求 (使用 T6)
func (t *HSMSTransport) LinkTestReq() error {
	header := BuildControlHeader(STypeLinktestReq, t.NextSystemBytes(), 0)
	return t.SendControlAndWait(header)
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

// Reconnect 主动触发重连。
func (t *HSMSTransport) Reconnect() {
	if t.IsSelected() {
		_ = t.SendSeparateReq()
	}
	t.handleDisconnect()
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
	// 取出资源引用，避免在持锁状态下做 Close() 阻塞
	t.mu.Lock()
	conn := t.conn
	listener := t.listener
	t.conn = nil
	t.listener = nil

	t.closeConnDoneLocked()
	t.mu.Unlock()

	if conn != nil {
		_ = conn.Close()
	}
	if listener != nil {
		_ = listener.Close()
	}
}

// ============================================================
// 内部方法
// ============================================================

// receiveLoop 读取协程 - 只负责 TCP 读取和消息分发，不做业务处理
func (t *HSMSTransport) receiveLoop() {
	defer t.wg.Done()

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

		// T8: 设置字符间超时（检测半开连接/对端崩溃不发 FIN）
		// _ = conn.SetReadDeadline(time.Now().Add(t.config.T8))

		header, itemData, err := ReadHSMSFrame(conn)
		if err != nil {
			if strings.Contains(err.Error(), "use of closed") {
				return // 静默返回，避免不必要的错误日志
			}

			// T8 超时：网络字符间超时，触发断连处理
			// var netErr net.Error
			// if errors.As(err, &netErr) && netErr.Timeout() {
			// 	t.logger.Warn("T8 timeout (network intercharacter): %v", err)
			// 	t.handleDisconnect()
			// 	return
			// }

			t.mu.RLock()
			currentConn := t.conn
			t.mu.RUnlock()
			if conn != currentConn {
				return // 旧连接已被替换，静默退出
			}

			t.logger.Error(fmt.Sprintf("read error: %s", err))
			t.handleDisconnect()
			return
		}

		// 只做分发，不做处理
		if header.IsDataMessage() {
			select {
			case t.dataChan <- DataMessage{Header: header, ItemData: itemData}:
			case <-t.stopChan:
				return
			}
		} else {
			select {
			case t.ctrlChan <- header:
			case <-t.stopChan:
				return
			}
			// Separate.req 是终端消息：对端发送后会关闭 TCP 连接，
			// 继续读取必然产生 EOF 误报，直接退出即可。
			if header.SType == STypeSeparateReq {
				return
			}
		}
	}
}

// ============================================================
// 消息消费协程 - 控制/数据分离
// ============================================================

// ctrlMessageConsumer 控制消息消费协程 - 独立处理控制消息
func (t *HSMSTransport) ctrlMessageConsumer() {
	defer t.wg.Done()

	for {
		select {
		case header := <-t.ctrlChan:
			t.processControlMessage(header)
		case <-t.stopChan:
			return
		}
	}
}

// dataMessageConsumer 数据消息消费协程 - 独立处理数据消息
func (t *HSMSTransport) dataMessageConsumer() {
	defer t.wg.Done()

	for {
		select {
		case msg := <-t.dataChan:
			t.processDataMessage(msg.Header, msg.ItemData)
		case <-t.stopChan:
			return
		}
	}
}

// ============================================================
// 消息处理 - 统一入口
// ============================================================

// processDataMessage 处理数据消息 - 路由到 secsgem 的回调
func (t *HSMSTransport) processDataMessage(header HSMSHeader, itemData []byte) {

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

	// 内部处理控制消息
	t.handleControlInternal(header)

	// 按 SystemBytes 查找并投递控制回复（仅非请求方发起的控制消息才匹配）
	if ch, ok := t.pendingControl.Load(header.SystemBytes); ok {
		select {
		case ch.(chan HSMSHeader) <- header:
		default:
		}
	}
}

// drainChannels 排空消息通道，避免 goroutine 泄漏
func (t *HSMSTransport) drainChannels() {
	// 排空 writeChan 中的待处理请求，回复错误
	for {
		select {
		case req := <-t.writeChan:
			req.errCh <- ErrTransportStopped
		default:
			goto drainOthers
		}
	}
drainOthers:
	// 关闭 writeChan 让 writeLoop 退出（range 会自动退出）
	close(t.writeChan)
	// 排空 ctrlChan 和 dataChan
	for {
		select {
		case <-t.ctrlChan:
		case <-t.dataChan:
		default:
			return
		}
	}
}

// ============================================================
// 写者协程 - 所有 conn.Write 操作的唯一执行点
// ============================================================

// writeLoop 单写者循环：从 writeChan 取请求，串行执行 SetWriteDeadline + Write
func (t *HSMSTransport) writeLoop() {
	defer t.wg.Done()

	for req := range t.writeChan {
		t.mu.RLock()
		c := t.conn
		t.mu.RUnlock()

		if c == nil {
			req.errCh <- ErrNotConnected
			continue
		}

		// c.SetWriteDeadline(time.Now().Add(t.config.T8))
		_, err := c.Write(req.data)
		req.errCh <- err
	}
}

// handleControlInternal 内部处理控制消息
func (t *HSMSTransport) handleControlInternal(header HSMSHeader) {
	switch header.SType {
	case STypeSelectRsp:
		if header.HeaderByte3 == 0x00 {
			t.transitionState(StateSelected)
			t.signalT7Cancel()
		}

	case STypeSelectReq: // 服务端收到 Select.req
		t.sendSelectRsp(header.SystemBytes, 0x00)
		t.transitionState(StateSelected)
		t.signalT7Cancel()

	case STypeDeselectRsp:
		t.transitionState(StateConnected)

	case STypeDeselectReq: // 服务端收到 Deselect.req
		t.sendDeselectRsp(header.SystemBytes, 0x00)
		t.transitionState(StateConnected)

	case STypeLinktestReq: // 自动回复 LinkTest.rsp
		t.sendLinkTestRsp(header.SystemBytes)

	case STypeLinktestRsp:
		// LinkTest 响应，心跳检测使用

	case STypeRejectReq:
		// Reject 请求，日志已记录

	case STypeSeparateReq:
		t.handleDisconnect()
	}
}

// sendLinkTestRsp 发送 LinkTest.rsp
func (t *HSMSTransport) sendLinkTestRsp(systemBytes uint32) {
	header := BuildControlHeader(STypeLinktestRsp, systemBytes, 0)
	t.SendControl(header)
}

// sendSelectRsp 发送 Select.rsp
func (t *HSMSTransport) sendSelectRsp(systemBytes uint32, status byte) {
	header := BuildControlHeader(STypeSelectRsp, systemBytes, status)
	t.SendControl(header)
}

// sendDeselectRsp 发送 Deselect.rsp
func (t *HSMSTransport) sendDeselectRsp(systemBytes uint32, status byte) {
	header := BuildControlHeader(STypeDeselectRsp, systemBytes, status)
	t.SendControl(header)
}

// handleDisconnect 处理断开连接
func (t *HSMSTransport) handleDisconnect() {
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
	// 防护: 如果 connDone 已关闭，说明另一协程已完成断连处理，避免重复执行
	if t.connDone != nil {
		select {
		case <-t.connDone:
			t.mu.Unlock()
			return
		default:
		}
	}
	prevState := t.state
	t.state = StateDisconnected
	t.closeConnDoneLocked()

	// 合并：在同一次 Lock 中取出 conn 引用并清空
	conn := t.conn
	t.conn = nil
	t.mu.Unlock()

	if conn != nil {
		_ = conn.Close()
	}

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

// OnStateChange 设置状态变更回调
func (t *HSMSTransport) OnStateChange(handler StateChangeHandler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stateHandler = handler
}

// transitionState 原子地转换连接状态并通知监听器（Lock → 读取旧状态 → 设置新状态 → Unlock → 通知）
func (t *HSMSTransport) transitionState(newState ConnectionState) {
	t.mu.Lock()
	prevState := t.state
	t.state = newState
	t.mu.Unlock()
	t.notifyStateChange(prevState, newState)
}

// notifyStateChange 通知状态变更
func (t *HSMSTransport) notifyStateChange(oldState, newState ConnectionState) {
	t.mu.RLock()
	handler := t.stateHandler
	t.mu.RUnlock()

	if handler != nil {
		go handler(oldState, newState)
	}
}

// OnMessage 设置消息处理回调（收到数据消息时调用）
func (t *HSMSTransport) OnMessage(handler func(HSMSHeader, []byte)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.dataHandler = handler
}

// ============================================================
// 辅助方法 - 日志与默认回复
// ============================================================

// logReceivedControl 记录控制消息接收日志 (一行格式)
func (t *HSMSTransport) logReceivedControl(header HSMSHeader) {
	t.logger.Info("<<< Recv %s (SystemBytes=%d) HEX: %s", header.SType, header.SystemBytes, formatHexData(header.RawFrame))
}

// logSendControl 记录控制消息发送日志 (一行格式)
func (t *HSMSTransport) logSendControl(header HSMSHeader, frameData []byte) {
	t.logger.Info(">>> Send %s (SystemBytes=%d) HEX: %s", header.SType, header.SystemBytes, formatHexData(frameData))
}

// hexTable 预计算的字节到十六进制字符串查找表，避免每次调用 fmt.Sprintf
var hexTable [256]string

func init() {
	for i := 0; i < 256; i++ {
		hexTable[i] = fmt.Sprintf("%02X", i)
	}
}

// formatHexData 格式化16进制数据(每个字节用空格隔开)
func formatHexData(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	var sb strings.Builder
	// 每个字节 "XX " 占3个字符
	sb.Grow(len(data)*3 - 1)
	sb.WriteString(hexTable[data[0]])
	for _, b := range data[1:] {
		sb.WriteByte(' ')
		sb.WriteString(hexTable[b])
	}
	return sb.String()
}
