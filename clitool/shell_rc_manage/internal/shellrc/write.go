package shellrc

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
		b.WriteString("alias ")
		b.WriteString(it.Name)
		b.WriteByte('=')
		b.WriteString(formatValue(it))
		b.WriteByte('\n')
	case StyleExport:
		b.WriteString("export ")
		b.WriteString(it.Name)
		b.WriteByte('=')
		b.WriteString(formatValue(it))
		b.WriteByte('\n')
	case StyleAssign:
		b.WriteString(it.Name)
		b.WriteByte('=')
		b.WriteString(formatValue(it))
		b.WriteByte('\n')
	case StyleFunction:
		b.WriteString(it.Name)
		b.WriteString("() {\n")
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

func formatValue(it *Item) string {
	q := it.Quote
	if q == "" {
		q = defaultQuote(it)
	}
	if q == QuoteNone && needsQuote(it.Value) {
		if strings.ContainsAny(it.Value, "$`") {
			q = QuoteDouble
		} else {
			q = QuoteSingle
		}
	}
	return quoteValue(it.Value, q)
}

func defaultQuote(it *Item) Quote {
	if it.Style == StyleAlias {
		return QuoteSingle
	}
	if it.Value == "" || needsQuote(it.Value) {
		if strings.ContainsAny(it.Value, "$`") {
			return QuoteDouble
		}
		return QuoteSingle
	}
	return QuoteNone
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
	tmp := f.Path + ".tmp"
	if err := os.WriteFile(tmp, []byte(f.String()), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, f.Path)
}
