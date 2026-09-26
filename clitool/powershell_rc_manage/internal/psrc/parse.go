package psrc

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
	reSnippetName   = regexp.MustCompile(`^(?:#|//)\s*name:\s*(.+?)\s*$`)
	reEnv           = regexp.MustCompile(`^(?i)\$(?:env:([A-Za-z_][A-Za-z0-9_]*)|\{env:([^}]+)\})\s*=\s*(.*)$`)
	reFuncHeader    = regexp.MustCompile(`^(?i)function\s+(?:(?:global|script|local|private):)?([A-Za-z_][A-Za-z0-9_-]*)\s*(\{)?\s*(.*)$`)
	reAliasCmd      = regexp.MustCompile(`^(?i)(?:set-alias|new-alias)\s+(.*)$`)
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
	src = NormalizeNewlines(src)
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
	m := reAliasCmd.FindStringSubmatch(trimmed)
	if m == nil {
		return nil, 0, false
	}
	name, value, q, ok := parseAliasArgs(m[1])
	if !ok || name == "" {
		return nil, 0, false
	}
	return &Item{Name: name, Value: value, Style: StyleAlias, Quote: q}, 1, true
}

func parseAliasArgs(s string) (name, value string, q Quote, ok bool) {
	tokens := tokenizePS(s)
	if len(tokens) == 0 {
		return "", "", QuoteNone, false
	}
	var positionals []psToken
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if strings.HasPrefix(tok.value, "-") {
			flag := strings.ToLower(strings.TrimPrefix(tok.value, "-"))
			switch {
			case flag != "" && strings.HasPrefix("name", flag):
				if i+1 < len(tokens) {
					name = tokens[i+1].value
					i++
				}
			case flag != "" && strings.HasPrefix("value", flag):
				if i+1 < len(tokens) {
					value = tokens[i+1].value
					q = tokens[i+1].quote
					i++
				}
			case flag != "" && (strings.HasPrefix("force", flag) || strings.HasPrefix("passthru", flag)):
			default:
				if i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1].value, "-") {
					i++
				}
			}
			continue
		}
		positionals = append(positionals, tok)
	}
	if name == "" && len(positionals) > 0 {
		name = positionals[0].value
		positionals = positionals[1:]
	}
	if value == "" && len(positionals) > 0 {
		value = positionals[0].value
		q = positionals[0].quote
	}
	if name == "" || value == "" {
		return "", "", QuoteNone, false
	}
	return name, value, q, true
}

func parseEnvLine(line string) (*Item, int, bool) {
	trimmed := strings.TrimSpace(line)
	m := reEnv.FindStringSubmatch(trimmed)
	if m == nil {
		return nil, 0, false
	}
	name := m[1]
	if name == "" {
		name = m[2]
	}
	val, q := unquoteEnvRHS(m[3])
	return &Item{Name: name, Value: val, Style: StyleEnv, Quote: q}, 1, true
}

func parseFunction(lines []string, i int) (*Item, int, bool) {
	header := strings.TrimSpace(lines[i])
	m := reFuncHeader.FindStringSubmatch(header)
	if m == nil {
		return nil, 0, false
	}
	name := m[1]
	hasBrace := m[2] != ""
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
	switch strings.ToLower(name) {
	case "function", "filter", "if", "foreach", "while", "switch", "param", "begin", "process", "end":
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
	i := open
	for i < len(s) {
		c := s[i]
		if quote != 0 {
			if quote == '\'' {
				if c == '\'' {
					if i+1 < len(s) && s[i+1] == '\'' {
						i += 2
						continue
					}
					quote = 0
				}
				i++
				continue
			}
			if c == '`' && i+1 < len(s) {
				i += 2
				continue
			}
			if c == quote {
				quote = 0
			}
			i++
			continue
		}
		if c == '<' && i+1 < len(s) && s[i+1] == '#' {
			j := strings.Index(s[i+2:], "#>")
			if j < 0 {
				return "", 0, false
			}
			i += 2 + j + 2
			continue
		}
		if c == '#' {
			j := strings.IndexByte(s[i:], '\n')
			if j < 0 {
				return "", 0, false
			}
			i += j
			continue
		}
		if i+1 < len(s) && c == '@' && (s[i+1] == '"' || s[i+1] == '\'') {
			q := s[i+1]
			endTag := "\n" + string(q) + "@"
			j := strings.Index(s[i+2:], endTag)
			if j < 0 {
				return "", 0, false
			}
			i += 2 + j + len(endTag)
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[open+1 : i], i + 1, true
			}
		}
		i++
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
	depth := 0
	for i+n < len(lines) {
		line := lines[i+n]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" && depth == 0 {
			break
		}
		if depth == 0 {
			if _, ok := parseCategory(trimmed); ok {
				break
			}
			if _, ok := parseSnippetName(trimmed); ok && n > 0 {
				break
			}
			if _, _, ok := parseTypedItem(kind, lines, i+n); ok {
				break
			}
		}
		body = append(body, line)
		depth += braceDelta(line)
		if depth < 0 {
			depth = 0
		}
		n++
		if depth == 0 && n > 0 && looksCompleteBlock(body) {
			break
		}
	}
	return strings.TrimRightFunc(strings.Join(body, "\n"), unicode.IsSpace), n
}

func braceDelta(line string) int {
	depth := 0
	quote := byte(0)
	for i := 0; i < len(line); i++ {
		c := line[i]
		if quote != 0 {
			if quote == '\'' {
				if c == '\'' {
					if i+1 < len(line) && line[i+1] == '\'' {
						i++
						continue
					}
					quote = 0
				}
				continue
			}
			if c == '`' && i+1 < len(line) {
				i++
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '#' {
			break
		}
		switch c {
		case '\'', '"':
			quote = c
		case '{':
			depth++
		case '}':
			depth--
		}
	}
	return depth
}

func looksCompleteBlock(body []string) bool {
	joined := strings.Join(body, "\n")
	trimmed := strings.TrimSpace(joined)
	if !strings.Contains(trimmed, "{") {
		return false
	}
	return braceDelta(joined) == 0 && strings.Contains(trimmed, "}")
}
