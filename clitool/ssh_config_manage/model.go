package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"sshconfig-manage/internal/sshconfig"
)

type mode int

const (
	modeList mode = iota
	modeForm
	modeBulk
	modeTemplateForm
	modeTemplatePick
	modeConfirmDelete
	modeConfirmDeleteTemplate
	modeConfirmQuit
)

var knownKeys = []string{"HostName", "User", "Port", "IdentityFile", "ProxyJump"}

type model struct {
	path  string
	cfg   *sshconfig.Config
	mode  mode
	dirty bool
	msg   string
	isErr bool

	cursor int

	inputs  []textinput.Model
	labels  []string
	extra   textarea.Model
	focus   int
	editing *sshconfig.Host

	bulkInputs []textinput.Model
	bulkLabels []string
	bulkHosts  textarea.Model

	templateInputs  []textinput.Model
	templateLabels  []string
	templateEditing int
	pickCursor      int

	width  int
	height int
}

func newModel(path string, cfg *sshconfig.Config) model {
	m := model{path: path, cfg: cfg, width: 80, height: 24}
	m.extra = textarea.New()
	m.extra.Placeholder = "每行一条，例如：\nForwardAgent yes\nLocalForward 8080 localhost:80"
	m.extra.ShowLineNumbers = false
	m.extra.SetHeight(4)
	m.bulkHosts = textarea.New()
	m.bulkHosts.Placeholder = "每行一台，例如：\nweb-1 10.0.0.1\nweb-2 10.0.0.2 root 22\n10.0.0.3"
	m.bulkHosts.ShowLineNumbers = false
	m.bulkHosts.SetHeight(8)
	return m
}

func (m model) Init() tea.Cmd { return nil }

func (m *model) newInputs() {
	defs := []struct{ label, placeholder string }{
		{"Host (别名)", "myserver"},
		{"HostName", "192.168.1.10"},
		{"User", "root"},
		{"Port", "22"},
		{"IdentityFile", "~/.ssh/id_rsa"},
		{"ProxyJump", "bastion"},
	}
	m.inputs = make([]textinput.Model, len(defs))
	m.labels = make([]string, len(defs))
	for i, d := range defs {
		ti := textinput.New()
		ti.Placeholder = d.placeholder
		ti.Prompt = ""
		ti.CharLimit = 256
		m.inputs[i] = ti
		m.labels[i] = d.label
	}
}

func (m *model) loadHostIntoForm(h *sshconfig.Host) {
	m.newInputs()
	if h != nil {
		vals := []string{h.Name, h.Get("HostName"), h.Get("User"), h.Get("Port"), h.Get("IdentityFile"), h.Get("ProxyJump")}
		for i, v := range vals {
			m.inputs[i].SetValue(v)
		}
		var extraLines []string
		for _, o := range h.Options {
			if !isKnown(o.Key) {
				extraLines = append(extraLines, o.Key+" "+o.Value)
			}
		}
		m.extra.SetValue(strings.Join(extraLines, "\n"))
	} else {
		m.extra.SetValue("")
	}
	m.focus = 0
	m.inputs[0].Focus()
}

func (m *model) loadBulkForm() {
	defs := []struct{ label, placeholder string }{
		{"User", "root"},
		{"Port", "22"},
		{"IdentityFile", "~/.ssh/id_rsa"},
		{"ProxyJump", "bastion"},
	}
	m.bulkInputs = make([]textinput.Model, len(defs))
	m.bulkLabels = make([]string, len(defs))
	for i, d := range defs {
		ti := textinput.New()
		ti.Placeholder = d.placeholder
		ti.Prompt = ""
		ti.CharLimit = 256
		m.bulkInputs[i] = ti
		m.bulkLabels[i] = d.label
	}
	if n := len(m.cfg.Hosts); n > 0 {
		h := m.cfg.Hosts[m.cursor]
		vals := []string{h.Get("User"), h.Get("Port"), h.Get("IdentityFile"), h.Get("ProxyJump")}
		for i, v := range vals {
			m.bulkInputs[i].SetValue(v)
		}
	}
	m.bulkHosts.SetValue("")
	m.focus = 0
	m.bulkInputs[0].Focus()
}

