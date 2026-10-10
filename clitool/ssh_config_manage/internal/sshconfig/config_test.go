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

func TestCloneIndependent(t *testing.T) {
	h := &Host{Name: "web", Options: []Option{{Key: "User", Value: "root"}}}
	c := h.Clone()
	c.Name = "web-2"
	c.Set("User", "deploy")
	if h.Name != "web" {
		t.Fatalf("orig name mutated: %q", h.Name)
	}
	if got := h.Get("User"); got != "root" {
		t.Fatalf("orig user mutated: %q", got)
	}
}

func TestUniqueHostName(t *testing.T) {
	hosts := []*Host{{Name: "web"}, {Name: "web-2"}}
	if got := UniqueHostName(hosts, "db"); got != "db" {
		t.Fatalf("unused = %q", got)
	}
	if got := UniqueHostName(hosts, "web"); got != "web-3" {
		t.Fatalf("used = %q", got)
	}
}

func TestParseBulkLines(t *testing.T) {
	text := `
# comment
web 10.0.0.1
db 10.0.0.2 root 2222
10.0.0.3
`
	got, err := ParseBulkLines(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Alias != "web" || got[0].HostName != "10.0.0.1" {
		t.Fatalf("web = %+v", got[0])
	}
	if got[1].User != "root" || got[1].Port != "2222" {
		t.Fatalf("db = %+v", got[1])
	}
	if got[2].Alias != "10.0.0.3" || got[2].HostName != "10.0.0.3" {
		t.Fatalf("ip = %+v", got[2])
	}
}

func TestParseBulkLinesErrors(t *testing.T) {
	if _, err := ParseBulkLines("\n# x\n"); err == nil {
		t.Fatal("want empty error")
	}
	if _, err := ParseBulkLines("a 1\na 2"); err == nil {
		t.Fatal("want duplicate error")
	}
	if _, err := ParseBulkLines("a b c d e"); err == nil {
		t.Fatal("want too many fields")
	}
}

func TestBulkEntryToHost(t *testing.T) {
	shared := []Option{{Key: "User", Value: "ubuntu"}, {Key: "IdentityFile", Value: "~/.ssh/id"}}
	h := BulkEntry{Alias: "db", HostName: "10.0.0.2", User: "root", Port: "2222"}.ToHost(shared)
	if h.Name != "db" {
		t.Fatalf("name = %q", h.Name)
	}
	if got := h.Get("User"); got != "root" {
		t.Fatalf("user override = %q", got)
	}
	if got := h.Get("Port"); got != "2222" {
		t.Fatalf("port = %q", got)
	}
	if got := h.Get("IdentityFile"); got != "~/.ssh/id" {
		t.Fatalf("shared identity = %q", got)
	}
	if got := h.Get("HostName"); got != "10.0.0.2" {
		t.Fatalf("hostname = %q", got)
	}
}
