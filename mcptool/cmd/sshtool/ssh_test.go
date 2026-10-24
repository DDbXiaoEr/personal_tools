package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseSSHConn(t *testing.T) {
	prevCfg := sshConfigFile
	prevIdent := lookupDefaultIdentities
	t.Cleanup(func() {
		sshConfigFile = prevCfg
		lookupDefaultIdentities = prevIdent
	})
	lookupDefaultIdentities = func() []string { return nil }

	t.Run("missing hostname", func(t *testing.T) {
		sshConfigFile = &sshConfig{}
		_, errMsg := parseSSHConn(map[string]any{"user": "root", "password": "x"})
		if errMsg != "hostname is required" {
			t.Fatalf("got %q", errMsg)
		}
	})

	t.Run("missing credentials keeps host", func(t *testing.T) {
		sshConfigFile = &sshConfig{}
		cfg, errMsg := parseSSHConn(map[string]any{"hostname": "1.2.3.4", "user": "root"})
		if errMsg != "no identity file found; set IdentityFile in ssh config or pass password / key_path" {
			t.Fatalf("got %q", errMsg)
		}
		if cfg.host != "1.2.3.4" || cfg.user != "root" {
			t.Fatalf("host/user not preserved: %+v", cfg)
		}
		if cfg.port != 22 || cfg.timeout != 30*time.Second {
			t.Fatalf("defaults not applied: %+v", cfg)
		}
	})

	t.Run("password ok", func(t *testing.T) {
		sshConfigFile = &sshConfig{}
		cfg, errMsg := parseSSHConn(map[string]any{
			"hostname": "h", "user": "u", "password": "p", "port": 2222, "timeout": 10,
		})
		if errMsg != "" {
			t.Fatalf("unexpected error: %s", errMsg)
		}
		if cfg.port != 2222 || cfg.password != "p" || cfg.timeout != 10*time.Second {
			t.Fatalf("unexpected cfg: %+v", cfg)
		}
	})

	t.Run("hostname from ssh config", func(t *testing.T) {
		keyDir := t.TempDir()
		keyFile := filepath.Join(keyDir, "id_ed25519")
		if err := os.WriteFile(keyFile, []byte("dummy"), 0o600); err != nil {
			t.Fatal(err)
		}
		cfgPath := filepath.Join(t.TempDir(), "config")
		content := "Host web\n    HostName 10.0.0.1\n    User deploy\n    Port 2222\n    IdentityFile " + keyFile + "\n    ConnectTimeout 8\n"
		if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		loaded, err := loadSSHConfig(cfgPath)
		if err != nil {
			t.Fatal(err)
		}
		sshConfigFile = loaded

		cfg, errMsg := parseSSHConn(map[string]any{"hostname": "web"})
		if errMsg != "" {
			t.Fatalf("unexpected error: %s", errMsg)
		}
		if cfg.alias != "web" || cfg.host != "10.0.0.1" || cfg.user != "deploy" {
			t.Fatalf("resolved fields: %+v", cfg)
		}
		if cfg.port != 2222 || cfg.timeout != 8*time.Second {
			t.Fatalf("port/timeout: %+v", cfg)
		}
		if len(cfg.keyPaths) != 1 || cfg.keyPaths[0] != keyFile {
			t.Fatalf("keyPaths = %v want [%s]", cfg.keyPaths, keyFile)
		}
	})

	t.Run("overrides ssh config", func(t *testing.T) {
		keyDir := t.TempDir()
		keyFile := filepath.Join(keyDir, "id_ed25519")
		if err := os.WriteFile(keyFile, []byte("dummy"), 0o600); err != nil {
			t.Fatal(err)
		}
		cfgPath := filepath.Join(t.TempDir(), "config")
		content := "Host web\n    HostName 10.0.0.1\n    User deploy\n    Port 2222\n    IdentityFile " + keyFile + "\n"
		if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		loaded, err := loadSSHConfig(cfgPath)
		if err != nil {
			t.Fatal(err)
		}
		sshConfigFile = loaded

		cfg, errMsg := parseSSHConn(map[string]any{
			"hostname": "web", "user": "root", "port": 22, "password": "p",
		})
		if errMsg != "" {
			t.Fatalf("unexpected error: %s", errMsg)
		}
		if cfg.user != "root" || cfg.port != 22 || cfg.password != "p" {
			t.Fatalf("overrides not applied: %+v", cfg)
		}
		if cfg.host != "10.0.0.1" {
			t.Fatalf("HostName should still come from config: %q", cfg.host)
		}
	})

	t.Run("accepts host alias field", func(t *testing.T) {
		sshConfigFile = &sshConfig{}
		cfg, errMsg := parseSSHConn(map[string]any{"host": "box", "user": "u", "password": "p"})
		if errMsg != "" {
			t.Fatalf("unexpected error: %s", errMsg)
		}
		if cfg.alias != "box" || cfg.host != "box" {
			t.Fatalf("got %+v", cfg)
		}
	})
}

func TestCollectKeyPaths(t *testing.T) {
	prev := lookupDefaultIdentities
	t.Cleanup(func() { lookupDefaultIdentities = prev })

	dir := t.TempDir()
	extra := filepath.Join(dir, "extra")
	ident := filepath.Join(dir, "ident")
	def := filepath.Join(dir, "default")
	for _, p := range []string{extra, ident, def} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	lookupDefaultIdentities = func() []string { return []string{def} }

	t.Run("identity plus defaults", func(t *testing.T) {
		got, err := collectKeyPaths(resolvedHost{IdentityFiles: []string{ident}}, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0] != ident || got[1] != def {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("identities only", func(t *testing.T) {
		got, err := collectKeyPaths(resolvedHost{IdentityFiles: []string{ident}, IdentitiesOnly: true}, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0] != ident {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("key_path overrides", func(t *testing.T) {
		got, err := collectKeyPaths(resolvedHost{IdentityFiles: []string{ident}}, extra)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0] != extra {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("missing extra errors", func(t *testing.T) {
		if _, err := collectKeyPaths(resolvedHost{}, filepath.Join(dir, "missing")); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestArgString(t *testing.T) {
	got, errMsg := argString(map[string]any{"k": "v"}, "k")
	if got != "v" || errMsg != "" {
		t.Fatalf("got %q %q", got, errMsg)
	}
	got, errMsg = argString(map[string]any{}, "k")
	if got != "" || errMsg != "" {
		t.Fatalf("missing key: %q %q", got, errMsg)
	}
	_, errMsg = argString(map[string]any{"k": []int{1}}, "k")
	if errMsg == "" {
		t.Fatal("expected type error")
	}
}
