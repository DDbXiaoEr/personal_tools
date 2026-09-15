package sshconfig

import (
	"os"
	"path/filepath"
	"testing"
)

const sample = `# global
Host *
    ServerAliveInterval 60

Host web
    HostName 10.0.0.1
    User deploy
    Port 2222
    IdentityFile ~/.ssh/id_ed25519
    ForwardAgent yes

Host db
    HostName 10.0.0.2
`

func TestParseRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(sample), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Hosts) != 3 {
		t.Fatalf("want 3 hosts, got %d", len(cfg.Hosts))
	}
	if got := cfg.Hosts[1].Name; got != "web" {
		t.Fatalf("host name = %q", got)
	}
	if got := cfg.Hosts[1].Get("Port"); got != "2222" {
		t.Fatalf("port = %q", got)
	}
	if got := cfg.Hosts[1].Get("ForwardAgent"); got != "yes" {
		t.Fatalf("forwardagent = %q", got)
	}

	if err := cfg.SaveFile(path); err != nil {
		t.Fatal(err)
	}
	cfg2, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg2.Hosts) != 3 {
		t.Fatalf("roundtrip hosts = %d", len(cfg2.Hosts))
	}
	if got := cfg2.Hosts[1].Get("IdentityFile"); got != "~/.ssh/id_ed25519" {
		t.Fatalf("roundtrip identity = %q", got)
	}
	if len(cfg2.Prologue) == 0 {
		t.Fatalf("prologue lost")
	}
}

func TestSetRemovesEmpty(t *testing.T) {
	h := &Host{Name: "x"}
	h.Set("User", "root")
	h.Set("User", "")
	if got := h.Get("User"); got != "" {
		t.Fatalf("want empty, got %q", got)
	}
}
