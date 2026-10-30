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
