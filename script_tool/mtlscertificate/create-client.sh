#!/usr/bin/env bash
# 用根 CA 签发 mTLS 客户端证书
# 用法: ./create-client.sh <CN> [输出目录] [有效天数]

set -euo pipefail

if [[ $# -lt 1 ]]; then
    echo "用法: $0 <CN> [输出目录] [有效天数]" >&2
    exit 1
fi

CN=$1
OUTPUT_DIR=${2:-./certs}
DAYS=${3:-3650}

CA_DIR="$OUTPUT_DIR/ca"
CA_KEY="$CA_DIR/ca.key"
CA_CRT="$CA_DIR/ca.crt"
CA_SRL="$CA_DIR/ca.srl"

SAFE_CN=$(printf '%s' "$CN" | tr '/:*' '___' | tr -cd 'A-Za-z0-9._-')
if [[ -z "$SAFE_CN" ]]; then
    echo "CN 无效: $CN" >&2
    exit 1
fi

CLIENT_DIR="$OUTPUT_DIR/client/$SAFE_CN"
CLIENT_KEY="$CLIENT_DIR/client.key"
CLIENT_CRT="$CLIENT_DIR/client.crt"
CLIENT_CSR="$CLIENT_DIR/client.csr"

if ! command -v openssl >/dev/null 2>&1; then
    echo "未找到 openssl" >&2
    exit 1
fi

if [[ ! -f "$CA_KEY" || ! -f "$CA_CRT" ]]; then
    echo "未找到 CA，请先运行: ./create-ca.sh $OUTPUT_DIR" >&2
    exit 1
fi

if [[ -f "$CLIENT_KEY" || -f "$CLIENT_CRT" ]]; then
    echo "客户端证书已存在: $CLIENT_DIR" >&2
    echo "如需重建请先删除该目录" >&2
    exit 1
fi

mkdir -p "$CLIENT_DIR"

TEMP_CNF=$(mktemp)
trap 'rm -f "$TEMP_CNF" "$CLIENT_CSR"' EXIT

cat > "$TEMP_CNF" << EOF
[req]
default_bits = 2048
prompt = no
default_md = sha256
distinguished_name = dn

[dn]
C = CN
ST = Beijing
L = Beijing
O = mTLS
OU = Client
CN = $CN

[v3_client]
basicConstraints = critical, CA:FALSE
keyUsage = critical, digitalSignature
extendedKeyUsage = clientAuth
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer
subjectAltName = DNS:$CN
EOF

openssl genrsa -out "$CLIENT_KEY" 2048
chmod 600 "$CLIENT_KEY"

openssl req -new -key "$CLIENT_KEY" -out "$CLIENT_CSR" -config "$TEMP_CNF"

openssl x509 -req -days "$DAYS" \
    -in "$CLIENT_CSR" \
    -CA "$CA_CRT" \
    -CAkey "$CA_KEY" \
    -CAserial "$CA_SRL" \
    -CAcreateserial \
    -out "$CLIENT_CRT" \
    -extfile "$TEMP_CNF" \
    -extensions v3_client

echo "客户端证书签发完成"
echo "私钥: $CLIENT_KEY"
echo "证书: $CLIENT_CRT"
echo ""
openssl x509 -in "$CLIENT_CRT" -noout -subject -dates
openssl verify -CAfile "$CA_CRT" "$CLIENT_CRT"
