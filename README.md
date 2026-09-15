# my-tools

个人自用工具集，收集日常开发和运维中常用的 CLI 工具和 MCP 工具。主要用于提升工作效率，解决重复性操作问题。

## 项目结构

```
my-tools/
├── clitool/              # CLI 工具
│   ├── markdown2pdf/     # Markdown 转 PDF
│   └── ssh_config_manage/# SSH 配置管理
├── mcptool/              # MCP 工具
│   └── cmd/sshtool/      # SSH MCP Server
└── bin/                  # 编译输出
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

- Go 1.21+
- Chrome/Chromium (markdown2pdf 需要)

---

# my-tools (English)

A collection of personal CLI and MCP tools for daily development and operations. Designed to improve efficiency and automate repetitive tasks.

## Project Structure

```
my-tools/
├── clitool/              # CLI tools
│   ├── markdown2pdf/     # Markdown to PDF converter
│   └── ssh_config_manage/# SSH config manager
├── mcptool/              # MCP tools
│   └── cmd/sshtool/      # SSH MCP Server
└── bin/                  # Build output
```

## Tools

### markdown2pdf

Convert Markdown files to PDF using Chrome Headless rendering. Ideal for quick document generation and note archiving.

```bash
# Usage
markdown2pdf -i input.md -o output.pdf

# Or simply
markdown2pdf input.md
```

### ssh_config_manage

TUI tool for managing SSH config files, built with Bubble Tea. Convenient for viewing, editing, and managing SSH connections to multiple servers.

```bash
# Usage
sshman -file ~/.ssh/config
```

### sshtool (MCP Server)

SSH remote command execution MCP Server with password and key authentication. Built-in high-risk command blocking. Can be integrated into AI assistants or automation tools via MCP protocol for secure remote server management.

**Available Tools:**
- `runsshcommand_via_ssh` - Execute commands on remote hosts

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| host | Yes | Remote host IP or hostname |
| user | Yes | SSH username |
| command | Yes | Command to execute |
| port | No | SSH port (default: 22) |
| password | No | SSH password |
| key_path | No | Path to SSH private key |
| timeout | No | Connection timeout in seconds (default: 30) |

## Build

```bash
# Build all tools
make

# Build for Linux
make linux

# Clean
make clean
```

## Requirements

- Go 1.21+
- Chrome/Chromium (required by markdown2pdf)
