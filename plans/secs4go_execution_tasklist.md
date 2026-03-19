# Secs4Go 可执行任务单（承接 A / C）

## 1. 说明

本任务单承接：
- `plans/secs4go_core_refactor_roadmap.md`
- `plans/secs4go_minimal_change_priority.md`

目标不是继续做分析，而是把“下一步具体做什么”写清楚，便于直接开工。

执行原则：
- 先小步修正确性；
- 每一步都应可单测；
- 先保兼容，再考虑 API 收口。

## 2. 当前执行进度（截至 2026-03-18）

### 已完成的核心层改造

- [x] Task 1：为 reply 链路补最小测试
- [x] Task 2：修复 reply 解析使用请求头的问题
- [x] Task 3：修复 `WBit=false` 主消息被误吞
- [x] Task 4：把配置校验前置到启动入口
- [x] Task 5：未知编码改为告警并回退 ASCII
- [x] Task 6：默认 logger 改为静默实现
- [x] Task 7：重新定义 `Message` 的职责边界
- [x] Task 8：减少日志路径的再编码行为

### 已完成的验证

1. 单测与回归：`go test ./secs4go -run "TestSecsGem|TestParseMessageCapturesProtocolSnapshot|TestNewItemCodec"` 已覆盖首轮关键变更
2. 全量回归：`go test ./...` 返回码为 `0`
3. example 联调：已在 `7000` 端口完成 `server/client` 端到端验证
   - server 进入 `Listening -> Connected -> Selected`
   - client 收到 `S6F11` 并发送 `S6F12`
   - 观察超过 `T3` 窗口未见超时失败日志，连接保持 `ESTABLISHED`

### 当前剩余项

- [ ] Task 9：为 `Item` 增加 typed accessor
- [ ] Task 10：补 README 和核心边界说明

## 3. 第一轮任务（必须先做，已完成）

### [x] Task 1：为 reply 链路补最小测试

**目标**：给后续修改 `SecsGem` 提供回归保护。

**涉及文件**：
- `secs4go/secsgem.go`
- 新增：`secs4go/secsgem_test.go`

**最少覆盖点**：
1. 请求发出后，收到 reply 时能返回完整 `Message`
2. reply 的 `SystemBytes` 与实际收到的一致
3. reply 的 `WBit`、`Stream`、`Function` 不沿用请求头
4. T3 超时返回 `ErrTimeoutT3`

**完成标志**：
- 这些行为可以先被测试描述出来，即使测试一开始失败也可以

---

### [x] Task 2：修复 reply 解析使用请求头的问题

**目标**：`Send()` 返回真实 reply 语义。

**涉及文件**：
- `secs4go/secsgem.go`

**实施步骤**：
1. 把 `replyResult` 从仅保存 `data []byte` 改为保存：
   - `header HSMSHeader`
   - `data []byte`
   - `err error`
2. `handleReply()` 投递真实 reply header 和 data
3. `sendAndWait()` 返回完整 replyResult，而不是只返回 `[]byte`
4. `Send()` 用真实 reply header 调 `ParseMessage()`

**完成标志**：
- `Send()` 返回的 reply 头字段来自真实回复帧

---

### [x] Task 3：修复 `WBit=false` 主消息被误吞

**目标**：区分“reply”和“无需回复的主消息”。

**涉及文件**：
- `secs4go/secsgem.go`
- `secs4go/secsgem_test.go`

**实施步骤**：
1. `handleDataMessage()` 不再用 `!msg.WBit` 直接判定 reply
2. 先按 `SystemBytes` 查询 `pendingReplies`
3. 若命中 pending request，则走 reply 通道
4. 若未命中，则作为普通上行消息回调给 `OnMessage()`

**完成标志**：
- `WBit=false` 且未命中 pending 的消息不会丢失

## 4. 第二轮任务（低风险高收益，已完成）

### [x] Task 4：把配置校验前置到启动入口

**目标**：无效配置尽早失败。

**涉及文件**：
- `secs4go/config.go`
- `secs4go/hsms_transport.go`

**实施步骤**：
1. 在 `Start()` 入口统一调用 `config.Validate()`
2. 对 nil config 做显式保护
3. 让空地址、非法超时等基础配置在启动前失败返回

