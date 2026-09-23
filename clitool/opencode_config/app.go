package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

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
	modeVariantList
)

type pendingKind int

const (
	pendingNone pendingKind = iota
	pendingDeleteProvider
	pendingDeleteMCP
	pendingDeleteVariant
	pendingDeleteSkill
	pendingQuit
)

type skillInstalledMsg struct {
	name string
	err  error
}

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

const (
	varFieldModel = iota
	varFieldID
	varFieldDisabled
	varFieldOptions
)

var modelIDRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]*$`)

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

	variantProvID   string
	variantCursor   int
	editingVarModel string
	editingVarID    string
	formKind        string

	skills []skill.Skill
	sview  viewport.Model

	filePicker filepicker.Model
	pickTarget int

	pending   pendingKind
	sources   []string
	sourceCur int

	status     string
	isErr      bool
	showHelp   bool
	installing bool
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
	case skillInstalledMsg:
		m.installing = false
		if msg.err != nil {
			m.setErr("安装技能失败: " + msg.err.Error())
			return m, nil
		}
		m.reloadSkills()
		for i, s := range m.skills {
			if s.Dir == msg.name || s.Name == msg.name {
				m.skillCursor = i
				break
			}
		}
		m.setOK("已安装 " + msg.name)
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
		case modeVariantList:
			return m.updateVariantList(msg)
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
	case "v":
		m.openVariantList()
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
		if m.formKind == "variant" {
			m.mode = modeVariantList
		} else {
			m.mode = modeList
		}
		m.formKind = ""
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
	switch m.formKind {
	case "variant":
		err = m.applyVariantForm()
	case "mcp":
		err = m.applyMCPForm()
	case "skill":
		return m.applySkillForm()
	default:
		err = m.applyProviderForm()
	}
	if err != nil {
		m.setErr(err.Error())
		return m, nil
	}
	if m.formKind == "variant" {
		m.mode = modeVariantList
	} else {
		m.mode = modeList
	}
	m.formKind = ""
	m.setOK("已更新，按 s 保存到文件")
	return m, nil
}

func (m model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	kind := m.pending
	if msg.String() == "y" || msg.String() == "Y" {
		switch kind {
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
		case pendingDeleteVariant:
			m.deleteCurrentVariant()
		case pendingDeleteSkill:
			m.deleteCurrentSkill()
		case pendingQuit:
			return m, tea.Quit
		}
	}
	m.pending = pendingNone
	if kind == pendingDeleteVariant || (kind == pendingQuit && m.variantProvID != "") {
		m.mode = modeVariantList
	} else {
		m.mode = modeList
	}
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
		m.formKind = "provider"
		m.buildProviderForm()
		m.mode = modeForm
	case tabMCP:
		m.editingMCP = ""
		m.editingMCPRaw = nil
		m.formKind = "mcp"
		m.buildMCPForm()
		m.mode = modeForm
	case tabSkills:
		if m.installing {
			m.setErr("正在安装技能，请稍候")
			return
		}
		m.formKind = "skill"
		m.buildSkillForm()
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
		m.formKind = "provider"
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
		m.formKind = "mcp"
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
	case tabSkills:
		if len(m.skills) > 0 {
			m.pending = pendingDeleteSkill
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

func (m *model) currentProvider() (*opencode.Provider, string, bool) {
	id := m.variantProvID
	if id == "" {
		ids := m.cfg.ProviderIDs()
		if m.provCursor >= len(ids) {
			return nil, "", false
		}
		id = ids[m.provCursor]
	}
	return opencode.ParseProvider(id, m.cfg.Provider(id)), id, true
}

func (m *model) writeProvider(id string, p *opencode.Provider) error {
	raw, err := p.Encode()
	if err != nil {
		return err
	}
	m.cfg.SetProvider(id, raw)
	return nil
}

func (m *model) openVariantList() {
	if m.tab != tabProviders {
		return
	}
	ids := m.cfg.ProviderIDs()
	if len(ids) == 0 {
		m.setErr("请先新增 Provider")
		return
	}
	m.variantProvID = ids[m.provCursor]
	m.variantCursor = 0
	m.mode = modeVariantList
}

func (m model) updateVariantList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		if m.cfg.Dirty() {
			m.pending = pendingQuit
			m.mode = modeConfirm
			return m, nil
		}
		return m, tea.Quit
	case "esc":
		m.variantProvID = ""
		m.mode = modeList
	case "up", "k":
		m.moveVariantCursor(-1)
	case "down", "j":
		m.moveVariantCursor(1)
	case "g", "home":
		m.setVariantCursor(0)
	case "G", "end":
		p, _, ok := m.currentProvider()
		if ok {
			m.setVariantCursor(p.NumVariants() - 1)
		}
	case "a":
		m.startAddVariant()
	case "enter", "e":
		m.startEditVariant()
	case "d", "delete", "backspace":
		p, _, ok := m.currentProvider()
		if ok && p.NumVariants() > 0 {
			m.pending = pendingDeleteVariant
			m.mode = modeConfirm
		}
	case " ":
		m.toggleVariant()
	case "s":
		m.save()
	case "r":
		m.reload()
		if !m.cfg.Dirty() {
			m.mode = modeList
			m.variantProvID = ""
		}
	case "?":
		m.showHelp = !m.showHelp
	}
	return m, nil
}

func (m *model) moveVariantCursor(d int) {
	p, _, ok := m.currentProvider()
	if !ok {
		return
	}
	n := p.NumVariants()
	if n == 0 {
		return
	}
	m.variantCursor = clamp(m.variantCursor+d, 0, n-1)
}

func (m *model) setVariantCursor(v int) {
	p, _, ok := m.currentProvider()
	if !ok {
		return
	}
	n := p.NumVariants()
	if n == 0 {
		return
	}
	m.variantCursor = clamp(v, 0, n-1)
}

func (m *model) currentVariantRef() (opencode.VariantRef, bool) {
	p, _, ok := m.currentProvider()
	if !ok {
		return opencode.VariantRef{}, false
	}
	refs := p.VariantRefs()
	if m.variantCursor < 0 || m.variantCursor >= len(refs) {
		return opencode.VariantRef{}, false
	}
	return refs[m.variantCursor], true
}

func (m *model) startAddVariant() {
	m.editingVarModel = ""
	m.editingVarID = ""
	m.formKind = "variant"
	m.buildVariantForm(nil)
	m.mode = modeForm
}

func (m *model) startEditVariant() {
	ref, ok := m.currentVariantRef()
	if !ok {
		return
	}
	m.editingVarModel = ref.ModelID
	m.editingVarID = ref.Variant.ID
	m.formKind = "variant"
	m.buildVariantForm(ref.Variant)
	m.mode = modeForm
}

func (m *model) buildVariantForm(v *opencode.Variant) {
	title := "新增 Variant"
	if m.editingVarID != "" {
		title = "编辑 Variant: " + m.editingVarModel + "/" + m.editingVarID
	}
	modelField := newText("model", "gpt-5")
	idField := newText("id", "high")
	disabled := false
	opts := ""
	if v != nil {
		modelField.setText(m.editingVarModel)
		idField.setText(v.ID)
		disabled = v.Disabled
		opts = kvToLines(v.Options, "=")
	}
	if m.editingVarID != "" {
		modelField.readOnly = true
		idField.readOnly = true
	}
	f := &form{title: title}
	f.add(
		modelField,
		idField,
		newBool("disabled", disabled),
		newArea("options (KEY=value)", "reasoningEffort=high\ntextVerbosity=low", 6),
	)
	f.fields[varFieldOptions].setArea(opts)
	f.focusFirst()
	m.form = *f
}

func (m *model) applyVariantForm() error {
	p, id, ok := m.currentProvider()
	if !ok {
		return fmt.Errorf("未选中 Provider")
	}
	modelID := m.form.fields[varFieldModel].text()
	varID := m.form.fields[varFieldID].text()
	if modelID == "" {
		return fmt.Errorf("model 不能为空")
	}
	if varID == "" {
		return fmt.Errorf("variant id 不能为空")
	}
	if !modelIDRe.MatchString(modelID) {
		return fmt.Errorf("model id 只能包含字母、数字、. _ : / -，且以字母或数字开头")
	}
	if !modelIDRe.MatchString(varID) {
		return fmt.Errorf("variant id 只能包含字母、数字、. _ : / -，且以字母或数字开头")
	}
	if m.editingVarID == "" {
		if p.FindVariant(modelID, varID) != nil {
			return fmt.Errorf("variant %s/%s 已存在", modelID, varID)
		}
	}
	v := &opencode.Variant{ID: varID}
	if m.editingVarID != "" {
		if existing := p.FindVariant(m.editingVarModel, m.editingVarID); existing != nil {
			v = existing.Clone()
			v.ID = varID
		}
	}
	v.Disabled = m.form.fields[varFieldDisabled].boolean
	v.Options = parseKV(m.form.fields[varFieldOptions].areaText(), "=")
	p.SetVariant(modelID, v)
	if err := m.writeProvider(id, p); err != nil {
		return err
	}
	refs := p.VariantRefs()
	for i, ref := range refs {
		if ref.ModelID == modelID && ref.Variant.ID == varID {
			m.variantCursor = i
			break
		}
	}
	return nil
}

func (m *model) deleteCurrentVariant() {
	ref, ok := m.currentVariantRef()
	if !ok {
		return
	}
	p, id, ok := m.currentProvider()
	if !ok {
		return
	}
	p.DeleteVariant(ref.ModelID, ref.Variant.ID)
	if err := m.writeProvider(id, p); err != nil {
		m.setErr(err.Error())
		return
	}
	n := p.NumVariants()
	if m.variantCursor >= n {
		m.variantCursor = n - 1
	}
	if m.variantCursor < 0 {
		m.variantCursor = 0
	}
	m.setOK(fmt.Sprintf("已删除 %s/%s，按 s 保存", ref.ModelID, ref.Variant.ID))
}

func (m *model) toggleVariant() {
	ref, ok := m.currentVariantRef()
	if !ok {
		return
	}
	p, id, ok := m.currentProvider()
	if !ok {
		return
	}
	if !p.ToggleVariantDisabled(ref.ModelID, ref.Variant.ID) {
		return
	}
	if err := m.writeProvider(id, p); err != nil {
		m.setErr(err.Error())
		return
	}
	state := "已启用"
	if p.FindVariant(ref.ModelID, ref.Variant.ID).Disabled {
		state = "已禁用"
	}
	m.setOK(fmt.Sprintf("%s/%s %s，按 s 保存", ref.ModelID, ref.Variant.ID, state))
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

func (m *model) buildSkillForm() {
	f := &form{title: "新增 Skill"}
	f.add(
		newText("目录名", "my-skill"),
		newText("zip URI", "https://example.com/skill.zip"),
	)
	f.focusFirst()
	m.form = *f
}

func (m model) applySkillForm() (tea.Model, tea.Cmd) {
	name := m.form.fields[0].text()
	uri := m.form.fields[1].text()
	if name == "" {
		name = skillNameFromURI(uri)
	}
	if name == "" {
		m.setErr("目录名不能为空")
		return m, nil
	}
	if !skill.ValidName(name) {
		m.setErr("目录名只能是小写字母、数字和连字符")
		return m, nil
	}
	if uri == "" {
		m.setErr("URI 不能为空")
		return m, nil
	}
	dir := m.skillsDir
	m.formKind = ""
	m.mode = modeList
	m.installing = true
	m.setOK("正在安装 " + name + " …")
	return m, func() tea.Msg {
		err := skill.Install(dir, name, uri)
		return skillInstalledMsg{name: name, err: err}
	}
}

func skillNameFromURI(uri string) string {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return ""
	}
	base := filepath.Base(uri)
	if i := strings.IndexByte(base, '?'); i >= 0 {
		base = base[:i]
	}
	base = strings.TrimSuffix(base, ".zip")
	base = strings.ToLower(base)
	var b strings.Builder
	prevDash := true
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case unicode.IsLetter(r) || r == '-' || r == '_' || r == '.':
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func (m *model) deleteCurrentSkill() {
	if m.skillCursor < 0 || m.skillCursor >= len(m.skills) {
		return
	}
	s := m.skills[m.skillCursor]
	if err := skill.Remove(m.skillsDir, s.Dir); err != nil {
		m.setErr("删除失败: " + err.Error())
		return
	}
	m.reloadSkills()
	if m.skillCursor >= len(m.skills) {
		m.skillCursor = len(m.skills) - 1
	}
	if m.skillCursor < 0 {
		m.skillCursor = 0
	}
	m.setOK("已删除 " + s.Dir)
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
		base := m.listView()
		if m.variantProvID != "" {
			base = m.variantListView()
		}
		return base + "\n\n" + errStyle.Render(m.confirmText()+" (y/n)")
	case modeSource:
		return m.sourceView()
	case modeSkillView:
		return m.skillView()
	case modeFilePick:
		return m.filePickView()
	case modeVariantList:
		return m.variantListView()
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
	case pendingDeleteVariant:
		if ref, ok := m.currentVariantRef(); ok {
			return fmt.Sprintf("确定删除 Variant %s/%s ?", ref.ModelID, ref.Variant.ID)
		}
		return "确定删除该 Variant ?"
	case pendingDeleteSkill:
		if m.skillCursor < len(m.skills) {
			return fmt.Sprintf("确定删除技能 %q ?", m.skills[m.skillCursor].Dir)
		}
		return "确定删除该技能 ?"
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

func (m model) variantListView() string {
	var b strings.Builder
	b.WriteString(m.headerView())
	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Variants: " + m.variantProvID))
	b.WriteString("\n\n")
	b.WriteString(m.variantBody())
	b.WriteString("\n")
	if m.status != "" {
		if m.isErr {
			b.WriteString(errStyle.Render(m.status))
		} else {
			b.WriteString(okStyle.Render(m.status))
		}
		b.WriteString("\n")
	}
	b.WriteString(helpStyle.Render("a 新增 · enter/e 编辑 · d 删除 · space 禁用 · s 保存 · esc 返回 · q 退出"))
	if m.showHelp {
		b.WriteString("\n\n")
		b.WriteString(m.helpView())
	}
	return b.String()
}

func (m model) variantBody() string {
	p, _, ok := m.currentProvider()
	if !ok {
		return dimStyle.Render("未选中 Provider。") + "\n"
	}
	refs := p.VariantRefs()
	if len(refs) == 0 {
		return dimStyle.Render("暂无 Variant，按 'a' 新增。") + "\n"
	}
	start, end := window(len(refs), m.variantCursor, m.listRows())
	var b strings.Builder
	lastModel := ""
	if start > 0 {
		lastModel = refs[start-1].ModelID
	}
	for i := start; i < end; i++ {
		ref := refs[i]
		if ref.ModelID != lastModel {
			b.WriteString(catStyle.Render("── " + ref.ModelID + " ──"))
			b.WriteString("\n")
			lastModel = ref.ModelID
		}
		state := "on "
		if ref.Variant.Disabled {
			state = "off"
		}
		plain := fmt.Sprintf("[%s] %-16s %s", state, ref.Variant.ID, variantOptionsSummary(ref.Variant))
		if i == m.variantCursor {
			b.WriteString(selectedStyle.Render("› " + plain))
		} else {
			b.WriteString("  " + plain)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func variantOptionsSummary(v *opencode.Variant) string {
	if len(v.Options) == 0 {
		return ""
	}
	parts := make([]string, 0, len(v.Options))
	for _, kv := range v.Options {
		if kv.Value == "" {
			parts = append(parts, kv.Key)
			continue
		}
		parts = append(parts, kv.Key+"="+truncate(kv.Value, 24))
	}
	return strings.Join(parts, " · ")
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
		if n := p.NumVariants(); n > 0 {
			detail = strings.TrimSpace(detail + fmt.Sprintf(" · %d variants", n))
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
		return dimStyle.Render("在 "+m.skillsDir+" 下没有找到技能，按 'a' 从 zip URI 安装。") + "\n"
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
		return "1/2/3 切换 · a 新增 · enter/e 编辑 · v variants · d 删除 · s 保存 · r 重载 · o 配置来源 · ? 帮助 · q 退出"
	case tabMCP:
		return "1/2/3 切换 · a 新增 · enter/e 编辑 · d 删除 · space 启停 · s 保存 · r 重载 · o 配置来源 · q 退出"
	default:
		return "1/2/3 切换 · a 安装 · enter 查看 · d 删除 · ↑↓ 选择 · r 重载 · q 退出"
	}
}

func (m model) helpView() string {
	return dimStyle.Render(
		"提示：\n" +
			"  · 修改保存在内存中，按 s 写回文件（保留注释与未知字段）。\n" +
			"  · 保存时会生成 <配置文件>.bak 备份。\n" +
			"  · 编辑 provider/MCP 时名称不可修改（改名请删除后新增）。\n" +
			"  · Provider 按 v 管理 models.variants；options 每行 KEY=value，JSON 值原样保留。\n" +
			"  · Provider 未在表单中暴露的键（如 blacklist）会被原样保留。\n" +
			"  · MCP local 类型：Tab 到「浏览可执行文件」按钮按 Enter（或按 ctrl+f）选择可执行文件。\n" +
			"  · Skills：a 从 zip URI 下载并解压到指定目录；d 删除技能目录；enter 查看 SKILL.md 全文。")
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
