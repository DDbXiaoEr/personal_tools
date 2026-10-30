# my-tools

[English](README_EN.md)

个人自用工具集，收集日常开发和运维中常用的 CLI 工具和 MCP 工具。主要用于提升工作效率，解决重复性操作问题。

## 项目结构

```
my-tools/
├── clitool/              # CLI 工具
│   ├── markdown2pdf/     # Markdown 转 PDF
│   ├── ssh_config_manage/# SSH 配置管理
│   ├── viewcsv_xlsx/     # CSV / XLSX 查看器
│   ├── opencode_config/  # opencode 配置管理 TUI
│   ├── shell_rc_manage/  # shell 别名 / 环境变量 / 函数 TUI
│   ├── ansible_inventory/# Ansible 清单管理 TUI
│   └── mtls_proxy/       # mTLS 代理 / 证书验证
└── mcptool/              # MCP 工具
    └── cmd/sshtool/      # SSH MCP Server
```

## 工具说明

### markdown2pdf

将 Markdown 文件转换为 PDF，基于 Chrome Headless 渲染。适合快速生成文档、笔记归档等场景。

```bash
# 使用
markdown2pdf -i input.md -o output.pdf

# 或直接
markdown2pdf input.md
```

### ssh_config_manage

SSH 配置文件管理 TUI 工具，基于 Bubble Tea 构建。方便查看、编辑和管理多台服务器的 SSH 连接配置。支持复制已有主机、一次添加多台。

```bash
# 使用
sshman -file ~/.ssh/config
```

**快捷键：**

| 按键 | 说明 |
|------|------|
| `a` | 新增一台 |
| `A` | 批量新增（公共 User/Port/密钥 + 每行一台） |
| `c` | 复制当前主机，别名自动加后缀 |
| `enter` / `e` | 编辑 |
| `d` | 删除（需确认） |
| `s` | 保存到文件 |
| `q` | 退出 |

批量行格式：`别名 [主机 [用户 [端口]]]`。只填一项时别名与 HostName 相同。公共选项应用到每一台，行内 User/Port 可覆盖。

表单中 `tab` 切字段，`ctrl+s` 应用到内存，`esc` 取消。保存时才写盘。

### viewcsv_xlsx

CSV / XLSX 文件查看工具，以表格形式展示内容，支持自定义分隔符、中文对齐，以及多 Sheet 的 Excel 文件。打开多个文件时进入全屏，用 `tab` / `shift+tab` 切换；单个文件或管道输出仍打印表格。

```bash
# 使用（按扩展名自动识别 CSV / XLSX）
viewcsv_xlsx input.csv
viewcsv_xlsx input.xlsx

# 读取当前目录所有 CSV
viewcsv_xlsx *.csv

# 指定分隔符
viewcsv_xlsx -d "	" input.tsv

# 指定 Sheet（名称或 1 起始的序号，默认展示全部）
viewcsv_xlsx --sheet 2 input.xlsx
viewcsv_xlsx --sheet 用户 input.xlsx

# 无表头模式
viewcsv_xlsx --no-header input.csv
```

**快捷键（多个文件全屏）：**

| 按键 | 说明 |
|------|------|
| `tab` / `shift+tab` | 切换文件 |
| `[` / `]` | 切换工作表 |
| `↑` `↓` `←` `→` / `j` `k` | 滚动 |
| `q` | 退出 |

### shellman

Shell 别名、环境变量、自定义函数管理 TUI，基于 Bubble Tea 构建。读写家目录下的 `.{shell}_{alias,env,functions}`，按分类注释分组保存。不修改 `~/.zshrc` / `~/.bashrc`，请自行 source。

**文件约定：**

| 文件 | 内容 |
|------|------|
| `~/.zsh_alias` / `~/.bash_alias` | `alias name='cmd'` |
| `~/.zsh_env` / `~/.bash_env` | `export` / 赋值 / snippet / path |
| `~/.zsh_functions` / `~/.bash_functions` | `name() { ... }` |

保存时同类条目写在同一分类下：

```sh
# ===== Docker =====
alias dim='docker images'
alias dps='docker ps'
```

