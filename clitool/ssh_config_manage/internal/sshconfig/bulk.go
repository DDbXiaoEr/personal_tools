package sshconfig

import (
	"fmt"
	"strings"
)

type BulkEntry struct {
	Alias    string
	HostName string
	User     string
	Port     string
}

func ParseBulkLines(text string) ([]BulkEntry, error) {
	var out []BulkEntry
	seen := make(map[string]int)
	for i, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		e, err := parseBulkFields(fields)
		if err != nil {
			return nil, fmt.Errorf("第 %d 行: %w", i+1, err)
		}
		if prev, ok := seen[e.Alias]; ok {
			return nil, fmt.Errorf("第 %d 行: 别名 %q 与第 %d 行重复", i+1, e.Alias, prev)
		}
		seen[e.Alias] = i + 1
		out = append(out, e)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("没有有效的主机行")
	}
	return out, nil
}

func parseBulkFields(fields []string) (BulkEntry, error) {
	var e BulkEntry
	switch len(fields) {
	case 1:
		e.Alias = fields[0]
		e.HostName = fields[0]
	case 2:
		e.Alias = fields[0]
		e.HostName = fields[1]
	default:
		e.Alias = fields[0]
		e.HostName = fields[1]
		e.User = fields[2]
		if len(fields) > 3 {
			e.Port = fields[3]
		}
		if len(fields) > 4 {
			return BulkEntry{}, fmt.Errorf("字段过多，格式为 别名 [主机 [用户 [端口]]]")
		}
	}
	if e.Alias == "" {
		return BulkEntry{}, fmt.Errorf("别名为空")
	}
	return e, nil
}

func (e BulkEntry) ToHost(shared []Option) *Host {
	h := &Host{Name: e.Alias}
	if len(shared) > 0 {
		h.Options = make([]Option, len(shared))
		copy(h.Options, shared)
	}
	if e.HostName != "" {
		h.Set("HostName", e.HostName)
	}
	if e.User != "" {
		h.Set("User", e.User)
	}
	if e.Port != "" {
		h.Set("Port", e.Port)
	}
	return h
}
