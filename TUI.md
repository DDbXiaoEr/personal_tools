# TUI 风格规范

新增 Bubble Tea TUI 时 **以本文件为准**，不要再翻现有工具源码。参考实现：`clitool/opencode_config`（最完整）、`clitool/shell_rc_manage`。`ssh_config_manage` 是早期版本，不要模仿它的内联样式和表单结构。

## 技术栈

- Go 1.25+，独立 `go.mod`（每个工具一个 module）
- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/bubbles`（`textinput` / `textarea` / 按需 `filepicker` / `viewport`）
- `github.com/charmbracelet/lipgloss`
- 启动：`tea.NewProgram(newModel(...), tea.WithAltScreen())`

## 目录与文件

```
clitool/<tool_dir>/
  main.go                 # flag、Load、tea.NewProgram
  app.go                  # model / Update / View / 列表与确认
  form.go                 # 通用表单字段（有编辑表单才需要）
  styles.go               # lipgloss 样式，禁止把颜色散落在 app.go
  internal/<pkg>/         # 解析、写出、领域类型；TUI 不直接读写文件细节
    <pkg>_test.go
```

- 目录用 snake_case（`shell_rc_manage`），二进制名可缩短（`shellman`、`occonfig`、`sshman`）。
- `package main` 只放 UI；解析/保存/默认路径全部进 `internal/`。
- 文案中文，标识符英文。不要加注释，除非行为不直观。

## 样式（必须原样复用）

`styles.go` 固定这套 ANSI 色，不要换主题：

```go
var (
	titleStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	activeTabStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Padding(0, 1)
	inactiveTabStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(0, 1)
	selectedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57"))
	dimStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	labelStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Width(16) // 字段名较长用 24
	focusedLabel       = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Width(16)
	helpStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	okStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("76"))
	warnStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	boxStyle           = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	buttonStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Border(lipgloss.RoundedBorder()).Padding(0, 1)
	buttonFocusedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Border(lipgloss.RoundedBorder()).Padding(0, 1)
	catStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true) // 仅分组列表
)
```

用途：

| 样式 | 用在 |
|------|------|
| `titleStyle` | 工具名、表单标题、子弹窗标题 |
| `activeTabStyle` / `inactiveTabStyle` | 顶栏标签 |
| `selectedStyle` | 当前列表行、choice 选中项 |
| `dimStyle` | 路径、次要详情、空状态、帮助正文 |
| `labelStyle` / `focusedLabel` | 表单字段名，宽度对齐 |
| `helpStyle` | 底栏快捷键一行 |
| `errStyle` / `okStyle` | 状态栏；确认提示用 `errStyle` |
| `warnStyle` | 校验警告（如 `⚠`） |
| `boxStyle` | 包住整个表单 `View()` |
| `buttonStyle` / `buttonFocusedStyle` | 表单动作按钮，文案前加 `▸ ` |
| `catStyle` | 分组标题 `── 分类 ──` |

## 程序骨架

```go
func main() {
	path := flag.String("file", pkg.DefaultPath(), "配置文件路径")
	flag.Parse()
	cfg, err := pkg.Load(*path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取配置失败: %v\n", err)
		os.Exit(1)
	}
	p := tea.NewProgram(newModel(*path, cfg), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "运行失败: %v\n", err)
		os.Exit(1)
	}
}
```

- 用 `flag`，不要 cobra。
- 文件不存在时 `internal` 返回空配置，不要让 TUI 起不来。
- `Init()` 通常 `nil`；有 textinput 时返回 `textinput.Blink`。

## Model

```go
type uiMode int
const (
	modeList uiMode = iota
	modeForm
	modeConfirm
	// 按需: modeSource / modeFilePick / modeView
)

type pendingKind int
const (
	pendingNone pendingKind = iota
	pendingDelete
	pendingQuit
)

