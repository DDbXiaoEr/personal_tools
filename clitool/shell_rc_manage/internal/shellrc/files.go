package shellrc

import (
	"os"
	"path/filepath"
	"strings"
)

func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "."
	}
	return home
}

func DetectShell() string {
	return NormalizeShell(os.Getenv("SHELL"))
}

func NormalizeShell(s string) string {
	s = strings.ToLower(strings.TrimSpace(filepath.Base(s)))
	if s == "bash" {
		return "bash"
	}
	return "zsh"
}

func FileName(shell string, kind Kind) string {
	return "." + NormalizeShell(shell) + "_" + string(kind)
}

func Path(dir, shell string, kind Kind) string {
	return filepath.Join(dir, FileName(shell, kind))
}

func Load(dir, shell string) (*Store, error) {
	shell = NormalizeShell(shell)
	s := &Store{Dir: dir, Shell: shell, Files: make(map[Kind]*File, 3)}
	for _, k := range AllKinds() {
		f, err := ParseFile(Path(dir, shell, k), k)
		if err != nil {
			return nil, err
		}
		s.Files[k] = f
	}
	return s, nil
}

func (s *Store) File(k Kind) *File {
	if s == nil || s.Files == nil {
		return &File{Kind: k}
	}
	if f := s.Files[k]; f != nil {
		return f
	}
	f := &File{Path: Path(s.Dir, s.Shell, k), Kind: k}
	s.Files[k] = f
	return f
}

func CategoryOrder(items []*Item) []string {
	seen := make(map[string]bool)
	order := make([]string, 0)
	for _, it := range items {
		cat := it.Cat()
		if !seen[cat] {
			seen[cat] = true
			order = append(order, cat)
		}
	}
	return order
}

func ItemsInCategory(items []*Item, cat string) []*Item {
	var out []*Item
	for _, it := range items {
		if it.Cat() == cat {
			out = append(out, it)
		}
	}
	return out
}
