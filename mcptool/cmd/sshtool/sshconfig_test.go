package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseSSHConfigResolve(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")
	content := `
Host web
    HostName 10.0.0.1
    User deploy
    Port 2222
    IdentityFile ~/.ssh/id_ed25519
    ConnectTimeout 12

Host db web-db
    HostName 10.0.0.2
    User dbadmin

Host *
    ServerAliveInterval 60
    User fallback
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadSSHConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	web := cfg.resolve("web")
	if web.HostName != "10.0.0.1" || web.User != "deploy" || web.Port != 2222 {
		t.Fatalf("web = %+v", web)
	}
	if web.ConnectTimeout != 12*time.Second {
		t.Fatalf("timeout = %s", web.ConnectTimeout)
	}
	if len(web.IdentityFiles) != 1 {
		t.Fatalf("identity = %v", web.IdentityFiles)
	}

	db := cfg.resolve("db")
	if db.HostName != "10.0.0.2" || db.User != "dbadmin" {
		t.Fatalf("db = %+v", db)
	}

	unknown := cfg.resolve("unknown")
	if unknown.HostName != "unknown" {
		t.Fatalf("unknown hostname = %q", unknown.HostName)
	}
	if unknown.User != "fallback" {
		t.Fatalf("wildcard user = %q", unknown.User)
	}
}

func TestHostPatternNegation(t *testing.T) {
	if hostPatternsMatch([]string{"!prod", "*"}, "prod") {
		t.Fatal("prod should be excluded")
	}
	if !hostPatternsMatch([]string{"!prod", "*"}, "web") {
		t.Fatal("web should match")
	}
	if !matchSSHPattern("web-*", "web-1") {
		t.Fatal("wildcard")
	}
	if matchSSHPattern("web-*", "db-1") {
		t.Fatal("should not match")
	}
}

func TestSSHConfigInclude(t *testing.T) {
	dir := t.TempDir()
	incDir := filepath.Join(dir, "conf.d")
	if err := os.MkdirAll(incDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(incDir, "web"), []byte("Host web\n    HostName 10.1.1.1\n    User deploy\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "config")
	if err := os.WriteFile(cfgPath, []byte("Include conf.d/*\nHost *\n    Port 22\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadSSHConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	web := cfg.resolve("web")
	if web.HostName != "10.1.1.1" || web.User != "deploy" {
		t.Fatalf("included host = %+v", web)
	}
}

func TestLoadSSHConfigMissingDefault(t *testing.T) {
	cfg, err := loadSSHConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected empty config")
	}
}

func TestLoadSSHConfigMissingExplicit(t *testing.T) {
	if _, err := loadSSHConfig(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("expected error")
	}
}

func TestExpandSSHTokens(t *testing.T) {
	got := expandSSHTokens("%d/.ssh/%n", map[string]string{
		"%d": "/home/u",
		"%n": "web",
		"%%": "%",
	})
	if got != "/home/u/.ssh/web" {
		t.Fatalf("got %q", got)
	}
	if expandSSHTokens("100%%", map[string]string{"%%": "%"}) != "100%" {
		t.Fatal("percent")
	}
}

func TestParseSSHConfigLine(t *testing.T) {
	k, v, ok := parseSSHConfigLine("    HostName 10.0.0.1 # comment")
	if !ok || k != "HostName" || v != "10.0.0.1" {
		t.Fatalf("got %q %q %v", k, v, ok)
	}
	k, v, ok = parseSSHConfigLine(`IdentityFile "~/.ssh/id"`)
	if !ok || v != "~/.ssh/id" {
		t.Fatalf("quoted: %q %q", k, v)
	}
	if _, _, ok := parseSSHConfigLine("# comment"); ok {
		t.Fatal("comment should be skipped")
	}
}