func isKnown(key string) bool {
	for _, k := range knownKeys {
		if strings.EqualFold(k, key) {
			return true
		}
	}
	return false
}

func (m model) totalItems() int {
	return len(m.cfg.Templates) + len(m.cfg.Hosts)
}

func (m model) selectedTemplate() (int, bool) {
	if m.cursor >= 0 && m.cursor < len(m.cfg.Templates) {
		return m.cursor, true
	}
	return 0, false
}

func (m model) selectedHost() (int, bool) {
	index := m.cursor - len(m.cfg.Templates)
	if index >= 0 && index < len(m.cfg.Hosts) {
		return index, true
	}
	return 0, false
}

func hostDetail(h *sshconfig.Host) string {
	detail := h.Get("HostName")
	if u := h.Get("User"); u != "" {
		detail = u + "@" + detail
	}
	if p := h.Get("Port"); p != "" && p != "22" {
		detail += ":" + p
	}
	return detail
}

func (m *model) startNewHost(tpl *sshconfig.Template) {
	m.mode = modeForm
	m.editing = nil
	m.msg = ""
	var h *sshconfig.Host
	if tpl != nil {
		h = &sshconfig.Host{}
		tpl.ApplyTo(h)
	}
	m.loadHostIntoForm(h)
	if tpl != nil {
		m.inputs[0].Placeholder = "留空则使用主机名/IP"
		m.focus = 1
		m.focusHostInputs()
		m.msg = "已套用模板 " + tpl.Name + "，填写 HostName 后按 ctrl+s 保存"
		m.isErr = false
	}
}

