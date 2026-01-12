# Item.A 编码配置实施计划

## 1. 依赖管理
- [ ] 添加 `golang.org/x/text` 依赖，用于支持 GBK 等编码。

## 2. 核心编解码重构 (`secs4go/secs_item_codec.go`)
- [ ] 定义 `ItemCodec` 结构体，包含 `encoding.Encoding` 字段。
- [ ] 实现 `NewItemCodec(encodingName string) (*ItemCodec, error)` 工厂方法。
- [ ] 将现有的编解码逻辑迁移为 `ItemCodec` 的方法：
    - `func (c *ItemCodec) EncodeItem(item *Item) ([]byte, error)`
    - `func (c *ItemCodec) DecodeItem(data []byte) (*Item, int, error)`
    - 相关的辅助函数（如 `itemValueToBytes`, `itemBytesToValue`, `encodeBinary` 等）也需要调整为方法或接受编码器参数。
- [ ] 保留包级函数 `EncodeItem` 和 `DecodeItem` 以保持兼容性，它们将使用默认的 UTF-8 编解码器。

## 3. 配置更新 (`secs4go/config.go`)
- [ ] 在 `Config` 结构体中添加 `ItemAEncoding string` 字段。
- [ ] 定义编码常量：`EncodingUTF8` (默认), `EncodingGBK`, `EncodingGB2312`。
- [ ] 在 `DefaultConfig` 中设置默认值为 `EncodingUTF8`。

## 4. 消息处理更新 (`secs4go/message.go`)
- [ ] 修改 `ParseMessage` 函数签名，增加 `codec *ItemCodec` 参数。
    - `func ParseMessage(header HSMSHeader, data []byte, sender *HSMSTransport, codec *ItemCodec) (*Message, error)`

## 5. 传输层集成 (`secs4go/hsms_transport.go`)
- [ ] 在 `HSMSTransport` 结构体中添加 `codec *ItemCodec` 字段。
- [ ] 在 `NewHSMSTransport` 中根据 `config.ItemAEncoding` 初始化 `codec`。
- [ ] 更新 `processDataMessage` 方法，在调用 `ParseMessage` 时传入 `t.codec`。

## 6. 应用层集成 (`secs4go/secsgem.go`)
- [ ] 在 `SecsGem` 结构体中添加 `codec *ItemCodec` 字段（或者复用 transport 的）。
- [ ] 在 `NewSecsGem` 中初始化 `codec` (可以与 transport 共享同一个实例)。
- [ ] 更新 `Send` 和 `SendReply` 方法，使用 `s.codec.EncodeItem` 替代包级函数。
- [ ] 更新 `logReceivedData` 方法，使用 `s.codec.EncodeItem` 进行日志记录时的编码。

## 7. 测试验证
- [ ] 创建测试用例，配置 `ItemAEncoding` 为 "GBK"。
- [ ] 构造包含中文字符的 `Item.A`。
- [ ] 验证编码后的字节序列是否符合 GBK 编码。
- [ ] 验证解码过程能否正确还原中文字符串。
