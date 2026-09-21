package shellrc

import (
	"strings"
	"unicode"
)

func unquote(s string) (value string, q Quote, rest string) {
	s = strings.TrimLeftFunc(s, unicode.IsSpace)
	if s == "" {
		return "", QuoteNone, ""
	}
	switch s[0] {
	case '\'':
		v, rest, ok := unquoteSingle(s)
		if !ok {
			return stripUnquoted(s)
		}
		return v, QuoteSingle, rest
	case '"':
		v, rest, ok := unquoteDouble(s)
		if !ok {
			return stripUnquoted(s)
		}
		return v, QuoteDouble, rest
	default:
		return stripUnquoted(s)
	}
}

func unquoteSingle(s string) (string, string, bool) {
	if s == "" || s[0] != '\'' {
		return "", s, false
	}
	var b strings.Builder
	i := 1
	for i < len(s) {
		if s[i] == '\'' {
			if strings.HasPrefix(s[i:], `'\''`) {
				b.WriteByte('\'')
				i += 4
				continue
			}
			return b.String(), strings.TrimSpace(s[i+1:]), true
		}
		b.WriteByte(s[i])
		i++
	}
	return "", s, false
}

func unquoteDouble(s string) (string, string, bool) {
	if s == "" || s[0] != '"' {
		return "", s, false
	}
	var b strings.Builder
	i := 1
	for i < len(s) {
		c := s[i]
		if c == '"' {
			return b.String(), strings.TrimSpace(s[i+1:]), true
		}
		if c == '\\' && i+1 < len(s) {
			n := s[i+1]
			switch n {
			case '"', '\\', '$', '`', '\n':
				b.WriteByte(n)
				i += 2
				continue
			}
		}
		b.WriteByte(c)
		i++
	}
	return "", s, false
}

func stripUnquoted(s string) (string, Quote, string) {
	end := len(s)
	inEscape := false
	for i := 0; i < len(s); i++ {
		if inEscape {
			inEscape = false
			continue
		}
		if s[i] == '\\' {
			inEscape = true
			continue
		}
		if s[i] == '#' && (i == 0 || unicode.IsSpace(rune(s[i-1]))) {
			end = i
			break
		}
	}
	return strings.TrimSpace(s[:end]), QuoteNone, strings.TrimSpace(s[end:])
}

func quoteValue(v string, q Quote) string {
	switch q {
	case QuoteDouble:
		return `"` + escapeDouble(v) + `"`
	case QuoteNone:
		if needsQuote(v) {
			return "'" + escapeSingle(v) + "'"
		}
		return v
	default:
		return "'" + escapeSingle(v) + "'"
	}
}

func escapeSingle(v string) string {
	return strings.ReplaceAll(v, "'", `'\''`)
}

func escapeDouble(v string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
	)
	return replacer.Replace(v)
}

func needsQuote(v string) bool {
	if v == "" {
		return true
	}
	for _, r := range v {
		if unicode.IsSpace(r) || strings.ContainsRune("|&;<>()$`\\\"'*?[]#~", r) {
			return true
		}
	}
	return false
}
