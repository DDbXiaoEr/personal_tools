package psrc

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

func FileName(kind Kind) string {
	return ".ps_" + string(kind) + ".ps1"
}

func Path(dir string, kind Kind) string {
	return filepath.Join(dir, FileName(kind))
}

func Load(dir string) (*Store, error) {
	s := &Store{Dir: dir, Files: make(map[Kind]*File, 3)}
	for _, k := range AllKinds() {
		f, err := ParseFile(Path(dir, k), k)
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
	f := &File{Path: Path(s.Dir, k), Kind: k}
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

func NormalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}
