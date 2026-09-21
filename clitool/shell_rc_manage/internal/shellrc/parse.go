package shellrc

import (
	"bufio"
	"os"
	"regexp"
	"strings"
	"unicode"
)

var (
	reCategoryBar   = regexp.MustCompile(`^#\s*={2,}\s*(.*?)\s*={2,}\s*$`)
	reCategoryDash  = regexp.MustCompile(`^#\s*-{2,}\s*(.*?)\s*-{2,}\s*$`)
	reCategoryPlain = regexp.MustCompile(`^#\s*(.+?)\s*$`)
	reSnippetName   = regexp.MustCompile(`^#\s*name:\s*(.+?)\s*$`)
	reAlias         = regexp.MustCompile(`^alias\s+([A-Za-z0-9_./+:@-]+)=(.*)$`)
	reExport        = regexp.MustCompile(`^export\s+([A-Za-z_][A-Za-z0-9_]*)=(.*)$`)
	reAssign        = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)=(.*)$`)
	reFuncHeader    = regexp.MustCompile(`^(?:function\s+)?([A-Za-z_][A-Za-z0-9_-]*)\s*(\(\s*\))?\s*(\{)?\s*(.*)$`)
)

func ParseFile(path string, kind Kind) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &File{Path: path, Kind: kind}, nil
		}
		return nil, err
	}
	f := Parse(string(data), kind)
	f.Path = path
	return f, nil
}

func Parse(src string, kind Kind) *File {
	scanner := bufio.NewScanner(strings.NewReader(src))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	f := &File{Kind: kind}
	cat := Uncategorized
	i := 0
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			i++
			continue
		}
		if name, ok := parseSnippetName(trimmed); ok {
			body, n := collectSnippet(kind, lines, i+1)
			if strings.TrimSpace(body) == "" && n == 0 {
				i++
				continue
			}
			f.Items = append(f.Items, &Item{
				Category: cat,
				Name:     name,
				Value:    body,
				Style:    StyleSnippet,
			})
			i += 1 + n
			continue
		}
		if name, ok := parseCategory(trimmed); ok {
			cat = name
			i++
			continue
		}
		if item, n, ok := parseTypedItem(kind, lines, i); ok {
			item.Category = cat
			f.Items = append(f.Items, item)
			i += n
			continue
		}
		body, n := collectSnippet(kind, lines, i)
		if strings.TrimSpace(body) == "" {
			i++
			continue
		}
		f.Items = append(f.Items, &Item{
			Category: cat,
			Value:    body,
			Style:    StyleSnippet,
		})
		i += n
	}
	return f
}

func parseCategory(trimmed string) (string, bool) {
	if strings.TrimSpace(trimmed) == "#" {
		return "", false
	}
	if m := reCategoryBar.FindStringSubmatch(trimmed); m != nil {
		name := strings.TrimSpace(m[1])
		if name == "" {
			return "", false
		}
		return name, true
	}
	if m := reCategoryDash.FindStringSubmatch(trimmed); m != nil {
		name := strings.TrimSpace(m[1])
		if name == "" {
			return "", false
		}
		return name, true
	}
	if reSnippetName.MatchString(trimmed) {
		return "", false
	}
	if m := reCategoryPlain.FindStringSubmatch(trimmed); m != nil {
		name := strings.TrimSpace(m[1])
		if name == "" || strings.HasPrefix(name, "!") {
			return "", false
		}
		return name, true
	}
	return "", false
}

func parseSnippetName(trimmed string) (string, bool) {
	m := reSnippetName.FindStringSubmatch(trimmed)
	if m == nil {
		return "", false
	}
	name := strings.TrimSpace(m[1])
	if name == "" {
		return "", false
	}
	return name, true
}

func parseTypedItem(kind Kind, lines []string, i int) (*Item, int, bool) {
	switch kind {
	case KindAlias:
		if item, n, ok := parseAliasLine(lines[i]); ok {
			return item, n, true
		}
		if item, n, ok := parseFunction(lines, i); ok {
			return item, n, true
		}
		if item, n, ok := parseEnvLine(lines[i]); ok {
			return item, n, true
		}
	case KindFunctions:
		if item, n, ok := parseFunction(lines, i); ok {
			return item, n, true
		}
		if item, n, ok := parseAliasLine(lines[i]); ok {
			return item, n, true
		}
		if item, n, ok := parseEnvLine(lines[i]); ok {
			return item, n, true
		}
	default:
		if item, n, ok := parseEnvLine(lines[i]); ok {
			return item, n, true
		}
		if item, n, ok := parseAliasLine(lines[i]); ok {
			return item, n, true
		}
		if item, n, ok := parseFunction(lines, i); ok {
			return item, n, true
		}
	}
	return nil, 0, false
}

func parseAliasLine(line string) (*Item, int, bool) {
	trimmed := strings.TrimSpace(line)
	m := reAlias.FindStringSubmatch(trimmed)
	if m == nil {
		return nil, 0, false
	}
	val, q, _ := unquote(m[2])
	return &Item{Name: m[1], Value: val, Style: StyleAlias, Quote: q}, 1, true
}

func parseEnvLine(line string) (*Item, int, bool) {
	trimmed := strings.TrimSpace(line)
	if m := reExport.FindStringSubmatch(trimmed); m != nil {
		val, q, _ := unquote(m[2])
		return &Item{Name: m[1], Value: val, Style: StyleExport, Quote: q}, 1, true
	}
	if strings.HasPrefix(trimmed, "export ") {
		return nil, 0, false
	}
	if m := reAssign.FindStringSubmatch(trimmed); m != nil {
		val, q, _ := unquote(m[2])
		return &Item{Name: m[1], Value: val, Style: StyleAssign, Quote: q}, 1, true
	}
	return nil, 0, false
}

func parseFunction(lines []string, i int) (*Item, int, bool) {
	header := strings.TrimSpace(lines[i])
	m := reFuncHeader.FindStringSubmatch(header)
	if m == nil {
		return nil, 0, false
	}
	name := m[1]
	hasParen := strings.TrimSpace(m[2]) != ""
	hasBrace := m[3] != ""
	hasFuncKw := strings.HasPrefix(header, "function ") || strings.HasPrefix(header, "function\t")
	if !hasParen && !hasFuncKw {
		return nil, 0, false
	}
	if reservedName(name) {
		return nil, 0, false
	}

	rest := strings.Join(lines[i:], "\n")
	braceAt := strings.IndexByte(rest, '{')
	if braceAt < 0 {
		return nil, 0, false
	}
	if !hasBrace {
		firstNL := strings.IndexByte(rest, '\n')
		after := ""
		if firstNL >= 0 {
			after = strings.TrimSpace(rest[firstNL+1:])
		}
		if !strings.HasPrefix(after, "{") {
			return nil, 0, false
		}
	}
	inner, end, ok := matchBrace(rest, braceAt)
	if !ok {
		return nil, 0, false
	}
	consumed := rest[:end]
	n := strings.Count(consumed, "\n") + 1
	if strings.HasSuffix(consumed, "\n") {
		n--
	}
	if n < 1 {
		n = 1
	}
	return &Item{
		Name:  name,
		Value: trimFuncBody(inner),
		Style: StyleFunction,
	}, n, true
}

func reservedName(name string) bool {
	switch name {
	case "export", "alias", "local", "declare", "typeset", "readonly", "unset", "source":
		return true
	}
	return false
}

func matchBrace(s string, open int) (inner string, end int, ok bool) {
	if open < 0 || open >= len(s) || s[open] != '{' {
		return "", 0, false
	}
	depth := 0
	quote := byte(0)
	escape := false
	for i := open; i < len(s); i++ {
		c := s[i]
		if escape {
			escape = false
			continue
		}
		if quote != 0 {
			if c == '\\' && quote == '"' {
				escape = true
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '\\':
			escape = true
		case '\'', '"', '`':
			quote = c
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[open+1 : i], i + 1, true
			}
		}
	}
	return "", 0, false
}

func trimFuncBody(s string) string {
	s = strings.TrimRightFunc(s, func(r rune) bool { return r == ' ' || r == '\t' })
	s = strings.TrimPrefix(s, "\n")
	s = strings.TrimRight(s, "\n")
	return s
}

func collectSnippet(kind Kind, lines []string, i int) (string, int) {
	var body []string
	n := 0
	for i+n < len(lines) {
		trimmed := strings.TrimSpace(lines[i+n])
		if trimmed == "" {
			break
		}
		if _, ok := parseCategory(trimmed); ok {
			break
		}
		if _, ok := parseSnippetName(trimmed); ok && n > 0 {
			break
		}
		if _, _, ok := parseTypedItem(kind, lines, i+n); ok {
			break
		}
		body = append(body, lines[i+n])
		n++
	}
	return strings.TrimRightFunc(strings.Join(body, "\n"), unicode.IsSpace), n
}
