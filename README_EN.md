# my-tools

[中文](README.md)

A collection of personal CLI and MCP tools for daily development and operations. Designed to improve efficiency and automate repetitive tasks.

## Project Structure

```
my-tools/
├── clitool/              # CLI tools
│   ├── markdown2pdf/     # Markdown to PDF converter
│   ├── ssh_config_manage/# SSH config manager
│   ├── viewcsv/          # CSV viewer
│   ├── opencode_config/  # opencode config manager TUI
│   └── shell_rc_manage/  # shell alias / env / function TUI
└── mcptool/              # MCP tools
    └── cmd/sshtool/      # SSH MCP Server
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

### viewcsv

CSV file viewer that displays content in table format with custom delimiter support and Chinese character alignment.

```bash
# Usage
viewcsv input.csv

# Read all CSV files in the current directory
viewcsv *.csv

# Custom delimiter
viewcsv input.tsv -d "	"

# No header mode
viewcsv input.csv --no-header
```

### shellman

TUI for managing shell aliases, environment variables, and custom functions, built with Bubble Tea. Reads/writes `.{shell}_{alias,env,functions}` in the home directory, grouped by category comments on save. Does not modify `~/.zshrc` / `~/.bashrc`; source those files yourself.

**File layout:**

| File | Content |
|------|---------|
| `~/.zsh_alias` / `~/.bash_alias` | `alias name='cmd'` |
| `~/.zsh_env` / `~/.bash_env` | `export` / assignment / snippet |
| `~/.zsh_functions` / `~/.bash_functions` | `name() { ... }` |

Items in the same category are written together:

```sh
# ===== Docker =====
alias dim='docker images'
alias dps='docker ps'
```

Env entries support `export KEY=value`, `KEY=value`, and multi-line snippets (nvm/bun loaders, etc.).

```bash
# Usage (defaults to current $SHELL and $HOME)
shellman

# Override shell and directory
shellman -shell bash
shellman -dir /custom
```

**Keybindings:**

| Key | Action |
|-----|--------|
| `1` / `2` / `3` | Switch Alias / Env / Functions |
| `↑` `↓` / `j` `k` | Move cursor |
| `a` | Add |
| `enter` / `e` | Edit |
| `d` | Delete (with confirmation) |
| `s` | Save the current tab's file |
| `r` | Reload from disk |
| `o` | Switch zsh / bash |
| `?` | Help |
| `q` | Quit |

In forms: `tab` moves fields, `ctrl+s` applies, `esc` cancels. Function bodies and snippets use a textarea.

> Saving writes a `<file>.bak` backup. Existing `~/.zsh_aliases` is not read; point your rc at `source ~/.zsh_alias` if you want to migrate.

### occonfig

TUI tool for managing opencode configuration, built with Bubble Tea. Manage the global or project `opencode.json` while preserving comments, key order, and untouched fields.

**Features:**
- **Provider**: add/edit/delete custom providers (`id` / `name` / `npm` / `options.baseURL` / `options.apiKey`). Keys not exposed in the form (e.g. `models`, `blacklist`) are preserved as-is.
- **MCP**: manage both `local` and `remote` servers covering `command`, `environment`, `cwd`, `url`, `headers`, `oauth`, `timeout`, `enabled`; press space to toggle enable/disable from the list.
- **Skills**: read-only browse of global `~/.config/opencode/skills/*/SKILL.md`, validating frontmatter (`name` / `description`) and viewing the full content.
- **Config source**: defaults to the global config; press `o` to switch to a project `opencode.json` found by walking up from the current directory.

```bash
# Usage (defaults to ~/.config/opencode/opencode.json)
occonfig

# Custom config and skills directory
occonfig -config /path/to/opencode.json -skills-dir ~/.config/opencode/skills
```

**Keybindings:**

| Key | Action |
|-----|--------|
| `1` / `2` / `3` | Switch Provider / MCP / Skills tabs |
| `↑` `↓` / `j` `k` | Move cursor |
| `a` | Add |
| `enter` / `e` | Edit (view full text on Skills tab) |
| `d` | Delete (with confirmation) |
| `space` | Enable / disable MCP |
| `s` | Save to file |
| `r` | Reload from file |
| `o` | Switch global / project config source |
| `?` | Help |
| `q` | Quit |

**MCP local command:** in the form, Tab to the "浏览可执行文件" button and press `Enter` (or press `ctrl+f`) to open a file picker; the selected executable path is auto-filled and existing argument lines are kept.

> Saving writes a `<config>.bak` backup and normalizes the file to tab-indented JSONC (opencode-compatible).

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

- Go 1.25+ (required by occonfig)
- Chrome/Chromium (required by markdown2pdf)
