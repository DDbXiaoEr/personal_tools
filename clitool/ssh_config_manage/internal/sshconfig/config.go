package sshconfig

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type Option struct {
	Key   string
	Value string
}

type Host struct {
	Name    string
	Options []Option
}

func (h *Host) Get(key string) string {
	for _, o := range h.Options {
		if strings.EqualFold(o.Key, key) {
			return o.Value
		}
	}
	return ""
}

func (h *Host) Set(key, value string) {
	for i := range h.Options {
		if strings.EqualFold(h.Options[i].Key, key) {
			if value == "" {
				h.Options = append(h.Options[:i], h.Options[i+1:]...)
				return
			}
			h.Options[i].Value = value
			return
		}
	}
	if value == "" {
		return
	}
	h.Options = append(h.Options, Option{Key: key, Value: value})
}

type Config struct {
	Prologue []string
	Hosts    []*Host
}

func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.ssh/config"
	}
	return filepath.Join(home, ".ssh", "config")
}

func ParseFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}
	defer f.Close()

	cfg := &Config{}
	var current *Host
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			if current == nil {
				cfg.Prologue = append(cfg.Prologue, line)
			}
			continue
		}

		fields := strings.Fields(trimmed)
		key := fields[0]
		if strings.EqualFold(key, "Host") && len(fields) >= 2 {
			current = &Host{Name: strings.Join(fields[1:], " ")}
			cfg.Hosts = append(cfg.Hosts, current)
			continue
		}

		if current == nil {
			cfg.Prologue = append(cfg.Prologue, line)
			continue
		}

		val := ""
		if idx := strings.IndexAny(trimmed, " \t="); idx >= 0 {
			val = strings.TrimSpace(strings.TrimLeft(trimmed[idx:], " \t="))
		}
		current.Options = append(current.Options, Option{Key: key, Value: val})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) String() string {
	var b strings.Builder
	for _, line := range c.Prologue {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if len(c.Prologue) > 0 && len(c.Hosts) > 0 {
		b.WriteByte('\n')
	}
	for i, h := range c.Hosts {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("Host ")
		b.WriteString(h.Name)
		b.WriteByte('\n')
		for _, o := range h.Options {
			b.WriteString("    ")
			b.WriteString(o.Key)
			b.WriteByte(' ')
			b.WriteString(o.Value)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func (c *Config) SaveFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(c.String()), 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
