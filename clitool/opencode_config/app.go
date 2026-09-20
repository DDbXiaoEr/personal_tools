package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"occonfig/internal/opencode"
	"occonfig/internal/skill"
)

type tabKind int

const (
	tabProviders tabKind = iota
	tabMCP
	tabSkills
	tabCount
)

type uiMode int

const (
	modeList uiMode = iota
	modeForm
	modeConfirm
	modeSource
	modeSkillView
	modeFilePick
)

type pendingKind int

const (
	pendingNone pendingKind = iota
	pendingDeleteProvider
	pendingDeleteMCP
	pendingQuit
)

var providerIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// MCP form field indexes.
const (
	mcpFieldName = iota
	mcpFieldType
	mcpFieldCommand
	mcpFieldBrowse
	mcpFieldEnvironment
	mcpFieldCWD
	mcpFieldURL
	mcpFieldHeaders
	mcpFieldTimeout
	mcpFieldOAuth
	mcpFieldEnabled
)

const actionPickCommand = "pick-command"

type model struct {
	cfg       *opencode.Config
	skillsDir string

	tab  tabKind
	mode uiMode

	width  int
	height int

	provCursor  int
	mcpCursor   int
	skillCursor int

	form form

	editingProvider    string
	editingProviderRaw []byte
	editingMCP         string
	editingMCPRaw      []byte

	skills []skill.Skill
	sview  viewport.Model

	filePicker filepicker.Model
	pickTarget int

	pending   pendingKind
	sources   []string
	sourceCur int

	status   string
	isErr    bool
	showHelp bool
}

func newModel(cfg *opencode.Config, skillsDir string) model {
	m := model{cfg: cfg, skillsDir: skillsDir, width: 100, height: 30}
	m.sview = viewport.New(96, 20)
	m.filePicker = newFilePicker()
	m.reloadSkills()
	return m
}

func newFilePicker() filepicker.Model {
	fp := filepicker.New()
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		fp.CurrentDirectory = home
	}
	fp.ShowHidden = true
	fp.FileAllowed = true
	fp.DirAllowed = false
	fp.AutoHeight = false
	fp.SetHeight(20)
	return fp
}

func (m *model) reloadSkills() {
	skills, err := skill.Scan(m.skillsDir)
	if err != nil {
		m.setErr("读取技能失败: " + err.Error())
		return
	}
	m.skills = skills
}

func (m *model) setOK(msg string)  { m.status, m.isErr = msg, false }
func (m *model) setErr(msg string) { m.status, m.isErr = msg, true }

func (m *model) resetCursors() {
	m.provCursor, m.mcpCursor, m.skillCursor = 0, 0, 0
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		w := m.width - 6
		if w < 20 {
			w = 20
		}
		h := m.height - 10
		if h < 5 {
			h = 5
		}
		m.sview.Width, m.sview.Height = w, h
		fh := m.height - 7
		if fh < 5 {
			fh = 5
		}
		m.filePicker.SetHeight(fh)
		return m, nil
	case tea.KeyMsg:
		switch m.mode {
		case modeForm:
			return m.updateForm(msg)
		case modeFilePick:
			return m.updateFilePick(msg)
		case modeConfirm:
			return m.updateConfirm(msg)
		case modeSource:
			return m.updateSource(msg)
		case modeSkillView:
			return m.updateSkillView(msg)
		default:
			return m.updateList(msg)
		}
	}
	if m.mode == modeFilePick {
		return m.updateFilePick(msg)
	}
	return m, nil
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		if m.cfg.Dirty() {
			m.pending = pendingQuit
			m.mode = modeConfirm
			return m, nil
		}
		return m, tea.Quit
	case "1":
		m.tab = tabProviders
	case "2":
		m.tab = tabMCP
	case "3":
		m.tab = tabSkills
	case "tab":
		m.tab = (m.tab + 1) % tabCount
	case "shift+tab":
		m.tab = (m.tab + tabCount - 1) % tabCount
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
		m.setCursor(0)
	case "G", "end":
		m.setCursor(m.listLen() - 1)
	case "a":
		m.startAdd()
	case "enter", "e":
		m.startEdit()
	case "d", "delete", "backspace":
		m.startDelete()
	case " ":
		m.toggleEnabled()
	}
	return m, nil
}

