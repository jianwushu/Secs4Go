# HSMSTransport 优化计划（代码验证版）

> 文件: [`core/hsms_transport.go`](../core/hsms_transport.go)
> 分析日期: 2026-06-17
> 最后更新: 2026-06-17（逐条代码验证）

---

## 实施状态总览

| # | 项目 | 优先级 | 状态 | 验证结论 |
|---|------|--------|------|----------|
| 1 | T7 超时从未生效 | P0 | ✅ 已修复 | — |
| 2 | Connect Select 失败后未清理 | P0 | ✅ 已修复 | — |
| 3 | handleDisconnect 重复派生 goroutine | P0 | ⚠️ 经核实非问题 | — |
| 4 | Connect 持锁期间阻塞拨号 | P1 | ✅ 已修复 | — |
| 5 | heartbeatLoop Timer 泄漏 | P1 | ⬜ 待处理 | ✅ 确认存在 |
| 6 | autoReconnectLoop time.After 泄漏 | P1→P2 | ⬜ 待处理 | ⚠️ 影响降级，见详述 |
| 7 | enqueueWrite ConnDone 临时分配 | P1→P2 | ⬜ 待处理 | ⚠️ 影响降级，见详述 |
| 8 | T8 字符间超时未实现 | P1 | ⬜ 待处理 | ✅ 确认存在 |
| 9 | readHSMSFrame 两次堆分配 | P2 | ⬜ 待处理 | ✅ 确认存在，低优先 |
| 10 | BuildCompleteFrame 三次分配 | P2 | ⬜ 待处理 | ✅ 确认存在，低优先 |
| 11 | formatHexData 分配 | P2 | ⬜ 待处理 | ✅ 确认存在，低优先 |
| 12 | 日志路径无条件重建完整帧 | P2 | ⬜ 待处理 | ✅ 确认存在，低优先 |
| 13 | enqueueWrite 每次创建 errCh | P2 | ⬜ 待处理 | ✅ 确认存在，低优先 |
| 14 | Channel 容量魔法数字 | P3 | ⬜ 待处理 | ✅ 确认存在 |
| 15 | drainChannels 使用 goto | P3 | ⬜ 待处理 | ✅ 确认存在 |
| 16 | handleControlInternal 状态转换重复 | P3 | ⬜ 待处理 | ✅ 确认存在 |
| 17 | stopping 用 atomic.Bool | P3 | ⬜ 待处理 | ✅ 确认存在 |
| 18 | Start() 防重入保护 | P3 | ⬜ 待处理 | ✅ 确认存在 |
| 19 | SendControlAndWait stopChan 错误码不一致 | P3 | ⬜ 待处理 | ✅ 确认存在 |

---

## 一、P0 — 逻辑缺陷/Bug（已全部完成）

### ✅ 1. T7 超时从未生效 — 已修复 (2026-06-17)

### ✅ 2. Connect Select 失败后未清理 — 已修复 (2026-06-17)

### ⚠️ 3. handleDisconnect 重复派生 — 经核实非问题

---

## 二、P1 — 并发安全/资源泄漏

### ✅ 4. Connect 持锁期间阻塞拨号 — 已修复 (2026-06-17)

### ⬜ 5. heartbeatLoop Timer 泄漏 — ✅ 确认存在

**位置**: [`heartbeatLoop()`](../core/hsms_transport.go:300) `:306-338`

```go
heartbeatTimer := time.NewTimer(t.config.HeartbeatInterval)
select {
case <-heartbeatTimer.C:
    ...
case <-t.stopChan:
    return  // ← Timer 未 Stop，残留至触发后才 GC
}
```

**验证**: 确认存在。`stopChan` 触发时直接 return，Timer 对象在 HeartbeatInterval（默认 60s）后才触发并被 GC。虽然最终会自清理，但在 Stop/Start 生命周期内残留不必要的 Timer。

**修法**: return 前调 `heartbeatTimer.Stop()`。一行改动。

---

### ⬜ 6. autoReconnectLoop time.After 泄漏 — ⚠️ 影响降级

**位置**: [`autoReconnectLoop()`](../core/hsms_transport.go:248) `:280`

```go
case <-time.After(t.config.T5):
```

**验证**: 问题存在但**严重程度被高估**。`time.After` 返回的 Timer 在 T5（默认 10s）后自动触发并被 GC，不是真正的"泄漏"——只是延迟回收。且 `autoReconnectLoop` 仅在断连重试时执行，不是热路径。

**建议**: 优先级从 P1 降为 P2。改用 `time.NewTimer` + `defer Stop()` 是好习惯，但非紧迫。

---

### ⬜ 7. ConnDone 热路径临时分配 — ⚠️ 影响降级

