package main

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"shellrc-manage/internal/shellrc"
)

type uiMode int

const (
	modeList uiMode = iota
	modeForm
	modeConfirm
)

type pendingKind int

const (
	pendingNone pendingKind = iota
	pendingDelete
	pendingQuit
	pendingSwitchShell
)

type listRow struct {
	header bool
	cat    string
	index  int
}

type model struct {
	dir   string
	store *shellrc.Store

	tab    shellrc.Kind
	mode   uiMode
	cursor int

	dirty map[shellrc.Kind]bool

	form    form
	editIdx int

	pending      pendingKind
	pendingShell string

	status   string
	isErr    bool
	showHelp bool

	width  int
	height int
}

func newModel(dir string, store *shellrc.Store) model {
	return model{
		dir:     dir,
		store:   store,
		tab:     shellrc.KindAlias,
		dirty:   map[shellrc.Kind]bool{},
		width:   100,
		height:  30,
		editIdx: -1,
	}
}

func (m *model) setOK(msg string)  { m.status, m.isErr = msg, false }
func (m *model) setErr(msg string) { m.status, m.isErr = msg, true }

func (m model) Init() tea.Cmd { return textinput.Blink }

func (m *model) file() *shellrc.File {
	return m.store.File(m.tab)
}

func (m *model) items() []*shellrc.Item {
	return m.file().Items
}

func (m *model) rows() []listRow {
	items := m.items()
	var out []listRow
	for _, cat := range shellrc.CategoryOrder(items) {
		out = append(out, listRow{header: true, cat: cat, index: -1})
		for i, it := range items {
			if it.Cat() == cat {
				out = append(out, listRow{cat: cat, index: i})
			}
		}
	}
	return out
}

func (m *model) currentItem() *shellrc.Item {
	items := m.items()
	if m.cursor < 0 || m.cursor >= len(items) {
		return nil
	}
	return items[m.cursor]
}

func (m *model) anyDirty() bool {
	for _, k := range shellrc.AllKinds() {
		if m.dirty[k] {
			return true
		}
	}
	return false
}

func (m *model) clampCursor() {
	n := len(m.items())
	if n == 0 {
		m.cursor = 0
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= n {
		m.cursor = n - 1
	}
}

func (m *model) moveCursor(delta int) {
	n := len(m.items())
	if n == 0 {
		m.cursor = 0
		return
	}
	m.cursor += delta
	m.clampCursor()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		switch m.mode {
		case modeForm:
			return m.updateForm(msg)
		case modeConfirm:
			return m.updateConfirm(msg)
		default:
			return m.updateList(msg)
		}
	}
	return m, nil
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		if m.anyDirty() {
			m.pending = pendingQuit
			m.mode = modeConfirm
			return m, nil
		}
		return m, tea.Quit
	case "1":
		m.tab = shellrc.KindAlias
		m.clampCursor()
	case "2":
		m.tab = shellrc.KindEnv
		m.clampCursor()
	case "3":
		m.tab = shellrc.KindFunctions
		m.clampCursor()
	case "tab":
		m.nextTab()
	case "shift+tab":
		m.prevTab()
	case "?":
		m.showHelp = !m.showHelp
	case "o":
		m.startSwitchShell()
	case "r":
		m.reload()
	case "s":
		m.save()
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case "g", "home":
		m.cursor = 0
	case "G", "end":
		if n := len(m.items()); n > 0 {
			m.cursor = n - 1
		}
	case "a":
		m.startAdd()
		return m, textinput.Blink
	case "enter", "e":
		m.startEdit()
		return m, textinput.Blink
	case "d", "delete", "backspace":
		if m.currentItem() != nil {
			m.pending = pendingDelete
			m.mode = modeConfirm
		}
	}
	return m, nil
}

func (m *model) nextTab() {
	switch m.tab {
	case shellrc.KindAlias:
		m.tab = shellrc.KindEnv
	case shellrc.KindEnv:
		m.tab = shellrc.KindFunctions
	default:
		m.tab = shellrc.KindAlias
	}
	m.clampCursor()
}

func (m *model) prevTab() {
	switch m.tab {
	case shellrc.KindAlias:
		m.tab = shellrc.KindFunctions
	case shellrc.KindEnv:
		m.tab = shellrc.KindAlias
	default:
		m.tab = shellrc.KindEnv
	}
	m.clampCursor()
}

func (m *model) startSwitchShell() {
	next := "bash"
	if m.store.Shell == "bash" {
		next = "zsh"
	}
	if m.anyDirty() {
		m.pending = pendingSwitchShell
		m.pendingShell = next
		m.mode = modeConfirm
		return
	}
	m.switchShell(next)
}