func (m *model) focusHostInputs() {
	for i := range m.inputs {
		if i == m.focus {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
}

func (m *model) newTemplateInputs() {
	defs := []struct{ label, placeholder string }{
		{"模板名", "prod"},
		{"User", "root"},
		{"Port", "22"},
		{"IdentityFile", "~/.ssh/id_rsa"},
	}
	m.templateInputs = make([]textinput.Model, len(defs))
	m.templateLabels = make([]string, len(defs))
	for i, d := range defs {
		ti := textinput.New()
		ti.Placeholder = d.placeholder
		ti.Prompt = ""
		ti.CharLimit = 256
		m.templateInputs[i] = ti
		m.templateLabels[i] = d.label
	}
}

func (m *model) loadTemplateIntoForm(index int) {
	m.newTemplateInputs()
	m.templateEditing = index
	if t := m.cfg.TemplateAt(index); t != nil {
		vals := []string{t.Name, t.User, t.Port, t.IdentityFile}
		for i, v := range vals {
			m.templateInputs[i].SetValue(v)
		}
		m.focus = 1
	} else {
		m.templateEditing = -1
		m.focus = 0
	}
	m.focusTemplateInputs()
}

func (m *model) focusTemplateInputs() {
	for i := range m.templateInputs {
		if i == m.focus && !(m.templateEditing >= 0 && i == 0) {
			m.templateInputs[i].Focus()
		} else {
			m.templateInputs[i].Blur()
		}
	}
}

func (m *model) templateFocusIndexes() []int {
	if m.templateEditing >= 0 {
		return []int{1, 2, 3}
	}
	return []int{0, 1, 2, 3}
}

func (m *model) moveTemplateFocus(delta int) {
	indexes := m.templateFocusIndexes()
	for i, idx := range indexes {
		if idx == m.focus {
			next := (i + delta + len(indexes)) % len(indexes)
			m.focus = indexes[next]
			return
		}
	}
	m.focus = indexes[0]
}

func (m *model) startTemplateFromHost(h *sshconfig.Host) {
	if h == nil {
		return
	}
	m.mode = modeTemplateForm
	m.msg = ""
	m.loadTemplateIntoForm(-1)
	m.templateInputs[0].SetValue(sshconfig.UniqueTemplateName(m.cfg.Templates, h.Name))
	m.templateInputs[1].SetValue(h.Get("User"))
	m.templateInputs[2].SetValue(h.Get("Port"))
	m.templateInputs[3].SetValue(h.Get("IdentityFile"))
	m.focus = 0
	m.focusTemplateInputs()
}

func (m *model) submitTemplate() {
	name := strings.TrimSpace(m.templateInputs[0].Value())
	if name == "" {
		m.msg = "模板名不能为空"
		m.isErr = true
		return
	}
	if m.templateEditing < 0 && m.cfg.HasTemplate(name) {
		m.msg = fmt.Sprintf("模板 %q 已存在", name)
		m.isErr = true
		return
	}
	user := strings.TrimSpace(m.templateInputs[1].Value())
	port := strings.TrimSpace(m.templateInputs[2].Value())
	identity := strings.TrimSpace(m.templateInputs[3].Value())
	if m.templateEditing >= 0 && m.templateEditing < len(m.cfg.Templates) {
		t := m.cfg.Templates[m.templateEditing]
		t.User = user
		t.Port = port
		t.IdentityFile = identity
		m.cursor = m.templateEditing
	} else {
		m.cfg.Templates = append(m.cfg.Templates, &sshconfig.Template{Name: name, User: user, Port: port, IdentityFile: identity})
		m.cursor = len(m.cfg.Templates) - 1
	}
	m.dirty = true
	m.mode = modeList
	m.msg = "模板已更新，按 s 保存到文件"
	m.isErr = false
}

func (m *model) buildHostFromForm() *sshconfig.Host {
	name := strings.TrimSpace(m.inputs[0].Value())
	if name == "" {
		name = strings.TrimSpace(m.inputs[1].Value())
	}
	if name == "" {
		name = "unnamed"
	}
	h := &sshconfig.Host{Name: name}
	for i, key := range knownKeys {
		v := strings.TrimSpace(m.inputs[i+1].Value())
		if v != "" {
			h.Options = append(h.Options, sshconfig.Option{Key: key, Value: v})
		}
	}
	for _, line := range strings.Split(m.extra.Value(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		key := fields[0]
		val := strings.TrimSpace(strings.TrimPrefix(line, key))
		h.Options = append(h.Options, sshconfig.Option{Key: key, Value: val})
	}
	return h
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeList:
			return m.updateList(msg)
		case modeForm:
			return m.updateForm(msg)
		case modeBulk:
			return m.updateBulk(msg)
		case modeTemplateForm:
			return m.updateTemplateForm(msg)
		case modeTemplatePick:
			return m.updateTemplatePick(msg)
		case modeConfirmDelete, modeConfirmDeleteTemplate, modeConfirmQuit:
			return m.updateConfirm(msg)
		}
	}
	return m, nil
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := m.totalItems()
	switch msg.String() {
	case "q", "ctrl+c":
		if m.dirty {
			m.mode = modeConfirmQuit
			return m, nil
		}
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < n-1 {
			m.cursor++
		}
	case "g", "home":
		m.cursor = 0
	case "G", "end":
		if n > 0 {
			m.cursor = n - 1
		}
	case "a":
		m.startNewHost(nil)
		return m, textinput.Blink
	case "t":
		if len(m.cfg.Templates) == 0 {
			m.msg = "暂无模板，按 n 新建模板"
			m.isErr = false
			return m, nil
		}
		m.mode = modeTemplatePick
		m.pickCursor = 0
		m.msg = ""
		return m, nil
	case "n":
		m.loadTemplateIntoForm(-1)
		m.mode = modeTemplateForm
		m.msg = ""
		return m, textinput.Blink
	case "T":
		if index, ok := m.selectedHost(); ok {
			m.startTemplateFromHost(m.cfg.Hosts[index])
			return m, textinput.Blink
		}
		m.msg = "请先选择一台主机"
		m.isErr = false
	case "c":
		if index, ok := m.selectedHost(); ok {
			src := m.cfg.Hosts[index]
			dup := src.Clone()
			dup.Name = sshconfig.UniqueHostName(m.cfg.Hosts, src.Name)
			m.mode = modeForm
			m.editing = nil
			m.msg = ""
			m.loadHostIntoForm(dup)
			return m, textinput.Blink
		}
		m.msg = "请先选择一台主机"
		m.isErr = false
	case "A":
		m.mode = modeBulk
		m.msg = ""
		m.loadBulkForm()
		return m, textinput.Blink
	case "enter", "e":
		if index, ok := m.selectedTemplate(); ok {
			m.loadTemplateIntoForm(index)
			m.mode = modeTemplateForm
			m.msg = ""
			return m, textinput.Blink
		}
		if index, ok := m.selectedHost(); ok {
			m.mode = modeForm
			m.editing = m.cfg.Hosts[index]
			m.msg = ""
			m.loadHostIntoForm(m.editing)
			return m, textinput.Blink
		}
	case "d", "delete", "backspace":
		if _, ok := m.selectedTemplate(); ok {
			m.mode = modeConfirmDeleteTemplate
		} else if _, ok := m.selectedHost(); ok {
			m.mode = modeConfirmDelete
		}
	case "s":
		m.save()
	}
	return m, nil
}

func (m model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	confirmMode := m.mode
	if msg.String() == "y" || msg.String() == "Y" {
		switch confirmMode {
		case modeConfirmQuit:
			return m, tea.Quit
		case modeConfirmDeleteTemplate:
			if index, ok := m.selectedTemplate(); ok {
				m.cfg.Templates = append(m.cfg.Templates[:index], m.cfg.Templates[index+1:]...)
				if m.cursor >= m.totalItems() && m.cursor > 0 {
					m.cursor = m.totalItems() - 1
				}
				m.dirty = true
				m.msg = "已删除模板，按 s 保存"
				m.isErr = false
			}
		case modeConfirmDelete:
			if index, ok := m.selectedHost(); ok {
				m.cfg.Hosts = append(m.cfg.Hosts[:index], m.cfg.Hosts[index+1:]...)
				if m.cursor >= m.totalItems() && m.cursor > 0 {
					m.cursor = m.totalItems() - 1
				}
				m.dirty = true
				m.msg = "已删除，按 s 保存"
				m.isErr = false
			}
		}
	}
	m.mode = modeList
	return m, nil
}

func (m model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	total := len(m.inputs) + 1
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode = modeList
		return m, nil
	case "ctrl+s":
		h := m.buildHostFromForm()
		if m.editing == nil && m.cfg.HasHost(h.Name) {
			m.msg = fmt.Sprintf("别名 %q 已存在", h.Name)
			m.isErr = true
			return m, nil
		}
		if m.editing != nil {
			m.editing.Name = h.Name
			m.editing.Options = h.Options
		} else {
			m.cfg.Hosts = append(m.cfg.Hosts, h)
			m.cursor = len(m.cfg.Hosts) - 1
		}
		m.dirty = true
		m.mode = modeList
		m.msg = "已更新，按 s 保存到文件"
		m.isErr = false
		return m, nil
	case "tab", "down":
		m.focus = (m.focus + 1) % total
	case "shift+tab", "up":
		m.focus = (m.focus - 1 + total) % total
	case "enter":
		if m.focus < len(m.inputs) {
			m.focus++
			if m.focus > len(m.inputs)-1 {
				m.focus = len(m.inputs)
			}
		}
	}

	for i := range m.inputs {
		if i == m.focus {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
	var cmd tea.Cmd
	if m.focus < len(m.inputs) {
		m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
	} else {
		m.extra.Focus()
		m.extra, cmd = m.extra.Update(msg)
	}
	return m, cmd
}

func (m model) updateBulk(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	total := len(m.bulkInputs) + 1
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode = modeList
		return m, nil
	case "ctrl+s":
		if err := m.submitBulk(); err != nil {
			m.msg = err.Error()
			m.isErr = true
			return m, nil
		}
		return m, nil
	case "tab", "down":
		m.focus = (m.focus + 1) % total
	case "shift+tab", "up":
		m.focus = (m.focus - 1 + total) % total
	case "enter":
		if m.focus < len(m.bulkInputs) {
			m.focus++
			if m.focus > len(m.bulkInputs)-1 {
				m.focus = len(m.bulkInputs)
			}
		}
	}

	for i := range m.bulkInputs {
		if i == m.focus {
			m.bulkInputs[i].Focus()
		} else {
			m.bulkInputs[i].Blur()
		}
	}
	var cmd tea.Cmd
	if m.focus < len(m.bulkInputs) {
		m.bulkInputs[m.focus], cmd = m.bulkInputs[m.focus].Update(msg)
		m.bulkHosts.Blur()
	} else {
		m.bulkHosts.Focus()
		m.bulkHosts, cmd = m.bulkHosts.Update(msg)
	}
	return m, cmd
}

func (m model) updateTemplateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode = modeList
		return m, nil
	case "ctrl+s":
		m.submitTemplate()
		return m, nil
	case "tab", "down":
		m.moveTemplateFocus(1)
	case "shift+tab", "up":
		m.moveTemplateFocus(-1)
	case "enter":
		if m.focus < len(m.templateInputs)-1 {
			m.moveTemplateFocus(1)
		}
	}

	m.focusTemplateInputs()
	var cmd tea.Cmd
	if m.focus >= 0 && m.focus < len(m.templateInputs) && !(m.templateEditing >= 0 && m.focus == 0) {
		m.templateInputs[m.focus], cmd = m.templateInputs[m.focus].Update(msg)
	}
	return m, cmd
}

