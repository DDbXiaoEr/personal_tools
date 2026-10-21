#!/usr/bin/env bash
# 用根 CA 签发二级（中间）CA
# 用法: ./create-intermediate-ca.sh [输出目录] [有效天数] [CN]

set -euo pipefail

OUTPUT_DIR=${1:-./certs}
DAYS=${2:-3650}
CN=${3:-My mTLS Intermediate CA}

ROOT_DIR="$OUTPUT_DIR/ca"
ROOT_KEY="$ROOT_DIR/ca.key"
ROOT_CRT="$ROOT_DIR/ca.crt"
ROOT_SRL="$ROOT_DIR/ca.srl"

INT_DIR="$OUTPUT_DIR/intermediate"
INT_KEY="$INT_DIR/intermediate.key"
INT_CRT="$INT_DIR/intermediate.crt"
INT_CSR="$INT_DIR/intermediate.csr"
INT_CHAIN="$INT_DIR/chain.crt"

if ! command -v openssl >/dev/null 2>&1; then
    echo "未找到 openssl" >&2
    exit 1
fi

if [[ ! -f "$ROOT_KEY" || ! -f "$ROOT_CRT" ]]; then
    echo "未找到根 CA，请先运行: ./create-ca.sh $OUTPUT_DIR" >&2
    exit 1
fi

if [[ -f "$INT_KEY" || -f "$INT_CRT" ]]; then
    echo "二级 CA 已存在: $INT_DIR" >&2
    echo "如需重建请先删除该目录" >&2
    exit 1
fi

mkdir -p "$INT_DIR"

TEMP_CNF=$(mktemp)
trap 'rm -f "$TEMP_CNF" "$INT_CSR"' EXIT

cat > "$TEMP_CNF" << EOF
[req]
default_bits = 4096
prompt = no
default_md = sha256
distinguished_name = dn

[dn]
C = CN
ST = Beijing
L = Beijing
O = mTLS
OU = Intermediate CA
CN = $CN

[v3_intermediate]
basicConstraints = critical, CA:TRUE, pathlen:0
keyUsage = critical, digitalSignature, keyCertSign, cRLSign
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid:always,issuer
EOF

openssl genrsa -out "$INT_KEY" 4096
chmod 600 "$INT_KEY"

openssl req -new -key "$INT_KEY" -out "$INT_CSR" -config "$TEMP_CNF"

openssl x509 -req -days "$DAYS" \
    -in "$INT_CSR" \
    -CA "$ROOT_CRT" \
    -CAkey "$ROOT_KEY" \
    -CAserial "$ROOT_SRL" \
    -CAcreateserial \
    -out "$INT_CRT" \
    -extfile "$TEMP_CNF" \
    -extensions v3_intermediate

cat "$INT_CRT" "$ROOT_CRT" > "$INT_CHAIN"

echo "二级 CA 创建完成"
echo "私钥: $INT_KEY"
echo "证书: $INT_CRT"
echo "证书链: $INT_CHAIN"
echo ""
openssl x509 -in "$INT_CRT" -noout -subject -issuer -dates
openssl verify -CAfile "$ROOT_CRT" "$INT_CRT"
