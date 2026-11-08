# opencode

OpenCode TUI 插件代码的备份存档，实际运行目录为 `~/.config/opencode/tui-plugins/`。

## daily-tokens

汽车仪表盘样式的 token 速度显示插件，挂在侧边栏底部：

- 实时输出速度（tokens/s）与本次会话最快速度
- 仪表盘刻度、指针和数字读数
- ODO 显示当天累计消耗

### 运行方式

插件由 `~/.config/opencode/cli.json` 加载，路径保持指向实际运行目录：

```json
{
  "plugins": [
    "./tui-plugins/daily-tokens"
  ]
}
```

本目录仅作备份，不参与加载。

### 恢复

把本目录复制回运行位置即可：

```sh
cp -R opencode/daily-tokens ~/.config/opencode/tui-plugins/daily-tokens
```

### 结构

```
daily-tokens/
├── package.json   # exports ./tui -> ./tui.tsx
└── tui.tsx        # 插件实现
```
