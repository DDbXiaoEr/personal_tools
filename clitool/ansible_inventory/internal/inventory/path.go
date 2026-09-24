package inventory

import (
	"os"
	"path/filepath"
	"strings"
)

func DetectAnsibleDefault() string {
	if p := firstPath(os.Getenv("ANSIBLE_INVENTORY")); p != "" {
		return resolveInventoryPath(expandPath(p))
	}
	if p := firstPath(os.Getenv("ANSIBLE_HOSTS")); p != "" {
		return resolveInventoryPath(expandPath(p))
	}
	if cfg := os.Getenv("ANSIBLE_CONFIG"); cfg != "" {
		if p := inventoryFromCfg(expandPath(cfg)); p != "" {
			return p
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if p := inventoryFromCfg(filepath.Join(home, ".ansible.cfg")); p != "" {
			return p
		}
	}
	if p := inventoryFromCfg("/etc/ansible/ansible.cfg"); p != "" {
		return p
	}
	return "/etc/ansible/hosts"
}

func DetectProject() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return DetectProjectFrom(dir)
}

func DetectProjectFrom(dir string) string {
	for {
		if p := inventoryFromCfg(filepath.Join(dir, "ansible.cfg")); p != "" {
			return p
		}
		for _, name := range []string{
			"inventory.yml", "inventory.yaml", "inventory.ini", "inventory",
			"hosts.yml", "hosts.yaml", "hosts.ini", "hosts",
		} {
			p := filepath.Join(dir, name)
			if isFile(p) {
				return p
			}
			if isDir(p) {
				if r := resolveInventoryPath(p); r != p || isFile(r) {
					return r
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func resolveInventoryPath(p string) string {
	if p == "" {
		return p
	}
	fi, err := os.Stat(p)
	if err != nil || !fi.IsDir() {
		return p
	}
	for _, name := range []string{
		"hosts", "hosts.yml", "hosts.yaml", "inventory.yml", "inventory.yaml",
		"hosts.ini", "inventory.ini", "inventory",
	} {
		c := filepath.Join(p, name)
		if isFile(c) {
			return c
		}
	}
	ents, err := os.ReadDir(p)
	if err != nil {
		return p
	}
	for _, e := range ents {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		return filepath.Join(p, e.Name())
	}
	return p
}

func inventoryFromCfg(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	inDefaults := false
	for _, line := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, ";") {
			continue
		}
		if strings.HasPrefix(t, "[") {
			inDefaults = strings.EqualFold(strings.TrimSpace(t), "[defaults]")
			continue
		}
		if !inDefaults {
			continue
		}
		key, val, ok := strings.Cut(t, "=")
		if !ok {
			continue
		}
		if strings.ToLower(strings.TrimSpace(key)) != "inventory" {
			continue
		}
		val = firstPath(strings.TrimSpace(val))
		if val == "" {
			return ""
		}
		val = expandPath(val)
		if !filepath.IsAbs(val) {
			val = filepath.Join(filepath.Dir(path), val)
		}
		return resolveInventoryPath(val)
	}
	return ""
}

func firstPath(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	parts := strings.Split(s, ",")
	return strings.TrimSpace(parts[0])
}

func expandPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return p
	}
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			p = filepath.Join(home, p[2:])
		}
	}
	return os.ExpandEnv(p)
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}