Env 支持四种形态：`export KEY=value`、`KEY=value`、nvm/bun 这类多行 snippet，以及按分类追加的 PATH。PATH 条目写成 `# path:`，保存时在文件末尾统一拼接：

```sh
# ===== Go =====
# path: $HOME/go/bin

# ===== local =====
# path: $HOME/.local/bin

export PATH="$HOME/go/bin:$HOME/.local/bin:$PATH"
```

已有的 `export PATH=$PATH:...` 读入时会拆成分类下的 path 条目；完整覆盖的 `export PATH=/usr/bin:...` 仍按普通 export 保留。

export / assign 可写多个备选值，列表里用 `tab` / `shift+tab` / 空格循环切换当前值（无备选时 `tab` 仍切标签页）。备选值以 `# alt:` 写在变量下一行：

```sh
export HTTP_PROXY=http://127.0.0.1:7890
# alt: http://127.0.0.1:1087
# alt: socks5://127.0.0.1:1080
```

```bash
# 使用（默认当前 $SHELL，目录 $HOME）
shellman

# 指定 shell 与目录
shellman -shell bash
shellman -dir /custom
```

**快捷键：**

| 按键 | 说明 |
|------|------|
| `1` / `2` / `3` | 切换 Alias / Env / Functions |
| `↑` `↓` / `j` `k` | 移动光标 |
| `tab` / `shift+tab` | Env 有备选值时切换当前值；否则切换标签页 |
| `空格` | Env 有备选值时切换当前值 |
| `a` | 新增 |
| `enter` / `e` | 编辑 |
| `d` | 删除（需确认） |
| `s` | 保存当前标签对应文件 |
| `r` | 从文件重载 |
| `o` | 切换 zsh / bash |
| `?` | 帮助 |
| `q` | 退出 |

表单中 `tab` 切字段，`ctrl+s` 应用，`esc` 取消。函数体和 snippet 用 textarea 编辑。

> 保存时会生成 `<文件>.bak` 备份。现有 `~/.zsh_aliases` 不会被读取，需要的话在 rc 里改成 `source ~/.zsh_alias` 等。

### occonfig

opencode 配置管理 TUI 工具，基于 Bubble Tea 构建。用于管理全局或项目的 `opencode.json`，写回时保留注释、键顺序和未编辑字段。

**功能：**
- **Provider**：增删改自定义 provider（`id` / `name` / `npm` / `options.baseURL` / `options.apiKey`）。配好 `baseURL` 和 `apiKey` 后按 `f`（或表单「从 API 拉取模型」）请求 OpenAI 兼容的 `/models` 并写入 `provider.models`；已有模型只补全空名称，variants 原样保留。新增且尚未配置模型时，`ctrl+s` 会自动拉取。`apiKey` 支持 `{env:VAR}`。按 `v` 管理 `models.variants`（`disabled` 与每行 `KEY=value` 的 options）；未在表单中暴露的键（如 `blacklist`）原样保留。
- **MCP**：管理 `local` / `remote` 两类 server。local 的 `command` 每行一个参数（例如 `sshtool`），可浏览可执行文件；另有 `environment`、`cwd`。remote 覆盖 `url`、`headers`、`oauth`。共用 `timeout`、`enabled`，列表中按空格快速启停。
- **Skills**：浏览全局 `~/.config/opencode/skills/*/SKILL.md`，校验 frontmatter（`name` / `description`）；按 `a` 从 zip URI 下载并解压到指定目录名，按 `d` 删除技能目录。
- **配置来源**：默认全局配置，按 `o` 可切换到从当前目录向上查找到的项目 `opencode.json`。

```bash
# 使用（默认全局 ~/.config/opencode/opencode.json）
occonfig

# 指定配置文件与技能目录
occonfig -config /path/to/opencode.json -skills-dir ~/.config/opencode/skills
```

**快捷键：**

