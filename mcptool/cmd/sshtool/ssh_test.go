package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseSSHConn(t *testing.T) {
	t.Run("missing host", func(t *testing.T) {
		_, errMsg := parseSSHConn(map[string]any{"user": "root", "password": "x"}, 22, 30)
		if errMsg != "host and user are required" {
			t.Fatalf("got %q", errMsg)
		}
	})
	t.Run("missing credentials keeps host", func(t *testing.T) {
		cfg, errMsg := parseSSHConn(map[string]any{"host": "1.2.3.4", "user": "root"}, 0, 0)
		if errMsg != "either password, key_path, or key_name must be provided" {
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
		cfg, errMsg := parseSSHConn(map[string]any{
			"host": "h", "user": "u", "password": "p",
		}, 2222, 10)
		if errMsg != "" {
			t.Fatalf("unexpected error: %s", errMsg)
		}
		if cfg.port != 2222 || cfg.password != "p" {
			t.Fatalf("unexpected cfg: %+v", cfg)
		}
	})
	t.Run("key_name resolves path", func(t *testing.T) {
		prev := keyPairIndex
		t.Cleanup(func() { keyPairIndex = prev })
		keyDir := t.TempDir()
		keyFile := filepath.Join(keyDir, "id_rsa")
		if err := os.WriteFile(keyFile, []byte("dummy"), 0o600); err != nil {
			t.Fatal(err)
		}
		keyPairIndex = map[string]string{"demookey": keyFile}
		cfg, errMsg := parseSSHConn(map[string]any{
			"host": "h", "user": "u", "key_name": "demookey",
		}, 22, 30)
		if errMsg != "" {
			t.Fatalf("unexpected error: %s", errMsg)
		}
		if cfg.keyPath != keyFile {
			t.Fatalf("keyPath = %q want %q", cfg.keyPath, keyFile)
		}
	})
	t.Run("unknown key_name", func(t *testing.T) {
		prev := keyPairIndex
		t.Cleanup(func() { keyPairIndex = prev })
		keyPairIndex = map[string]string{"demookey": "/tmp/x"}
		_, errMsg := parseSSHConn(map[string]any{
			"host": "h", "user": "u", "key_name": "missing",
		}, 22, 30)
		if errMsg != `unknown key_name "missing"` {
			t.Fatalf("got %q", errMsg)
		}
	})
	t.Run("key_name without inventory", func(t *testing.T) {
		prev := keyPairIndex
		t.Cleanup(func() { keyPairIndex = prev })
		keyPairIndex = map[string]string{}
		_, errMsg := parseSSHConn(map[string]any{
			"host": "h", "user": "u", "key_name": "demookey",
		}, 22, 30)
		if errMsg == "" {
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
