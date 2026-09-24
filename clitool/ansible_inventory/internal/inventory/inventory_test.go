package inventory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleINI = `[web]
web1 ansible_host=10.0.0.1 ansible_user=ubuntu ansible_port=22
web2 ansible_host=10.0.0.2 ansible_user=ubuntu

[web:vars]
http_port=80
nginx_env="prod env"

[db]
db1 ansible_host=10.0.0.10 ansible_user=root

[prod:children]
web
db
`

const sampleYAML = `all:
  children:
    web:
      hosts:
        web1:
          ansible_host: 10.0.0.1
          ansible_user: ubuntu
          ansible_port: 22
        web2:
          ansible_host: 10.0.0.2
          ansible_user: ubuntu
      vars:
        http_port: 80
    db:
      hosts:
        db1:
          ansible_host: 10.0.0.10
          ansible_user: root
`

func TestParseINI(t *testing.T) {
	inv := ParseINI(sampleINI)
	web := inv.Group("web")
	if web == nil || len(web.Hosts) != 2 {
		t.Fatalf("web = %+v", web)
	}
	h := web.Host("web1")
	if h == nil || h.Get("ansible_host") != "10.0.0.1" || h.Get("ansible_user") != "ubuntu" {
		t.Fatalf("web1 = %+v", h)
	}
	if web.GetVar("http_port") != "80" || web.GetVar("nginx_env") != "prod env" {
		t.Fatalf("web vars = %+v", web.Vars)
	}
	prod := inv.Group("prod")
	if prod == nil || strings.Join(prod.Children, ",") != "web,db" {
		t.Fatalf("prod children = %+v", prod)
	}
}

func TestParseYAML(t *testing.T) {
	inv, err := ParseYAML(sampleYAML)
	if err != nil {
		t.Fatal(err)
	}
	web := inv.Group("web")
	if web == nil || len(web.Hosts) != 2 {
		t.Fatalf("web = %+v", web)
	}
	h := web.Host("web1")
	if h == nil || h.Get("ansible_host") != "10.0.0.1" {
		t.Fatalf("web1 = %+v", h)
	}
	if web.GetVar("http_port") != "80" {
		t.Fatalf("http_port = %q", web.GetVar("http_port"))
	}
	db := inv.Group("db")
	if db == nil || db.Host("db1") == nil {
		t.Fatalf("db = %+v", db)
	}
}

func TestINIRoundTrip(t *testing.T) {
	inv := ParseINI(sampleINI)
	out := inv.EncodeINI()
	again := ParseINI(out)
	web := again.Group("web")
	if web == nil || web.Host("web1").Get("ansible_host") != "10.0.0.1" {
		t.Fatalf("roundtrip web1 missing: %s", out)
	}
	if web.GetVar("nginx_env") != "prod env" {
		t.Fatalf("quoted var lost: %s", out)
	}
	if again.Group("prod") == nil {
		t.Fatalf("prod lost: %s", out)
	}
}

func TestYAMLRoundTrip(t *testing.T) {
	inv, err := ParseYAML(sampleYAML)
	if err != nil {
		t.Fatal(err)
	}
	out, err := inv.EncodeYAML()
	if err != nil {
		t.Fatal(err)
	}
	again, err := ParseYAML(out)
	if err != nil {
		t.Fatal(err)
	}
	h := again.Group("web").Host("web1")
	if h == nil || h.Get("ansible_host") != "10.0.0.1" {
		t.Fatalf("roundtrip failed: %s", out)
	}
}

func TestLoadMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hosts")
	inv, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if inv.Path != path || len(inv.Groups) != 0 {
		t.Fatalf("empty load = %+v", inv)
	}
}

func TestSaveBackupAndCRUD(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hosts")
	if err := os.WriteFile(path, []byte(sampleINI), 0644); err != nil {
		t.Fatal(err)
	}
	inv, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := inv.AddHost("app", "app1", []KV{{Key: "ansible_host", Value: "10.1.1.1"}}); err != nil {
		t.Fatal(err)
	}
	if err := inv.AddVar("app", "role", "api"); err != nil {
		t.Fatal(err)
	}
	if err := inv.Save(); err != nil {
		t.Fatal(err)
	}
	bak, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatal(err)
	}
	if string(bak) != sampleINI {
		t.Fatalf("bak mismatch: %q", bak)
	}
	again, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	app := again.Group("app")
	if app == nil || app.Host("app1") == nil || app.GetVar("role") != "api" {
		t.Fatalf("saved app = %+v", app)
	}
	refs := again.HostRefs()
	var ref HostRef
	found := false
	for _, r := range refs {
		_, h := again.HostAt(r)
		if h != nil && h.Name == "app1" {
			ref = r
			found = true
			break
		}
	}
	if !found {
		t.Fatal("app1 ref missing")
	}
	if err := again.DeleteHost(ref); err != nil {
		t.Fatal(err)
	}
	if again.Group("app").Host("app1") != nil {
		t.Fatal("host not deleted")
	}
}

func TestDeleteGroupMovesHosts(t *testing.T) {
	inv := ParseINI("[web]\nweb1 ansible_host=1.1.1.1\n")
	if err := inv.DeleteGroup("web"); err != nil {
		t.Fatal(err)
	}
	if inv.Group("web") != nil {
		t.Fatal("web still present")
	}
	u := inv.Group(GroupUngrouped)
	if u == nil || u.Host("web1") == nil {
		t.Fatalf("host not moved: %+v", u)
	}
}

func TestDetectProjectFrom(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "play")
	if err := os.MkdirAll(proj, 0755); err != nil {
		t.Fatal(err)
	}
	invPath := filepath.Join(proj, "inventory.yml")
	if err := os.WriteFile(invPath, []byte("web:\n  hosts:\n    a: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got := DetectProjectFrom(proj)
	if got != invPath {
		t.Fatalf("got %q want %q", got, invPath)
	}
	cfg := filepath.Join(proj, "ansible.cfg")
	if err := os.WriteFile(cfg, []byte("[defaults]\ninventory = hosts.ini\n"), 0644); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(proj, "hosts.ini")
	got = DetectProjectFrom(proj)
	if got != want {
		t.Fatalf("cfg inventory = %q want %q", got, want)
	}
}

func TestFormatFromPath(t *testing.T) {
	if formatFromPath("inv.yml", "") != FormatYAML {
		t.Fatal("yml")
	}
	if formatFromPath("hosts", "[web]\na\n") != FormatINI {
		t.Fatal("ini content")
	}
	if formatFromPath("hosts", "---\nweb:\n  hosts: {}\n") != FormatYAML {
		t.Fatal("yaml content")
	}
}

func TestQuotedInlineVars(t *testing.T) {
	inv := ParseINI("h1 ansible_host=10.0.0.1 note=\"hello world\" empty=\"\"\n")
	h := inv.Group(GroupUngrouped).Host("h1")
	if h.Get("note") != "hello world" {
		t.Fatalf("note = %q", h.Get("note"))
	}
	out := inv.EncodeINI()
	if !strings.Contains(out, `note="hello world"`) {
		t.Fatalf("encode lost quotes: %s", out)
	}
}
