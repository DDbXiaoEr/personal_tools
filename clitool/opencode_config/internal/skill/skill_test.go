package skill

import (
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
