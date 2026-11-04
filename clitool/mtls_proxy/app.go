package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mtlsproxy/internal/mtls"
)

type uiMode int

const (
	modeList uiMode = iota
	modeForm
	modeConfirm
	modeFilePick
)

const (
	actionPickCert = "pick-cert"
	actionPickKey  = "pick-key"
	actionPickCA   = "pick-ca"
)

type pendingKind int

const (
	pendingNone pendingKind = iota
	pendingDelete
	pendingQuit
)

const (
	fwName = iota
	fwMode
	fwListen
	fwTarget
	fwCert
	fwKey
	fwCA
	fwSNI
	fwALPN
	fwTimeout
	fwInsecure
)

type model struct {
	ctl      *mtls.Control
	forwards []mtls.ForwardStatus

	mode    uiMode
	pending pendingKind
	cursor  int
	form    form
	editID  string

	filePicker filepicker.Model
	pickTarget int

	status   string
	isErr    bool
	showHelp bool

	width  int
	height int
}

type tickMsg time.Time

func newModel(ctl *mtls.Control) model {
	m := model{
		ctl:    ctl,
		width:  100,
		height: 30,
	}
	m.filePicker = newFilePicker()
	m.refresh()
	return m
}

func newFilePicker() filepicker.Model {
	fp := filepicker.New()
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		fp.CurrentDirectory = home
	}
	fp.ShowHidden = true
	fp.FileAllowed = true
	fp.DirAllowed = false
	fp.AutoHeight = false
	fp.SetHeight(20)
	return fp
}

func (m *model) setOK(msg string)  { m.status, m.isErr = msg, false }
func (m *model) setErr(msg string) { m.status, m.isErr = msg, true }

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tickCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		h := m.height - 8
		if h < 5 {
			h = 5
		}
		m.filePicker.SetHeight(h)
		return m, nil
	case tickMsg:
		m.refresh()
		return m, tickCmd()
	case tea.KeyMsg:
		switch m.mode {
		case modeForm:
			return m.updateForm(msg)
		case modeConfirm:
			return m.updateConfirm(msg)
		case modeFilePick:
			return m.updateFilePick(msg)
		default:
			return m.updateList(msg)
		}
	default:
		if m.mode == modeFilePick {
			return m.updateFilePick(msg)
		}
	}
	return m, nil
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m.requestQuit()
	case "?":
		m.showHelp = !m.showHelp
	case "r":
		m.refresh()
		m.setOK("已刷新")
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case "g", "home":
		m.cursor = 0
	case "G", "end":
		if n := len(m.forwards); n > 0 {
			m.cursor = n - 1
		}
	case "a":
		cmd := m.startAdd()
		return m, cmd
	case "e", "enter":
		cmd := m.startEdit()
		return m, cmd
	case " ":
		m.toggleCurrent()
	case "d", "delete", "backspace":
		if len(m.forwards) > 0 {
			m.pending = pendingDelete
			m.mode = modeConfirm
		}
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
		return m.requestQuit()
	case "esc":
		m.mode = modeList
		m.editID = ""
		m.status = ""
		return m, nil
	case "ctrl+o":
		if target, ok := pathFieldForPick(m.form.focusedIndex()); ok {
			return m.openFilePick(target)
		}
		m.setErr("请先把光标移到证书/私钥/CA 路径字段")
		return m, nil
	case "tab":
		cmd := m.form.next()
		return m, cmd
	case "shift+tab":
		cmd := m.form.prev()
		return m, cmd
	case "up":
		if moved, cmd := m.form.moveUp(); moved {
			return m, cmd
		}
	case "down":
		if moved, cmd := m.form.moveDown(); moved {
			return m, cmd
		}
	case "ctrl+s":
		editing := m.editID != ""
		wasRunning := false
		if editing {
			if fw := m.currentForward(); fw != nil {
				wasRunning = fw.Running
			}
		}
		if err := m.submitForward(); err != nil {
			m.setErr(err.Error())
			return m, nil
		}
		m.editID = ""
		m.mode = modeList
		if !editing {
			m.setOK("已添加配置，按空格启动")
		} else if wasRunning {
			m.setOK("已更新配置并停止，按空格重新启动")
		} else {
			m.setOK("已更新配置，按空格启动")
		}
		return m, nil
	}
	cmd := m.form.updateFocused(msg)
	return m, cmd
}

