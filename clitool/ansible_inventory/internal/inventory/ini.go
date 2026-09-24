package inventory

import (
	"bufio"
	"strings"
)

func ParseINI(src string) *Inventory {
	inv := &Inventory{Format: FormatINI}
	scanner := bufio.NewScanner(strings.NewReader(src))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	section := GroupUngrouped
	kind := "hosts"
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.TrimSpace(line[1 : len(line)-1])
			section, kind = splitSection(name)
			inv.EnsureGroup(section)
			continue
		}
		g := inv.EnsureGroup(section)
		switch kind {
		case "vars":
			k, v, ok := cutKV(line)
			if !ok {
				continue
			}
			g.SetVar(k, v)
		case "children":
			name := firstToken(line)
			if name == "" {
				continue
			}
			g.Children = appendUnique(g.Children, name)
			inv.EnsureGroup(name)
		default:
			host, rest := splitHostLine(line)
			if host == "" {
				continue
			}
			h := g.Host(host)
			if h == nil {
				h = &Host{Name: host}
				g.Hosts = append(g.Hosts, h)
			}
			for _, kv := range parseInlineVars(rest) {
				h.Set(kv.Key, kv.Value)
			}
		}
	}
	return inv
}

func splitSection(name string) (string, string) {
	name = strings.TrimSpace(name)
	switch {
	case strings.HasSuffix(name, ":vars"):
		return strings.TrimSpace(strings.TrimSuffix(name, ":vars")), "vars"
	case strings.HasSuffix(name, ":children"):
		return strings.TrimSpace(strings.TrimSuffix(name, ":children")), "children"
	default:
		return name, "hosts"
	}
}

func splitHostLine(line string) (string, string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", ""
	}
	i := 0
	for i < len(line) && line[i] != ' ' && line[i] != '\t' {
		i++
	}
	host := line[:i]
	rest := strings.TrimSpace(line[i:])
	return host, rest
}

func parseInlineVars(rest string) []KV {
	if rest == "" {
		return nil
	}
	var out []KV
	for len(rest) > 0 {
		rest = strings.TrimSpace(rest)
		if rest == "" {
			break
		}
		eq := strings.IndexByte(rest, '=')
		if eq <= 0 {
			break
		}
		key := strings.TrimSpace(rest[:eq])
		if key == "" {
			break
		}
		rest = strings.TrimSpace(rest[eq+1:])
		val, n := readINIValue(rest)
		out = append(out, KV{Key: key, Value: val})
		rest = strings.TrimSpace(rest[n:])
	}
	return out
}

func readINIValue(s string) (string, int) {
	if s == "" {
		return "", 0
	}
	if s[0] == '"' || s[0] == '\'' {
		q := s[0]
		i := 1
		for i < len(s) {
			if s[i] == '\\' && i+1 < len(s) {
				i += 2
				continue
			}
			if s[i] == q {
				inner := unescapeINI(s[1:i])
				return inner, i + 1
			}
			i++
		}
		return unescapeINI(s[1:]), len(s)
	}
	i := 0
	for i < len(s) && s[i] != ' ' && s[i] != '\t' {
		i++
	}
	return s[:i], i
}

func unescapeINI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			b.WriteByte(s[i])
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func cutKV(line string) (string, string, bool) {
	eq := strings.IndexByte(line, '=')
	if eq <= 0 {
		return "", "", false
	}
	k := strings.TrimSpace(line[:eq])
	v := strings.TrimSpace(line[eq+1:])
	if k == "" {
		return "", "", false
	}
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			v = unescapeINI(v[1 : len(v)-1])
		}
	}
	return k, v, true
}

func firstToken(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	i := 0
	for i < len(line) && line[i] != ' ' && line[i] != '\t' && line[i] != '=' {
		i++
	}
	return line[:i]
}

func appendUnique(ss []string, v string) []string {
	for _, s := range ss {
		if s == v {
			return ss
		}
	}
	return append(ss, v)
}

func (inv *Inventory) EncodeINI() string {
	var b strings.Builder
	wrote := false
	for _, g := range inv.Groups {
		if g.Name == GroupAll && g.empty() {
			continue
		}
		if g.empty() {
			continue
		}
		if wrote {
			b.WriteByte('\n')
		}
		wrote = true
		b.WriteByte('[')
		b.WriteString(g.Name)
		b.WriteString("]\n")
		for _, h := range g.Hosts {
			b.WriteString(h.Name)
			for _, kv := range h.Vars {
				b.WriteByte(' ')
				b.WriteString(kv.Key)
				b.WriteByte('=')
				b.WriteString(quoteINI(kv.Value))
			}
			b.WriteByte('\n')
		}
		if len(g.Vars) > 0 {
			b.WriteByte('\n')
			b.WriteByte('[')
			b.WriteString(g.Name)
			b.WriteString(":vars]\n")
			for _, kv := range g.Vars {
				b.WriteString(kv.Key)
				b.WriteByte('=')
				b.WriteString(quoteINI(kv.Value))
				b.WriteByte('\n')
			}
		}
		if len(g.Children) > 0 {
			b.WriteByte('\n')
			b.WriteByte('[')
			b.WriteString(g.Name)
			b.WriteString(":children]\n")
			for _, c := range g.Children {
				b.WriteString(c)
				b.WriteByte('\n')
			}
		}
	}
	return b.String()
}

func quoteINI(v string) string {
	if v == "" {
		return `""`
	}
	need := false
	for _, r := range v {
		if r == ' ' || r == '\t' || r == '"' || r == '\'' || r == '#' || r == '=' {
			need = true
			break
		}
	}
	if !need {
		return v
	}
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range v {
		if r == '"' || r == '\\' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}
