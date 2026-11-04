package sshconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const templateSample = `# sshman-template: prod	root	2222	~/.ssh/id_ed25519
# global
Host web
    HostName 10.0.0.1
    User deploy

# sshman-template: aws	ubuntu	22	/path with space/id_rsa
Host db
    HostName 10.0.0.2
`

func TestParseTemplates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(templateSample), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Templates) != 2 {
		t.Fatalf("templates = %d", len(cfg.Templates))
	}
	if got := cfg.Templates[0]; got.Name != "prod" || got.User != "root" || got.Port != "2222" || got.IdentityFile != "~/.ssh/id_ed25519" {
		t.Fatalf("template[0] = %+v", got)
	}
	if got := cfg.Templates[1]; got.Name != "aws" || got.IdentityFile != "/path with space/id_rsa" {
		t.Fatalf("template[1] = %+v", got)
	}
	if len(cfg.Hosts) != 2 {
		t.Fatalf("hosts = %d", len(cfg.Hosts))
	}
	if got := cfg.Hosts[0].Get("User"); got != "deploy" {
		t.Fatalf("host user = %q", got)
	}
	for _, line := range cfg.Prologue {
		if strings.Contains(line, templatePrefix) {
			t.Fatalf("template leaked into prologue: %q", line)
		}
	}
}

func TestTemplateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(templateSample), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveFile(path); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), templatePrefix); got != 2 {
		t.Fatalf("template marker count = %d", got)
	}

	cfg2, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg2.Templates) != 2 {
		t.Fatalf("roundtrip templates = %d", len(cfg2.Templates))
	}
	if got := cfg2.Templates[1].IdentityFile; got != "/path with space/id_rsa" {
		t.Fatalf("roundtrip identity = %q", got)
	}
	if len(cfg2.Hosts) != 2 {
		t.Fatalf("roundtrip hosts = %d", len(cfg2.Hosts))
	}
}

func TestTemplateApplyTo(t *testing.T) {
	tpl := &Template{Name: "prod", User: "root", Port: "2222", IdentityFile: "~/.ssh/id"}
	h := &Host{Name: "web"}
	tpl.ApplyTo(h)
	if h.Get("User") != "root" || h.Get("Port") != "2222" || h.Get("IdentityFile") != "~/.ssh/id" {
		t.Fatalf("host = %+v", h.Options)
	}
}

func TestUniqueTemplateName(t *testing.T) {
	templates := []*Template{{Name: "prod"}, {Name: "prod-2"}}
	if got := UniqueTemplateName(templates, "aws"); got != "aws" {
		t.Fatalf("unused = %q", got)
	}
	if got := UniqueTemplateName(templates, "prod"); got != "prod-3" {
		t.Fatalf("used = %q", got)
	}
}

func TestParseTemplateLine(t *testing.T) {
	line := "# SSHMAN-TEMPLATE: prod	root	22	~/.ssh/id_rsa"
	tpl, ok := ParseTemplateLine(line)
	if !ok {
		t.Fatal("want template")
	}
	if tpl.Name != "prod" || tpl.User != "root" || tpl.Port != "22" || tpl.IdentityFile != "~/.ssh/id_rsa" {
		t.Fatalf("template = %+v", tpl)
	}
	if _, ok := ParseTemplateLine("# sshman-template:"); ok {
		t.Fatal("empty template should be ignored")
	}
	if _, ok := ParseTemplateLine("# normal comment"); ok {
		t.Fatal("normal comment should be ignored")
	}
}
