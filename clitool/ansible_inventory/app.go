package main

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"ansiman/internal/inventory"
)

type tabKind int

const (
	tabHosts tabKind = iota
	tabGroups
	tabVars
	tabCount
)

func (t tabKind) label() string {
	switch t {
	case tabHosts:
		return "主机"
	case tabGroups:
		return "组"
	default:
		return "变量"
	}
}

type uiMode int

const (
	modeList uiMode = iota
	modeForm
	modeConfirm
	modeSource
)

type pendingKind int

const (
	pendingNone pendingKind = iota
	pendingDelete
	pendingDeleteProject
	pendingQuit
	pendingSwitch
)

type formKind int

const (
	formHost formKind = iota
	formGroup
	formVar
	formDefault
	formProject
)

type sourcePane int

const (
	paneConfig sourcePane = iota
	paneProjects
)

type listRow struct {
	header bool
	cat    string
	index  int
}

type model struct {
	inv   *inventory.Inventory
	prefs *inventory.Prefs

	tab    tabKind
	mode   uiMode
	cursor int

	form     form
	formKind formKind
	edit     bool

	pending     pendingKind
	pendingPath string

	sourcePane sourcePane
	sourceCur  int
	editName   string

	status   string
	isErr    bool
	showHelp bool

	width  int
	height int
}

func newModel(_ string, inv *inventory.Inventory) model {
	prefs, err := inventory.LoadPrefs()
	if err != nil || prefs == nil {
		prefs = &inventory.Prefs{Dir: inventory.ConfigDir(), DefaultInventory: inventory.DetectAnsibleDefault()}
	}
	return model{
		inv:    inv,
		prefs:  prefs,
		width:  100,
		height: 30,
	}
}

func (m *model) setOK(msg string)  { m.status, m.isErr = msg, false }
func (m *model) setErr(msg string) { m.status, m.isErr = msg, true }

func (m model) Init() tea.Cmd { return textinput.Blink }

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
		case modeSource:
			return m.updateSource(msg)
		default:
			return m.updateList(msg)
		}
	}
	return m, nil
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		if m.inv.Dirty {
			m.pending = pendingQuit
			m.mode = modeConfirm
			return m, nil
		}
		return m, tea.Quit
	case "1":
		m.tab = tabHosts
		m.clampCursor()
	case "2":
		m.tab = tabGroups
		m.clampCursor()
	case "3":
		m.tab = tabVars
		m.clampCursor()
	case "tab":
		m.tab = (m.tab + 1) % tabCount
		m.clampCursor()
	case "shift+tab":
		m.tab = (m.tab + tabCount - 1) % tabCount
		m.clampCursor()
	case "?":
		m.showHelp = !m.showHelp
	case "o":
		m.openSourcePicker()
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
		if n := m.listLen(); n > 0 {
			m.cursor = n - 1
		}
	case "a":
		m.startAdd()
		return m, textinput.Blink
	case "enter", "e":
		m.startEdit()
		return m, textinput.Blink
	case "d", "delete", "backspace":
		if m.listLen() > 0 {
			m.pending = pendingDelete
			m.mode = modeConfirm
		}
	}
	return m, nil
}

func (m *model) listLen() int {
	switch m.tab {
	case tabHosts:
		return len(m.inv.HostRefs())
	case tabGroups:
		return len(m.inv.Groups)
	default:
		return len(m.inv.VarRefs())
	}
}

func (m *model) clampCursor() {
	n := m.listLen()
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
	n := m.listLen()
	if n == 0 {
		m.cursor = 0
		return
	}
	m.cursor += delta
	m.clampCursor()
}

func (m *model) save() {
	if !m.inv.Dirty {
		m.setOK("没有需要保存的修改")
		return
	}
	if err := m.inv.Save(); err != nil {
		m.setErr("保存失败: " + err.Error())
		return
	}
	m.setOK("已保存到 " + m.inv.Path)
}

