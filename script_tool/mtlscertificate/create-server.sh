#!/usr/bin/env bash
# 签发 mTLS 服务端证书（有二级 CA 则用二级 CA，否则用根 CA）
# 用法: ./create-server.sh <CN> [输出目录] [有效天数] [SAN...]
# SAN 未指定时默认: CN、localhost、127.0.0.1、::1

set -euo pipefail

if [[ $# -lt 1 ]]; then
    echo "用法: $0 <CN> [输出目录] [有效天数] [SAN...]" >&2
    exit 1
fi

CN=$1
OUTPUT_DIR=${2:-./certs}
DAYS=${3:-3650}
shift $(( $# >= 3 ? 3 : $# ))

ROOT_DIR="$OUTPUT_DIR/ca"
ROOT_KEY="$ROOT_DIR/ca.key"
ROOT_CRT="$ROOT_DIR/ca.crt"
INT_DIR="$OUTPUT_DIR/intermediate"
INT_KEY="$INT_DIR/intermediate.key"
INT_CRT="$INT_DIR/intermediate.crt"
INT_SRL="$INT_DIR/intermediate.srl"
ROOT_SRL="$ROOT_DIR/ca.srl"

SAFE_CN=$(printf '%s' "$CN" | tr '/:*' '___' | tr -cd 'A-Za-z0-9._-')
if [[ -z "$SAFE_CN" ]]; then
    echo "CN 无效: $CN" >&2
    exit 1
fi

SERVER_DIR="$OUTPUT_DIR/server/$SAFE_CN"
SERVER_KEY="$SERVER_DIR/server.key"
SERVER_CRT="$SERVER_DIR/server.crt"
SERVER_CSR="$SERVER_DIR/server.csr"
FULLCHAIN="$SERVER_DIR/fullchain.crt"

if ! command -v openssl >/dev/null 2>&1; then
    echo "未找到 openssl" >&2
    exit 1
fi

if [[ ! -f "$ROOT_KEY" || ! -f "$ROOT_CRT" ]]; then
    echo "未找到根 CA，请先运行: ./create-ca.sh $OUTPUT_DIR" >&2
    exit 1
fi

if [[ -f "$INT_KEY" && -f "$INT_CRT" ]]; then
    SIGNER_KEY="$INT_KEY"
    SIGNER_CRT="$INT_CRT"
    SIGNER_SRL="$INT_SRL"
    SIGNER_NAME="二级 CA"
else
    SIGNER_KEY="$ROOT_KEY"
    SIGNER_CRT="$ROOT_CRT"
    SIGNER_SRL="$ROOT_SRL"
    SIGNER_NAME="根 CA"
fi

if [[ -f "$SERVER_KEY" || -f "$SERVER_CRT" ]]; then
    echo "服务端证书已存在: $SERVER_DIR" >&2
    echo "如需重建请先删除该目录" >&2
    exit 1
fi

mkdir -p "$SERVER_DIR"

is_ip() {
    local v=$1
    [[ "$v" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]] && return 0
    [[ "$v" == *:* ]] && return 0
    return 1
}

declare -a SANS=()
add_san() {
    local v=$1 i
    for i in "${SANS[@]+"${SANS[@]}"}"; do
        [[ "$i" == "$v" ]] && return 0
    done
    SANS+=("$v")
}

if [[ $# -gt 0 ]]; then
    add_san "$CN"
    for san in "$@"; do
        add_san "$san"
    done
else
    add_san "$CN"
    add_san "localhost"
    add_san "127.0.0.1"
    add_san "::1"
fi

DNS_I=0
IP_I=0
ALT_BLOCK=""
for san in "${SANS[@]}"; do
    if is_ip "$san"; then
        IP_I=$((IP_I + 1))
        ALT_BLOCK+=$'IP.'"$IP_I"' = '"$san"$'\n'
    else
        DNS_I=$((DNS_I + 1))
        ALT_BLOCK+=$'DNS.'"$DNS_I"' = '"$san"$'\n'
    fi
done

TEMP_CNF=$(mktemp)
trap 'rm -f "$TEMP_CNF" "$SERVER_CSR"' EXIT

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
OU = Server
CN = $CN

[v3_server]
basicConstraints = critical, CA:FALSE
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer
subjectAltName = @alt_names

[alt_names]
$ALT_BLOCK
EOF

openssl genrsa -out "$SERVER_KEY" 2048
chmod 600 "$SERVER_KEY"

openssl req -new -key "$SERVER_KEY" -out "$SERVER_CSR" -config "$TEMP_CNF"

openssl x509 -req -days "$DAYS" \
    -in "$SERVER_CSR" \
    -CA "$SIGNER_CRT" \
    -CAkey "$SIGNER_KEY" \
    -CAserial "$SIGNER_SRL" \
    -CAcreateserial \
    -out "$SERVER_CRT" \
    -extfile "$TEMP_CNF" \
    -extensions v3_server

if [[ -f "$INT_CRT" ]]; then
    cat "$SERVER_CRT" "$INT_CRT" "$ROOT_CRT" > "$FULLCHAIN"
else
    cat "$SERVER_CRT" "$ROOT_CRT" > "$FULLCHAIN"
fi

echo "服务端证书签发完成（签发者: ${SIGNER_NAME}）"
echo "私钥: $SERVER_KEY"
echo "证书: $SERVER_CRT"
echo "证书链: $FULLCHAIN"
echo ""
openssl x509 -in "$SERVER_CRT" -noout -subject -issuer -dates
if [[ -f "$INT_CRT" ]]; then
    openssl verify -CAfile "$ROOT_CRT" -untrusted "$INT_CRT" "$SERVER_CRT"
else
    openssl verify -CAfile "$ROOT_CRT" "$SERVER_CRT"
fi
