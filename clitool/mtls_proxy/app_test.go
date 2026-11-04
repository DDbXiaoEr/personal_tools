package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mtlsproxy/internal/mtls"
)

func TestForwardFormSubmitAndToggle(t *testing.T) {
	ctl := mtls.NewControl()
	defer ctl.Close()
	m := newModel(ctl)

	m.form = forwardForm()
	m.form.fields[fwName].setText("tui 测试")
	m.form.fields[fwListen].setText("127.0.0.1:0")
	m.form.fields[fwTarget].setText("https://127.0.0.1:1")
	if err := m.submitForward(); err != nil {
		t.Fatal(err)
	}
	if len(m.forwards) != 1 {
		t.Fatalf("want 1 forward, got %d", len(m.forwards))
	}
	if fw := m.forwards[0]; fw.Name != "tui 测试" || fw.Mode != "http" || fw.Running {
		t.Fatalf("bad forward: %+v", fw)
	}

	m.toggleCurrent()
	if len(m.forwards) != 1 || !m.forwards[0].Running || m.forwards[0].ActualListen == "" {
		t.Fatalf("start failed: %+v", m.forwards)
	}

	m.toggleCurrent()
	if m.forwards[0].Running {
		t.Fatalf("stop failed: %+v", m.forwards[0])
	}

	m.deleteCurrent()
	if len(m.forwards) != 0 {
		t.Fatalf("delete failed: %+v", m.forwards)
	}
}

func TestForwardFormRequestMapping(t *testing.T) {
	m := newModel(mtls.NewControl())
	m.form = forwardForm()
	m.form.fields[fwName].setText("grpc")
	m.form.fields[fwMode].choice = 1
	m.form.fields[fwListen].setText("127.0.0.1:9000")
	m.form.fields[fwTarget].setText("example.com:50051")
	m.form.fields[fwCert].setText("/tmp/client.crt")
	m.form.fields[fwKey].setText("/tmp/client.key")
	m.form.fields[fwCA].setText("/tmp/ca.crt")
	m.form.fields[fwSNI].setText("api.example.com")
	m.form.fields[fwALPN].setText("h2,http/1.1")
	m.form.fields[fwTimeout].setText("3")
	m.form.fields[fwInsecure].boolean = true

	req, err := m.formRequest()
	if err != nil {
		t.Fatal(err)
	}
	if req.Mode != "tcp" || req.Listen != "127.0.0.1:9000" || req.Target != "example.com:50051" {
		t.Fatalf("bad request: %+v", req)
	}
	if req.CertPath != "/tmp/client.crt" || req.KeyPath != "/tmp/client.key" || req.CAPath != "/tmp/ca.crt" {
		t.Fatalf("bad cert paths: %+v", req)
	}
	if req.SNI != "api.example.com" || req.ALPN != "h2,http/1.1" || req.TimeoutSeconds != 3 || !req.Insecure {
		t.Fatalf("bad options: %+v", req)
	}
}

func TestForwardFormValidation(t *testing.T) {
	m := newModel(mtls.NewControl())
	m.form = forwardForm()
	m.form.fields[fwListen].setText("127.0.0.1:9000")
	m.form.fields[fwTarget].setText("https://example.com:8443")

	m.form.fields[fwTimeout].setText("abc")
	if _, err := m.formRequest(); err == nil {
		t.Fatal("invalid timeout should fail")
	}
	m.form.fields[fwTimeout].setText("10")
	m.form.fields[fwListen].setText("")
	if _, err := m.formRequest(); err == nil {
		t.Fatal("empty listen should fail")
	}
	m.form.fields[fwListen].setText("127.0.0.1:9000")
	m.form.fields[fwTarget].setText("")
	if _, err := m.formRequest(); err == nil {
		t.Fatal("empty target should fail")
	}
}

func TestViewModelRenders(t *testing.T) {
	ctl := mtls.NewControl()
	defer ctl.Close()
	m := newModel(ctl)
	m.width, m.height = 80, 24

	if out := m.View(); !strings.Contains(out, "暂无") {
		t.Fatalf("empty view: %s", out)
	}

	m.form = forwardForm()
	m.form.fields[fwName].setText("tui 测试")
	m.form.fields[fwListen].setText("127.0.0.1:0")
	m.form.fields[fwTarget].setText("https://127.0.0.1:1")
	if err := m.submitForward(); err != nil {
		t.Fatal(err)
	}
	if out := m.View(); !strings.Contains(out, "tui 测试") || !strings.Contains(out, "127.0.0.1:0") {
		t.Fatalf("list view: %s", out)
	}

	m.mode = modeForm
	if out := m.View(); !strings.Contains(out, "新增转发配置") {
		t.Fatalf("form view: %s", out)
	}

	m.mode = modeConfirm
	m.pending = pendingDelete
	if out := m.View(); !strings.Contains(out, "确定删除") {
		t.Fatalf("confirm view: %s", out)
	}
}

