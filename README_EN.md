# my-tools

[中文](README.md)

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