func (m *model) reload() {
	if m.inv.Dirty {
		m.setErr("有未保存的修改，请先保存或按 o 切换来源")
		return
	}
	if err := m.inv.Reload(); err != nil {
		m.setErr("重载失败: " + err.Error())
		return
	}
	m.clampCursor()
	m.setOK("已从文件重载")
}

func (m *model) openSourcePicker() {
	prefs, err := inventory.LoadPrefs()
	if err != nil {
		m.setErr("读取配置失败: " + err.Error())
		return
	}
	m.prefs = prefs
	m.sourcePane = paneConfig
	m.sourceCur = 0
	if m.inv.Path != m.defaultPath() {
		for i, p := range m.prefs.Projects {
			if p.Path == m.inv.Path {
				m.sourcePane = paneProjects
				m.sourceCur = i
				break
			}
		}
	}
	m.mode = modeSource
}

func (m *model) defaultPath() string {
	if m.prefs != nil && m.prefs.DefaultInventory != "" {
		return m.prefs.DefaultInventory
	}
	return inventory.DetectAnsibleDefault()
}

func (m model) updateSource(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "o":
		m.mode = modeList
	case "tab", "right", "l":
		m.sourcePane = paneProjects
		m.clampSourceCursor()
	case "shift+tab", "left", "h":
		m.sourcePane = paneConfig
		m.sourceCur = 0
	case "up", "k":
		if m.sourcePane == paneProjects && m.sourceCur > 0 {
			m.sourceCur--
		}
	case "down", "j":
		if m.sourcePane == paneProjects && m.sourceCur < len(m.prefs.Projects)-1 {
			m.sourceCur++
		}
	case "g", "home":
		m.sourceCur = 0
	case "G", "end":
		if m.sourcePane == paneProjects && len(m.prefs.Projects) > 0 {
			m.sourceCur = len(m.prefs.Projects) - 1
		}
	case "q", "ctrl+c":
		if m.inv.Dirty {
			m.pending = pendingQuit
			m.mode = modeConfirm
			return m, nil
		}
		return m, tea.Quit
	case "a":
		m.sourcePane = paneProjects
		m.startAddProject()
		return m, textinput.Blink
	case "e":
		return m.startEditSource()
	case "d", "delete", "backspace":
		if m.sourcePane == paneProjects && m.currentProject() != nil {
			m.pending = pendingDeleteProject
			m.mode = modeConfirm
		}
	case "enter":
		m.openSelectedSource()
	}
	return m, nil
}

func (m *model) clampSourceCursor() {
	n := len(m.prefs.Projects)
	if n == 0 {
		m.sourceCur = 0
		return
	}
	if m.sourceCur < 0 {
		m.sourceCur = 0
	}
	if m.sourceCur >= n {
		m.sourceCur = n - 1
	}
}

func (m *model) currentProject() *inventory.Project {
	if m.prefs == nil || m.sourceCur < 0 || m.sourceCur >= len(m.prefs.Projects) {
		return nil
	}
	return &m.prefs.Projects[m.sourceCur]
}

func (m *model) openSelectedSource() {
	path := m.defaultPath()
	if m.sourcePane == paneProjects {
		p := m.currentProject()
		if p == nil {
			return
		}
		path = p.Path
	}
	if path == m.inv.Path {
		m.mode = modeList
		return
	}
	if m.inv.Dirty {
		m.pending = pendingSwitch
		m.pendingPath = path
		m.mode = modeConfirm
		return
	}
	m.switchTo(path)
}

func (m *model) startAddProject() {
	m.edit = false
	m.formKind = formProject
	m.editName = ""
	path := ""
	if p := inventory.DetectProject(); p != "" && m.prefs.ProjectByPath(p) == nil {
		path = p
	}
	m.buildProjectForm("", path)
	m.mode = modeForm
	m.status = ""
}

