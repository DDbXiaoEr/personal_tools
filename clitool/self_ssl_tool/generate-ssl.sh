#!/bin/bash

# SSL证书自签名快捷生成脚本
# 用法: ./generate-ssl.sh [域名] [输出目录]

set -e

DOMAIN=${1:-localhost}
OUTPUT_DIR=${2:-./ssl}
DAYS=${3:-3650}

# 创建输出目录
mkdir -p "$OUTPUT_DIR"

# 生成CA私钥
openssl genrsa -out "$OUTPUT_DIR/ca.key" 2048

# 生成CA证书
openssl req -new -x509 -days "$DAYS" -key "$OUTPUT_DIR/ca.key" \
    -out "$OUTPUT_DIR/ca.crt" \
    -subj "/C=CN/ST=Beijing/L=Beijing/O=CA/OU=CA/CN=MyCA"

# 生成服务器私钥
openssl genrsa -out "$OUTPUT_DIR/server.key" 2048

# 创建临时配置文件
TEMP_CNF=$(mktemp)
cat > "$TEMP_CNF" << EOF
[req]
default_bits = 2048
prompt = no
default_md = sha256
req_extensions = req_ext
distinguished_name = dn

[dn]
C = CN
ST = Beijing
L = Beijing
O = Development
OU = Development
CN = $DOMAIN

[req_ext]
subjectAltName = @alt_names

[alt_names]
DNS.1 = $DOMAIN
DNS.2 = *.$DOMAIN
IP.1 = 127.0.0.1
IP.2 = ::1
EOF

# 生成证书签名请求
openssl req -new -key "$OUTPUT_DIR/server.key" \
    -out "$OUTPUT_DIR/server.csr" \
    -config "$TEMP_CNF"

# 使用CA签名生成服务器证书
openssl x509 -req -days "$DAYS" \
    -in "$OUTPUT_DIR/server.csr" \
    -CA "$OUTPUT_DIR/ca.crt" \
    -CAkey "$OUTPUT_DIR/ca.key" \
    -CAcreateserial \
    -out "$OUTPUT_DIR/server.crt" \
    -extfile "$TEMP_CNF" \
    -extensions req_ext

# 清理临时文件
rm -f "$TEMP_CNF" "$OUTPUT_DIR/server.csr" "$OUTPUT_DIR/ca.srl"

echo "SSL证书生成完成!"
echo "CA证书: $OUTPUT_DIR/ca.crt"
echo "服务器证书: $OUTPUT_DIR/server.crt"
echo "服务器私钥: $OUTPUT_DIR/server.key"
echo ""
echo "信任CA证书后即可使用服务器证书。"