| 按键 | 说明 |
|------|------|
| `1` / `2` / `3` | 切换 Provider / MCP / Skills 标签页 |
| `↑` `↓` / `j` `k` | 移动光标 |
| `a` | 新增（Skills 页为从 zip URI 安装） |
| `enter` / `e` | 编辑（Skills 页为查看全文） |
| `f` | 从当前 Provider 的 API 拉取模型 |
| `v` | 管理当前 Provider 的 variants |
| `d` | 删除（需确认） |
| `space` | 启用 / 禁用 MCP；variants 列表中禁用 / 启用 variant |
| `s` | 保存到文件 |
| `r` | 从文件重载 |
| `o` | 切换全局 / 项目配置来源 |
| `?` | 帮助 |
| `q` | 退出 |

**MCP local 配置命令：** 在表单中 Tab 到「浏览可执行文件」按钮按 `Enter`（或按 `ctrl+f`）打开文件选择器，选中后自动填入可执行文件路径，已有参数行会保留。

> 保存时会生成 `<配置文件>.bak` 备份，并将文件格式化为制表符缩进的 JSONC（opencode 兼容）。

### ansiman

Ansible 主机清单管理 TUI，基于 Bubble Tea 构建。管理默认清单和项目级 inventory（INI / YAML），覆盖主机、组、组变量。

**配置目录 `~/.config/ansiman/`：**

| 文件 | 内容 |
|------|------|
| `config` | `default_inventory=/path/to/hosts` |
| `projects` | 每行 `名称=路径`，与 config 分开 |

未配置时，默认路径回退 `ANSIBLE_INVENTORY` / ansible.cfg / `/etc/ansible/hosts`。按 `o` 打开来源页：左侧改默认路径，右侧管理项目列表。

```bash
# 使用（默认清单）
ansiman

# 指定清单文件
ansiman -file ./inventory.yml
```

**快捷键：**

| 按键 | 说明 |
|------|------|
| `1` / `2` / `3` | 切换 主机 / 组 / 变量 |
| `↑` `↓` / `j` `k` | 移动光标 |
| `a` | 新增 |
| `enter` / `e` | 编辑 |
| `d` | 删除（需确认） |
| `s` | 保存到文件 |
| `r` | 从文件重载 |
| `o` | 来源：默认路径 / 项目列表 |
| `?` | 帮助 |
| `q` | 退出 |

表单中 `tab` 切字段，`ctrl+s` 应用，`esc` 取消。主机额外变量与子组用 textarea，每行一条。

> 保存时会生成 `<清单文件>.bak` 备份。写回保持原格式（INI 或 YAML）。删除组时，组内主机移到 `ungrouped`。改名请删除后新增。

### mtlsproxy

mTLS 证书验证 CLI。一次性探测目标，或监听明文端口当反向代理：浏览器不用装 p12，工具代持客户端证书去连 mTLS 服务。

```bash
# 网页控制台：在浏览器里配置目标、监听端口和客户端证书
mtlsproxy -ui 127.0.0.1:9000

# 直接探测
mtlsproxy -target https://api.example.com:8443 -cert client.crt -key client.key -ca ca.crt

# HTTP 反向代理：浏览器打开 http://127.0.0.1:8000 即访问目标
mtlsproxy -listen 127.0.0.1:8000 -target https://api.example.com:8443 -cert client.crt -key client.key -ca ca.crt

# TCP 隧道（gRPC / 裸 TCP）：客户端连明文端口
mtlsproxy -tcp -listen 127.0.0.1:8000 -target host:50051 -alpn h2 -cert client.crt -key client.key -ca ca.crt

# 只检查本地证书（过期、私钥是否匹配、可选 CA 链）
mtlsproxy -inspect -cert client.crt -key client.key -ca ca.crt
```

**flag：**

