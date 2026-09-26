package psrc

import (
	"os"
	"path/filepath"
	"strings"
)

func (f *File) String() string {
	if f == nil || len(f.Items) == 0 {
		return ""
	}
	var b strings.Builder
	order := CategoryOrder(f.Items)
	for i, cat := range order {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("# ===== ")
		b.WriteString(cat)
		b.WriteString(" =====\n")
		for _, it := range ItemsInCategory(f.Items, cat) {
			writeItem(&b, it)
		}
	}
	return b.String()
}

func writeItem(b *strings.Builder, it *Item) {
	switch it.Style {
	case StyleAlias:
		b.WriteString("Set-Alias -Name ")
		b.WriteString(formatIdent(it.Name))
		b.WriteString(" -Value ")
		b.WriteString(formatValue(it))
		b.WriteByte('\n')
	case StyleEnv:
		b.WriteString("$env:")
		b.WriteString(it.Name)
		b.WriteString(" = ")
		b.WriteString(formatValue(it))
		b.WriteByte('\n')
	case StyleFunction:
		b.WriteString("function ")
		b.WriteString(it.Name)
		b.WriteString(" {\n")
		body := it.Value
		if body != "" {
			b.WriteString(body)
			if !strings.HasSuffix(body, "\n") {
				b.WriteByte('\n')
			}
		}
		b.WriteString("}\n")
	default:
		if name := strings.TrimSpace(it.Name); name != "" {
			b.WriteString("# name: ")
			b.WriteString(name)
			b.WriteByte('\n')
		}
		b.WriteString(it.Value)
		if it.Value != "" && !strings.HasSuffix(it.Value, "\n") {
			b.WriteByte('\n')
		}
	}
}

func formatIdent(name string) string {
	if needsQuote(name) {
		return "'" + escapeSingle(name) + "'"
	}
	return name
}

func formatValue(it *Item) string {
	q := it.Quote
	if q == "" {
		q = defaultQuote(it)
	}
	if q == QuoteNone {
		if it.Value == "" {
			return "''"
		}
		if needsQuote(it.Value) && !isExpression(it.Value) {
			if strings.ContainsAny(it.Value, "$") {
				return quoteExpandable(it.Value)
			}
			return "'" + escapeSingle(it.Value) + "'"
		}
		return it.Value
	}
	return quoteValue(it.Value, q)
}

func isExpression(v string) bool {
	return strings.ContainsAny(v, "+&|") || strings.Contains(v, "$env:")
}

func (f *File) Save() error {
	if f == nil || f.Path == "" {
		return os.ErrInvalid
	}
	dir := filepath.Dir(f.Path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	if prev, err := os.ReadFile(f.Path); err == nil && len(prev) > 0 {
		_ = os.WriteFile(f.Path+".bak", prev, 0644)
	}
	body := strings.ReplaceAll(f.String(), "\n", "\r\n")
	tmp := f.Path + ".tmp"
	if err := os.WriteFile(tmp, []byte(body), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, f.Path)
}