func (m model) updateTemplatePick(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	total := len(m.cfg.Templates)
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode = modeList
	case "up", "k":
		if m.pickCursor > 0 {
			m.pickCursor--
		}
	case "down", "j":
		if m.pickCursor < total-1 {
			m.pickCursor++
		}
	case "g", "home":
		m.pickCursor = 0
	case "G", "end":
		if total > 0 {
			m.pickCursor = total - 1
		}
	case "enter":
		if m.pickCursor >= 0 && m.pickCursor < total {
			m.startNewHost(m.cfg.Templates[m.pickCursor])
			return m, textinput.Blink
		}
	}
	return m, nil
}

func (m *model) submitBulk() error {
	entries, err := sshconfig.ParseBulkLines(m.bulkHosts.Value())
	if err != nil {
		return err
	}
	for _, e := range entries {
		if m.cfg.HasHost(e.Alias) {
			return fmt.Errorf("别名 %q 已存在", e.Alias)
		}
	}
	shared := m.bulkSharedOptions()
	for _, e := range entries {
		m.cfg.Hosts = append(m.cfg.Hosts, e.ToHost(shared))
	}
	m.cursor = len(m.cfg.Hosts) - 1
	m.dirty = true
	m.mode = modeList
	m.msg = fmt.Sprintf("已添加 %d 台主机，按 s 保存到文件", len(entries))
	m.isErr = false
	return nil
}

