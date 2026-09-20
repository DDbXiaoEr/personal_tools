package opencode

import (
	"bytes"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

// KV is an ordered key/value pair (environment or headers).
type KV struct {
	Key   string
	Value string
}

// MCP is an editable view over an mcp server entry. Unknown keys are preserved
// on Encode.
type MCP struct {
	Name        string
	Type        string // "local" or "remote"
	Command     []string
	Environment []KV
	CWD         string
	URL         string
	Headers     []KV
	OAuth       bool
	Timeout     string
	Enabled     bool

	raw map[string]json.RawMessage
}

// ParseMCP decodes a raw mcp entry.
func ParseMCP(name string, raw json.RawMessage) *MCP {
	m := &MCP{
		Name:    name,
		Type:    "local",
		Enabled: true,
		raw:     map[string]json.RawMessage{},
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &m.raw)
	}
	if v, ok := m.raw["type"]; ok {
		if s := asString(v); s != "" {
			m.Type = s
		}
	}
	if v, ok := m.raw["command"]; ok {
		_ = json.Unmarshal(v, &m.Command)
	}
	if v, ok := m.raw["cwd"]; ok {
		m.CWD = asString(v)
	}
	if v, ok := m.raw["url"]; ok {
		m.URL = asString(v)
	}
	if v, ok := m.raw["timeout"]; ok {
		m.Timeout = rawNumberString(v)
	}
	if v, ok := m.raw["enabled"]; ok {
		m.Enabled = asBool(v)
	}
	if v, ok := m.raw["oauth"]; ok {
		m.OAuth = !bytes.Equal(bytes.TrimSpace(v), []byte("false"))
	}
	if v, ok := m.raw["environment"]; ok {
		m.Environment = decodeKV(v)
	}
	if v, ok := m.raw["headers"]; ok {
		m.Headers = decodeKV(v)
	}
	return m
}

// Encode serializes the mcp entry, keeping unknown keys.
func (m *MCP) Encode() (json.RawMessage, error) {
	out := cloneRawMap(m.raw)

	typ := m.Type
	if typ == "" {
		typ = "local"
	}
	out["type"] = mustRaw(typ)
	out["enabled"] = json.RawMessage(strconv.FormatBool(m.Enabled))

	if t := strings.TrimSpace(m.Timeout); t != "" {
		if _, err := strconv.Atoi(t); err == nil {
			out["timeout"] = json.RawMessage(t)
		}
	} else {
		delete(out, "timeout")
	}

	if typ == "remote" {
		out["url"] = mustRaw(m.URL)
		delete(out, "command")
		delete(out, "cwd")
		delete(out, "environment")
		if len(m.Headers) > 0 {
			b, err := json.Marshal(kvToMap(m.Headers))
			if err != nil {
				return nil, err
			}
			out["headers"] = b
		} else {
			delete(out, "headers")
		}
		if m.OAuth {
			if _, ok := out["oauth"]; !ok {
				out["oauth"] = json.RawMessage("{}")
			}
		} else {
			delete(out, "oauth")
		}
	} else {
		if len(m.Command) > 0 {
			b, err := json.Marshal(m.Command)
			if err != nil {
				return nil, err
			}
			out["command"] = b
		} else {
			delete(out, "command")
		}
		if m.CWD != "" {
			out["cwd"] = mustRaw(m.CWD)
		} else {
			delete(out, "cwd")
		}
		if len(m.Environment) > 0 {
			b, err := json.Marshal(kvToMap(m.Environment))
			if err != nil {
				return nil, err
			}
			out["environment"] = b
		} else {
			delete(out, "environment")
		}
		delete(out, "url")
		delete(out, "headers")
		delete(out, "oauth")
	}
	return json.Marshal(out)
}

// ExtraKeys lists keys that are preserved but not editable in the UI.
func (m *MCP) ExtraKeys() []string {
	known := map[string]bool{
		"type": true, "command": true, "environment": true, "cwd": true,
		"url": true, "headers": true, "oauth": true, "timeout": true, "enabled": true,
	}
	var keys []string
	for k := range m.raw {
		if !known[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}
