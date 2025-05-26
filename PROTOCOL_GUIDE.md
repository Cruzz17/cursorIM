# 协议支持指南

## 概述

本 IM 系统支持双协议架构：
- **Web端**: JSON over WebSocket（降级兼容）
- **App端**: Protobuf over TCP/WebSocket（高性能）

## 协议类型

### 1. JSON 协议 (Web端)
- **传输方式**: WebSocket 文本消息
- **序列化**: JSON
- **适用场景**: Web浏览器、轻量级客户端
- **连接地址**: `ws://localhost:8082/api/ws?token=<JWT_TOKEN>`

### 2. Protobuf 协议 (App端)
- **传输方式**: TCP 二进制流 / WebSocket 二进制消息
- **序列化**: Protocol Buffers
- **适用场景**: 移动App、桌面应用
- **连接地址**: 
  - TCP: `localhost:8083`
  - WebSocket: `ws://localhost:8082/api/ws-tcp`

## 消息格式

### JSON 消息示例
```json
{
  "type": "message",
  "id": "msg-123",
  "sender_id": "user1",
  "recipient_id": "user2",
  "content": "Hello World!",
  "conversation_id": "conv-456",
  "timestamp": 1640995200,
  "is_group": false
}
```

### Protobuf 消息定义
```protobuf
message Message {
  string version = 1;
  MessageType type = 2;
  string id = 6;
  string sender_id = 7;
  string recipient_id = 8;
  string content = 9;
  int64 timestamp = 10;
  string conversation_id = 11;
  bool is_group = 12;
  string group_id = 13;
  // ... 更多字段
}
```

## 协议自动检测

系统会根据连接类型自动选择协议：

| 连接类型 | 协议类型 | 说明 |
|---------|---------|------|
| `websocket` | JSON | Web端标准连接 |
| `tcp_ws` | Protobuf | App端WebSocket连接 |
| `tcp` | Protobuf | App端TCP连接 |

## TCP 协议格式

TCP 连接使用以下帧格式：
```
[协议标识符:1字节][消息长度:4字节][消息数据:N字节]
```

- **协议标识符**: 
  - `0x01`: JSON
  - `0x02`: Protobuf
- **消息长度**: 大端序 uint32
- **消息数据**: 序列化后的消息内容

## 认证流程

### WebSocket 认证
```
GET /api/ws?token=<JWT_TOKEN>
```

### TCP 认证
```
客户端 -> 服务器: AUTH <JWT_TOKEN>\n
服务器 -> 客户端: OK\n (成功) 或 ERROR <reason>\n (失败)
```

## 客户端示例

### JavaScript (Web端)
```javascript
const ws = new WebSocket('ws://localhost:8082/api/ws?token=' + token);

ws.onopen = () => {
  ws.send(JSON.stringify({
    type: 'message',
    recipient_id: 'user2',
    content: 'Hello!',
    conversation_id: 'conv-1'
  }));
};

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  console.log('收到消息:', message);
};
```

### Go (App端 - TCP)
```go
// 连接
conn, err := net.Dial("tcp", "localhost:8083")

// 认证
fmt.Fprintf(conn, "AUTH %s\n", token)

// 发送 Protobuf 消息
adapter := protocol.NewMessageAdapter()
data, _ := adapter.SerializeMessage(msg, protocol.ProtocolTypeProtobuf)

// 写入帧头
writer.WriteByte(0x02) // Protobuf 标识符
binary.Write(writer, binary.BigEndian, uint32(len(data)))
writer.Write(data)
writer.Flush()
```

## 性能对比

| 特性 | JSON | Protobuf |
|------|------|----------|
| 消息大小 | 较大 | 较小 (约50%减少) |
| 解析速度 | 中等 | 快速 |
| 可读性 | 高 | 低 |
| 兼容性 | 优秀 | 需要schema |
| 适用场景 | Web端 | 移动端/高性能 |

## 测试工具

### 运行协议测试
```bash
# 编译服务器
go build -o bin/server cmd/main.go

# 运行协议测试
./test_protocols.sh
```

### 手动测试 Protobuf 客户端
```bash
# 获取用户token
TOKEN="your_jwt_token_here"

# 运行 Protobuf 测试客户端
go run test/protobuf_client.go $TOKEN
```

## 故障排除

### 常见问题

1. **连接被拒绝**
   - 检查服务器是否启动
   - 验证端口是否正确

2. **认证失败**
   - 检查JWT token是否有效
   - 确认token格式正确

3. **消息解析失败**
   - 检查协议类型是否匹配
   - 验证消息格式是否正确

### 调试技巧

1. **启用详细日志**
   ```bash
   export GIN_MODE=debug
   ./bin/server
   ```

2. **检查网络连接**
   ```bash
   # 测试TCP端口
   telnet localhost 8083
   
   # 测试WebSocket
   curl -i -N -H "Connection: Upgrade" \
        -H "Upgrade: websocket" \
        -H "Sec-WebSocket-Key: test" \
        -H "Sec-WebSocket-Version: 13" \
        http://localhost:8082/api/ws?token=test
   ```

## 扩展开发

### 添加新的消息类型

1. 更新 `proto/message.proto`
2. 重新生成 Go 代码: `protoc --go_out=. proto/message.proto`
3. 更新适配器中的类型转换逻辑
4. 在客户端实现对应的处理逻辑

### 自定义协议

可以通过实现 `EnhancedConnection` 接口来支持自定义协议：

```go
type CustomConnection struct {
    *ProtocolAwareConnection
    // 自定义字段
}

func (c *CustomConnection) SendMessageWithProtocol(msg *protocol.Message, protocolType protocol.ProtocolType) error {
    // 自定义协议实现
}
``` 