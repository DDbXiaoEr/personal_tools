package main

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const maxIncludeDepth = 16

type sshConfigOption struct {
	key   string
	value string
}

type sshHostBlock struct {
	patterns []string
	options  []sshConfigOption
}

type sshConfig struct {
	blocks []sshHostBlock
}

type resolvedHost struct {
	Alias          string
	HostName       string
	User           string
	Port           int
	IdentityFiles  []string
	IdentitiesOnly bool
	ConnectTimeout time.Duration
}

func defaultSSHConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".ssh", "config")
}

func loadSSHConfig(path string) (*sshConfig, error) {
	path = strings.TrimSpace(path)
	explicit := path != ""
	if path == "" {
		path = defaultSSHConfigPath()
		if path == "" {
			return &sshConfig{}, nil
		}
	}

	resolved, err := expandLocalPath(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) && !explicit {
			return &sshConfig{}, nil
		}
		return nil, fmt.Errorf("ssh config: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("ssh config %q is a directory", resolved)
	}
	return parseSSHConfigFile(resolved, 0, map[string]bool{})
}

func parseSSHConfigFile(path string, depth int, visiting map[string]bool) (*sshConfig, error) {
	if depth > maxIncludeDepth {
		return nil, fmt.Errorf("ssh config include depth exceeded at %s", path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if visiting[abs] {
		return &sshConfig{}, nil
	}
	visiting[abs] = true

	f, err := os.Open(abs)
	if err != nil {
		return nil, fmt.Errorf("failed to read ssh config %s: %w", abs, err)
	}
	defer f.Close()

	cfg := &sshConfig{}
	current := -1
	skippingMatch := false
	baseDir := filepath.Dir(abs)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		key, value, ok := parseSSHConfigLine(scanner.Text())
		if !ok {
			continue
		}

		switch strings.ToLower(key) {
		case "host":
			skippingMatch = false
			cfg.blocks = append(cfg.blocks, sshHostBlock{patterns: strings.Fields(value)})
			current = len(cfg.blocks) - 1
		case "match":
			skippingMatch = true
			current = -1
		case "include":
			if skippingMatch {
				continue
			}
			included, err := loadIncludes(baseDir, value, depth+1, visiting)
			if err != nil {
				return nil, err
			}
			if current < 0 {
				cfg.blocks = append(cfg.blocks, included.blocks...)
			} else {
				for _, b := range included.blocks {
					if len(b.patterns) == 0 {
						cfg.blocks[current].options = append(cfg.blocks[current].options, b.options...)
						continue
					}
					cfg.blocks = append(cfg.blocks, b)
					current = len(cfg.blocks) - 1
				}
			}
		default:
			if skippingMatch {
				continue
			}
			opt := sshConfigOption{key: key, value: value}
			if current < 0 {
				cfg.blocks = append(cfg.blocks, sshHostBlock{
					patterns: []string{"*"},
					options:  []sshConfigOption{opt},
				})
				current = len(cfg.blocks) - 1
				continue
			}
			cfg.blocks[current].options = append(cfg.blocks[current].options, opt)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func loadIncludes(baseDir, spec string, depth int, visiting map[string]bool) (*sshConfig, error) {
	out := &sshConfig{}
	for _, pattern := range splitSSHInclude(spec) {
		files, err := expandIncludePattern(baseDir, pattern)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			info, err := os.Stat(file)
			if err != nil || info.IsDir() {
				continue
			}
			part, err := parseSSHConfigFile(file, depth, visiting)
			if err != nil {
				return nil, err
			}
			out.blocks = append(out.blocks, part.blocks...)
		}
	}
	return out, nil
}

func splitSSHInclude(spec string) []string {
	var parts []string
	for _, field := range strings.Fields(spec) {
		if p := unquoteSSH(field); p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func expandIncludePattern(baseDir, pattern string) ([]string, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, nil
	}
	if pattern == "~" || strings.HasPrefix(pattern, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to resolve home directory: %w", err)
		}
		if pattern == "~" {
			pattern = home
		} else {
			pattern = filepath.Join(home, pattern[2:])
		}
	}
	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(baseDir, pattern)
	}
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("ssh config include %q: %w", pattern, err)
	}
	if len(matches) == 0 {
		if _, err := os.Stat(pattern); err == nil {
			return []string{pattern}, nil
		}
	}
	return matches, nil
}

func parseSSHConfigLine(line string) (key, value string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	i := 0
	for i < len(trimmed) && !unicode.IsSpace(rune(trimmed[i])) && trimmed[i] != '=' {
		i++
	}
	if i == 0 {
		return "", "", false
	}
	key = trimmed[:i]
	rest := strings.TrimLeft(trimmed[i:], " \t=")
	rest = stripUnquotedComment(rest)
	return key, unquoteSSH(strings.TrimSpace(rest)), true
}

func stripUnquotedComment(s string) string {
	inQuote := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inQuote != 0 {
			if c == inQuote {
				inQuote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			inQuote = c
			continue
		}
		if c == '#' {
			return strings.TrimSpace(s[:i])
		}
	}
	return s
}

func unquoteSSH(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func (c *sshConfig) resolve(alias string) resolvedHost {
	alias = strings.TrimSpace(alias)
	first := map[string]string{}
	var identity []string
	for _, block := range c.blocks {
		if !hostPatternsMatch(block.patterns, alias) {
			continue
		}
		for _, opt := range block.options {
			k := strings.ToLower(opt.key)
			if k == "identityfile" {
				if opt.value != "" {
					identity = append(identity, opt.value)
				}
				continue
			}
			if _, exists := first[k]; !exists {
				first[k] = opt.value
			}
		}
	}

	hostName := first["hostname"]
	if hostName == "" {
		hostName = alias
	}
	userName := first["user"]
	if userName == "" {
		if u, err := user.Current(); err == nil {
			userName = u.Username
		}
	}
	port := 22
	if p := strings.TrimSpace(first["port"]); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			port = n
		}
	}
	timeout := time.Duration(0)
	if t := strings.TrimSpace(first["connecttimeout"]); t != "" {
		if n, err := strconv.Atoi(t); err == nil && n > 0 {
			timeout = time.Duration(n) * time.Second
		}
	}

	tokens := map[string]string{
		"%n": alias,
		"%h": hostName,
		"%r": userName,
		"%u": localUsername(),
		"%d": localHomeDir(),
		"%%": "%",
	}
	hostName = expandSSHTokens(hostName, tokens)
	tokens["%h"] = hostName

	files := make([]string, 0, len(identity))
	for _, raw := range identity {
		files = append(files, expandSSHTokens(raw, tokens))
	}

	return resolvedHost{
		Alias:          alias,
		HostName:       hostName,
		User:           userName,
		Port:           port,
		IdentityFiles:  files,
		IdentitiesOnly: isSSHYes(first["identitiesonly"]),
		ConnectTimeout: timeout,
	}
}

