package psrc

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
	StyleEnv      Style = "env"
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
	Style    Style
	Quote    Quote
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
	Files map[Kind]*File
}
