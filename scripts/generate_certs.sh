#!/bin/bash

# CursorIM TLS证书生成脚本
# 用于开发和测试环境

set -e

echo "🔐 CursorIM TLS证书生成工具"
echo "=================================="

# 检查openssl是否安装
if ! command -v openssl &> /dev/null; then
    echo "❌ OpenSSL未安装，请先安装OpenSSL"
    exit 1
fi

# 创建证书目录
CERT_DIR="./certs"
mkdir -p $CERT_DIR

echo "📁 证书将保存到: $CERT_DIR"

# 证书配置
DOMAIN="localhost"
COUNTRY="CN"
STATE="Beijing"
CITY="Beijing"
ORG="CursorIM"
OU="Development"
EMAIL="admin@cursorim.dev"

echo "🏗️ 生成私钥..."
openssl genrsa -out $CERT_DIR/server.key 2048

echo "🏗️ 生成证书签名请求..."
openssl req -new -key $CERT_DIR/server.key -out $CERT_DIR/server.csr -subj "/C=$COUNTRY/ST=$STATE/L=$CITY/O=$ORG/OU=$OU/CN=$DOMAIN/emailAddress=$EMAIL"

echo "🏗️ 生成自签名证书..."
openssl x509 -req -days 365 -in $CERT_DIR/server.csr -signkey $CERT_DIR/server.key -out $CERT_DIR/server.crt

# 创建配置文件，支持SAN扩展
cat > $CERT_DIR/server.conf << EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
C = $COUNTRY
ST = $STATE
L = $CITY
O = $ORG
OU = $OU
CN = $DOMAIN
emailAddress = $EMAIL

[v3_req]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = *.localhost
DNS.3 = 127.0.0.1
IP.1 = 127.0.0.1
IP.2 = ::1
EOF

echo "🏗️ 重新生成支持多域名的证书..."
openssl req -new -key $CERT_DIR/server.key -out $CERT_DIR/server.csr -config $CERT_DIR/server.conf
openssl x509 -req -days 365 -in $CERT_DIR/server.csr -signkey $CERT_DIR/server.key -out $CERT_DIR/server.crt -extensions v3_req -extfile $CERT_DIR/server.conf

# 清理临时文件
rm $CERT_DIR/server.csr

echo "✅ 证书生成完成!"
echo ""
echo "📄 证书文件:"
echo "  🔑 私钥: $CERT_DIR/server.key"
echo "  🏆 证书: $CERT_DIR/server.crt"
echo ""

# 验证证书
echo "🔍 证书信息验证:"
openssl x509 -in $CERT_DIR/server.crt -text -noout | grep -A 5 "Subject:"
echo ""

echo "📝 使用方法:"
echo "  1. 开发环境: 在浏览器中访问 https://localhost:8082"
echo "  2. 忽略证书警告（自签名证书）"
echo "  3. 生产环境请使用CA签发的证书"
echo ""

echo "⚙️ 服务器配置:"
echo '  tls := server.NewTLSConfig("./certs/server.crt", "./certs/server.key", true)'
echo '  tls.StartHTTPSServer(router, ":8082")'
echo ""

echo "⚠️  注意:"
echo "  - 此证书仅用于开发测试"
echo "  - 生产环境请使用Let's Encrypt或其他CA证书"
echo "  - 定期更新证书（当前有效期365天）" 