func (m model) startEditSource() (tea.Model, tea.Cmd) {
	if m.sourcePane == paneConfig {
		m.edit = true
		m.formKind = formDefault
		m.buildDefaultForm()
		m.mode = modeForm
		m.status = ""
		return m, textinput.Blink
	}
	p := m.currentProject()
	if p == nil {
		return m, nil
	}
	m.edit = true
	m.formKind = formProject
	m.editName = p.Name
	m.buildProjectForm(p.Name, p.Path)
	m.mode = modeForm
	m.status = ""
	return m, textinput.Blink
}

func (m *model) switchTo(path string) {
	inv, err := inventory.Load(path)
	if err != nil {
		m.setErr("切换失败: " + err.Error())
		m.mode = modeList
		return
	}
	m.inv = inv
	m.cursor = 0
	m.mode = modeList
	m.setOK("已切换到 " + path)
}

func (m model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	kind := m.pending
	if msg.String() == "y" || msg.String() == "Y" {
		switch kind {
		case pendingDelete:
			m.deleteCurrent()
		case pendingQuit:
			return m, tea.Quit
		case pendingSwitch:
			m.switchTo(m.pendingPath)
			m.pending = pendingNone
			m.pendingPath = ""
			return m, nil
		case pendingDeleteProject:
			m.deleteProject()
			m.pending = pendingNone
			m.pendingPath = ""
			m.mode = modeSource
			return m, nil
		}
	}
	m.pending = pendingNone
	m.pendingPath = ""
	if kind == pendingDeleteProject || kind == pendingSwitch {
		m.mode = modeSource
		return m, nil
	}
	m.mode = modeList
	return m, nil
}

func (m *model) deleteProject() {
	p := m.currentProject()
	if p == nil {
		return
	}
	name := p.Name
	if err := m.prefs.DeleteProject(name); err != nil {
		m.setErr(err.Error())
		return
	}
	m.clampSourceCursor()
	m.setOK("已删除项目 " + name)
}

func (m *model) deleteCurrent() {
	switch m.tab {
	case tabHosts:
		refs := m.inv.HostRefs()
		if m.cursor < 0 || m.cursor >= len(refs) {
			return
		}
		_, h := m.inv.HostAt(refs[m.cursor])
		name := ""
		if h != nil {
			name = h.Name
		}
		if err := m.inv.DeleteHost(refs[m.cursor]); err != nil {
			m.setErr(err.Error())
			return
		}
		m.clampCursor()
		m.setOK("已删除 " + name + "，按 s 保存")
	case tabGroups:
		if m.cursor < 0 || m.cursor >= len(m.inv.Groups) {
			return
		}
		name := m.inv.Groups[m.cursor].Name
		if err := m.inv.DeleteGroup(name); err != nil {
			m.setErr(err.Error())
			return
		}
		m.clampCursor()
		m.setOK("已删除组 " + name + "，按 s 保存")
	case tabVars:
		refs := m.inv.VarRefs()
		if m.cursor < 0 || m.cursor >= len(refs) {
			return
		}
		_, kv := m.inv.VarAt(refs[m.cursor])
		if err := m.inv.DeleteVar(refs[m.cursor]); err != nil {
			m.setErr(err.Error())
			return
		}
		m.clampCursor()
		m.setOK("已删除变量 " + kv.Key + "，按 s 保存")
	}
}

func (m *model) startAdd() {
	m.edit = false
	switch m.tab {
	case tabHosts:
		m.formKind = formHost
		m.buildHostForm(nil, m.defaultGroup())
	case tabGroups:
		m.formKind = formGroup
		m.buildGroupForm(nil)
	default:
		m.formKind = formVar
		m.buildVarForm("", "", m.defaultGroup())
	}
	m.mode = modeForm
	m.status = ""
}