func (m *model) switchShell(shell string) {
	store, err := shellrc.Load(m.dir, shell)
	if err != nil {
		m.setErr("切换失败: " + err.Error())
		return
	}
	m.store = store
	m.dirty = map[shellrc.Kind]bool{}
	m.cursor = 0
	m.setOK("已切换到 " + store.Shell)
}

func (m *model) reload() {
	store, err := shellrc.Load(m.dir, m.store.Shell)
	if err != nil {
		m.setErr("重载失败: " + err.Error())
		return
	}
	m.store = store
	m.dirty = map[shellrc.Kind]bool{}
	m.clampCursor()
	m.setOK("已从文件重载")
}

func (m *model) save() {
	f := m.file()
	if err := f.Save(); err != nil {
		m.setErr("保存失败: " + err.Error())
		return
	}
	m.dirty[m.tab] = false
	m.setOK("已保存到 " + f.Path)
}

func (m model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "y" || msg.String() == "Y" {
		switch m.pending {
		case pendingDelete:
			m.deleteCurrent()
		case pendingQuit:
			return m, tea.Quit
		case pendingSwitchShell:
			m.switchShell(m.pendingShell)
		}
	}
	m.pending = pendingNone
	m.pendingShell = ""
	m.mode = modeList
	return m, nil
}

func (m *model) deleteCurrent() {
	items := m.items()
	if m.cursor < 0 || m.cursor >= len(items) {
		return
	}
	name := items[m.cursor].DisplayName()
	m.file().Items = append(items[:m.cursor], items[m.cursor+1:]...)
	m.dirty[m.tab] = true
	m.clampCursor()
	m.setOK("已删除 " + name + "，按 s 保存")
}

func (m *model) startAdd() {
	m.editIdx = -1
	m.buildForm(nil)
	m.mode = modeForm
	m.status = ""
}

func (m *model) startEdit() {
	it := m.currentItem()
	if it == nil {
		return
	}
	m.editIdx = m.cursor
	m.buildForm(it)
	m.mode = modeForm
	m.status = ""
}

func (m *model) buildForm(it *shellrc.Item) {
	cat := ""
	if it != nil {
		cat = it.Category
		if cat == shellrc.Uncategorized {
			cat = ""
		}
	} else if cur := m.currentItem(); cur != nil && cur.Cat() != shellrc.Uncategorized {
		cat = cur.Cat()
	}

	f := form{}
	switch m.tab {
	case shellrc.KindAlias:
		title := "新增别名"
		if it != nil {
			title = "编辑别名: " + it.Name
		}
		name := newText("名称", "ll")
		cmd := newText("命令", "ls -la")
		category := newText("分类", "文件与目录")
		if it != nil {
			name.setText(it.Name)
			cmd.setText(it.Value)
			category.setText(cat)
		} else {
			category.setText(cat)
		}
		f.title = title
		f.add(category, name, cmd)
	case shellrc.KindEnv:
		title := "新增环境变量"
		if it != nil {
			title = "编辑环境变量: " + it.DisplayName()
		}
		styleIdx := 0
		if it != nil {
			switch it.Style {
			case shellrc.StyleAssign:
				styleIdx = 1
			case shellrc.StyleSnippet:
				styleIdx = 2
			}
		}
		category := newText("分类", "Go")
		style := newChoice("类型", []string{"export", "assign", "snippet"}, styleIdx)
		name := newText("名称", "PATH")
		value := newText("值", "$HOME/bin:$PATH")
		body := newArea("内容", "[ -s \"$NVM_DIR/nvm.sh\" ] && . \"$NVM_DIR/nvm.sh\"", 8)
		if it != nil {
			category.setText(cat)
			name.setText(it.Name)
			if it.Style == shellrc.StyleSnippet {
				body.setArea(it.Value)
			} else {
				value.setText(it.Value)
			}
		} else {
			category.setText(cat)
		}
		applyEnvVisibility := func(frm *form) {
			snippet := frm.fields[1].choiceValue() == "snippet"
			frm.fields[3].hidden = snippet
			frm.fields[4].hidden = !snippet
			if snippet {
				frm.fields[2].label = "标签"
				frm.fields[2].input.Placeholder = "nvm"
			} else {
				frm.fields[2].label = "名称"
				frm.fields[2].input.Placeholder = "PATH"
			}
		}
		style.onChoice = applyEnvVisibility
		f.title = title
		f.add(category, style, name, value, body)
		applyEnvVisibility(&f)
	case shellrc.KindFunctions:
		title := "新增函数"
		if it != nil {
			title = "编辑函数: " + it.Name
		}
		category := newText("分类", "工具")
		name := newText("名称", "ip")
		body := newArea("函数体", "echo hi", 10)
		if it != nil {
			category.setText(cat)
			name.setText(it.Name)
			body.setArea(it.Value)
		} else {
			category.setText(cat)
		}
		f.title = title
		f.add(category, name, body)
	}
	f.focusFirst()
	m.form = f
}