func pathFieldForPick(idx int) (int, bool) {
	switch idx {
	case fwCert, fwKey, fwCA:
		return idx, true
	}
	return 0, false
}

func (m model) activateAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case actionPickCert:
		return m.openFilePick(fwCert)
	case actionPickKey:
		return m.openFilePick(fwKey)
	case actionPickCA:
		return m.openFilePick(fwCA)
	}
	return m, nil
}

func (m model) openFilePick(target int) (tea.Model, tea.Cmd) {
	m.pickTarget = target
	m.mode = modeFilePick
	m.status = ""
	m.isErr = false
	if dir := m.pickStartDir(target); dir != "" {
		m.filePicker.CurrentDirectory = dir
	}
	return m, m.filePicker.Init()
}

func (m model) pickStartDir(target int) string {
	if target >= 0 && target < len(m.form.fields) {
		if v := m.form.fields[target].text(); v != "" {
			if st, err := os.Stat(v); err == nil {
				if st.IsDir() {
					return v
				}
				return filepath.Dir(v)
			}
			if dir := filepath.Dir(v); dir != "" && dir != "." {
				if st, err := os.Stat(dir); err == nil && st.IsDir() {
					return dir
				}
			}
		}
	}
	return ""
}

func (m model) updateFilePick(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.mode = modeForm
			return m, nil
		case "ctrl+c":
			return m.requestQuit()
		}
	}
	var cmd tea.Cmd
	m.filePicker, cmd = m.filePicker.Update(msg)
	if didSelect, path := m.filePicker.DidSelectFile(msg); didSelect {
		m.applyPickedFile(path)
		m.mode = modeForm
		if m.pickTarget >= 0 && m.pickTarget < len(m.form.fields) {
			m.form.focus = m.pickTarget
		}
		return m, m.form.applyFocus()
	}
	return m, cmd
}

func (m *model) applyPickedFile(path string) {
	if m.pickTarget < 0 || m.pickTarget >= len(m.form.fields) {
		return
	}
	m.form.fields[m.pickTarget].setText(path)
}

func (m model) filePickView() string {
	header := titleStyle.Render(pickTitle(m.pickTarget))
	dir := dimStyle.Render(m.filePicker.CurrentDirectory)
	return header + "\n" + dir + "\n\n" + m.filePicker.View() + "\n" +
		helpStyle.Render("↑↓ 移动 · enter 进入目录/选择文件 · ←/backspace 上级目录 · esc 取消")
}

func pickTitle(target int) string {
	switch target {
	case fwCert:
		return "选择客户端证书"
	case fwKey:
		return "选择客户端私钥"
	case fwCA:
		return "选择 CA 证书"
	}
	return "选择证书文件"
}

func (m model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	kind := m.pending
	m.pending = pendingNone
	if msg.String() == "y" || msg.String() == "Y" {
		switch kind {
		case pendingDelete:
			m.deleteCurrent()
		case pendingQuit:
			m.ctl.Close()
			return m, tea.Quit
		}
	}
	m.mode = modeList
	return m, nil
}

func (m model) requestQuit() (tea.Model, tea.Cmd) {
	if m.runningCount() > 0 {
		m.pending = pendingQuit
		m.mode = modeConfirm
		return m, nil
	}
	m.ctl.Close()
	return m, tea.Quit
}

func (m *model) startAdd() tea.Cmd {
	m.form = forwardForm()
	m.editID = ""
	m.mode = modeForm
	m.status = ""
	m.isErr = false
	return m.form.focusFirst()
}

func (m *model) startEdit() tea.Cmd {
	fw := m.currentForward()
	if fw == nil {
		return nil
	}
	m.form = forwardFormFromStatus(*fw)
	m.editID = fw.ID
	m.mode = modeForm
	m.status = ""
	m.isErr = false
	return m.form.focusFirst()
}