func (m *model) bulkSharedOptions() []sshconfig.Option {
	keys := []string{"User", "Port", "IdentityFile", "ProxyJump"}
	var opts []sshconfig.Option
	for i, key := range keys {
		v := strings.TrimSpace(m.bulkInputs[i].Value())
		if v != "" {
			opts = append(opts, sshconfig.Option{Key: key, Value: v})
		}
	}
	return opts
}

func (m *model) save() {
	if err := m.cfg.SaveFile(m.path); err != nil {
		m.msg = "保存失败: " + err.Error()
		m.isErr = true
		return
	}
	m.dirty = false
	m.msg = "已保存到 " + m.path
	m.isErr = false
}

func (m model) View() string {
	switch m.mode {
	case modeForm:
		return m.formView()
	case modeBulk:
		return m.bulkView()
	case modeTemplateForm:
		return m.templateFormView()
	case modeTemplatePick:
		return m.templatePickView()
	case modeConfirmDelete:
		base := m.listView()
		if index, ok := m.selectedHost(); ok {
			return base + "\n\n" + errStyle.Render(fmt.Sprintf("确定删除 %q ? (y/n)", m.cfg.Hosts[index].Name))
		}
		return base
	case modeConfirmDeleteTemplate:
		base := m.listView()
		if index, ok := m.selectedTemplate(); ok {
			return base + "\n\n" + errStyle.Render(fmt.Sprintf("确定删除模板 %q ? (y/n)", m.cfg.Templates[index].Name))
		}
		return base
	case modeConfirmQuit:
		return m.listView() + "\n\n" + errStyle.Render("有未保存的修改，确定退出 ? (y/n)")
	default:
		return m.listView()
	}
}

