package psrc

import (
	"strings"
	"unicode"
)

type psToken struct {
	value string
	quote Quote
}

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
			if i+1 < len(s) && s[i+1] == '\'' {
				b.WriteByte('\'')
				i += 2
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
		if c == '`' && i+1 < len(s) {
			n := s[i+1]
			switch n {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			default:
				b.WriteByte(n)
			}
			i += 2
			continue
		}
		b.WriteByte(c)
		i++
	}
	return "", s, false
}

func stripUnquoted(s string) (string, Quote, string) {
	end := len(s)
	for i := 0; i < len(s); i++ {
		if s[i] == '#' && (i == 0 || unicode.IsSpace(rune(s[i-1]))) {
			end = i
			break
		}
		if unicode.IsSpace(rune(s[i])) {
			end = i
			break
		}
	}
	return strings.TrimSpace(s[:end]), QuoteNone, strings.TrimSpace(s[end:])
}

func tokenizePS(s string) []psToken {
	rest := strings.TrimSpace(s)
	var tokens []psToken
	for rest != "" {
		if rest[0] == '#' {
			break
		}
		v, q, next := unquote(rest)
		if next == rest && v == "" {
			break
		}
		tokens = append(tokens, psToken{value: v, quote: q})
		if next == "" {
			break
		}
		rest = next
	}
	return tokens
}

func unquoteEnvRHS(s string) (string, Quote) {
	s = strings.TrimLeftFunc(s, unicode.IsSpace)
	if s == "" {
		return "", QuoteNone
	}
	if s[0] == '\'' || s[0] == '"' {
		v, q, rest := unquote(s)
		rest = strings.TrimSpace(rest)
		if rest == "" || strings.HasPrefix(rest, "#") {
			return v, q
		}
		return trimRHS(s), QuoteNone
	}
	return trimRHS(s), QuoteNone
}

func trimRHS(s string) string {
	end := len(s)
	quote := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if quote == '\'' {
				if c == '\'' {
					if i+1 < len(s) && s[i+1] == '\'' {
						i++
						continue
					}
					quote = 0
				}
				continue
			}
			if c == '`' && i+1 < len(s) {
				i++
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			continue
		}
		if c == '#' && (i == 0 || unicode.IsSpace(rune(s[i-1]))) {
			end = i
			break
		}
	}
	return strings.TrimRightFunc(s[:end], unicode.IsSpace)
}

func quoteValue(v string, q Quote) string {
	switch q {
	case QuoteDouble:
		return quoteExpandable(v)
	case QuoteNone:
		if v == "" || needsQuote(v) {
			return "'" + escapeSingle(v) + "'"
		}
		return v
	default:
		return "'" + escapeSingle(v) + "'"
	}
}

func quoteExpandable(v string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch c {
		case '"', '`':
			b.WriteByte('`')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func escapeSingle(v string) string {
	return strings.ReplaceAll(v, "'", "''")
}

func needsQuote(v string) bool {
	if v == "" {
		return true
	}
	for _, r := range v {
		if unicode.IsSpace(r) || strings.ContainsRune("|&;<>()$`\\\"'*?[]#~@{},=", r) {
			return true
		}
	}
	return false
}

func defaultQuote(it *Item) Quote {
	if strings.ContainsAny(it.Value, "$") {
		return QuoteDouble
	}
	if it.Value == "" || needsQuote(it.Value) {
		return QuoteSingle
	}
	return QuoteNone
}
