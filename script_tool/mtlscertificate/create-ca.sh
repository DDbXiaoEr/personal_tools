#!/usr/bin/env bash
# 创建 mTLS 根 CA（私钥 + 自签 CA 证书），按名称建立目录存放
# 用法: ./create-ca.sh [CA名称] [输出目录] [有效天数] [CN]
# 例如: ./create-ca.sh ca1  ->  ./certs/ca/ca1/

set -euo pipefail

NAME=${1:-My mTLS Root CA}
OUTPUT_DIR=${2:-./certs}
DAYS=${3:-3650}
CN=${4:-$NAME}

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
source "$SCRIPT_DIR/ca-common.sh"

CA_NAME=$(sanitize_ca_name "$NAME")
if [[ -z "$CA_NAME" ]]; then
    echo "名称无效，无法生成目录名: $NAME" >&2
    exit 1
fi

CA_DIR="$OUTPUT_DIR/ca/$CA_NAME"
CA_KEY="$CA_DIR/ca.key"
CA_CRT="$CA_DIR/ca.crt"

if ! command -v openssl >/dev/null 2>&1; then
    echo "未找到 openssl" >&2
    exit 1
fi

if [[ -f "$CA_KEY" || -f "$CA_CRT" ]]; then
    echo "CA 已存在: $CA_DIR" >&2
    echo "如需重建请先删除该目录" >&2
    exit 1
fi

mkdir -p "$CA_DIR"

TEMP_CNF=$(mktemp)
trap 'rm -f "$TEMP_CNF"' EXIT

cat > "$TEMP_CNF" << EOF
[req]
default_bits = 4096
prompt = no
default_md = sha256
distinguished_name = dn
x509_extensions = v3_ca

[dn]
C = CN
ST = Beijing
L = Beijing
O = mTLS
OU = CA
CN = $CN

[v3_ca]
basicConstraints = critical, CA:TRUE, pathlen:1
keyUsage = critical, digitalSignature, keyCertSign, cRLSign
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid:always,issuer
EOF

openssl genrsa -out "$CA_KEY" 4096
chmod 600 "$CA_KEY"

openssl req -new -x509 -days "$DAYS" \
    -key "$CA_KEY" \
    -out "$CA_CRT" \
    -config "$TEMP_CNF"

echo "CA 创建完成（名称: ${CA_NAME}）"
echo "CA 目录: $CA_DIR"
echo "CA 私钥: $CA_KEY"
echo "CA 证书: $CA_CRT"
echo ""
openssl x509 -in "$CA_CRT" -noout -subject -dates
