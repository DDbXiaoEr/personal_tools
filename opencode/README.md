# opencode

OpenCode TUI 插件代码的备份存档，实际运行目录为 `~/.config/opencode/tui-plugins/`。

当前运行的是 OpenCode v2，通过 `~/.config/opencode/cli.json` 加载 `daily-tokens` 目录版插件；`tui.json` 中的单文件写法属于旧版遗留。

## daily-tokens

汽车仪表盘样式的 token 速度显示插件，挂在侧边栏底部：

- 实时输出速度（tokens/s）与本次会话最快速度
- 仪表盘刻度、指针和数字读数
- ODO 显示当天累计消耗
- DeepSeek 余额显示

目录版（当前使用）：

```
daily-tokens/
├── package.json   # exports ./tui -> ./tui.tsx
└── tui.tsx        # 插件实现
```

## daily-tokens.tsx

单文件旧版实现，功能与目录版相同，供旧版 `tui.json` 加载，当前 v2 不加载。

## reload-mcp.tsx

MCP 重载插件，提供 `/reload-mcp` 命令与 `mcp.reload` 动作。

## 运行方式

目录版由 `~/.config/opencode/cli.json` 加载，路径保持指向实际运行目录：

```json
{
  "plugins": [
    "./tui-plugins/daily-tokens"
  ]
}
```

旧版单文件插件由 `~/.config/opencode/tui.json` 加载（仅存档参考）：

```json
{
  "plugin": [
    "./tui-plugins/daily-tokens.tsx",
    "./tui-plugins/reload-mcp.tsx"
  ]
}
```

本目录仅作备份，不参与加载。

## 恢复

把文件复制回运行位置即可：

```sh
cp -R opencode/daily-tokens ~/.config/opencode/tui-plugins/daily-tokens
cp opencode/daily-tokens.tsx ~/.config/opencode/tui-plugins/daily-tokens.tsx
cp opencode/reload-mcp.tsx ~/.config/opencode/tui-plugins/reload-mcp.tsx
```