func (m *model) startEdit() {
	switch m.tab {
	case tabHosts:
		refs := m.inv.HostRefs()
		if m.cursor < 0 || m.cursor >= len(refs) {
			return
		}
		g, h := m.inv.HostAt(refs[m.cursor])
		if h == nil {
			return
		}
		m.edit = true
		m.formKind = formHost
		m.buildHostForm(h, g.Name)
	case tabGroups:
		if m.cursor < 0 || m.cursor >= len(m.inv.Groups) {
			return
		}
		m.edit = true
		m.formKind = formGroup
		m.buildGroupForm(m.inv.Groups[m.cursor])
	default:
		refs := m.inv.VarRefs()
		if m.cursor < 0 || m.cursor >= len(refs) {
			return
		}
		g, kv := m.inv.VarAt(refs[m.cursor])
		if g == nil || kv.Key == "" {
			return
		}
		m.edit = true
		m.formKind = formVar
		m.buildVarForm(kv.Key, kv.Value, g.Name)
	}
	m.mode = modeForm
	m.status = ""
}

func (m *model) defaultGroup() string {
	switch m.tab {
	case tabHosts:
		refs := m.inv.HostRefs()
		if m.cursor >= 0 && m.cursor < len(refs) {
			if g, _ := m.inv.HostAt(refs[m.cursor]); g != nil {
				return g.Name
			}
		}
	case tabGroups:
		if m.cursor >= 0 && m.cursor < len(m.inv.Groups) {
			return m.inv.Groups[m.cursor].Name
		}
	case tabVars:
		refs := m.inv.VarRefs()
		if m.cursor >= 0 && m.cursor < len(refs) {
			if g, _ := m.inv.VarAt(refs[m.cursor]); g != nil {
				return g.Name
			}
		}
	}
	if len(m.inv.Groups) > 0 {
		return m.inv.Groups[0].Name
	}
	return inventory.GroupUngrouped
}

const (
	hostFieldGroup = iota
	hostFieldName
	hostFieldAddr
	hostFieldUser
	hostFieldPort
	hostFieldConn
	hostFieldVars
)

func (m *model) buildHostForm(h *inventory.Host, group string) {
	title := "新增主机"
	name := newText("名称", "web1")
	grp := newText("组", "web")
	addr := newText("ansible_host", "10.0.0.1")
	user := newText("ansible_user", "ubuntu")
	port := newText("ansible_port", "22")
	conn := newText("connection", "ssh")
	extra := newArea("额外变量", "每行 KEY=value", 6)
	grp.setText(group)
	if h != nil {
		title = "编辑主机: " + h.Name
		name.setText(h.Name)
		name.readOnly = true
		grp.readOnly = true
		addr.setText(h.Get("ansible_host"))
		user.setText(h.Get("ansible_user"))
		port.setText(h.Get("ansible_port"))
		conn.setText(h.Get("ansible_connection"))
		extra.setArea(kvToLines(h.ExtraVars()))
	}
	f := form{title: title}
	f.add(grp, name, addr, user, port, conn, extra)
	f.focusFirst()
	if h != nil {
		f.focus = hostFieldAddr
		f.applyFocus()
	}
	m.form = f
}

func (m *model) buildGroupForm(g *inventory.Group) {
	title := "新增组"
	name := newText("名称", "web")
	children := newArea("子组", "每行一个组名", 6)
	if g != nil {
		title = "编辑组: " + g.Name
		name.setText(g.Name)
		name.readOnly = true
		children.setArea(strings.Join(g.Children, "\n"))
	}
	f := form{title: title}
	f.add(name, children)
	f.focusFirst()
	m.form = f
}

func (m *model) buildVarForm(key, value, group string) {
	title := "新增变量"
	grp := newText("组", "web")
	k := newText("名称", "http_port")
	v := newText("值", "80")
	grp.setText(group)
	if key != "" {
		title = "编辑变量: " + key
		k.setText(key)
		k.readOnly = true
		grp.readOnly = true
		v.setText(value)
	}
	f := form{title: title}
	f.add(grp, k, v)
	f.focusFirst()
	if key != "" {
		f.focus = 2
		f.applyFocus()
	}
	m.form = f
}

func (m *model) buildDefaultForm() {
	path := newText("路径", "/etc/ansible/hosts")
	path.setText(m.defaultPath())
	f := form{title: "编辑默认清单"}
	f.add(path)
	f.focusFirst()
	m.form = f
}