**完成标志**：
- 非法配置在启动前直接返回错误

---

### [x] Task 5：未知编码改为告警并回退 ASCII

**目标**：保留 ASCII 作为标准默认编码，同时让未知编码配置具备可观测性。

**涉及文件**：
- `secs4go/secs_item_codec.go`
- 新增或补充：`secs4go/secs_item_codec_test.go`

**实施步骤**：
1. 未知编码输出 warning
2. 默认回退到 ASCII 路径继续编解码
3. 补充测试覆盖 warning 与 fallback 行为

**完成标志**：
- 未知编码不会中断流程，且会显式告警后回退到 ASCII

---

### [x] Task 6：默认 logger 改为静默实现

**目标**：让库默认行为零输出、零落盘。

**涉及文件**：
- `secs4go/secsgem.go`
- `secs4go/logger.go`

**实施步骤**：
1. `logger=nil` 时默认使用 `NewSilentLogger()`
2. 保留 `NewFileLogger()`，但改为显式选择
3. `SecsGem` 与 `HSMSTransport` 独立使用时都保持静默默认值

**完成标志**：
- 默认创建 `SecsGem` / `HSMSTransport` 不会自动打印或落盘到 `logs/`

## 5. 第三轮任务（结构净化，已完成）

### [x] Task 7：重新定义 `Message` 的职责边界

**目标**：让 `Message` 更像协议对象，而不是运行时粘合对象。

**涉及文件**：
- `secs4go/message.go`
- `secs4go/secsgem.go`

**建议方向**：
1. `Message` 表示“一条完整数据消息”
2. 可以包含：
   - `HSMSHeader`
   - 原始字节数据
   - `Item`
   - 时间戳
3. 不建议继续把 `sender *HSMSTransport` 作为核心字段长期保留
4. “请求-回复对”建议单独抽成 `Transaction` / `Exchange`

**完成标志**：
- `Message` 能独立表达消息本身
- reply 能力不再依赖消息对象偷偷持有 transport

---

### [x] Task 8：减少日志路径的再编码行为

**目标**：日志观察不再深度耦合主流程。

**涉及文件**：
- `secs4go/secsgem.go`

**实施步骤**：
1. 接收日志优先基于真实收到的 `Header` / `RawFrame` 输出
2. `FormatSML(msg.Item)` 仍可保留
3. 避免为了日志重新编码 `Item` 或重新拼装 frame

**完成标志**：
- 日志不再依赖“重编码成功”才能输出

## 6. 第四轮任务（体验收口，待继续）

### [ ] Task 9：为 `Item` 增加 typed accessor

**目标**：减少裸类型断言。

**涉及文件**：
- `secs4go/item.go`
- `secs4go/secs_item_codec_test.go` 或 `item_test.go`

**建议优先加的 helper**：
- `AsBytes()`
- `AsString()`
- `AsUint16Slice()`
- `AsList()`

**完成标志**：
- 常见业务读取不需要大量 `item.Value.(...)`

---

### [ ] Task 10：补 README 和核心边界说明

**目标**：把结构收敛结果文档化。

**涉及文件**：
- `README.md`

**最少应写清楚**：
1. `HSMSTransport` 做什么
2. `SecsGem` 做什么
3. `Message` 表示什么
4. 如何发送、接收、回复
5. 日志和编码策略如何配置

## 7. 推荐执行顺序

建议按下面顺序实际施工：

1. Task 1
2. Task 2
3. Task 3
4. Task 4
5. Task 5
6. Task 6
7. Task 7
8. Task 8
9. Task 9
10. Task 10

## 8. 当前结论

当前任务单里，最关键的首轮改造已经落地：

1. **reply 链路测试、真实 reply 解析、`WBit=false` 主消息路由修复已完成；**
2. **配置前置校验、编码告警回退、默认静默 logger 已完成；**
3. **`Message` 纯协议快照化、接收日志 `RawFrame` 化、example 端到端验证已完成。**

当前可以继续推进的主线，已经收敛为两类：

- 核心体验收口：`Item` typed accessor、README / 边界文档
- 后续结构演进：`Transaction / Exchange` 建模（可在下一版任务单中单列）