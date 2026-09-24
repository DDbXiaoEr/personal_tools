package inventory

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	configFileName   = "config"
	projectsFileName = "projects"
)

type Project struct {
	Name string
	Path string
}

type Prefs struct {
	Dir              string
	DefaultInventory string
	Projects         []Project
}

func ConfigDir() string {
	if p := strings.TrimSpace(os.Getenv("ANSIMAN_CONFIG_DIR")); p != "" {
		return expandPath(p)
	}
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, "ansiman")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "ansiman"
	}
	return filepath.Join(home, ".config", "ansiman")
}

func ConfigPath(dir string) string {
	return filepath.Join(dir, configFileName)
}

func ProjectsPath(dir string) string {
	return filepath.Join(dir, projectsFileName)
}

func DefaultPath() string {
	prefs, err := LoadPrefs()
	if err == nil && prefs.DefaultInventory != "" {
		return prefs.DefaultInventory
	}
	return DetectAnsibleDefault()
}

func LoadPrefs() (*Prefs, error) {
	return LoadPrefsFrom(ConfigDir())
}

func LoadPrefsFrom(dir string) (*Prefs, error) {
	p := &Prefs{Dir: dir, DefaultInventory: DetectAnsibleDefault()}
	if data, err := os.ReadFile(ConfigPath(dir)); err == nil {
		if v := parseConfigValue(string(data), "default_inventory"); v != "" {
			p.DefaultInventory = resolveInventoryPath(expandPath(v))
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if data, err := os.ReadFile(ProjectsPath(dir)); err == nil {
		p.Projects = parseProjects(string(data))
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return p, nil
}

func (p *Prefs) Project(name string) *Project {
	for i := range p.Projects {
		if p.Projects[i].Name == name {
			return &p.Projects[i]
		}
	}
	return nil
}

func (p *Prefs) ProjectByPath(path string) *Project {
	for i := range p.Projects {
		if p.Projects[i].Path == path {
			return &p.Projects[i]
		}
	}
	return nil
}

func (p *Prefs) SetDefaultInventory(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errf("默认清单路径不能为空")
	}
	p.DefaultInventory = resolveInventoryPath(expandPath(path))
	return p.SaveConfig()
}

func (p *Prefs) AddProject(name, path string) error {
	name, path, err := normalizeProject(name, path)
	if err != nil {
		return err
	}
	if p.Project(name) != nil {
		return errf("项目 %s 已存在", name)
	}
	if p.ProjectByPath(path) != nil {
		return errf("路径已在项目列表中")
	}
	p.Projects = append(p.Projects, Project{Name: name, Path: path})
	return p.SaveProjects()
}

func (p *Prefs) UpdateProject(oldName, name, path string) error {
	name, path, err := normalizeProject(name, path)
	if err != nil {
		return err
	}
	idx := -1
	for i, it := range p.Projects {
		if it.Name == oldName {
			idx = i
			break
		}
	}
	if idx < 0 {
		return errf("项目不存在")
	}
	if other := p.Project(name); other != nil && other.Name != oldName {
		return errf("项目 %s 已存在", name)
	}
	if other := p.ProjectByPath(path); other != nil && other.Name != oldName {
		return errf("路径已在项目列表中")
	}
	p.Projects[idx] = Project{Name: name, Path: path}
	return p.SaveProjects()
}

func (p *Prefs) DeleteProject(name string) error {
	idx := -1
	for i, it := range p.Projects {
		if it.Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return errf("项目不存在")
	}
	p.Projects = append(p.Projects[:idx], p.Projects[idx+1:]...)
	return p.SaveProjects()
}

func (p *Prefs) SaveConfig() error {
	if p.Dir == "" {
		p.Dir = ConfigDir()
	}
	if err := os.MkdirAll(p.Dir, 0755); err != nil {
		return err
	}
	body := "default_inventory=" + p.DefaultInventory + "\n"
	return writeFile(ConfigPath(p.Dir), body)
}

func (p *Prefs) SaveProjects() error {
	if p.Dir == "" {
		p.Dir = ConfigDir()
	}
	if err := os.MkdirAll(p.Dir, 0755); err != nil {
		return err
	}
	var b strings.Builder
	for _, it := range p.Projects {
		b.WriteString(it.Name)
		b.WriteByte('=')
		b.WriteString(it.Path)
		b.WriteByte('\n')
	}
	return writeFile(ProjectsPath(p.Dir), b.String())
}

func normalizeProject(name, path string) (string, string, error) {
	path = resolveInventoryPath(expandPath(strings.TrimSpace(path)))
	if path == "" {
		return "", "", errf("路径不能为空")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = projectNameFromPath(path)
	}
	if name == "" {
		return "", "", errf("项目名不能为空")
	}
	if strings.Contains(name, "=") {
		return "", "", errf("项目名不能包含 '='")
	}
	return name, path, nil
}

func projectNameFromPath(path string) string {
	base := filepath.Base(path)
	switch base {
	case "hosts", "inventory", "inventory.yml", "inventory.yaml", "inventory.ini", "hosts.yml", "hosts.yaml", "hosts.ini":
		return filepath.Base(filepath.Dir(path))
	}
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func parseConfigValue(src, key string) string {
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, ";") {
			continue
		}
		k, v, ok := strings.Cut(t, "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(k) == key {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func parseProjects(src string) []Project {
	var out []Project
	seenName := map[string]bool{}
	seenPath := map[string]bool{}
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, ";") {
			continue
		}
		name, path := "", t
		if k, v, ok := strings.Cut(t, "="); ok {
			name = strings.TrimSpace(k)
			path = strings.TrimSpace(v)
		}
		path = resolveInventoryPath(expandPath(path))
		if path == "" {
			continue
		}
		if name == "" {
			name = projectNameFromPath(path)
		}
		if name == "" || seenName[name] || seenPath[path] {
			continue
		}
		seenName[name] = true
		seenPath[path] = true
		out = append(out, Project{Name: name, Path: path})
	}
	return out
}

func writeFile(path, body string) error {
	if prev, err := os.ReadFile(path); err == nil && len(prev) > 0 {
		_ = os.WriteFile(path+".bak", prev, 0644)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(body), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
