package skill

import (
	"archive/zip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseBlockScalar(t *testing.T) {
	raw := "---\n" +
		"name: adk-cheatsheet\n" +
		"description: >\n" +
		"  MUST READ before writing ADK code.\n" +
		"  Second line.\n" +
		"license: Apache-2.0\n" +
		"metadata:\n" +
		"  author: Google\n" +
		"---\n" +
		"# Body\n"

	s := Parse("adk-cheatsheet", "SKILL.md", raw)
	if s.Name != "adk-cheatsheet" {
		t.Errorf("name = %q", s.Name)
	}
	if !strings.Contains(s.Description, "MUST READ before writing ADK code.") {
		t.Errorf("description = %q", s.Description)
	}
	if s.License != "Apache-2.0" {
		t.Errorf("license = %q", s.License)
	}
	if s.Metadata["author"] != "Google" {
		t.Errorf("metadata = %v", s.Metadata)
	}
	if len(s.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", s.Warnings)
	}
	if !strings.Contains(s.Body, "# Body") {
		t.Errorf("body = %q", s.Body)
	}
}

func TestParseWarnings(t *testing.T) {
	raw := "---\nname: BadName\ndescription: \"\"\n---\nbody"
	s := Parse("other-dir", "SKILL.md", raw)
	if s.Name != "BadName" {
		t.Errorf("name = %q", s.Name)
	}
	warn := strings.Join(s.Warnings, "|")
	for _, want := range []string{"name 与目录名不一致", "name 不符合命名规范", "缺少 description"} {
		if !strings.Contains(warn, want) {
			t.Errorf("warnings %q missing %q", warn, want)
		}
	}
}

func TestParseNoFrontmatter(t *testing.T) {
	s := Parse("plain", "SKILL.md", "# just a body")
	if s.Name != "plain" {
		t.Errorf("name = %q", s.Name)
	}
	if len(s.Warnings) == 0 {
		t.Error("expected a warning about missing frontmatter")
	}
}

func TestScan(t *testing.T) {
	dir := t.TempDir()
	mk := func(name, content string) {
		if err := os.MkdirAll(filepath.Join(dir, name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name, "SKILL.md"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	mk("beta", "---\nname: beta\ndescription: b\n---\nbody")
	mk("alpha", "---\nname: alpha\ndescription: a\n---\nbody")
	// directory without SKILL.md should be skipped
	if err := os.MkdirAll(filepath.Join(dir, "empty"), 0755); err != nil {
		t.Fatal(err)
	}

	skills, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 2 {
		t.Fatalf("got %d skills: %+v", len(skills), skills)
	}
	if skills[0].Name != "alpha" || skills[1].Name != "beta" {
		t.Errorf("not sorted: %s, %s", skills[0].Name, skills[1].Name)
	}
}

func TestScanMissingDir(t *testing.T) {
	skills, err := Scan(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatal(err)
	}
	if skills != nil {
		t.Errorf("expected nil, got %v", skills)
	}
}

func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := zip.NewWriter(f)
	for name, content := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestInstallAndRemoveLocalZip(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(root, "skill.zip")
	writeZip(t, zipPath, map[string]string{
		"repo-main/SKILL.md": "---\nname: demo-skill\ndescription: d\n---\n# hi\n",
		"repo-main/notes.md": "extra",
	})
	skillsDir := filepath.Join(root, "skills")
	if err := Install(skillsDir, "demo-skill", zipPath); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(skillsDir, "demo-skill", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# hi") {
		t.Fatalf("SKILL.md content = %s", data)
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "demo-skill", "notes.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(zipPath); !os.IsNotExist(err) {
		t.Fatalf("zip still exists after install: %v", err)
	}
	writeZip(t, zipPath, map[string]string{
		"SKILL.md": "---\nname: demo-skill\ndescription: d\n---\n# hi\n",
	})
	if err := Install(skillsDir, "demo-skill", zipPath); err == nil {
		t.Fatal("expected duplicate install to fail")
	}
	if _, err := os.Stat(zipPath); err != nil {
		t.Fatalf("zip removed after failed install: %v", err)
	}
	if err := Remove(skillsDir, "demo-skill"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "demo-skill")); !os.IsNotExist(err) {
		t.Fatalf("dir still exists: %v", err)
	}
}

func TestInstallHTTPZip(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(root, "s.zip")
	writeZip(t, zipPath, map[string]string{
		"SKILL.md": "---\nname: http-skill\ndescription: d\n---\nbody\n",
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, zipPath)
	}))
	defer srv.Close()

	skillsDir := filepath.Join(root, "skills")
	if err := Install(skillsDir, "http-skill", srv.URL+"/s.zip"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "http-skill", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}

func TestInstallRejectsBadName(t *testing.T) {
	err := Install(t.TempDir(), "BadName", "x.zip")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRemoveRejectsPathEscape(t *testing.T) {
	if err := Remove(t.TempDir(), "../etc"); err == nil {
		t.Fatal("expected error")
	}
}