func (m model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if fld := m.form.focused(); fld != nil && fld.kind == kindButton {
		switch msg.String() {
		case "enter", " ":
			return m.activateAction(fld.action)
		}
	}
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
	case "ctrl+f":
		return m.openCommandPicker()
	case "ctrl+s":
		return m.submitForm()
	}
	cmd := m.form.updateFocused(msg)
	return m, cmd
}

func (m model) activateAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case actionPickCommand:
		return m.openCommandPicker()
	}
	return m, nil
}

func (m model) openCommandPicker() (tea.Model, tea.Cmd) {
	if m.tab != tabMCP {
		return m, nil
	}
	if m.form.fields[mcpFieldType].choiceValue() == "remote" {
		m.setErr("remote 类型不需要 command，请先把 type 切换为 local")
		return m, nil
	}
	m.pickTarget = mcpFieldCommand
	m.mode = modeFilePick
	return m, m.filePicker.Init()
}

func (m model) updateFilePick(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.mode = modeForm
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.filePicker, cmd = m.filePicker.Update(msg)
	if didSelect, path := m.filePicker.DidSelectFile(msg); didSelect {
		m.applyPickedFile(path)
		m.mode = modeForm
		return m, nil
	}
	return m, cmd
}

func (m *model) applyPickedFile(path string) {
	if m.pickTarget < 0 || m.pickTarget >= len(m.form.fields) {
		return
	}
	fld := m.form.fields[m.pickTarget]
	existing := parseLines(fld.areaText())
	var args []string
	if len(existing) > 0 {
		args = existing[1:]
	}
	lines := append([]string{path}, args...)
	fld.setArea(strings.Join(lines, "\n"))
	m.setOK("已选择 " + path)
}

func (m model) submitForm() (tea.Model, tea.Cmd) {
	var err error
	switch m.tab {
	case tabProviders:
		err = m.applyProviderForm()
	case tabMCP:
		err = m.applyMCPForm()
	}
	if err != nil {
		m.setErr(err.Error())
		return m, nil
	}
	m.mode = modeList
	m.setOK("已更新，按 s 保存到文件")
	return m, nil
}

func (m model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "y" || msg.String() == "Y" {
		switch m.pending {
		case pendingDeleteProvider:
			ids := m.cfg.ProviderIDs()
			if m.provCursor < len(ids) {
				m.cfg.DeleteProvider(ids[m.provCursor])
				m.setOK("已删除，按 s 保存")
			}
		case pendingDeleteMCP:
			ids := m.cfg.MCPIDs()
			if m.mcpCursor < len(ids) {
				m.cfg.DeleteMCP(ids[m.mcpCursor])
				m.setOK("已删除，按 s 保存")
			}
		case pendingQuit:
			return m, tea.Quit
		}
	}
	m.pending = pendingNone
	m.mode = modeList
	return m, nil
}

func (m model) updateSource(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "o":
		m.mode = modeList
	case "up", "k":
		if m.sourceCur > 0 {
			m.sourceCur--
		}
	case "down", "j":
		if m.sourceCur < len(m.sources)-1 {
			m.sourceCur++
		}
	case "q", "ctrl+c":
		return m, tea.Quit
	case "enter":
		path := m.sources[m.sourceCur]
		cfg, err := opencode.Load(path)
		if err != nil {
			m.setErr("切换失败: " + err.Error())
			m.mode = modeList
			return m, nil
		}
		m.cfg = cfg
		m.resetCursors()
		m.mode = modeList
		m.setOK("已切换到 " + path)
	}
	return m, nil
}

func (m model) updateSkillView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "enter":
		m.mode = modeList
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.sview, cmd = m.sview.Update(msg)
	return m, cmd
}

func (m *model) save() {
	if !m.cfg.Dirty() {
		m.setOK("没有需要保存的修改")
		return
	}
	if err := m.cfg.Save(); err != nil {
		m.setErr("保存失败: " + err.Error())
		return
	}
	m.setOK("已保存到 " + m.cfg.Path())
}

func (m *model) reload() {
	if m.cfg.Dirty() {
		m.setErr("有未保存的修改，请先保存，或按 o 切换配置来源")
		return
	}
	if err := m.cfg.Reload(); err != nil {
		m.setErr("重新加载失败: " + err.Error())
		return
	}
	m.reloadSkills()
	m.resetCursors()
	m.setOK("已重新加载")
}

