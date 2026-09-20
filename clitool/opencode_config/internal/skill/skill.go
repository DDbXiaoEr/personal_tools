package skill

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var nameRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Skill is a parsed global agent skill.
type Skill struct {
	Name          string
	Description   string
	License       string
	Compatibility string
	Metadata      map[string]string
	Dir           string
	Path          string
	Raw           string
	Body          string
	Warnings      []string
}

type frontmatter struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	License       string            `yaml:"license"`
	Compatibility string            `yaml:"compatibility"`
	Metadata      map[string]string `yaml:"metadata"`
}

// Scan loads every <dir>/<name>/SKILL.md. A missing dir yields no skills.
func Scan(dir string) ([]Skill, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var skills []Skill
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name(), "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		skills = append(skills, Parse(e.Name(), path, string(data)))
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills, nil
}

// Parse parses a SKILL.md document and reports validation warnings.
func Parse(dirName, path, raw string) Skill {
	s := Skill{Dir: dirName, Path: path, Raw: raw, Metadata: map[string]string{}}
	fm, body, ok := splitFrontmatter(raw)
	if !ok {
		s.Name = dirName
		s.Body = raw
		s.Warnings = append(s.Warnings, "缺少 YAML frontmatter")
		return s
	}
	s.Body = body

	var f frontmatter
	if err := yaml.Unmarshal([]byte(fm), &f); err != nil {
		s.Warnings = append(s.Warnings, "frontmatter 解析失败: "+err.Error())
	}
	s.Description = f.Description
	s.License = f.License
	s.Compatibility = f.Compatibility
	if f.Metadata != nil {
		s.Metadata = f.Metadata
	}
	if f.Name == "" {
		s.Name = dirName
		s.Warnings = append(s.Warnings, "缺少 name")
	} else {
		s.Name = f.Name
		if f.Name != dirName {
			s.Warnings = append(s.Warnings, "name 与目录名不一致")
		}
		if !nameRe.MatchString(f.Name) {
			s.Warnings = append(s.Warnings, "name 不符合命名规范")
		}
	}
	if strings.TrimSpace(f.Description) == "" {
		s.Warnings = append(s.Warnings, "缺少 description")
	}
	return s
}

func splitFrontmatter(raw string) (string, string, bool) {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	if !strings.HasPrefix(raw, "---\n") {
		return "", raw, false
	}
	rest := raw[len("---\n"):]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", raw, false
	}
	fm := rest[:idx]
	body := rest[idx+len("\n---"):]
	body = strings.TrimPrefix(body, "\n")
	return fm, body, true
}
