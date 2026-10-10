package shellrc

import "strings"

type Kind string

const (
	KindAlias     Kind = "alias"
	KindEnv       Kind = "env"
	KindFunctions Kind = "functions"
)

func AllKinds() []Kind {
	return []Kind{KindAlias, KindEnv, KindFunctions}
}

func (k Kind) TabLabel() string {
	switch k {
	case KindAlias:
		return "Alias"
	case KindEnv:
		return "Env"
	case KindFunctions:
		return "Functions"
	default:
		return string(k)
	}
}

type Style string

const (
	StyleAlias    Style = "alias"
	StyleExport   Style = "export"
	StyleAssign   Style = "assign"
	StyleSnippet  Style = "snippet"
	StyleFunction Style = "function"
)

type Quote string

const (
	QuoteNone   Quote = "none"
	QuoteSingle Quote = "single"
	QuoteDouble Quote = "double"
)

const Uncategorized = "未分类"

type Item struct {
	Category string
	Name     string
	Value    string
	Alts     []string
	Style    Style
	Quote    Quote
}

func (it *Item) allValues() []string {
	if it == nil {
		return nil
	}
	all := NormalizeAlts("", it.Alts)
	if strings.TrimSpace(it.Value) == "" {
		return all
	}
	for _, a := range all {
		if a == it.Value {
			return all
		}
	}
	return append([]string{it.Value}, all...)
}

func (it *Item) CanCycle() bool {
	if it == nil {
		return false
	}
	if it.Style != StyleExport && it.Style != StyleAssign {
		return false
	}
	return len(it.allValues()) > 1
}

func (it *Item) Cycle(delta int) bool {
	if !it.CanCycle() {
		return false
	}
	all := it.allValues()
	idx := 0
	for i, a := range all {
		if a == it.Value {
			idx = i
			break
		}
	}
	n := len(all)
	idx = (idx + delta) % n
	if idx < 0 {
		idx += n
	}
	it.Value = all[idx]
	it.Alts = all
	return true
}

func (it *Item) ValueIndex() (int, int) {
	all := it.allValues()
	if len(all) == 0 {
		return 0, 0
	}
	for i, a := range all {
		if a == it.Value {
			return i + 1, len(all)
		}
	}
	return 1, len(all)
}

func NormalizeAlts(value string, alts []string) []string {
	seen := map[string]bool{strings.TrimSpace(value): true}
	var out []string
	for _, a := range alts {
		a = strings.TrimSpace(a)
		if a == "" || seen[a] {
			continue
		}
		seen[a] = true
		out = append(out, a)
	}
	return out
}

func ParseAltLines(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

func (it *Item) Cat() string {
	if it == nil {
		return Uncategorized
	}
	if c := strings.TrimSpace(it.Category); c != "" {
		return c
	}
	return Uncategorized
}

func (it *Item) DisplayName() string {
	if it == nil {
		return ""
	}
	if n := strings.TrimSpace(it.Name); n != "" {
		return n
	}
	if it.Style == StyleSnippet {
		return snippetPreview(it.Value, 24)
	}
	return "(unnamed)"
}

func (it *Item) Preview() string {
	if it == nil {
		return ""
	}
	if it.Style == StyleFunction || it.Style == StyleSnippet {
		return snippetPreview(it.Value, 60)
	}
	return oneLine(it.Value, 60)
}

func snippetPreview(v string, max int) string {
	v = strings.TrimSpace(v)
	if i := strings.IndexByte(v, '\n'); i >= 0 {
		return oneLine(v[:i]+"…", max)
	}
	return oneLine(v, max)
}

func oneLine(v string, max int) string {
	v = strings.ReplaceAll(v, "\t", " ")
	if max > 0 && len([]rune(v)) > max {
		r := []rune(v)
		return string(r[:max]) + "…"
	}
	return v
}

type File struct {
	Path  string
	Kind  Kind
	Items []*Item
}

type Store struct {
	Dir   string
	Shell string
	Files map[Kind]*File
}
