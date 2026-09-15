package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"sshconfig-manage/internal/sshconfig"
)

type mode int

const (
	modeList mode = iota
	modeForm
	modeConfirmDelete
)

var knownKeys = []string{"HostName", "User", "Port", "IdentityFile", "ProxyJump"}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	labelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Width(14)
	focusedLabel  = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Width(14)
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	okStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("76"))
)

type model struct {
	path  string
	cfg   *sshconfig.Config
	mode  mode
	dirty bool
	msg   string
	isErr bool

	cursor int
	offset int

	inputs  []textinput.Model
	labels  []string
	extra   textarea.Model
	focus   int
	editing *sshconfig.Host

	width  int
	height int
}

func newModel(path string, cfg *sshconfig.Config) model {
	m := model{path: path, cfg: cfg, width: 80, height: 24}
	m.extra = textarea.New()
	m.extra.Placeholder = "每行一条，例如：\nForwardAgent yes\nLocalForward 8080 localhost:80"
	m.extra.ShowLineNumbers = false
	m.extra.SetHeight(4)
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

func isKnown(key string) bool {
	for _, k := range knownKeys {
		if strings.EqualFold(k, key) {
			return true
		}
	}
	return false
}

func (m *model) buildHostFromForm() *sshconfig.Host {
	name := strings.TrimSpace(m.inputs[0].Value())
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
		case modeConfirmDelete:
			return m.updateConfirm(msg)
		}
	}
	return m, nil
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(m.cfg.Hosts)
	switch msg.String() {
	case "q", "ctrl+c":
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
		m.mode = modeForm
		m.editing = nil
		m.msg = ""
		m.loadHostIntoForm(nil)
		return m, textinput.Blink
	case "enter", "e":
		if n > 0 {
			m.mode = modeForm
			m.editing = m.cfg.Hosts[m.cursor]
			m.msg = ""
			m.loadHostIntoForm(m.editing)
			return m, textinput.Blink
		}
	case "d", "delete", "backspace":
		if n > 0 {
			m.mode = modeConfirmDelete
		}
	case "s":
		m.save()
	}
	return m, nil
}

func (m model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if m.cursor < len(m.cfg.Hosts) {
			m.cfg.Hosts = append(m.cfg.Hosts[:m.cursor], m.cfg.Hosts[m.cursor+1:]...)
			if m.cursor >= len(m.cfg.Hosts) && m.cursor > 0 {
				m.cursor--
			}
			m.dirty = true
			m.msg = "已删除，按 s 保存"
			m.isErr = false
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
	case modeConfirmDelete:
		return m.listView() + "\n\n" + errStyle.Render(fmt.Sprintf("确定删除 %q ? (y/n)", m.cfg.Hosts[m.cursor].Name))
	default:
		return m.listView()
	}
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

	if len(m.cfg.Hosts) == 0 {
		b.WriteString(dimStyle.Render("暂无主机，按 'a' 添加。"))
		b.WriteString("\n")
	}

	rows := m.height - 6
	if rows < 1 {
		rows = 1
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
	end := m.offset + rows
	if end > len(m.cfg.Hosts) {
		end = len(m.cfg.Hosts)
	}

	for i := m.offset; i < end; i++ {
		h := m.cfg.Hosts[i]
		detail := h.Get("HostName")
		if u := h.Get("User"); u != "" {
			detail = u + "@" + detail
		}
		if p := h.Get("Port"); p != "" && p != "22" {
			detail += ":" + p
		}
		plain := fmt.Sprintf("%-24s %s", h.Name, detail)
		if i == m.cursor {
			b.WriteString(selectedStyle.Render("› " + plain))
		} else {
			b.WriteString("  ")
			b.WriteString(fmt.Sprintf("%-24s %s", h.Name, dimStyle.Render(detail)))
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
	b.WriteString(helpStyle.Render("a 新增 · enter/e 编辑 · d 删除 · s 保存 · q 退出"))
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

	b.WriteString(helpStyle.Render("tab/shift+tab 切换 · ctrl+s 确认 · esc 取消"))
	return b.String()
}