func (m *model) openSourcePicker() {
	sources := []string{m.cfg.Path()}
	if p := detectProjectConfig(); p != "" && p != m.cfg.Path() {
		sources = append(sources, p)
	}
	m.sources = sources
	m.sourceCur = 0
	m.mode = modeSource
}

func detectProjectConfig() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		for _, name := range []string{"opencode.json", "opencode.jsonc"} {
			p := filepath.Join(dir, name)
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				return p
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func (m *model) startAdd() {
	switch m.tab {
	case tabProviders:
		m.editingProvider = ""
		m.editingProviderRaw = nil
		m.buildProviderForm()
		m.mode = modeForm
	case tabMCP:
		m.editingMCP = ""
		m.editingMCPRaw = nil
		m.buildMCPForm()
		m.mode = modeForm
	}
}

func (m *model) startEdit() {
	switch m.tab {
	case tabProviders:
		ids := m.cfg.ProviderIDs()
		if len(ids) == 0 {
			return
		}
		id := ids[m.provCursor]
		m.editingProvider = id
		m.editingProviderRaw = m.cfg.Provider(id)
		m.buildProviderForm()
		m.mode = modeForm
	case tabMCP:
		ids := m.cfg.MCPIDs()
		if len(ids) == 0 {
			return
		}
		name := ids[m.mcpCursor]
		m.editingMCP = name
		m.editingMCPRaw = m.cfg.MCP(name)
		m.buildMCPForm()
		m.mode = modeForm
	case tabSkills:
		m.openSkillView()
	}
}

func (m *model) startDelete() {
	switch m.tab {
	case tabProviders:
		if len(m.cfg.ProviderIDs()) > 0 {
			m.pending = pendingDeleteProvider
			m.mode = modeConfirm
		}
	case tabMCP:
		if len(m.cfg.MCPIDs()) > 0 {
			m.pending = pendingDeleteMCP
			m.mode = modeConfirm
		}
	}
}

func (m *model) toggleEnabled() {
	if m.tab != tabMCP {
		return
	}
	ids := m.cfg.MCPIDs()
	if len(ids) == 0 {
		return
	}
	name := ids[m.mcpCursor]
	mc := opencode.ParseMCP(name, m.cfg.MCP(name))
	mc.Enabled = !mc.Enabled
	raw, err := mc.Encode()
	if err != nil {
		m.setErr(err.Error())
		return
	}
	m.cfg.SetMCP(name, raw)
	state := "已启用"
	if !mc.Enabled {
		state = "已禁用"
	}
	m.setOK(fmt.Sprintf("%s %s，按 s 保存", name, state))
}

func (m *model) buildProviderForm() {
	p := opencode.ParseProvider(m.editingProvider, m.editingProviderRaw)
	title := "新增 Provider"
	if m.editingProvider != "" {
		title = "编辑 Provider: " + m.editingProvider
	}
	idField := newText("id", "my-provider")
	idField.setText(p.ID)
	if m.editingProvider != "" {
		idField.readOnly = true
	}
	f := &form{title: title}
	f.add(
		idField,
		newText("name", "显示名称"),
		newText("npm", "@ai-sdk/openai-compatible"),
		newText("baseURL", "https://api.example.com/v1"),
		newText("apiKey", "{env:MY_API_KEY}"),
	)
	f.fields[1].setText(p.Name)
	f.fields[2].setText(p.NPM)
	f.fields[3].setText(p.BaseURL)
	f.fields[4].setText(p.APIKey)
	f.focusFirst()
	m.form = *f
}

func (m *model) applyProviderForm() error {
	id := m.form.fields[0].text()
	if id == "" {
		return fmt.Errorf("provider id 不能为空")
	}
	if !providerIDRe.MatchString(id) {
		return fmt.Errorf("provider id 只能包含小写字母、数字、. _ -，且以字母或数字开头")
	}
	if m.editingProvider == "" {
		if _, ok := m.cfg.Providers[id]; ok {
			return fmt.Errorf("provider %q 已存在", id)
		}
	}
	p := opencode.ParseProvider(id, m.editingProviderRaw)
	p.ID = id
	p.Name = m.form.fields[1].text()
	p.NPM = m.form.fields[2].text()
	p.BaseURL = m.form.fields[3].text()
	p.APIKey = m.form.fields[4].text()
	raw, err := p.Encode()
	if err != nil {
		return err
	}
	m.cfg.SetProvider(id, raw)
	return nil
}

func (m *model) buildMCPForm() {
	mc := opencode.ParseMCP(m.editingMCP, m.editingMCPRaw)
	title := "新增 MCP Server"
	if m.editingMCP != "" {
		title = "编辑 MCP: " + m.editingMCP
	}
	typeIdx := 0
	if mc.Type == "remote" {
		typeIdx = 1
	}
	nameField := newText("name", "my-mcp")
	nameField.setText(mc.Name)
	if m.editingMCP != "" {
		nameField.readOnly = true
	}
	f := &form{title: title}
	f.add(
		nameField,
		newChoice("type", []string{"local", "remote"}, typeIdx),
		newArea("command (每行一个参数)", "npx\n-y\n@modelcontextprotocol/server-everything", 3),
		newButton("浏览可执行文件…", actionPickCommand),
		newArea("environment (KEY=value)", "MY_VAR=value", 3),
		newText("cwd", ""),
		newText("url", "https://mcp.example.com/mcp"),
		newArea("headers (Key: value)", "Authorization: Bearer TOKEN", 3),
		newText("timeout (ms)", "5000"),
		newBool("oauth", mc.OAuth),
		newBool("enabled", mc.Enabled),
	)
	f.fields[mcpFieldCommand].setArea(strings.Join(mc.Command, "\n"))
	f.fields[mcpFieldEnvironment].setArea(kvToLines(mc.Environment, "="))
	f.fields[mcpFieldCWD].setText(mc.CWD)
	f.fields[mcpFieldURL].setText(mc.URL)
	f.fields[mcpFieldHeaders].setArea(kvToLines(mc.Headers, ": "))
	f.fields[mcpFieldTimeout].setText(mc.Timeout)
	f.focusFirst()
	m.form = *f
}

func (m *model) applyMCPForm() error {
	name := m.form.fields[mcpFieldName].text()
	if name == "" {
		return fmt.Errorf("MCP 名称不能为空")
	}
	if m.editingMCP == "" {
		if _, ok := m.cfg.MCPServers[name]; ok {
			return fmt.Errorf("MCP %q 已存在", name)
		}
	}
	mc := opencode.ParseMCP(name, m.editingMCPRaw)
	mc.Name = name
	mc.Type = m.form.fields[mcpFieldType].choiceValue()
	mc.Command = parseLines(m.form.fields[mcpFieldCommand].areaText())
	mc.Environment = parseKV(m.form.fields[mcpFieldEnvironment].areaText(), "=")
	mc.CWD = m.form.fields[mcpFieldCWD].text()
	mc.URL = m.form.fields[mcpFieldURL].text()
	mc.Headers = parseKV(m.form.fields[mcpFieldHeaders].areaText(), ":")
	mc.Timeout = m.form.fields[mcpFieldTimeout].text()
	mc.OAuth = m.form.fields[mcpFieldOAuth].boolean
	mc.Enabled = m.form.fields[mcpFieldEnabled].boolean

	if mc.Type == "local" && len(mc.Command) == 0 {
		return fmt.Errorf("local 类型必须填写 command")
	}
	if mc.Type == "remote" && strings.TrimSpace(mc.URL) == "" {
		return fmt.Errorf("remote 类型必须填写 url")
	}
	if mc.Timeout != "" {
		if _, err := strconv.Atoi(mc.Timeout); err != nil {
			return fmt.Errorf("timeout 必须是毫秒整数")
		}
	}
	raw, err := mc.Encode()
	if err != nil {
		return err
	}
	m.cfg.SetMCP(name, raw)
	return nil
}

func (m *model) openSkillView() {
	if len(m.skills) == 0 {
		return
	}
	s := m.skills[m.skillCursor]
	m.sview.SetContent(formatSkill(s))
	m.sview.GotoTop()
	m.mode = modeSkillView
}

func formatSkill(s skill.Skill) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(s.Name))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(s.Path))
	b.WriteString("\n")
	for _, w := range s.Warnings {
		b.WriteString(warnStyle.Render("⚠ " + w))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(s.Raw)
	return b.String()
}

