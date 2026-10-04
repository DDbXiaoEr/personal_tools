package main

import (
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
		if errMsg != "either password or key_path must be provided" {
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