func forwardFormFromStatus(fw mtls.ForwardStatus) form {
	f := forwardForm()
	f.title = "编辑转发配置: " + forwardName(fw)
	f.fields[fwName].setText(fw.Name)
	if fw.Mode == "tcp" {
		f.fields[fwMode].choice = 1
	}
	f.fields[fwListen].setText(fw.Listen)
	f.fields[fwTarget].setText(fw.Target)
	f.fields[fwCert].setText(fw.CertPath)
	f.fields[fwKey].setText(fw.KeyPath)
	f.fields[fwCA].setText(fw.CAPath)
	f.fields[fwSNI].setText(fw.SNI)
	f.fields[fwALPN].setText(fw.ALPN)
	timeout := fw.TimeoutSeconds
	if timeout <= 0 {
		timeout = 10
	}
	f.fields[fwTimeout].setText(strconv.Itoa(timeout))
	f.fields[fwInsecure].boolean = fw.Insecure
	return f
}

func forwardForm() form {
	name := newText("名称", "测试环境 API")
	mode := newChoice("模式", []string{"HTTP 反向代理", "TCP 隧道"}, 0)
	listen := newText("监听地址", "127.0.0.1:8000")
	target := newText("目标地址", "https://api.example.com:8443")
	cert := newText("证书路径", "/path/client.crt")
	key := newText("私钥路径", "留空则与证书相同")
	ca := newText("CA 路径", "/path/ca.crt")
	sni := newText("SNI 覆盖", "默认取目标主机名")
	alpn := newText("ALPN", "gRPC 用 h2")
	timeout := newText("超时（秒）", "10")
	timeout.setText("10")
	insecure := newBool("跳过校验", false)

	f := form{title: "新增转发配置"}
	f.add(name, mode, listen, target, cert, key, ca, sni, alpn, timeout, insecure,
		newButton("选择证书文件…", actionPickCert),
		newButton("选择私钥文件…", actionPickKey),
		newButton("选择 CA 文件…", actionPickCA),
	)
	return f
}

func (m *model) submitForward() error {
	req, err := m.formRequest()
	if err != nil {
		return err
	}
	var fw mtls.ForwardStatus
	if m.editID != "" {
		fw, err = m.ctl.Update(m.editID, req)
	} else {
		fw, err = m.ctl.Add(req)
	}
	if err != nil {
		return err
	}
	m.refresh()
	for i, item := range m.forwards {
		if item.ID == fw.ID {
			m.cursor = i
			break
		}
	}
	return nil
}

func (m model) formRequest() (mtls.ForwardRequest, error) {
	fields := m.form.fields
	if len(fields) <= fwInsecure {
		return mtls.ForwardRequest{}, fmt.Errorf("表单字段不完整")
	}
	listen := fields[fwListen].text()
	if listen == "" {
		return mtls.ForwardRequest{}, fmt.Errorf("请填写监听地址")
	}
	target := fields[fwTarget].text()
	if target == "" {
		return mtls.ForwardRequest{}, fmt.Errorf("请填写目标地址")
	}
	timeout := 10
	if v := fields[fwTimeout].text(); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return mtls.ForwardRequest{}, fmt.Errorf("超时必须是正整数秒")
		}
		timeout = n
	}
	mode := "http"
	if fields[fwMode].choice == 1 {
		mode = "tcp"
	}
	return mtls.ForwardRequest{
		Name:           fields[fwName].text(),
		Mode:           mode,
		Listen:         listen,
		Target:         target,
		CertPath:       fields[fwCert].text(),
		KeyPath:        fields[fwKey].text(),
		CAPath:         fields[fwCA].text(),
		SNI:            fields[fwSNI].text(),
		ALPN:           fields[fwALPN].text(),
		TimeoutSeconds: timeout,
		Insecure:       fields[fwInsecure].boolean,
	}, nil
}

