# AGENTS

本仓库是个人 CLI / MCP 工具集。改代码前先看对应目录，不要跨工具乱改。

## 新增或修改 TUI

**必须先读 [TUI.md](TUI.md)**，按该规范实现，不要再去翻 `opencode_config` / `shell_rc_manage` / `ssh_config_manage` 摸风格。

要点：

- 每个工具独立 module，放 `clitool/<snake_case>/`
- Bubble Tea + Bubbles + Lipgloss，`tea.WithAltScreen()`
- 文件拆分：`main.go` / `app.go` / `form.go` / `styles.go` / `internal/<pkg>/`
- 颜色、快捷键、列表/表单/确认布局以 `TUI.md` 为准
- 编辑留内存，`s` 才写盘并打 `.bak`；删除和脏退出要 `y/n` 确认
- 文案中文；`Makefile` 增加 `build-*` / `linux-*`；更新 `README.md` 与 `README_EN.md`

## 仓库结构

```
clitool/     # CLI，各子目录一个工具
mcptool/     # MCP server
```

现有 TUI：`ssh_config_manage`（sshman）、`opencode_config`（occonfig）、`shell_rc_manage`（shellman）。后两个才是当前风格；sshman 偏旧，新工具不要仿它。

## 构建

```
make            # 全部
make linux      # linux amd64
make clean
```

单个工具在其子目录 `go build` / `go test ./...`。