type listRow struct {
	header   bool
	template bool
	name     string
	detail   string
	selected bool
}

func (m model) buildListRows() ([]listRow, int) {
	var rows []listRow
	selectedLine := -1
	if len(m.cfg.Templates) > 0 {
		rows = append(rows, listRow{header: true, name: "── 模板 ──"})
		for i, t := range m.cfg.Templates {
			rows = append(rows, listRow{
				template: true,
				name:     t.Name,
				detail:   templateDetail(t),
				selected: m.cursor == i,
			})
			if m.cursor == i {
				selectedLine = len(rows) - 1
			}
		}
	}
	if len(m.cfg.Hosts) > 0 {
		rows = append(rows, listRow{header: true, name: "── 主机 ──"})
		for i, h := range m.cfg.Hosts {
			index := len(m.cfg.Templates) + i
			rows = append(rows, listRow{
				name:     h.Name,
				detail:   hostDetail(h),
				selected: m.cursor == index,
			})
			if m.cursor == index {
				selectedLine = len(rows) - 1
			}
		}
	}
	return rows, selectedLine
}

func (m model) listView() string {
	var b strings.Builder
	title := "SSH Config"
	if m.dirty {
		title += " *"
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString(dimStyle.Render("  " + m.path))
	b.WriteString("\n\n")

	if m.totalItems() == 0 {
		b.WriteString(dimStyle.Render("暂无条目，按 'a' 新增主机，按 'n' 新建模板。"))
		b.WriteString("\n")
	}

	rows, selectedLine := m.buildListRows()
	visible := m.height - 8
	if visible < 3 {
		visible = 3
	}
	start := 0
	if selectedLine >= visible {
		start = selectedLine - visible + 1
	}
	end := start + visible
	if end > len(rows) {
		end = len(rows)
	}
	for i := start; i < end; i++ {
		row := rows[i]
		switch {
		case row.header:
			b.WriteString(catStyle.Render(row.name))
		case row.selected:
			symbol := "●"
			if row.template {
				symbol = "◆"
			}
			b.WriteString(selectedStyle.Render(fmt.Sprintf("› %s %-20s %s", symbol, row.name, row.detail)))
		default:
			symbol := "●"
			if row.template {
				symbol = catStyle.Render("◆")
			}
			b.WriteString("  ")
			b.WriteString(symbol)
			b.WriteString(" ")
			b.WriteString(fmt.Sprintf("%-20s %s", row.name, dimStyle.Render(row.detail)))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if m.msg != "" {
		if m.isErr {
			b.WriteString(errStyle.Render(m.msg))
		} else {
			b.WriteString(okStyle.Render(m.msg))
		}
		b.WriteString("\n")
	}
	b.WriteString(helpStyle.Render("a 新增 · n 新建模板 · t 模板主机 · T 存为模板 · A 批量 · c 复制"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓ 选择 · enter/e 编辑 · d 删除 · s 保存 · q 退出"))
	return b.String()
}

func templateDetail(t *sshconfig.Template) string {
	detail := ""
	if t.User != "" {
		detail = t.User
	}
	if t.Port != "" {
		if detail != "" {
			detail += ":"
		}
		detail += t.Port
	}
	if t.IdentityFile != "" {
		if detail != "" {
			detail += "  "
		}
		detail += t.IdentityFile
	}
	return detail
}

func (m model) templatePickView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("从模板创建主机"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("选择模板后进入编辑界面，User/Port/IdentityFile 自动带入。"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("只需填写 HostName（IP 或主机名），别名留空会自动使用 HostName。"))
	b.WriteString("\n\n")

	rows := make([]string, 0, len(m.cfg.Templates))
	for _, t := range m.cfg.Templates {
		rows = append(rows, fmt.Sprintf("◆ %-20s %s", t.Name, templateDetail(t)))
	}
	if len(rows) == 0 {
		b.WriteString(dimStyle.Render("暂无模板，按 n 新建模板。"))
		b.WriteString("\n")
	}
	visible := m.height - 10
	if visible < 1 {
		visible = 1
	}
	offset := 0
	if m.pickCursor >= visible {
		offset = m.pickCursor - visible + 1
	}
	end := offset + visible
	if end > len(rows) {
		end = len(rows)
	}
	for i := offset; i < end; i++ {
		if i == m.pickCursor {
			b.WriteString(selectedStyle.Render("› " + rows[i]))
		} else {
			b.WriteString("  ")
			b.WriteString(catStyle.Render("◆"))
			b.WriteString(strings.TrimPrefix(rows[i], "◆"))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓ 选择 · enter 使用 · esc 取消"))
	return b.String()
}

func (m model) templateFormView() string {
	var b strings.Builder
	action := "新增模板"
	if m.templateEditing >= 0 {
		action = "编辑模板: " + m.cfg.Templates[m.templateEditing].Name
	}
	b.WriteString(titleStyle.Render(action))
	b.WriteString("\n\n")

	for i, in := range m.templateInputs {
		label := labelStyle
		if i == m.focus {
			label = focusedLabel
		}
		b.WriteString(label.Render(m.templateLabels[i]))
		if i == 0 && m.templateEditing >= 0 {
			b.WriteString(dimStyle.Render(in.Value()))
		} else {
			b.WriteString(in.View())
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if m.msg != "" && m.isErr {
		b.WriteString(errStyle.Render(m.msg))
		b.WriteString("\n")
	}
	b.WriteString(helpStyle.Render("tab/shift+tab 切换 · ctrl+s 确认 · esc 取消"))
	return b.String()
}

func (m model) formView() string {
	var b strings.Builder
	action := "新增主机"
	if m.editing != nil {
		action = "编辑主机: " + m.editing.Name
	}
	b.WriteString(titleStyle.Render(action))
	b.WriteString("\n\n")

	for i, in := range m.inputs {
		label := labelStyle
		if i == m.focus {
			label = focusedLabel
		}
		b.WriteString(label.Render(m.labels[i]))
		b.WriteString(in.View())
		b.WriteString("\n")
	}

	label := labelStyle
	if m.focus >= len(m.inputs) {
		label = focusedLabel
	}
	b.WriteString("\n")
	b.WriteString(label.Render("其他选项"))
	b.WriteString("\n")
	b.WriteString(m.extra.View())
	b.WriteString("\n")
	if m.msg != "" {
		if m.isErr {
			b.WriteString(errStyle.Render(m.msg))
		} else {
			b.WriteString(okStyle.Render(m.msg))
		}
		b.WriteString("\n")
	}
	b.WriteString(helpStyle.Render("tab/shift+tab 切换 · ctrl+s 确认 · esc 取消"))
	return b.String()
}

func (m model) bulkView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("批量新增主机"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("公共选项应用到每一台；主机行可覆盖 User / Port。"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("每行格式：别名 [主机 [用户 [端口]]]，仅填一项时别名与 HostName 相同。"))
	b.WriteString("\n\n")

	for i, in := range m.bulkInputs {
		label := labelStyle
		if i == m.focus {
			label = focusedLabel
		}
		b.WriteString(label.Render(m.bulkLabels[i]))
		b.WriteString(in.View())
		b.WriteString("\n")
	}

	label := labelStyle
	if m.focus >= len(m.bulkInputs) {
		label = focusedLabel
	}
	b.WriteString("\n")
	b.WriteString(label.Render("主机列表"))
	b.WriteString("\n")
	b.WriteString(m.bulkHosts.View())
	b.WriteString("\n")
	if m.msg != "" && m.isErr {
		b.WriteString(errStyle.Render(m.msg))
		b.WriteString("\n")
	}
	b.WriteString(helpStyle.Render("tab/shift+tab 切换 · ctrl+s 确认 · esc 取消"))
	return b.String()
}