func (m *model) toggleCurrent() {
	fw := m.currentForward()
	if fw == nil {
		return
	}
	name := forwardName(*fw)
	if fw.Running {
		if _, err := m.ctl.StopForward(fw.ID); err != nil {
			m.setErr("停止失败: " + err.Error())
			return
		}
		m.setOK("已停止 " + name)
	} else {
		st, err := m.ctl.StartForward(fw.ID)
		if err != nil {
			m.setErr("启动失败: " + err.Error())
			return
		}
		listen := st.ActualListen
		if listen == "" {
			listen = st.Listen
		}
		m.setOK("已启动 " + name + "，监听 " + listen)
	}
	m.refresh()
}

func (m *model) deleteCurrent() {
	fw := m.currentForward()
	if fw == nil {
		return
	}
	name := forwardName(*fw)
	if err := m.ctl.DeleteForward(fw.ID); err != nil {
		m.setErr("删除失败: " + err.Error())
		return
	}
	m.refresh()
	m.setOK("已删除 " + name)
}

func (m *model) refresh() {
	m.forwards = m.ctl.Status().Forwards
	m.clampCursor()
}

func (m *model) clampCursor() {
	n := len(m.forwards)
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
	n := len(m.forwards)
	if n == 0 {
		m.cursor = 0
		return
	}
	m.cursor += delta
	m.clampCursor()
}

func (m model) currentForward() *mtls.ForwardStatus {
	if m.cursor < 0 || m.cursor >= len(m.forwards) {
		return nil
	}
	fw := m.forwards[m.cursor]
	return &fw
}

func (m model) runningCount() int {
	n := 0
	for _, fw := range m.forwards {
		if fw.Running {
			n++
		}
	}
	return n
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
		return m.listViewWith(errStyle.Render(m.confirmText() + " (y/n)"))
	case modeFilePick:
		return m.filePickView()
	default:
		return m.listView()
	}
}

func (m model) confirmText() string {
	switch m.pending {
	case pendingDelete:
		if fw := m.currentForward(); fw != nil {
			return fmt.Sprintf("确定删除 %q ?", forwardName(*fw))
		}
		return "确定删除该项 ?"
	case pendingQuit:
		return "有正在运行的转发，确定退出并停止 ?"
	}
	return "确定 ?"
}

func (m model) listView() string {
	return m.listViewWith("")
}

func (m model) listViewWith(extra string) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("mtlsproxy"))
	b.WriteString("  ")
	b.WriteString(dimStyle.Render("TUI 控制台 · 多路转发 · 客户端证书代持"))
	b.WriteString("\n\n")
	b.WriteString(m.bodyView())
	b.WriteString("\n")
	if detail := m.detailView(); detail != "" {
		b.WriteString(detail)
		b.WriteString("\n")
	}
	if m.status != "" {
		if m.isErr {
			b.WriteString(errStyle.Render(m.status))
		} else {
			b.WriteString(okStyle.Render(m.status))
		}
		b.WriteString("\n")
	}
	content := strings.TrimRight(b.String(), "\n")
	if extra != "" {
		content += "\n\n" + extra
	}
	footer := helpStyle.Render(m.helpLine())
	if m.showHelp {
		footer += "\n\n" + m.helpView()
	}
	return pinBottom(content, footer, m.height)
}

func pinBottom(content, footer string, height int) string {
	pad := height - lipgloss.Height(content) - lipgloss.Height(footer)
	if pad < 0 {
		pad = 0
	}
	return content + "\n" + strings.Repeat("\n", pad) + footer
}

