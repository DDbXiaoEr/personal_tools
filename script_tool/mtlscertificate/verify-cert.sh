#!/usr/bin/env bash
# 校验 mTLS 证书：证书链、私钥匹配、有效期、SAN
# 用法: ./verify-cert.sh [证书文件] [输出目录]
#       无参数时校验输出目录下全部服务端 / 客户端证书
#       输出目录默认为 ./certs

set -euo pipefail

OUTPUT_DIR=./certs
TARGET=""

if [[ $# -ge 1 ]]; then
    if [[ -d "$1" ]]; then
        OUTPUT_DIR=$1
    elif [[ -f "$1" ]]; then
        TARGET=$1
        OUTPUT_DIR=${2:-./certs}
    else
        echo "文件或目录不存在: $1" >&2
        exit 1
    fi
fi

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
source "$SCRIPT_DIR/ca-common.sh"

ROOT_DIR=$(resolve_ca_dir "$OUTPUT_DIR/ca") || {
    echo "请确认输出目录正确" >&2
    exit 1
}
ROOT_CRT="$ROOT_DIR/ca.crt"

if ! command -v openssl >/dev/null 2>&1; then
    echo "未找到 openssl" >&2
    exit 1
fi

INT_DIR=$(resolve_intermediate_dir "$OUTPUT_DIR/intermediate" "$ROOT_CRT" "$(basename "$ROOT_DIR")") || INT_DIR=""
HAS_INT=0
INT_CRT=""
if [[ -n "$INT_DIR" ]]; then
    INT_CRT="$INT_DIR/intermediate.crt"
    HAS_INT=1
fi

PASS=0
FAIL=0

cert_chains_to() {
    local crt=$1 root=$2 int=$3
    if [[ -n "$int" ]]; then
        openssl verify -no_check_time -CAfile "$root" -untrusted "$int" "$crt" >/dev/null 2>&1
    else
        openssl verify -no_check_time -CAfile "$root" "$crt" >/dev/null 2>&1
    fi
}

# 判断证书是否属于其他 CA，用于指定 CA_NAME 校验时跳过其他 CA 的证书
belongs_to_other_ca() {
    local crt=$1 root int dir
    local -a roots=()

    for dir in "$OUTPUT_DIR"/ca/*/; do
        [[ -d "$dir" ]] || continue
        dir=${dir%/}
        [[ -f "$dir/ca.crt" ]] && roots+=("$dir/ca.crt")
    done
    [[ -f "$OUTPUT_DIR/ca/ca.crt" ]] && roots+=("$OUTPUT_DIR/ca/ca.crt")

    for root in "${roots[@]+"${roots[@]}"}"; do
        [[ "$root" == "$ROOT_CRT" ]] && continue
        int=$(resolve_intermediate_dir "$OUTPUT_DIR/intermediate" "$root" "$(basename "$(dirname "$root")")") || int=""
        [[ -n "$int" ]] && int="$int/intermediate.crt"
        if cert_chains_to "$crt" "$root" "$int"; then
            return 0
        fi
    done
    return 1
}

key_pubkey() {
    openssl pkey -in "$1" -pubout -outform der 2>/dev/null \
        | openssl dgst -sha256 2>/dev/null
}

cert_fingerprint() {
    openssl x509 -in "$1" -noout -pubkey 2>/dev/null \
        | openssl pkey -pubin -outform der 2>/dev/null \
        | openssl dgst -sha256 2>/dev/null
}

verify_one() {
    local crt=$1
    local failed=0
    local subject issuer

    subject=$(openssl x509 -in "$crt" -noout -subject 2>/dev/null | sed 's/^subject=//')
    issuer=$(openssl x509 -in "$crt" -noout -issuer 2>/dev/null | sed 's/^issuer=//')

    printf '证书: %s\n' "$crt"
    printf '  主题: %s\n' "$subject"
    printf '  签发者: %s\n' "$issuer"

    local verify_out
    if [[ $HAS_INT -eq 1 ]]; then
        if verify_out=$(openssl verify -CAfile "$ROOT_CRT" -untrusted "$INT_CRT" "$crt" 2>&1); then
            printf '  [通过] 证书链\n'
        else
            printf '  [失败] 证书链: %s\n' "$verify_out"
            failed=1
        fi
    else
        if verify_out=$(openssl verify -CAfile "$ROOT_CRT" "$crt" 2>&1); then
            printf '  [通过] 证书链\n'
        else
            printf '  [失败] 证书链: %s\n' "$verify_out"
            failed=1
        fi
    fi

    local nb na
    nb=$(openssl x509 -in "$crt" -noout -startdate 2>/dev/null | cut -d= -f2-)
    na=$(openssl x509 -in "$crt" -noout -enddate 2>/dev/null | cut -d= -f2-)
    if ! openssl x509 -in "$crt" -noout -checkend 0 >/dev/null 2>&1; then
        printf '  [失败] 已过期（%s 至 %s）\n' "$nb" "$na"
        failed=1
    elif ! openssl x509 -in "$crt" -noout -checkend 2592000 >/dev/null 2>&1; then
        printf '  [警告] 30 天内过期（至 %s）\n' "$na"
    else
        printf '  [通过] 有效期（%s 至 %s）\n' "$nb" "$na"
    fi

    local san
    san=$(openssl x509 -in "$crt" -noout -ext subjectAltName 2>/dev/null | tail -n +2 | tr -d '\n' | sed 's/^ *//')
    if [[ -n "$san" ]]; then
        printf '  SAN: %s\n' "$san"
    fi

    local eku
    eku=$(openssl x509 -in "$crt" -noout -ext extendedKeyUsage 2>/dev/null | tail -n +2 | tr -d '\n' | sed 's/^ *//')
    if [[ -n "$eku" ]]; then
        printf '  用途: %s\n' "$eku"
    fi

    local cert_fp key matched=0 dir
    cert_fp=$(cert_fingerprint "$crt")
    dir=$(dirname "$crt")
    for key in "$dir"/*.key; do
        [[ -f "$key" ]] || continue
        if [[ -n "$cert_fp" && "$cert_fp" == "$(key_pubkey "$key")" ]]; then
            printf '  [通过] 私钥匹配: %s\n' "$key"
            matched=1
            break
        fi
    done
    if [[ $matched -eq 0 ]]; then
        printf '  [提示] 同目录未找到匹配私钥\n'
    fi

    echo ""
    if [[ $failed -eq 0 ]]; then
        PASS=$((PASS + 1))
    else
        FAIL=$((FAIL + 1))
    fi
}

if [[ -n "$TARGET" ]]; then
    verify_one "$TARGET"
else
    shopt -s nullglob
    found=0
    for crt in "$OUTPUT_DIR"/server/*/*.crt; do
        case "$crt" in
            *fullchain.crt) continue ;;
        esac
        if ! cert_chains_to "$crt" "$ROOT_CRT" "$INT_CRT" && belongs_to_other_ca "$crt"; then
            continue
        fi
        verify_one "$crt"
        found=1
    done
    for crt in "$OUTPUT_DIR"/client/*/*.crt; do
        if ! cert_chains_to "$crt" "$ROOT_CRT" "$INT_CRT" && belongs_to_other_ca "$crt"; then
            continue
        fi
        verify_one "$crt"
        found=1
    done
    if [[ $found -eq 0 ]]; then
        echo "未找到待校验证书（$OUTPUT_DIR/server 或 $OUTPUT_DIR/client）" >&2
        exit 1
    fi
fi

printf '通过: %d  失败: %d\n' "$PASS" "$FAIL"
[[ $FAIL -eq 0 ]]