func (m *model) listLen() int {
	switch m.tab {
	case tabProviders:
		return len(m.cfg.ProviderIDs())
	case tabMCP:
		return len(m.cfg.MCPIDs())
	case tabSkills:
		return len(m.skills)
	}
	return 0
}

func (m *model) moveCursor(d int) {
	n := m.listLen()
	if n == 0 {
		return
	}
	switch m.tab {
	case tabProviders:
		m.provCursor = clamp(m.provCursor+d, 0, n-1)
	case tabMCP:
		m.mcpCursor = clamp(m.mcpCursor+d, 0, n-1)
	case tabSkills:
		m.skillCursor = clamp(m.skillCursor+d, 0, n-1)
	}
}

func (m *model) setCursor(v int) {
	n := m.listLen()
	if n == 0 {
		return
	}
	v = clamp(v, 0, n-1)
	switch m.tab {
	case tabProviders:
		m.provCursor = v
	case tabMCP:
		m.mcpCursor = v
	case tabSkills:
		m.skillCursor = v
	}
}

func (m model) View() string {
	switch m.mode {
	case modeForm:
		return boxStyle.Render(m.form.View())
	case modeConfirm:
		return m.listView() + "\n\n" + errStyle.Render(m.confirmText()+" (y/n)")
	case modeSource:
		return m.sourceView()
	case modeSkillView:
		return m.skillView()
	case modeFilePick:
		return m.filePickView()
	default:
		return m.listView()
	}
}