func (m model) bodyView() string {
	if len(m.forwards) == 0 {
		return dimStyle.Render("暂无转发配置，按 'a' 新增。") + "\n"
	}
	names := make([]string, 0, len(m.forwards))
	for _, fw := range m.forwards {
		names = append(names, forwardName(fw))
	}
	nameWidth := nameColWidth(names, m.width)
	start, end := window(len(m.forwards), m.cursor, m.listRows())
	var b strings.Builder
	for i := start; i < end; i++ {
		fw := m.forwards[i]
		mark := "[off]"
		markStyle := dimStyle
		if fw.Running {
			mark = "[on ]"
			markStyle = okStyle
		}
		if fw.Error != "" {
			markStyle = errStyle
		}
		mode := "HTTP"
		if fw.Mode == "tcp" {
			mode = "TCP"
		}
		listen := fw.Listen
		if fw.Running && fw.ActualListen != "" {
			listen = fw.ActualListen
		}
		detail := listen + " → " + fw.Target
		if fw.Running {
			detail += "  ·  " + fmtUptime(fw.UptimeSeconds)
		}
		if fw.Error != "" {
			detail += "  ⚠"
		}
		name := padName(forwardName(fw), nameWidth)
		if i == m.cursor {
			plain := mark + " " + name + "  " + fmt.Sprintf("%-4s", mode) + "  " + detail
			b.WriteString(selectedStyle.Render("› " + truncate(plain, m.width-2)))
		} else {
			rest := fmt.Sprintf("%-4s", mode) + "  " + detail
			b.WriteString("  ")
			b.WriteString(markStyle.Render(mark))
			b.WriteString(" ")
			b.WriteString(name)
			b.WriteString("  ")
			b.WriteString(dimStyle.Render(truncate(rest, m.columnRest(nameWidth))))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (m model) columnRest(nameWidth int) int {
	w := m.width - nameWidth - 8
	if w < 16 {
		return 16
	}
	return w
}

func (m model) detailView() string {
	fw := m.currentForward()
	if fw == nil {
		return ""
	}
	var lines []string
	if fw.Cert != nil {
		s := "证书：" + fw.Cert.Subject
		if !fw.Cert.NotAfter.IsZero() {
			s += "（有效期至 " + fw.Cert.NotAfter.Format("2006-01-02") + "）"
		}
		if fw.Cert.Expired {
			s += " 已过期"
		}
		if fw.Cert.NotYetValid {
			s += " 尚未生效"
		}
		lines = append(lines, dimStyle.Render(truncate(s, m.width)))
	}
	if fw.Error != "" {
		lines = append(lines, errStyle.Render(truncate("错误："+fw.Error, m.width)))
	}
	return strings.Join(lines, "\n")
}

func (m model) listRows() int {
	rows := m.height - 10
	if m.showHelp {
		rows -= 8
	}
	if rows < 3 {
		rows = 3
	}
	return rows
}

func (m model) helpLine() string {
	return "a 新增 · enter/e 编辑 · space 启停 · d 删除 · r 刷新 · ? 帮助 · q 退出"
}

func (m model) helpView() string {
	return dimStyle.Render(
		"提示：\n" +
			"  · 控制台在内存中维护多条转发，退出后配置清空。\n" +
			"  · 新增后按空格启动；运行中编辑会先停止，保存后需再次按空格启动。\n" +
			"  · 删除运行中的转发会先停止它。\n" +
			"  · 证书/私钥/CA 可聚焦字段按 ctrl+o 或用表单底部按钮选择文件。\n" +
			"  · HTTP 模式浏览器访问监听地址；TCP 模式客户端连明文端口，gRPC 加 ALPN=h2。\n" +
			"  · 监听地址冲突会在启动时报错；端口 0 表示随机端口。")
}

func forwardName(fw mtls.ForwardStatus) string {
	if fw.Name != "" {
		return fw.Name
	}
	return fw.ID
}

func fmtUptime(sec int64) string {
	if sec < 0 {
		sec = 0
	}
	h := sec / 3600
	m := (sec % 3600) / 60
	s := sec % 60
	if h > 0 {
		return fmt.Sprintf("%d 小时 %d 分", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%d 分 %d 秒", m, s)
	}
	return fmt.Sprintf("%d 秒", s)
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
	w := 10
	for _, n := range names {
		if c := lipgloss.Width(n); c > w {
			w = c
		}
	}
	if w > 20 {
		w = 20
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
	s = truncate(s, width)
	if w := lipgloss.Width(s); w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r)) > width {
		r = r[:len(r)-1]
	}
	return string(r)
}