func (m *model) buildProjectForm(name, path string) {
	title := "新增项目"
	nf := newText("名称", "prod")
	pf := newText("路径", "./inventory.yml")
	if name != "" {
		title = "编辑项目: " + name
		nf.setText(name)
	}
	if path != "" {
		pf.setText(path)
	}
	f := form{title: title}
	f.add(nf, pf)
	f.focusFirst()
	m.form = f
}

func (m *model) submitDefault() error {
	path := m.form.fields[0].text()
	if err := m.prefs.SetDefaultInventory(path); err != nil {
		return err
	}
	m.setOK("已保存默认清单")
	return nil
}

func (m *model) submitProject() error {
	name := m.form.fields[0].text()
	path := m.form.fields[1].text()
	if m.edit {
		if err := m.prefs.UpdateProject(m.editName, name, path); err != nil {
			return err
		}
		for i, p := range m.prefs.Projects {
			if p.Name == name {
				m.sourceCur = i
				break
			}
		}
		m.setOK("已更新项目 " + name)
		return nil
	}
	if err := m.prefs.AddProject(name, path); err != nil {
		return err
	}
	m.sourceCur = len(m.prefs.Projects) - 1
	m.sourcePane = paneProjects
	m.setOK("已添加项目 " + name)
	return nil
}

func (m model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		if m.formKind == formDefault || m.formKind == formProject {
			m.mode = modeSource
		} else {
			m.mode = modeList
		}
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
		if m.formKind == formDefault || m.formKind == formProject {
			m.mode = modeSource
			return m, nil
		}
		m.mode = modeList
		m.setOK("已更新，按 s 保存到文件")
		return m, nil
	}
	return m, m.form.updateFocused(msg)
}

func (m *model) submitForm() error {
	switch m.formKind {
	case formHost:
		return m.submitHost()
	case formGroup:
		return m.submitGroup()
	case formDefault:
		return m.submitDefault()
	case formProject:
		return m.submitProject()
	default:
		return m.submitVar()
	}
}

func (m *model) submitHost() error {
	group := m.form.fields[hostFieldGroup].text()
	name := m.form.fields[hostFieldName].text()
	if name == "" {
		return fmt.Errorf("主机名不能为空")
	}
	vars := []inventory.KV{}
	set := func(k, v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		vars = append(vars, inventory.KV{Key: k, Value: v})
	}
	set("ansible_host", m.form.fields[hostFieldAddr].text())
	set("ansible_user", m.form.fields[hostFieldUser].text())
	set("ansible_port", m.form.fields[hostFieldPort].text())
	set("ansible_connection", m.form.fields[hostFieldConn].text())
	vars = append(vars, parseKV(m.form.fields[hostFieldVars].areaText())...)
	if m.edit {
		refs := m.inv.HostRefs()
		if m.cursor < 0 || m.cursor >= len(refs) {
			return fmt.Errorf("主机不存在")
		}
		return m.inv.UpdateHost(refs[m.cursor], vars)
	}
	if err := m.inv.AddHost(group, name, vars); err != nil {
		return err
	}
	m.cursor = len(m.inv.HostRefs()) - 1
	return nil
}

func (m *model) submitGroup() error {
	name := m.form.fields[0].text()
	if name == "" {
		return fmt.Errorf("组名不能为空")
	}
	children := parseLines(m.form.fields[1].areaText())
	if m.edit {
		return m.inv.UpdateGroup(name, children)
	}
	if err := m.inv.AddGroup(name, children); err != nil {
		return err
	}
	m.cursor = len(m.inv.Groups) - 1
	return nil
}

