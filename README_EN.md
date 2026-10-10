# my-tools

[中文](README.md)

A collection of personal CLI and MCP tools for daily development and operations. Designed to improve efficiency and automate repetitive tasks.

## Project Structure

```
my-tools/
├── clitool/              # CLI tools
│   ├── markdown2pdf/     # Markdown to PDF converter
│   ├── ssh_config_manage/# SSH config manager
│   ├── viewcsv_xlsx/     # CSV / XLSX viewer
│   ├── opencode_config/  # opencode config manager TUI
│   ├── shell_rc_manage/  # shell alias / env / function TUI
│   └── ansible_inventory/# Ansible inventory manager TUI
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

TUI tool for managing SSH config files, built with Bubble Tea. Convenient for viewing, editing, and managing SSH connections to multiple servers. Supports cloning a host and adding many hosts at once.

```bash
# Usage
sshman -file ~/.ssh/config
```

**Keys:**

| Key | Action |
|-----|--------|
| `a` | Add one host |
| `A` | Bulk add (shared User/Port/key + one host per line) |
| `c` | Clone the current host; alias gets a numeric suffix |
| `enter` / `e` | Edit |
| `d` | Delete (confirm) |
| `s` | Save to file |
| `q` | Quit |

Bulk line format: `alias [hostname [user [port]]]`. A single field is used as both alias and HostName. Shared options apply to every host; per-line User/Port override them.

In forms, `tab` moves fields, `ctrl+s` applies to memory, `esc` cancels. Disk write happens only on save.

### viewcsv_xlsx

CSV / XLSX file viewer that displays content in table format with custom delimiter support, Chinese character alignment, and multi-sheet Excel files. Multiple files open fullscreen; `tab` / `shift+tab` switch files. A single file or piped (non-TTY) output still prints the table.

```bash
# Usage (format auto-detected by extension)
viewcsv_xlsx input.csv
viewcsv_xlsx input.xlsx

# Read all CSV files in the current directory
viewcsv_xlsx *.csv

# Custom delimiter
viewcsv_xlsx -d "	" input.tsv

# Select a sheet (name or 1-based index, defaults to all)
viewcsv_xlsx --sheet 2 input.xlsx
viewcsv_xlsx --sheet Users input.xlsx

# No header mode
viewcsv_xlsx --no-header input.csv
```

**Keys (fullscreen, multiple files):**

| Key | Action |
|-----|--------|
| `tab` / `shift+tab` | Switch file |
| `[` / `]` | Switch sheet |
| `↑` `↓` `←` `→` / `j` `k` | Scroll |
| `q` | Quit |

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

Env entries support `export KEY=value`, `KEY=value`, and multi-line snippets (nvm/bun loaders, etc.). export / assign can keep alternate values; in the list, `tab` / `shift+tab` / space cycles the active value (with no alternates, `tab` still switches tabs). Alternates are stored as `# alt:` lines under the variable:

```sh
export HTTP_PROXY=http://127.0.0.1:7890
# alt: http://127.0.0.1:1087
# alt: socks5://127.0.0.1:1080
```

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
| `tab` / `shift+tab` | Cycle env value when alternates exist; otherwise switch tabs |
| `space` | Cycle env value when alternates exist |
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
- **Provider**: add/edit/delete custom providers (`id` / `name` / `npm` / `options.baseURL` / `options.apiKey`). Press `v` to manage `models.variants` (`disabled` plus per-line `KEY=value` options). Keys not exposed in the form (e.g. `blacklist`) are preserved as-is.
- **MCP**: manage both `local` and `remote` servers covering `command`, `environment`, `cwd`, `url`, `headers`, `oauth`, `timeout`, `enabled`; press space to toggle enable/disable from the list.
- **Skills**: browse global `~/.config/opencode/skills/*/SKILL.md`, validating frontmatter (`name` / `description`). Press `a` to download a zip from a URI and extract it under a chosen directory name; press `d` to delete a skill directory.
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
| `a` | Add (install from zip URI on Skills tab) |
| `enter` / `e` | Edit (view full text on Skills tab) |
| `v` | Manage variants for the current Provider |
| `d` | Delete (with confirmation) |
| `space` | Enable / disable MCP; disable / enable a variant in the variants list |
| `s` | Save to file |
| `r` | Reload from file |
| `o` | Switch global / project config source |
| `?` | Help |
| `q` | Quit |

**MCP local command:** in the form, Tab to the "浏览可执行文件" button and press `Enter` (or press `ctrl+f`) to open a file picker; the selected executable path is auto-filled and existing argument lines are kept.

> Saving writes a `<config>.bak` backup and normalizes the file to tab-indented JSONC (opencode-compatible).

### ansiman

TUI for Ansible inventories, built with Bubble Tea. Manage a default inventory and project inventories (INI / YAML): hosts, groups, and group vars.

**Config dir `~/.config/ansiman/`:**

| File | Content |
|------|---------|
| `config` | `default_inventory=/path/to/hosts` |
| `projects` | one `name=path` per line, separate from config |

If unset, the default path falls back to `ANSIBLE_INVENTORY` / ansible.cfg / `/etc/ansible/hosts`. Press `o` for the source page: left pane edits the default path, right pane manages the project list.

```bash
# Usage (default inventory)
ansiman

# Explicit inventory file
ansiman -file ./inventory.yml
```

**Keybindings:**

| Key | Action |
|-----|--------|
| `1` / `2` / `3` | Switch Hosts / Groups / Vars |
| `↑` `↓` / `j` `k` | Move cursor |
| `a` | Add |
| `enter` / `e` | Edit |
| `d` | Delete (with confirmation) |
| `s` | Save to file |
| `r` | Reload from disk |
| `o` | Source: default path / project list |
| `?` | Help |
| `q` | Quit |

In forms: `tab` moves fields, `ctrl+s` applies, `esc` cancels. Extra host vars and child groups use a textarea, one item per line.

> Saving writes a `<inventory>.bak` backup and keeps the original format (INI or YAML). Deleting a group moves its hosts to `ungrouped`. Rename by deleting and adding.

### sshtool (MCP Server)

SSH remote command execution / file transfer MCP Server with password and key authentication. Built-in high-risk command blocking. Commands run over an SSH session; uploads and downloads use SFTP. Can be integrated into AI assistants or automation tools via MCP.

**Available Tools:**
- `runsshcommand_via_ssh` - Execute commands on remote hosts
- `uploadfile_via_ssh` - Upload a local file over SFTP
- `downloadfile_via_ssh` - Download a remote file over SFTP

**Shared parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| host | Yes | Remote host IP or hostname |
| user | Yes | SSH username |
| port | No | SSH port (default: 22) |
| password | No | SSH password (either this or key_path) |
| key_path | No | Path to SSH private key (either this or password) |
| timeout | No | Connection timeout in seconds (default: 30) |

**runsshcommand_via_ssh extra parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| command | Yes | Command to execute |

**uploadfile_via_ssh / downloadfile_via_ssh extra parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| local_path | Yes | Local file path |
| remote_path | Yes | Remote file path. On upload, if the destination is a directory (exists or ends with `/`), the local filename is appended |
| overwrite | No | Overwrite an existing destination (default: false) |
| create_dirs | No | Create missing parent directories (default: false) |

Directory trees are not supported. Existing files are not overwritten by default.

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
