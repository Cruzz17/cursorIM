#!/bin/bash

# CursorIM 启动脚本

echo "🚀 启动 CursorIM 即时通讯系统..."

# 检查Go环境
if ! command -v go &> /dev/null; then
    echo "❌ 错误: 未找到Go环境，请先安装Go 1.19+"
    exit 1
fi

# 检查配置文件
if [ ! -f "config.yaml" ]; then
    echo "❌ 错误: 未找到配置文件 config.yaml"
    echo "请参考README.md配置数据库连接信息"
    exit 1
fi

# 安装依赖
echo "📦 安装依赖包..."
go mod download

# 编译项目
echo "🔨 编译项目..."
go build -o cursorIM cmd/main.go

if [ $? -ne 0 ]; then
    echo "❌ 编译失败"
    exit 1
fi

# 启动服务
echo "🎉 启动服务..."
echo "服务地址: http://localhost:8082"
echo "前端页面: http://localhost:8082/web/index.html"
echo "按 Ctrl+C 停止服务"
echo ""

./cursorIM 