func (m model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode = modeList
		return m, nil
	case "tab":
		m.form.next()
		return m, nil
	case "shift+tab":
		m.form.prev()
		return m, nil
	case "up":
		if m.form.moveUp() {
			return m, nil
		}
	case "down":
		if m.form.moveDown() {
			return m, nil
		}
	case "ctrl+s":
		if err := m.submitForm(); err != nil {
			m.setErr(err.Error())
			return m, nil
		}
		m.mode = modeList
		m.setOK("已更新，按 s 保存到文件")
		return m, nil
	}
	return m, m.form.updateFocused(msg)
}

func (m *model) submitForm() error {
	it, err := m.itemFromForm()
	if err != nil {
		return err
	}
	f := m.file()
	if m.editIdx >= 0 && m.editIdx < len(f.Items) {
		f.Items[m.editIdx] = it
		m.cursor = m.editIdx
	} else {
		f.Items = append(f.Items, it)
		m.cursor = len(f.Items) - 1
	}
	m.dirty[m.tab] = true
	return nil
}

func (m *model) itemFromForm() (*shellrc.Item, error) {
	switch m.tab {
	case shellrc.KindAlias:
		name := m.form.fields[1].text()
		if name == "" {
			return nil, fmt.Errorf("名称不能为空")
		}
		cmd := m.form.fields[2].text()
		if cmd == "" {
			return nil, fmt.Errorf("命令不能为空")
		}
		return &shellrc.Item{
			Category: m.form.fields[0].text(),
			Name:     name,
			Value:    cmd,
			Style:    shellrc.StyleAlias,
			Quote:    shellrc.QuoteSingle,
		}, nil
	case shellrc.KindEnv:
		styleName := m.form.fields[1].choiceValue()
		cat := m.form.fields[0].text()
		switch styleName {
		case "snippet":
			body := m.form.fields[4].areaText()
			if strings.TrimSpace(body) == "" {
				return nil, fmt.Errorf("snippet 内容不能为空")
			}
			return &shellrc.Item{
				Category: cat,
				Name:     m.form.fields[2].text(),
				Value:    body,
				Style:    shellrc.StyleSnippet,
			}, nil
		case "assign":
			name := m.form.fields[2].text()
			if name == "" {
				return nil, fmt.Errorf("名称不能为空")
			}
			return &shellrc.Item{
				Category: cat,
				Name:     name,
				Value:    m.form.fields[3].input.Value(),
				Style:    shellrc.StyleAssign,
				Quote:    inferQuote(m.form.fields[3].input.Value()),
			}, nil
		default:
			name := m.form.fields[2].text()
			if name == "" {
				return nil, fmt.Errorf("名称不能为空")
			}
			return &shellrc.Item{
				Category: cat,
				Name:     name,
				Value:    m.form.fields[3].input.Value(),
				Style:    shellrc.StyleExport,
				Quote:    inferQuote(m.form.fields[3].input.Value()),
			}, nil
		}
	default:
		name := m.form.fields[1].text()
		if name == "" {
			return nil, fmt.Errorf("名称不能为空")
		}
		body := m.form.fields[2].areaText()
		return &shellrc.Item{
			Category: m.form.fields[0].text(),
			Name:     name,
			Value:    body,
			Style:    shellrc.StyleFunction,
		}, nil
	}
}

func inferQuote(v string) shellrc.Quote {
	if strings.ContainsAny(v, "$`") {
		return shellrc.QuoteDouble
	}
	return shellrc.QuoteSingle
}

func (m model) View() string {
	switch m.mode {
	case modeForm:
		out := boxStyle.Render(m.form.View())
		if m.status != "" {
			if m.isErr {
				out += "\n" + errStyle.Render(m.status)
			} else {
				out += "\n" + okStyle.Render(m.status)
			}
		}
		return out
	case modeConfirm:
		return m.listView() + "\n\n" + errStyle.Render(m.confirmText()+" (y/n)")
	default:
		return m.listView()
	}
}

func (m model) confirmText() string {
	switch m.pending {
	case pendingDelete:
		if it := m.currentItem(); it != nil {
			return fmt.Sprintf("确定删除 %q ?", it.DisplayName())
		}
		return "确定删除该项 ?"
	case pendingQuit:
		return "有未保存的修改，确定退出 ?"
	case pendingSwitchShell:
		return fmt.Sprintf("有未保存的修改，确定切换到 %s ?", m.pendingShell)
	}
	return "确定 ?"
}