func (m model) filePickView() string {
	header := titleStyle.Render("选择可执行文件")
	dir := dimStyle.Render(m.filePicker.CurrentDirectory)
	return header + "\n" + dir + "\n\n" + m.filePicker.View() + "\n" +
		helpStyle.Render("↑↓ 移动 · enter 进入目录/选择文件 · ←/backspace 上级目录 · esc 取消")
}

func (m model) confirmText() string {
	switch m.pending {
	case pendingDeleteProvider:
		ids := m.cfg.ProviderIDs()
		if m.provCursor < len(ids) {
			return fmt.Sprintf("确定删除 Provider %q ?", ids[m.provCursor])
		}
		return "确定删除该 Provider ?"
	case pendingDeleteMCP:
		ids := m.cfg.MCPIDs()
		if m.mcpCursor < len(ids) {
			return fmt.Sprintf("确定删除 MCP %q ?", ids[m.mcpCursor])
		}
		return "确定删除该 MCP ?"
	case pendingQuit:
		return "有未保存的修改，确定退出 ?"
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
	tabs := []string{"Provider", "MCP", "Skills"}
	var parts []string
	for i, t := range tabs {
		if tabKind(i) == m.tab {
			parts = append(parts, activeTabStyle.Render(t))
		} else {
			parts = append(parts, inactiveTabStyle.Render(t))
		}
	}
	dirty := ""
	if m.cfg.Dirty() {
		dirty = " *"
	}
	return strings.Join(parts, "") + dimStyle.Render("   "+m.cfg.Path()+dirty)
}

func (m model) bodyView() string {
	switch m.tab {
	case tabProviders:
		return m.providersBody()
	case tabMCP:
		return m.mcpBody()
	case tabSkills:
		return m.skillsBody()
	}
	return ""
}

func (m model) listRows() int {
	rows := m.height - 9
	if m.showHelp {
		rows -= 8
	}
	if rows < 3 {
		rows = 3
	}
	return rows
}

func (m model) providersBody() string {
	ids := m.cfg.ProviderIDs()
	if len(ids) == 0 {
		return dimStyle.Render("暂无自定义 Provider，按 'a' 新增。") + "\n"
	}
	start, end := window(len(ids), m.provCursor, m.listRows())
	var b strings.Builder
	for i := start; i < end; i++ {
		id := ids[i]
		p := opencode.ParseProvider(id, m.cfg.Provider(id))
		detail := p.Name
		if p.NPM != "" {
			detail = strings.TrimSpace(detail + " · " + p.NPM)
		}
		if p.BaseURL != "" {
			detail = strings.TrimSpace(detail + " · " + p.BaseURL)
		}
		extra := ""
		if keys := p.ExtraKeys(); len(keys) > 0 {
			extra = dimStyle.Render(fmt.Sprintf("  [保留 %d 个键]", len(keys)))
		}
		plain := fmt.Sprintf("%-22s %s", id, detail)
		if i == m.provCursor {
			b.WriteString(selectedStyle.Render("› "+plain) + extra)
		} else {
			b.WriteString("  " + plain + extra)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (m model) mcpBody() string {
	ids := m.cfg.MCPIDs()
	if len(ids) == 0 {
		return dimStyle.Render("暂无 MCP Server，按 'a' 新增。") + "\n"
	}
	start, end := window(len(ids), m.mcpCursor, m.listRows())
	var b strings.Builder
	for i := start; i < end; i++ {
		name := ids[i]
		mc := opencode.ParseMCP(name, m.cfg.MCP(name))
		state := "on "
		if !mc.Enabled {
			state = "off"
		}
		detail := mc.Type
		if mc.Type == "remote" {
			detail += " · " + mc.URL
		} else {
			detail += " · " + strings.Join(mc.Command, " ")
		}
		plain := fmt.Sprintf("[%s] %-20s %s", state, name, detail)
		if i == m.mcpCursor {
			b.WriteString(selectedStyle.Render("› " + plain))
		} else {
			b.WriteString("  " + plain)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (m model) skillsBody() string {
	if len(m.skills) == 0 {
		return dimStyle.Render("在 "+m.skillsDir+" 下没有找到技能。") + "\n"
	}
	start, end := window(len(m.skills), m.skillCursor, m.listRows())
	var b strings.Builder
	for i := start; i < end; i++ {
		s := m.skills[i]
		desc := oneLine(s.Description)
		if desc == "" {
			desc = s.Path
		}
		plain := fmt.Sprintf("%-24s %s", s.Name, truncate(desc, 60))
		mark := ""
		if len(s.Warnings) > 0 {
			mark = warnStyle.Render(" ⚠")
		}
		if i == m.skillCursor {
			b.WriteString(selectedStyle.Render("› "+plain) + mark)
		} else {
			b.WriteString("  " + plain + mark)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (m model) helpLine() string {
	switch m.tab {
	case tabProviders:
		return "1/2/3 切换 · a 新增 · enter/e 编辑 · d 删除 · s 保存 · r 重载 · o 配置来源 · ? 帮助 · q 退出"
	case tabMCP:
		return "1/2/3 切换 · a 新增 · enter/e 编辑 · d 删除 · space 启停 · s 保存 · r 重载 · o 配置来源 · q 退出"
	default:
		return "1/2/3 切换 · enter 查看 · ↑↓ 选择 · r 重载 · q 退出"
	}
}

func (m model) helpView() string {
	return dimStyle.Render(
		"提示：\n" +
			"  · 修改保存在内存中，按 s 写回文件（保留注释与未知字段）。\n" +
			"  · 保存时会生成 <配置文件>.bak 备份。\n" +
			"  · 编辑 provider/MCP 时名称不可修改（改名请删除后新增）。\n" +
			"  · Provider 未在表单中暴露的键（如 models）会被原样保留。\n" +
			"  · MCP local 类型：Tab 到「浏览可执行文件」按钮按 Enter（或按 ctrl+f）选择可执行文件。\n" +
			"  · Skills 标签页为只读，enter 查看 SKILL.md 全文。")
}

func (m model) sourceView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("选择配置来源"))
	b.WriteString("\n\n")
	for i, s := range m.sources {
		marker := "  "
		if i == m.sourceCur {
			marker = "› "
		}
		label := s
		if s == m.cfg.Path() {
			label += "  (当前)"
		}
		if i == m.sourceCur {
			b.WriteString(selectedStyle.Render(marker + label))
		} else {
			b.WriteString(marker + label)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑↓ 选择 · enter 切换 · esc 取消"))
	return b.String()
}

func (m model) skillView() string {
	return m.sview.View() + "\n" + helpStyle.Render("↑↓/pgup/pgdn 滚动 · esc 返回")
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

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func parseLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func parseKV(s, sep string) []opencode.KV {
	var out []opencode.KV
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value := line, ""
		if idx := strings.Index(line, sep); idx >= 0 {
			key = strings.TrimSpace(line[:idx])
			value = strings.TrimSpace(line[idx+len(sep):])
		}
		if key == "" {
			continue
		}
		out = append(out, opencode.KV{Key: key, Value: value})
	}
	return out
}

func kvToLines(kvs []opencode.KV, sep string) string {
	var lines []string
	for _, kv := range kvs {
		lines = append(lines, kv.Key+sep+kv.Value)
	}
	return strings.Join(lines, "\n")
}