**位置**: [`ConnDone()`](../core/hsms_transport.go:227) `:231-234`

```go
if t.connDone == nil {
    done := make(chan struct{})
    close(done)
    return done
}
```

**验证**: 问题存在但**"热路径"描述不准确**。`connDone == nil` 仅在 `StateDisconnected` 时成立（无连接时）。正常通信期间 `connDone` 非 nil，走 `return t.connConn` 无分配。`enqueueWrite` 是热路径，但其中 `ConnDone()` 的 `nil` 分支只在断连后触发——此时上层通常已停止发送。

**建议**: 优先级从 P1 降为 P2。缓存包级 closed chan 的修法正确且简单（约 3 行），顺手做即可。

---

### ⬜ 8. T8 字符间超时未实现 — ✅ 确认存在

**位置**: [`receiveLoop()`](../core/hsms_transport.go:770) `:775`

```go
// T8: 设置读取超时
// conn.SetReadDeadline(time.Now().Add(t.config.T8))
```

**验证**: 确认存在。T8 被注释掉。HSMS 规范要求 T8 检测半开连接（对端崩溃不发 FIN）。当前依赖 `heartbeatLoop`（默认 60s 间隔）间接检测——心跳 `LinkTestReq` 发送失败会触发 `handleDisconnect`，但检测延迟远大于 T8（默认 5s）。

**修法**: 取消注释并在 `ReadHSMSFrame` 前设置 `SetReadDeadline`。超时后检查错误类型：`net.Error.Timeout()` → 调 `handleDisconnect`。

---

## 三、P2 — 性能优化

### ⬜ 9. ReadHSMSFrame 两次堆分配

**位置**: [`ReadHSMSFrame()`](../core/hsms_header.go:126) `:128,140`

```go
lengthBuf := make([]byte, 4)     // 分配 1
frameData := make([]byte, frameLen) // 分配 2
```

**验证**: 确认存在。每帧读取两次 `make`。`lengthBuf` 可改栈数组 `[4]byte`；`frameData` 因长度由帧决定，sync.Pool 需注意 itemData slice 生命周期。

**建议**: 按需优化。SECS/GEM 典型消息频率不高（非高频交易场景），GC 压力可忽略。仅在 benchmark 证实瓶颈时再做。

---

### ⬜ 13. enqueueWrite 每次创建 errCh

**位置**: [`enqueueWrite()`](../core/hsms_transport.go:611) `:612`

```go
errCh := make(chan error, 1)
```

**验证**: 确认存在。每次发送创建一个 buffered chan。但 `chan error` 本身开销很小（一个 runtime hchan 结构 ≈ 96 bytes），且消息发送频率受 SECS/GEM 协议限制，不是微秒级热路径。

**建议**: 低优先。sync.Pool 复用 `writeRequest` 可做，但增加了 drain 复杂度，投入产出比不高。

---

## 四、P3 — 代码质量/可维护性

### ⬜ 16. handleControlInternal 状态转换重复

**位置**: [`handleControlInternal()`](../core/hsms_transport.go:929) `:942-977`

**验证**: 确认存在。`Lock → prevState → state=... → Unlock → notifyStateChange` 模式在 SelectRsp/SelectReq/DeselectRsp/DeselectReq 四处重复。可提取 `transitionState(newState)` 辅助方法。

---

### ⬜ 17. stopping 用 atomic.Bool

**位置**: [`stopping`](../core/hsms_transport.go:58) `:58`

**验证**: 确认存在。当前用 `mu.Lock/RLock` 保护，功能正确。改 `atomic.Bool` 更轻量，但当前方案无 bug。


**验证**: 确认存在。`enqueueWrite` 的 stopChan 分支返回 `ErrTransportStopped`，但 `SendControlAndWait` 返回 `ErrNotConnected`，语义不一致。

**修法**: 改为 `return ErrTransportStopped`。一行改动。

---


---

## 实施建议

### 值得做（有实际收益）

| 优先级 | 项目 | 改动量 | 理由 |
|--------|------|--------|------|
| **P1** | #5 heartbeatLoop Timer 泄漏 | 1 行 | Stop 流程资源清理 |
| **P1** | #8 T8 字符间超时 | ~10 行 | HSMS 规范合规，加速死连接检测 |
| **P2** | #6 time.After 泄漏 | ~5 行 | 好习惯，顺手做 |
| **P2** | #7 ConnDone 缓存 | ~3 行 | 简单优化 |

### 按需做（收益有限，不建议单独排期）

| 项目 | 说明 |
|------|------|
| #9-#13 性能分配 | SECS/GEM 消息频率不高，GC 压力可忽略，仅在 benchmark 证实瓶颈时再做 |
| #14-#17 代码质量 | 日常维护中逐步改进即可 |

