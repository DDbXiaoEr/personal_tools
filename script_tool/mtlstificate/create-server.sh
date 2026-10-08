#!/usr/bin/env bash
# 用根 CA 签发 mTLS 服务端证书
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

CA_DIR="$OUTPUT_DIR/ca"
CA_KEY="$CA_DIR/ca.key"
CA_CRT="$CA_DIR/ca.crt"
CA_SRL="$CA_DIR/ca.srl"

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

if [[ ! -f "$CA_KEY" || ! -f "$CA_CRT" ]]; then
    echo "未找到 CA，请先运行: ./create-ca.sh $OUTPUT_DIR" >&2
    exit 1
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
    -CA "$CA_CRT" \
    -CAkey "$CA_KEY" \
    -CAserial "$CA_SRL" \
    -CAcreateserial \
    -out "$SERVER_CRT" \
    -extfile "$TEMP_CNF" \
    -extensions v3_server

cat "$SERVER_CRT" "$CA_CRT" > "$FULLCHAIN"

echo "服务端证书签发完成"
echo "私钥: $SERVER_KEY"
echo "证书: $SERVER_CRT"
echo "证书链: $FULLCHAIN"
echo ""
openssl x509 -in "$SERVER_CRT" -noout -subject -dates
openssl verify -CAfile "$CA_CRT" "$SERVER_CRT"
