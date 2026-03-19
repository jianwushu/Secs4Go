# Secs4Go example 层优化建议（2026-03-18）

## 1. 当前结论

基于本轮联调结果，`example/client` 与 `example/server` 已经具备以下能力：

- 可独立编译运行
- 可在 `7000` 端口完成建连与 `Selected` 会话建立
- 可完成 `S6F11 -> S6F12` 事件上报/回复链路验证

这说明 example **已经具备演示价值**，但从代码组织和长期维护角度看，它目前更像“可运行脚本集合”，还不是“稳定、可扩展、可教学的示例层”。

## 2. 建议优先级总览

### P0：优先修正正确性与可控性

1. 端口、设备号、超时、事件周期改为可配置
2. 修复 example 中已出现的拷贝式错误与协议响应不一致问题
3. 避免事件发送循环空转与不可控 goroutine
4. 为 Item 解析增加防御，避免异常报文直接 panic
5. 停止吞掉关键转换/更新错误

### P1：优化结构与可维护性

1. 去掉 `server` / `client` / 各类 map 的全局状态
2. 为 `SvMap` / `DvMap` / `EventLinks` / `ReportLinks` 增加并发保护
3. 拆分“启动逻辑”和“协议处理逻辑”
4. 收敛重复的 Item 解析与回复构造代码

### P2：提升演示与交付体验

1. 增加 example README 与启动说明
2. 增加 smoke test / demo script
3. 增加更明确的日志与场景开关

## 3. 关键问题分析

### 3.1 配置硬编码过多，联调成本偏高

当前 `example/client/main.go` 与 `example/server/main.go` 中直接写死了：

- 地址与端口
- `DeviceID`
- `T3`
- 心跳开关
- 事件发送节奏

这会导致：

- 环境一旦有端口冲突，就需要改代码重新编译
- 很难并行演示多个实例
- example 更像“本机临时代码”，不利于别人复现

**建议**：

- 优先支持命令行参数或环境变量，例如 `--addr`、`--device-id`、`--t3`、`--event-interval`
- 保留默认值，但不要把默认值硬编码成唯一入口

### 3.2 存在明显的拷贝式错误，降低示例可信度

目前 example 层已经能跑通，但代码中存在几处容易误导使用者的问题：

- `example/client/main.go` 的 `S6F11` 分支里，失败日志仍写成了“发送S1F4失败”
- `example/server/device.go` 的 `HandleS1F11()` 在主路径里返回的是 `NewMessage(1, 4)`，与函数名语义不一致
- `example/server/device.go` 的 `HandleS2F37()` 成功分支返回 `LRACKMessage(ERACK0)`，ACK 类型表达不统一

这些问题未必都会立刻导致联调失败，但会：

- 误导阅读者理解协议
- 让 example 失去“参考实现”的可信度

**建议**：

- 把 example 当成“对外示范代码”来维护
- 增加最小 smoke test 或 handler 单测，专门覆盖这些 S/F 返回值和 ACK 类型

### 3.3 事件发送循环存在空转与生命周期失控问题

`example/server/main.go` 中的 `testSendMessage()` 目前有几个问题：

- 当 `server.IsSelected()` 为 `false` 时，循环没有 sleep，存在空转风险
- 发送逻辑通过匿名 goroutine 启动，但没有上下文控制与退出机制
- 事件周期写死为 `45s`，不利于演示、测试与故障排查

**建议**：

- 改为 `ticker + context.Context` 的结构
- 未进入 `Selected` 时至少增加小 sleep/backoff
- 把事件发送器封装成独立组件，例如 `EventPublisher`

## 4. 结构层面建议

### 4.1 去全局状态

当前 example 中大量依赖全局变量：

- `var server *secs4go.SecsGem`
- `var client *secs4go.SecsGem`
- `SvMap` / `DvMap` / `EvMap` / `EventLinks` / `ReportLinks`

这会导致：

- 启动、停止、重连、测试都难以隔离
- 逻辑难以复用
- 单测很难并行跑

**建议**：

- 抽出 `ServerApp` / `ClientApp` 结构体
- 再抽出 `DeviceModel` 保存动态变量、报告和事件配置

### 4.2 为共享状态增加并发保护

`UpdateDv()`、`Trigger10020()`、`buildEvent()`、`HandleS2F33()`、`HandleS2F35()`、`HandleS2F37()` 都会读写共享 map。

一旦未来出现：

- Host 动态下发报告定义
- 设备侧并行上报事件
- 多 goroutine 同时读写数据变量

就可能触发数据竞争。

**建议**：

- 至少为 `DeviceModel` 增加 `sync.RWMutex`
- 所有 map 的读写都通过方法封装，不直接裸访问

### 4.3 拆分启动层与协议层

当前 `main.go` 同时负责：

- 组装配置
- 启动连接
- 注册回调
- 业务消息处理
- 演示事件发送

职责过于集中。

**建议**：

- `main.go` 只做 wiring
- `handlers.go` 放协议处理
- `app.go` 管理生命周期
- `model.go` 管理设备数据与事件配置

## 5. 健壮性建议

### 5.1 Item 解析要从“示例能跑”升级到“异常可控”

`device.go` 中大量直接使用：

- `child.Value.([]uint16)[0]`
- `child.Value.([]uint32)[0]`
- `item.GetItem(1)`

这种写法对“理想报文”很方便，但对异常输入非常脆弱。

**建议**：

- 封装统一 helper，例如 `readU2IDs(item)`、`readU4ID(item)`
- 先检查 `IsList()`、`GetLength()`、`Type`
- 返回协议级错误，而不是直接 panic

### 5.2 不要吞掉基础转换错误

`example/server/tool.go` 中的 `String2UInt32()` 一类 helper 在转换失败时直接返回 `0`。

这会让很多错误变成“表面正常、语义错误”的状态。

**建议**：

- 返回 `(value, error)`
- 在上层决定是否 fallback
- 至少在 example 中打印清晰错误

### 5.3 日志输出策略要显式

目前 example 更多依赖标准库 `log`，输出大多走 stderr；在自动化验证时，stdout/stderr 分离会让观察成本变高。

**建议**：

- 明确约定 example 的日志输出方式
- 为关键日志附带 `S/F`、`SystemBytes`、连接状态
- 如果要保留默认静默库行为，example 应显式创建自己的示例 logger

## 6. 推荐落地顺序

### 第一批（建议立刻做）

1. 端口/设备号/超时改为可配置
2. 修复 `S6F11` 日志文案、`HandleS1F11()`、`HandleS2F37()` 等明显不一致点
3. 把 `testSendMessage()` 改为 `ticker + context`，并避免未选中时空转

### 第二批（建议随后做）

1. 引入 `DeviceModel + RWMutex`
2. 去掉全局 map 与全局 `server/client`
3. 抽统一 Item 解析 helper

### 第三批（建议作为交付收口）

1. 补 `example/README.md`
2. 增加一条自动化 smoke path
3. 提供一键启动脚本或命令示例

## 7. 建议新增的示例层产物

- `example/common/config.go`：统一解析 CLI / ENV 配置
- `example/common/model.go`：封装设备变量、报告、事件配置与锁
- `example/server/app.go`：服务端生命周期管理
- `example/client/app.go`：客户端生命周期管理
- `example/README.md`：说明启动方式、报文流程、默认端口与预期日志

## 8. 一句话结论

当前 example 已经“能跑且验证通过”，下一步最值得做的不是继续堆功能，而是把它从“本地演示脚本”收敛成“可配置、可维护、可复用、可教学”的正式示例层。