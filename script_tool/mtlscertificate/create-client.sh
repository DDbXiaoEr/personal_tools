#!/usr/bin/env bash
# 签发 mTLS 客户端证书并导出 PKCS12（有二级 CA 则用二级 CA，否则用根 CA）
# 用法: ./create-client.sh <CA名称> <CN> [输出目录] [有效天数] [PKCS12密码]

set -euo pipefail

if [[ $# -lt 2 ]]; then
    echo "用法: $0 <CA名称> <CN> [输出目录] [有效天数] [PKCS12密码]" >&2
    exit 1
fi

CA_NAME=$1
CN=$2
OUTPUT_DIR=${3:-./certs}
DAYS=${4:-3650}
P12_PASS=${5:-}

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
source "$SCRIPT_DIR/ca-common.sh"

if ! command -v openssl >/dev/null 2>&1; then
    echo "未找到 openssl" >&2
    exit 1
fi

ROOT_DIR=$(resolve_ca_dir "$OUTPUT_DIR/ca") || exit 1
ROOT_KEY="$ROOT_DIR/ca.key"
ROOT_CRT="$ROOT_DIR/ca.crt"
ROOT_SRL="$ROOT_DIR/ca.srl"
INT_DIR=$(resolve_intermediate_dir "$OUTPUT_DIR/intermediate" "$ROOT_CRT" "$(basename "$ROOT_DIR")") || INT_DIR=""

SAFE_CN=$(printf '%s' "$CN" | tr '/:*' '___' | tr -cd 'A-Za-z0-9._-')
if [[ -z "$SAFE_CN" ]]; then
    echo "CN 无效: $CN" >&2
    exit 1
fi

CLIENT_DIR="$OUTPUT_DIR/client/$SAFE_CN"
CLIENT_KEY="$CLIENT_DIR/client.key"
CLIENT_CRT="$CLIENT_DIR/client.crt"
CLIENT_CSR="$CLIENT_DIR/client.csr"
CLIENT_P12="$CLIENT_DIR/client.p12"

if [[ -n "$INT_DIR" ]]; then
    SIGNER_KEY="$INT_DIR/intermediate.key"
    SIGNER_CRT="$INT_DIR/intermediate.crt"
    SIGNER_SRL="$INT_DIR/intermediate.srl"
    SIGNER_NAME="二级 CA"
else
    SIGNER_KEY="$ROOT_KEY"
    SIGNER_CRT="$ROOT_CRT"
    SIGNER_SRL="$ROOT_SRL"
    SIGNER_NAME="根 CA"
fi

if [[ -f "$CLIENT_KEY" || -f "$CLIENT_CRT" ]]; then
    echo "客户端证书已存在: $CLIENT_DIR" >&2
    echo "如需重建请先删除该目录" >&2
    exit 1
fi

mkdir -p "$CLIENT_DIR"

TEMP_CNF=$(mktemp)
P12_CERTFILE=$(mktemp)
trap 'rm -f "$TEMP_CNF" "$CLIENT_CSR" "$P12_CERTFILE"' EXIT

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
    -CA "$SIGNER_CRT" \
    -CAkey "$SIGNER_KEY" \
    -CAserial "$SIGNER_SRL" \
    -CAcreateserial \
    -out "$CLIENT_CRT" \
    -extfile "$TEMP_CNF" \
    -extensions v3_client

if [[ "$SIGNER_NAME" == "二级 CA" ]]; then
    cat "$SIGNER_CRT" "$ROOT_CRT" > "$P12_CERTFILE"
else
    cat "$ROOT_CRT" > "$P12_CERTFILE"
fi

openssl pkcs12 -export \
    -inkey "$CLIENT_KEY" \
    -in "$CLIENT_CRT" \
    -certfile "$P12_CERTFILE" \
    -out "$CLIENT_P12" \
    -name "$CN" \
    -passout "pass:$P12_PASS"
chmod 600 "$CLIENT_P12"

echo "客户端证书签发完成（签发者: ${SIGNER_NAME}）"
echo "私钥: $CLIENT_KEY"
echo "证书: $CLIENT_CRT"
echo "PKCS12: $CLIENT_P12"
echo ""
openssl x509 -in "$CLIENT_CRT" -noout -subject -issuer -dates
if [[ "$SIGNER_NAME" == "二级 CA" ]]; then
    openssl verify -CAfile "$ROOT_CRT" -untrusted "$SIGNER_CRT" "$CLIENT_CRT"
else
    openssl verify -CAfile "$ROOT_CRT" "$CLIENT_CRT"
fi
