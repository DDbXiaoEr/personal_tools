package inventory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPrefsMissing(t *testing.T) {
	dir := t.TempDir()
	p, err := LoadPrefsFrom(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.DefaultInventory == "" {
		t.Fatal("expected fallback default")
	}
	if len(p.Projects) != 0 {
		t.Fatalf("projects = %+v", p.Projects)
	}
}

func TestPrefsConfigAndProjectsSeparated(t *testing.T) {
	dir := t.TempDir()
	p, err := LoadPrefsFrom(dir)
	if err != nil {
		t.Fatal(err)
	}
	inv := filepath.Join(dir, "hosts")
	if err := p.SetDefaultInventory(inv); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ConfigPath(dir)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ProjectsPath(dir)); !os.IsNotExist(err) {
		t.Fatalf("projects file should not exist yet: %v", err)
	}

	proj := filepath.Join(dir, "play", "inventory.yml")
	if err := os.MkdirAll(filepath.Dir(proj), 0755); err != nil {
		t.Fatal(err)
	}
	if err := p.AddProject("play", proj); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ProjectsPath(dir)); err != nil {
		t.Fatal(err)
	}

	again, err := LoadPrefsFrom(dir)
	if err != nil {
		t.Fatal(err)
	}
	if again.DefaultInventory != inv {
		t.Fatalf("default = %q want %q", again.DefaultInventory, inv)
	}
	if len(again.Projects) != 1 || again.Projects[0].Name != "play" || again.Projects[0].Path != proj {
		t.Fatalf("projects = %+v", again.Projects)
	}
}

func TestPrefsProjectCRUD(t *testing.T) {
	dir := t.TempDir()
	p, err := LoadPrefsFrom(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := filepath.Join(dir, "a", "hosts")
	b := filepath.Join(dir, "b", "hosts")
	if err := p.AddProject("a", a); err != nil {
		t.Fatal(err)
	}
	if err := p.AddProject("a", b); err == nil {
		t.Fatal("duplicate name")
	}
	if err := p.UpdateProject("a", "alpha", b); err != nil {
		t.Fatal(err)
	}
	if p.Project("a") != nil || p.Project("alpha") == nil {
		t.Fatalf("rename failed: %+v", p.Projects)
	}
	if err := p.DeleteProject("alpha"); err != nil {
		t.Fatal(err)
	}
	if len(p.Projects) != 0 {
		t.Fatalf("not empty: %+v", p.Projects)
	}
}

func TestParseProjectsSkipsComments(t *testing.T) {
	got := parseProjects("# x\nprod=/tmp/prod/hosts\n\n; skip\nstaging=/tmp/stg/hosts\n")
	if len(got) != 2 || got[0].Name != "prod" || got[1].Name != "staging" {
		t.Fatalf("%+v", got)
	}
}

func TestDefaultPathUsesPrefs(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, "custom-hosts")
	p, err := LoadPrefsFrom(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.SetDefaultInventory(want); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ANSIMAN_CONFIG_DIR", dir)
	if got := DefaultPath(); got != want {
		t.Fatalf("DefaultPath = %q want %q", got, want)
	}
}

func TestProjectNameFromPath(t *testing.T) {
	if got := projectNameFromPath("/opt/play/inventory.yml"); got != "play" {
		t.Fatalf("got %q", got)
	}
	if got := projectNameFromPath("/opt/play/custom.ini"); got != "custom" {
		t.Fatalf("got %q", got)
	}
}
