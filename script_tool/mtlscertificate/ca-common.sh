#!/usr/bin/env bash
# 公共函数：CA 名称处理与根 CA 目录解析

# 将 CA 名称转成安全的目录名
sanitize_ca_name() {
    printf '%s' "$1" | tr '/:*' '___' | tr -s '[:space:]' '_' | tr -cd 'A-Za-z0-9._-'
}

# 解析根 CA 目录，用法: resolve_ca_dir <ca 根目录>
# 优先环境变量 CA_NAME，其次兼容旧版直接存放的 <ca>/ca.key，
# 最后自动选择唯一的子目录 <ca>/<名称>/ca.key
resolve_ca_dir() {
    local base=$1
    local name=${CA_NAME:-}
    local safe d
    local -a found=()

    if [[ -n "$name" ]]; then
        safe=$(sanitize_ca_name "$name")
        if [[ -n "$safe" && -f "$base/$safe/ca.key" && -f "$base/$safe/ca.crt" ]]; then
            printf '%s\n' "$base/$safe"
            return 0
        fi
        if [[ -f "$base/$name/ca.key" && -f "$base/$name/ca.crt" ]]; then
            printf '%s\n' "$base/$name"
            return 0
        fi
        # 兼容旧版根 CA：目录名就是 ca 根目录名（如 ca）
        if [[ "$name" == "$(basename "$base")" && -f "$base/ca.key" && -f "$base/ca.crt" ]]; then
            printf '%s\n' "$base"
            return 0
        fi
        echo "未找到 CA: ${name}（${base} 下）" >&2
        return 1
    fi

    if [[ -f "$base/ca.key" && -f "$base/ca.crt" ]]; then
        printf '%s\n' "$base"
        return 0
    fi

    for d in "$base"/*/; do
        [[ -d "$d" ]] || continue
        d=${d%/}
        if [[ -f "$d/ca.key" && -f "$d/ca.crt" ]]; then
            found+=("$d")
        fi
    done

    if [[ ${#found[@]} -eq 0 ]]; then
        echo "未找到根 CA（${base}），请先运行: ./create-ca.sh" >&2
        return 1
    fi
    if [[ ${#found[@]} -gt 1 ]]; then
        echo "存在多个 CA，请用 CA_NAME 指定：" >&2
        printf '  %s\n' "${found[@]##*/}" >&2
        return 1
    fi
    printf '%s\n' "${found[0]}"
}

# 解析二级 CA 目录，用法: resolve_intermediate_dir <intermediate 根目录> <根证书> <CA目录名>
# 优先 <base>/<CA目录名>，其次兼容旧版 <base>，最后检查其他子目录
resolve_intermediate_dir() {
    local base=$1
    local root_crt=$2
    local ca_name=$3
    local d
    local -a candidates=()

    [[ -n "$ca_name" ]] && candidates+=("$base/$ca_name")
    candidates+=("$base")

    for d in "$base"/*/; do
        [[ -d "$d" ]] || continue
        candidates+=("${d%/}")
    done

    for d in "${candidates[@]+"${candidates[@]}"}"; do
        if [[ -f "$d/intermediate.key" && -f "$d/intermediate.crt" ]] && \
           openssl verify -CAfile "$root_crt" "$d/intermediate.crt" >/dev/null 2>&1; then
            printf '%s\n' "$d"
            return 0
        fi
    done
    return 1
}