type model struct {
	mode     uiMode
	pending  pendingKind
	cursor   int
	status   string
	isErr    bool
	showHelp bool
	width    int
	height   int
	form     form
	// dirty 用 bool，或多 tab 用 map[Kind]bool
}
```

- `setOK` / `setErr` 成对更新 `status` + `isErr`。
- `Update` 先处理 `tea.WindowSizeMsg`，再按 `mode` 分发 `tea.KeyMsg`。
- 默认宽高给一个合理初值（约 `100x30`），避免首帧布局崩。

## 模式与快捷键

列表模式：

| 键 | 行为 |
|----|------|
| `q` / `ctrl+c` | 退出；有未保存修改则进确认 |
| `1` `2` `3` | 切 tab（有 tab 才要） |
| `tab` / `shift+tab` | 循环 tab |
| `?` | 展开/收起帮助 |
| `o` | 切换来源/上下文（如有） |
| `r` | 从文件重载 |
| `s` | 写回文件 |
| `↑` `k` / `↓` `j` | 移动光标 |
| `g` / `home`、`G` / `end` | 跳到首/末 |
| `a` | 新增，进表单 |
| `enter` / `e` | 编辑（只读页则查看） |
| `d` / `delete` / `backspace` | 删除，进确认 |
| `space` | 列表内开关（如启用/禁用） |

表单模式：

| 键 | 行为 |
|----|------|
| `esc` | 丢弃，回列表 |
| `ctrl+s` | 校验并应用到内存，回列表 |
| `tab` / `shift+tab` | 下一/上一字段 |
| `↑` `↓` | 切字段；textarea 内还有行则把按键交给 textarea |
| `←` `h` / `→` `l` / `space` / `enter` | choice 循环、bool 翻转、按钮触发 |
| `ctrl+c` | 直接退出 |

确认模式：`y`/`Y` 执行 pending，其它键取消。文案：`确定删除 "name" ? (y/n)`，整行 `errStyle`。未保存退出：`有未保存的修改，确定退出 ?`。

## 列表布局

从上到下：

1. **顶栏**：有 tab 就画 tab；当前 tab 用 `activeTabStyle`。后面 `dimStyle` 跟当前文件路径。dirty 在标题或路径后加 ` *`。
2. 空行。
3. **列表**：可见行数约 `height-8`（展开帮助再减 8），最少 3 行。用窗口函数只渲染光标附近。
4. **空状态**：`dimStyle.Render("暂无条目，按 'a' 新增。")`
5. **状态行**：`okStyle` 或 `errStyle`。
6. **底栏**：`helpStyle`，中文，用 ` · ` 分隔，例如：
   `1/2/3 切换 · a 新增 · enter/e 编辑 · d 删除 · s 保存 · r 重载 · ? 帮助 · q 退出`
7. **`?` 帮助**：`dimStyle`，以 `提示：` 开头的项目符号，说明文件约定、内存编辑、`.bak`、本工具不改的文件。

列表行格式：

- 选中：`selectedStyle.Render("› " + plain)`
- 未选中：`"  " + name`，次要字段用 `dimStyle`
- 名称列左对齐固定宽度（约 20–24，或按 rune 宽动态 pad）
- 开关项：`[on ]` / `[off]` 前缀
- 分组：`catStyle` 的 `── 分类 ──`，header 行不可选，光标只停在条目上

## 表单

`form.go` 提供字段工厂，app 只负责 `buildXxxForm` / `submitForm`。

字段类型：

- `kindText`：`textinput`，`Prompt = ""`，有 `placeholder`、`CharLimit`
- `kindArea`：`textarea`，`ShowLineNumbers = false`，显式 `SetHeight`
- `kindChoice`：选项横排，选中 `selectedStyle.Render("["+opt+"]")`，未选 `dimStyle`
- `kindBool`：`[x]` / `[ ]`
- `kindButton`：圆角按钮，聚焦时 `buttonFocusedStyle`，`enter`/`space` 触发 `action`

约定：

- 表单整体 `boxStyle.Render(m.form.View())`
- 标题 `新增 X` / `编辑 X: name`
- 主键在编辑时 `readOnly`，显示 `dimStyle`（改名 = 删了再加）
- 动态显隐用 `hidden` + `onChoice`（见 shellman env 的 export/assign/snippet）
- `ctrl+s` **只改内存**，成功后 `setOK("已更新，按 s 保存到文件")`，不要直接写盘
- 校验失败 `setErr`，留在表单
- 多行 KV 用 textarea，「每行一条」；空行和 `#` 注释跳过

## 保存 / 重载 / dirty

- 编辑、删除、开关都先改内存，标 dirty。
- `s` 才写文件。成功：`已保存到 <path>`。无变更：`没有需要保存的修改`。
- 写盘前若文件已存在，先写 `<path>.bak`。
- 删除后提示 `已删除 …，按 s 保存`。
- `r` 从磁盘重载；若 dirty，应报错或先确认，避免丢改。
- 退出时若 dirty，必须确认。
- 尽量保留原文件里未暴露的字段、注释、键顺序。
- 不要顺手改用户没让管的文件（例如 shellman 不改 `~/.zshrc`）。

## 领域层

`internal/<pkg>` 负责：

- `DefaultPath()` / `DetectXxx()`
- `Load` / `Parse` / `Save`
- 领域 struct 与编解码

TUI 只调这些 API。测试写在 `internal/`，覆盖解析、写出、备份。

## Makefile

根目录 `Makefile` 为每个工具加一对 target，TUI 用 `-ldflags "-s -w"`：

```make
build-<bin>:
	@echo "Building <bin>..."
	cd clitool/<tool_dir> && go build -ldflags "-s -w" -o ../../$(BIN_DIR)/clitool/<bin> .

linux-<bin>:
	@echo "Building <bin> for Linux..."
	cd clitool/<tool_dir> && GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ../../$(BIN_DIR)/clitool/<bin>-linux .
```

并挂到 `build-clitool` / `linux-clitool`。

## README

中英 README 都要加一节：做什么、命令、flag、快捷键表、保存/备份约定。快捷键表与底栏文案一致。

## 反例（不要做）

- 新配色、无边框表单、英文底栏
- 表单 `ctrl+s` 直接写文件
- 删除/脏退出不确认
- 把 lipgloss 样式写进 `model.go`
- 在 `app.go` 里手写文件解析
- 用 TUI 框架以外的 UI 库
- 改名通过直接改主键（应只读 + 删增）