func TestFilePickFillsPathField(t *testing.T) {
	m := newModel(mtls.NewControl())
	m.form = forwardForm()

	if target, ok := pathFieldForPick(fwCert); !ok || target != fwCert {
		t.Fatalf("cert field should be pickable")
	}
	if _, ok := pathFieldForPick(fwName); ok {
		t.Fatalf("name field should not be pickable")
	}

	m.pickTarget = fwCA
	m.applyPickedFile("/tmp/ca.pem")
	if got := m.form.fields[fwCA].text(); got != "/tmp/ca.pem" {
		t.Fatalf("picked path not applied: %q", got)
	}
}

func TestFormKeyNavigation(t *testing.T) {
	ctl := mtls.NewControl()
	defer ctl.Close()
	m := newModel(ctl)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	got := next.(model)
	if got.mode != modeForm || got.form.focus != fwName {
		t.Fatalf("a should open form at first field: mode=%v focus=%d", got.mode, got.form.focus)
	}

	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyTab})
	got = next.(model)
	if got.form.focus != fwMode {
		t.Fatalf("tab should move focus to mode, got %d", got.form.focus)
	}

	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyTab})
	got = next.(model)
	if got.form.focus != fwListen {
		t.Fatalf("tab should move focus to listen, got %d", got.form.focus)
	}

	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	got = next.(model)
	if got.form.focus != fwMode {
		t.Fatalf("shift+tab should move focus back to mode, got %d", got.form.focus)
	}

	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got = next.(model)
	if got.mode != modeList {
		t.Fatalf("esc should return to list, got %v", got.mode)
	}
}

func TestForwardEdit(t *testing.T) {
	ctl := mtls.NewControl()
	defer ctl.Close()
	m := newModel(ctl)

	m.form = forwardForm()
	m.form.fields[fwName].setText("原始配置")
	m.form.fields[fwListen].setText("127.0.0.1:0")
	m.form.fields[fwTarget].setText("https://127.0.0.1:1")
	if err := m.submitForward(); err != nil {
		t.Fatal(err)
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	got := next.(model)
	if got.mode != modeForm || got.editID == "" {
		t.Fatalf("e should open edit form: mode=%v editID=%q", got.mode, got.editID)
	}
	if got.form.fields[fwName].text() != "原始配置" || got.form.fields[fwListen].text() != "127.0.0.1:0" {
		t.Fatalf("form not prefilled: name=%q listen=%q",
			got.form.fields[fwName].text(), got.form.fields[fwListen].text())
	}
	if out := got.View(); !strings.Contains(out, "编辑转发配置") {
		t.Fatalf("edit title missing: %s", out)
	}

	got.form.fields[fwName].setText("改过的配置")
	got.form.fields[fwMode].choice = 1
	got.form.fields[fwTarget].setText("example.com:8443")
	if err := got.submitForward(); err != nil {
		t.Fatal(err)
	}
	if len(got.forwards) != 1 {
		t.Fatalf("edit should not add entry: %d", len(got.forwards))
	}
	if fw := got.forwards[0]; fw.Name != "改过的配置" || fw.Mode != "tcp" || fw.Target != "example.com:8443" {
		t.Fatalf("bad updated forward: %+v", fw)
	}
}

func TestFormEditSavesStatus(t *testing.T) {
	ctl := mtls.NewControl()
	defer ctl.Close()
	m := newModel(ctl)

	m.form = forwardForm()
	m.form.fields[fwListen].setText("127.0.0.1:0")
	m.form.fields[fwTarget].setText("https://127.0.0.1:1")
	if err := m.submitForward(); err != nil {
		t.Fatal(err)
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	got := next.(model)
	got.form.fields[fwName].setText("ctrl+s 修改")

	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	got = next.(model)
	if got.mode != modeList || got.editID != "" {
		t.Fatalf("ctrl+s should finish edit: mode=%v editID=%q", got.mode, got.editID)
	}
	if fw := got.forwards[0]; fw.Name != "ctrl+s 修改" {
		t.Fatalf("name not updated: %+v", fw)
	}
	if !strings.Contains(got.status, "已更新配置") {
		t.Fatalf("unexpected status: %s", got.status)
	}
}

func TestHelpLinePinnedToBottom(t *testing.T) {
	ctl := mtls.NewControl()
	defer ctl.Close()
	m := newModel(ctl)
	m.width, m.height = 80, 24

	out := m.View()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 24 {
		t.Fatalf("want 24 lines, got %d:\n%s", len(lines), out)
	}
	if last := lines[len(lines)-1]; !strings.Contains(last, "a 新增") {
		t.Fatalf("help not on bottom line: %q", last)
	}

	m.showHelp = true
	out = m.View()
	if !strings.Contains(out, "提示：") {
		t.Fatalf("expanded help missing: %s", out)
	}
}