func (m model) listView() string {
	var b strings.Builder
	b.WriteString(m.headerView())
	b.WriteString("\n\n")
	b.WriteString(m.bodyView())
	b.WriteString("\n")
	if m.status != "" {
		if m.isErr {
			b.WriteString(errStyle.Render(m.status))
		} else {
			b.WriteString(okStyle.Render(m.status))
		}
		b.WriteString("\n")
	}
	b.WriteString(helpStyle.Render(m.helpLine()))
	if m.showHelp {
		b.WriteString("\n\n")
		b.WriteString(m.helpView())
	}
	return b.String()
}

func (m model) headerView() string {
	var parts []string
	for _, k := range shellrc.AllKinds() {
		label := k.TabLabel()
		if m.dirty[k] {
			label += "*"
		}
		if k == m.tab {
			parts = append(parts, activeTabStyle.Render(label))
		} else {
			parts = append(parts, inactiveTabStyle.Render(label))
		}
	}
	path := m.file().Path
	mark := ""
	if m.dirty[m.tab] {
		mark = " *"
	}
	return titleStyle.Render("shellman  "+m.store.Shell) + "  " + strings.Join(parts, "") + dimStyle.Render("  "+path+mark)
}

func (m model) bodyView() string {
	rows := m.rows()
	if len(rows) == 0 {
		return dimStyle.Render("暂无条目，按 'a' 新增。") + "\n"
	}
	visible := m.listRows()
	start, end := windowRows(rows, m.cursor, visible)
	nameWidth := nameColWidth(m.items(), m.width)
	var b strings.Builder
	for i := start; i < end; i++ {
		row := rows[i]
		if row.header {
			b.WriteString(catStyle.Render("── " + row.cat + " ──"))
			b.WriteString("\n")
			continue
		}
		it := m.items()[row.index]
		name := padName(it.DisplayName(), nameWidth)
		preview := it.Preview()
		tag := styleTag(it)
		plain := fmt.Sprintf("%s  %s%s", name, preview, tag)
		if row.index == m.cursor {
			b.WriteString(selectedStyle.Render("› " + plain))
		} else {
			b.WriteString("  " + name + "  " + dimStyle.Render(preview) + tag)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func styleTag(it *shellrc.Item) string {
	switch it.Style {
	case shellrc.StyleAssign:
		return dimStyle.Render("  assign")
	case shellrc.StyleSnippet:
		return dimStyle.Render("  snippet")
	default:
		return ""
	}
}

func (m model) listRows() int {
	rows := m.height - 8
	if m.showHelp {
		rows -= 8
	}
	if rows < 3 {
		rows = 3
	}
	return rows
}

func (m model) helpLine() string {
	return "1/2/3 切换 · a 新增 · enter/e 编辑 · d 删除 · s 保存 · r 重载 · o 切换 shell · ? 帮助 · q 退出"
}

func (m model) helpView() string {
	return dimStyle.Render(
		"提示：\n" +
			"  · 文件：~/.{zsh,bash}_{alias,env,functions}，按分类注释分组写出。\n" +
			"  · 修改先留在内存，按 s 只写回当前标签对应文件，并生成 .bak。\n" +
			"  · Env 类型：export / assign / snippet（nvm source、多行脚本等）。\n" +
			"  · 函数体和 snippet 在表单 textarea 中编辑，ctrl+s 应用。\n" +
			"  · 本工具不修改 ~/.zshrc / ~/.bashrc，请自行 source 这些文件。")
}

func windowRows(rows []listRow, cursor, visible int) (int, int) {
	focus := 0
	for i, r := range rows {
		if !r.header && r.index == cursor {
			focus = i
			break
		}
	}
	if visible >= len(rows) {
		return 0, len(rows)
	}
	start := focus - visible/3
	if start < 0 {
		start = 0
	}
	end := start + visible
	if end > len(rows) {
		end = len(rows)
		start = end - visible
		if start < 0 {
			start = 0
		}
	}
	return start, end
}

func nameColWidth(items []*shellrc.Item, total int) int {
	w := 8
	for _, it := range items {
		n := utf8.RuneCountInString(it.DisplayName())
		if n > w {
			w = n
		}
	}
	if w > 28 {
		w = 28
	}
	if total > 0 && w > total/3 {
		w = total / 3
		if w < 8 {
			w = 8
		}
	}
	return w
}

func padName(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		r := []rune(s)
		if len(r) > width {
			return string(r[:width])
		}
		return s
	}
	return s + strings.Repeat(" ", width-n)
}