| 参数 | 说明 |
|------|------|
| `-ui` | 启动网页控制台，如 `127.0.0.1:9000`；页面上配置证书、目标与本地端口并启停代理 |
| `-target` | 目标地址，如 `https://host:8443` |
| `-cert` / `-key` | 客户端证书与私钥；`-key` 省略则与 `-cert` 同一文件 |
| `-ca` | 校验服务端的 CA |
| `-listen` | 监听地址；只写端口时绑定 `127.0.0.1` |
| `-tcp` | TCP 字节流隧道，适用于 gRPC / 裸 TCP |
| `-alpn` | TLS ALPN，逗号分隔；gRPC 用 `h2` |
| `-inspect` | 不连服务端，只检查本地证书 |
| `-method` | 握手后再发 HTTP，如 `GET` |
| `-path` | HTTP 路径，默认 `/` |
| `-sni` | 覆盖 TLS ServerName |
| `-insecure` | 跳过服务端证书校验 |
| `-timeout` | 超时，默认 10s |
| `-json` | JSON 输出 |

`-listen` 默认 HTTP 反向代理（页面和 API）。`/__mtls/check` 只探测握手（失败 502），`/__mtls/health` 返回 `ok`。`-tcp` 为明文 TCP 隧道，客户端不用证书；gRPC 请加 `-alpn h2`，客户端连 `127.0.0.1:8000` 走 insecure/h2c。失败退出码 1。

`-ui` 模式内嵌独立 HTML，不依赖外部静态资源；证书既可以直接上传 PEM 文件（只进内存），也可以填写服务器上的路径。控制台建议只监听 `127.0.0.1`。

### sshtool (MCP Server)

SSH 远程命令执行 / 文件传输 MCP Server。调用时传入 SSH config 里的 Host 别名（`hostname`），自动读取 `HostName` / `User` / `Port` / `IdentityFile`。内置高危命令拦截。命令走 SSH session，上传下载走 SFTP。可通过 MCP 协议集成到 AI 助手或自动化工具中。

默认读取 `~/.ssh/config`，也可用 `-config` 指定其它文件。支持 `Include`。

```bash
sshtool
# 或
sshtool -config ~/.ssh/config
```

`~/.ssh/config` 示例：

```
Host web
    HostName 10.0.0.1
    User deploy
    Port 2222
    IdentityFile ~/.ssh/id_ed25519
```

之后工具只需传 `hostname=web`。

**可用工具：**
- `runsshcommand_via_ssh` - 在远程主机执行命令
- `uploadfile_via_ssh` - 通过 SFTP 上传本地文件或目录（目录自动递归）
- `downloadfile_via_ssh` - 通过 SFTP 下载远程文件或目录（目录自动递归）

**公共参数：**
| 参数 | 必填 | 说明 |
|------|------|------|
| hostname | 是 | SSH config 的 Host 别名；未配置时也可填真实主机名 / IP |
| user | 否 | SSH 用户名（覆盖 config 的 User） |
| port | 否 | SSH 端口（覆盖 config 的 Port，默认 22） |
| password | 否 | SSH 密码（可选；默认用 IdentityFile 或 `~/.ssh/id_*`） |
| key_path | 否 | SSH 私钥路径（覆盖 config 的 IdentityFile） |
| timeout | 否 | 连接超时秒数（覆盖 config 的 ConnectTimeout，默认 30） |

**runsshcommand_via_ssh 额外参数：**
| 参数 | 必填 | 说明 |
|------|------|------|
| command | 是 | 要执行的命令 |

**uploadfile_via_ssh / downloadfile_via_ssh 额外参数：**
| 参数 | 必填 | 说明 |
|------|------|------|
| local_path | 是 | 本地文件或目录路径 |
| remote_path | 是 | 远程文件或目录路径。上传时若目标是目录（已存在或以 `/` 结尾），自动拼接本地文件/目录名 |
| overwrite | 否 | 目标已存在时是否覆盖（默认 false） |
| create_dirs | 否 | 单文件传输时是否创建缺失的父目录（默认 false） |

目录会自动递归传输并创建缺失目录、保留权限；符号链接等特殊文件会跳过，结果中的 `skipped` 为跳过数量，`files` 为传输文件数。默认不覆盖已有文件（递归时只要有任一目标文件已存在就会在传输前报错）。

## 构建

```bash
# 构建所有工具
make

# 构建 Linux 版本
make linux

# 清理
make clean
```

## 环境要求

- Go 1.25+（occonfig 需要）
- Chrome/Chromium (markdown2pdf 需要)