func hostPatternsMatch(patterns []string, name string) bool {
	matched := false
	for _, p := range patterns {
		neg := false
		if strings.HasPrefix(p, "!") {
			neg = true
			p = p[1:]
		}
		if matchSSHPattern(p, name) {
			if neg {
				return false
			}
			matched = true
		}
	}
	return matched
}

func matchSSHPattern(pattern, name string) bool {
	if pattern == name {
		return true
	}
	var b strings.Builder
	b.WriteByte('^')
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteByte('.')
		default:
			b.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	b.WriteByte('$')
	re, err := regexp.Compile(b.String())
	if err != nil {
		return false
	}
	return re.MatchString(name)
}

func expandSSHTokens(s string, tokens map[string]string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+1 < len(s) {
			key := s[i : i+2]
			if repl, ok := tokens[key]; ok {
				b.WriteString(repl)
				i++
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isSSHYes(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "yes", "true", "1":
		return true
	default:
		return false
	}
}

func localUsername() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return ""
}

func localHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func defaultIdentityFiles() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	names := []string{"id_rsa", "id_ecdsa", "id_ecdsa_sk", "id_ed25519", "id_ed25519_sk", "id_dsa"}
	var out []string
	for _, name := range names {
		p := filepath.Join(home, ".ssh", name)
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}
