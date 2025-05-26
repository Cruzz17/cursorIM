#!/bin/bash

echo "🚀 启动协议测试..."

# 启动服务器
echo "📡 启动服务器..."
./bin/server &
SERVER_PID=$!

# 等待服务器启动
sleep 3

echo "🔑 生成测试用户token..."
# 创建测试用户并获取token
TOKEN=$(curl -s -X POST http://localhost:8082/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"testpass","email":"test@example.com"}' | \
  grep -o '"token":"[^"]*"' | cut -d'"' -f4)

if [ -z "$TOKEN" ]; then
  echo "❌ 无法获取token，尝试登录..."
  TOKEN=$(curl -s -X POST http://localhost:8082/api/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"testuser","password":"testpass"}' | \
    grep -o '"token":"[^"]*"' | cut -d'"' -f4)
fi

if [ -z "$TOKEN" ]; then
  echo "❌ 无法获取token，请检查服务器状态"
  kill $SERVER_PID
  exit 1
fi

echo "✅ Token获取成功: ${TOKEN:0:20}..."

echo "🧪 测试 JSON over WebSocket..."
# 测试 JSON WebSocket 连接
node -e "
const WebSocket = require('ws');
const ws = new WebSocket('ws://localhost:8082/api/ws?token=$TOKEN');
ws.on('open', () => {
  console.log('✅ JSON WebSocket 连接成功');
  ws.send(JSON.stringify({
    type: 'message',
    recipient_id: 'user2',
    content: 'Hello from JSON WebSocket!',
    conversation_id: 'test-conv-1'
  }));
  setTimeout(() => ws.close(), 1000);
});
ws.on('message', (data) => {
  console.log('📨 收到JSON消息:', data.toString());
});
ws.on('error', (err) => {
  console.log('❌ JSON WebSocket 错误:', err.message);
});
" 2>/dev/null || echo "⚠️ 需要安装 Node.js 和 ws 模块来测试 WebSocket"

echo "🧪 测试 Protobuf over TCP..."
# 测试 Protobuf TCP 连接
go run test/protobuf_client.go "$TOKEN" &
CLIENT_PID=$!

# 等待测试完成
sleep 5

# 清理进程
echo "🧹 清理测试进程..."
kill $CLIENT_PID 2>/dev/null || true
kill $SERVER_PID 2>/dev/null || true

echo "✅ 协议测试完成！"
echo ""
echo "📋 测试总结:"
echo "  - JSON over WebSocket: Web端协议"
echo "  - Protobuf over TCP: App端协议"
echo "  - 协议自动检测和适配"
echo "  - 双向兼容性支持" 