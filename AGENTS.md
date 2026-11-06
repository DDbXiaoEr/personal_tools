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

现有 TUI：`ssh_config_manage`（sshman）、`opencode_config`（occonfig）、`shell_rc_manage`（shellman）、`ansible_inventory`（ansiman）。occonfig / shellman / ansiman 才是当前风格；sshman 偏旧，新工具不要仿它。

## 构建

```
make            # 全部
make linux      # linux amd64
make clean
```

单个工具在其子目录 `go build` / `go test ./...`。

## 任务完成通知

每次完成用户请求的任务后，在给出最终回复前，调用 `notify` MCP 服务的消息工具发送一次完成通知。

### 钉钉

调用 `send_dingtalk_message`（工具名可能显示为 `notify_send_dingtalk_message`）：

- `content`：`任务完成：<一句话概括本次完成内容>`
- `at_user_ids`：`["1882179537"]`
- `is_at_all`：`false`

### 企业微信

调用 `send_wecom_message`（工具名可能显示为 `notify_send_wecom_message`）：

- `content`：与钉钉相同
- `mentioned_list`：`["quanxiaoer"]`

### 注意事项

- 通知内容保持简洁，只说明完成了什么，不要包含密钥、token、密码等敏感信息。
- 如果 `notify` 服务未配置对应平台的 key，跳过该平台即可，不要反复重试，也不要因此阻塞任务。
- 发送通知本身不计为新的任务，不要为通知再次发送通知。

