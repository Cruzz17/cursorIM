#!/bin/bash

# CursorIM 全栈启动脚本

echo "🚀 启动 CursorIM 即时通讯系统 (前端 + 后端)..."

# 检查Go环境
if ! command -v go &> /dev/null; then
    echo "❌ 错误: 未找到Go环境，请先安装Go 1.19+"
    exit 1
fi

# 检查Node.js环境
if ! command -v node &> /dev/null; then
    echo "❌ 错误: 未找到Node.js环境，请先安装Node.js 16+"
    exit 1
fi

# 检查配置文件
if [ ! -f "config.yaml" ]; then
    echo "❌ 错误: 未找到配置文件 config.yaml"
    echo "请参考README.md配置数据库连接信息"
    exit 1
fi

# 启动后端服务
echo "📦 启动后端服务..."
echo "安装Go依赖..."
go mod download

echo "编译后端项目..."
go build -o cursorIM cmd/main.go

if [ $? -ne 0 ]; then
    echo "❌ 后端编译失败"
    exit 1
fi

echo "启动后端服务 (端口: 8082)..."
./cursorIM &
BACKEND_PID=$!

# 等待后端启动
sleep 3

# 启动前端应用
echo "🎨 启动前端应用..."
cd web/im-web

echo "安装前端依赖..."
npm install

if [ $? -ne 0 ]; then
    echo "❌ 前端依赖安装失败"
    kill $BACKEND_PID
    exit 1
fi

echo "启动前端开发服务器 (端口: 3000)..."
npm start &
FRONTEND_PID=$!

cd ../..

echo ""
echo "🎉 系统启动成功!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🖥️  前端地址: http://localhost:3000"
echo "🔧 后端API: http://localhost:8082/api"
echo "📚 简单前端: http://localhost:8082/web/index.html"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "按 Ctrl+C 停止所有服务"

# 等待用户中断
wait

# 清理进程
echo ""
echo "🛑 正在停止服务..."
kill $BACKEND_PID 2>/dev/null
kill $FRONTEND_PID 2>/dev/null
echo "✅ 服务已停止" 