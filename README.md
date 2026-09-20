# my-tools

[English](README_EN.md)

个人自用工具集，收集日常开发和运维中常用的 CLI 工具和 MCP 工具。主要用于提升工作效率，解决重复性操作问题。

## 项目结构

```
my-tools/
├── clitool/              # CLI 工具
│   ├── markdown2pdf/     # Markdown 转 PDF
│   ├── ssh_config_manage/# SSH 配置管理
│   ├── viewcsv/          # CSV 查看器
│   └── opencode_config/  # opencode 配置管理 TUI
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

SSH 配置文件管理 TUI 工具，基于 Bubble Tea 构建。方便查看、编辑和管理多台服务器的 SSH 连接配置。

```bash
# 使用
sshman -file ~/.ssh/config
```

### viewcsv

CSV 文件查看工具，以表格形式展示 CSV 内容，支持自定义分隔符和中文对齐。

```bash
# 使用
viewcsv -i input.csv

# 指定分隔符
viewcsv -i input.tsv -d "	"

# 无表头模式
viewcsv -i input.csv --no-header
```

### occonfig

opencode 配置管理 TUI 工具，基于 Bubble Tea 构建。用于管理全局或项目的 `opencode.json`，写回时保留注释、键顺序和未编辑字段。

**功能：**
- **Provider**：增删改自定义 provider（`id` / `name` / `npm` / `options.baseURL` / `options.apiKey`）；未在表单中暴露的键（如 `models`、`blacklist`）原样保留。
- **MCP**：管理 `local` / `remote` 两类 server，覆盖 `command`、`environment`、`cwd`、`url`、`headers`、`oauth`、`timeout`、`enabled`，列表中按空格快速启停。
- **Skills**：只读浏览全局 `~/.config/opencode/skills/*/SKILL.md`，校验 frontmatter（`name` / `description`）并可查看全文。
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
| `a` | 新增 |
| `enter` / `e` | 编辑（Skills 页为查看全文） |
| `d` | 删除（需确认） |
| `space` | 启用 / 禁用 MCP |
| `s` | 保存到文件 |
| `r` | 从文件重载 |
| `o` | 切换全局 / 项目配置来源 |
| `?` | 帮助 |
| `q` | 退出 |

**MCP local 配置命令：** 在表单中 Tab 到「浏览可执行文件」按钮按 `Enter`（或按 `ctrl+f`）打开文件选择器，选中后自动填入可执行文件路径，已有参数行会保留。

> 保存时会生成 `<配置文件>.bak` 备份，并将文件格式化为制表符缩进的 JSONC（opencode 兼容）。

### sshtool (MCP Server)

SSH 远程命令执行 MCP Server，支持密码和密钥认证，内置高危命令拦截。可通过 MCP 协议集成到 AI 助手或自动化工具中，实现安全的远程服务器管理。

**可用工具：**
- `runsshcommand_via_ssh` - 在远程主机执行命令

**参数：**
| 参数 | 必填 | 说明 |
|------|------|------|
| host | 是 | 远程主机 IP 或域名 |
| user | 是 | SSH 用户名 |
| command | 是 | 要执行的命令 |
| port | 否 | SSH 端口 (默认 22) |
| password | 否 | SSH 密码 |
| key_path | 否 | SSH 私钥路径 |
| timeout | 否 | 连接超时秒数 (默认 30) |

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