func (m *model) submitVar() error {
	group := m.form.fields[0].text()
	key := m.form.fields[1].text()
	value := m.form.fields[2].input.Value()
	if key == "" {
		return fmt.Errorf("变量名不能为空")
	}
	if m.edit {
		refs := m.inv.VarRefs()
		if m.cursor < 0 || m.cursor >= len(refs) {
			return fmt.Errorf("变量不存在")
		}
		return m.inv.UpdateVar(refs[m.cursor], value)
	}
	if err := m.inv.AddVar(group, key, value); err != nil {
		return err
	}
	m.cursor = len(m.inv.VarRefs()) - 1
	return nil
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
		base := m.listView()
		if m.pending == pendingDeleteProject || m.pending == pendingSwitch {
			base = m.sourceView()
		}
		return base + "\n\n" + errStyle.Render(m.confirmText()+" (y/n)")
	case modeSource:
		return m.sourceView()
	default:
		return m.listView()
	}
}

func (m model) confirmText() string {
	switch m.pending {
	case pendingDelete:
		switch m.tab {
		case tabHosts:
			refs := m.inv.HostRefs()
			if m.cursor >= 0 && m.cursor < len(refs) {
				if _, h := m.inv.HostAt(refs[m.cursor]); h != nil {
					return fmt.Sprintf("确定删除 %q ?", h.Name)
				}
			}
			return "确定删除该项 ?"
		case tabGroups:
			if m.cursor >= 0 && m.cursor < len(m.inv.Groups) {
				return fmt.Sprintf("确定删除 %q ?", m.inv.Groups[m.cursor].Name)
			}
			return "确定删除该项 ?"
		default:
			refs := m.inv.VarRefs()
			if m.cursor >= 0 && m.cursor < len(refs) {
				if _, kv := m.inv.VarAt(refs[m.cursor]); kv.Key != "" {
					return fmt.Sprintf("确定删除 %q ?", kv.Key)
				}
			}
			return "确定删除该项 ?"
		}
	case pendingDeleteProject:
		if p := m.currentProject(); p != nil {
			return fmt.Sprintf("确定删除项目 %q ?", p.Name)
		}
		return "确定删除该项目 ?"
	case pendingQuit:
		return "有未保存的修改，确定退出 ?"
	case pendingSwitch:
		return "有未保存的修改，确定切换清单 ?"
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
	for t := tabHosts; t < tabCount; t++ {
		label := t.label()
		if t == m.tab {
			parts = append(parts, activeTabStyle.Render(label))
		} else {
			parts = append(parts, inactiveTabStyle.Render(label))
		}
	}
	mark := ""
	if m.inv.Dirty {
		mark = " *"
	}
	scope := "默认"
	if p := m.prefs.ProjectByPath(m.inv.Path); p != nil {
		scope = p.Name
	} else if m.inv.Path != m.defaultPath() {
		scope = "其他"
	}
	return titleStyle.Render("ansiman") + "  " + strings.Join(parts, "") +
		dimStyle.Render("  "+scope+" · "+m.inv.Format.String()+"  "+m.inv.Path+mark)
}

func (m model) bodyView() string {
	switch m.tab {
	case tabHosts:
		return m.hostsBody()
	case tabGroups:
		return m.groupsBody()
	default:
		return m.varsBody()
	}
}

func (m model) hostsBody() string {
	refs := m.inv.HostRefs()
	if len(refs) == 0 {
		return dimStyle.Render("暂无条目，按 'a' 新增。") + "\n"
	}
	var rows []listRow
	last := ""
	for i, ref := range refs {
		g, _ := m.inv.HostAt(ref)
		cat := ""
		if g != nil {
			cat = g.Name
		}
		if cat != last {
			rows = append(rows, listRow{header: true, cat: cat, index: -1})
			last = cat
		}
		rows = append(rows, listRow{cat: cat, index: i})
	}
	visible := m.listRows()
	start, end := windowRows(rows, m.cursor, visible)
	names := make([]string, 0, len(refs))
	for _, ref := range refs {
		if _, h := m.inv.HostAt(ref); h != nil {
			names = append(names, h.Name)
		}
	}
	nameWidth := nameColWidth(names, m.width)
	var b strings.Builder
	for i := start; i < end; i++ {
		r := rows[i]
		if r.header {
			b.WriteString(catStyle.Render("── " + r.cat + " ──"))
			b.WriteString("\n")
			continue
		}
		_, h := m.inv.HostAt(refs[r.index])
		if h == nil {
			continue
		}
		name := padName(h.Name, nameWidth)
		preview := h.Summary()
		plain := name + "  " + preview
		if r.index == m.cursor {
			b.WriteString(selectedStyle.Render("› " + plain))
		} else {
			b.WriteString("  " + name + "  " + dimStyle.Render(preview))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (m model) groupsBody() string {
	if len(m.inv.Groups) == 0 {
		return dimStyle.Render("暂无条目，按 'a' 新增。") + "\n"
	}
	start, end := window(len(m.inv.Groups), m.cursor, m.listRows())
	names := make([]string, 0, len(m.inv.Groups))
	for _, g := range m.inv.Groups {
		names = append(names, g.Name)
	}
	nameWidth := nameColWidth(names, m.width)
	var b strings.Builder
	for i := start; i < end; i++ {
		g := m.inv.Groups[i]
		name := padName(g.Name, nameWidth)
		detail := fmt.Sprintf("%d 主机", len(g.Hosts))
		if len(g.Vars) > 0 {
			detail += fmt.Sprintf(" · %d 变量", len(g.Vars))
		}
		if len(g.Children) > 0 {
			detail += " · children: " + strings.Join(g.Children, ",")
		}
		plain := name + "  " + detail
		if i == m.cursor {
			b.WriteString(selectedStyle.Render("› " + plain))
		} else {
			b.WriteString("  " + name + "  " + dimStyle.Render(detail))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (m model) varsBody() string {
	refs := m.inv.VarRefs()
	if len(refs) == 0 {
		return dimStyle.Render("暂无条目，按 'a' 新增。") + "\n"
	}
	var rows []listRow
	last := ""
	for i, ref := range refs {
		g, _ := m.inv.VarAt(ref)
		cat := ""
		if g != nil {
			cat = g.Name
		}
		if cat != last {
			rows = append(rows, listRow{header: true, cat: cat, index: -1})
			last = cat
		}
		rows = append(rows, listRow{cat: cat, index: i})
	}
	visible := m.listRows()
	start, end := windowRows(rows, m.cursor, visible)
	keys := make([]string, 0, len(refs))
	for _, ref := range refs {
		if _, kv := m.inv.VarAt(ref); kv.Key != "" {
			keys = append(keys, kv.Key)
		}
	}
	nameWidth := nameColWidth(keys, m.width)
	var b strings.Builder
	for i := start; i < end; i++ {
		r := rows[i]
		if r.header {
			b.WriteString(catStyle.Render("── " + r.cat + " ──"))
			b.WriteString("\n")
			continue
		}
		_, kv := m.inv.VarAt(refs[r.index])
		name := padName(kv.Key, nameWidth)
		plain := name + "  " + kv.Value
		if r.index == m.cursor {
			b.WriteString(selectedStyle.Render("› " + plain))
		} else {
			b.WriteString("  " + name + "  " + dimStyle.Render(kv.Value))
		}
		b.WriteString("\n")
	}
	return b.String()
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
	return "1/2/3 切换 · a 新增 · enter/e 编辑 · d 删除 · s 保存 · r 重载 · o 切换来源 · ? 帮助 · q 退出"
}

func (m model) helpView() string {
	return dimStyle.Render(
		"提示：\n" +
			"  · 配置目录 ~/.config/ansiman/：config 存默认清单路径，projects 存项目列表。\n" +
			"  · 首次未配置时，默认路径回退 ANSIBLE_INVENTORY / ansible.cfg / /etc/ansible/hosts。\n" +
			"  · 按 o 打开来源页：左侧改默认路径，右侧管理项目列表（与配置文件分开）。\n" +
			"  · 支持 INI 与 YAML；按扩展名或内容判断，写回保持原格式。\n" +
			"  · 修改先留在内存，按 s 写回清单文件并生成 .bak。\n" +
			"  · 主机名、组名、变量名编辑时只读（改名请删除后新增）。\n" +
			"  · 删除组时，组内主机移到 ungrouped。")
}

func (m model) sourceView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("清单来源"))
	b.WriteString(dimStyle.Render("  " + inventory.ConfigDir()))
	b.WriteString("\n\n")
	cfg := m.configPane()
	proj := m.projectsPane()
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, cfg, "  ", proj))
	b.WriteString("\n\n")
	if m.status != "" {
		if m.isErr {
			b.WriteString(errStyle.Render(m.status))
		} else {
			b.WriteString(okStyle.Render(m.status))
		}
		b.WriteString("\n")
	}
	b.WriteString(helpStyle.Render("tab 切框 · enter 打开 · e 编辑 · a 新增项目 · d 删除项目 · esc 返回"))
	return b.String()
}

func (m model) configPane() string {
	path := m.defaultPath()
	cur := ""
	if path == m.inv.Path {
		cur = "  (当前)"
	}
	line := path + cur
	if m.sourcePane == paneConfig {
		line = selectedStyle.Render("› " + line)
	} else {
		line = "  " + dimStyle.Render(line)
	}
	body := titleStyle.Render("默认清单") + "\n" + dimStyle.Render(inventory.ConfigPath(m.prefs.Dir)) + "\n\n" + line
	return boxStyle.Width(m.paneWidth()).Render(body)
}

func (m model) projectsPane() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("项目列表"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(inventory.ProjectsPath(m.prefs.Dir)))
	b.WriteString("\n\n")
	if len(m.prefs.Projects) == 0 {
		b.WriteString(dimStyle.Render("暂无项目，按 'a' 新增。"))
	} else {
		nameWidth := 12
		for _, p := range m.prefs.Projects {
			if n := utf8.RuneCountInString(p.Name); n > nameWidth {
				nameWidth = n
			}
		}
		if nameWidth > 20 {
			nameWidth = 20
		}
		visible := m.listRows()
		if visible > 12 {
			visible = 12
		}
		start, end := window(len(m.prefs.Projects), m.sourceCur, visible)
		for i := start; i < end; i++ {
			p := m.prefs.Projects[i]
			name := padName(p.Name, nameWidth)
			mark := ""
			if p.Path == m.inv.Path {
				mark = "  (当前)"
			}
			plain := name + "  " + p.Path + mark
			if m.sourcePane == paneProjects && i == m.sourceCur {
				b.WriteString(selectedStyle.Render("› " + plain))
			} else {
				b.WriteString("  " + name + "  " + dimStyle.Render(p.Path+mark))
			}
			if i < end-1 {
				b.WriteByte('\n')
			}
		}
	}
	return boxStyle.Width(m.paneWidth()).Render(b.String())
}

func (m model) paneWidth() int {
	w := (m.width - 6) / 2
	if w < 28 {
		w = 28
	}
	return w
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

func window(n, cursor, rows int) (int, int) {
	if n == 0 {
		return 0, 0
	}
	if rows < 1 {
		rows = 1
	}
	if cursor < 0 {
		cursor = 0
	}
	start := 0
	if cursor >= rows {
		start = cursor - rows + 1
	}
	end := start + rows
	if end > n {
		end = n
	}
	return start, end
}

func nameColWidth(names []string, total int) int {
	w := 8
	for _, n := range names {
		c := utf8.RuneCountInString(n)
		if c > w {
			w = c
		}
	}
	if w > 24 {
		w = 24
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

func parseLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

func parseKV(s string) []inventory.KV {
	var out []inventory.KV
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value := line, ""
		if idx := strings.Index(line, "="); idx >= 0 {
			key = strings.TrimSpace(line[:idx])
			value = strings.TrimSpace(line[idx+1:])
		}
		if key == "" {
			continue
		}
		out = append(out, inventory.KV{Key: key, Value: value})
	}
	return out
}

func kvToLines(kvs []inventory.KV) string {
	var lines []string
	for _, kv := range kvs {
		lines = append(lines, kv.Key+"="+kv.Value)
	}
	return strings.Join(lines, "\n")
}
