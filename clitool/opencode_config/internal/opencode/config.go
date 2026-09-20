package opencode

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/tailscale/hujson"
)

const (
	sectionProvider = "provider"
	sectionMCP      = "mcp"
)

// Config holds an opencode config file as a comment-preserving AST together
// with the decoded provider/mcp sections that the UI edits.
type Config struct {
	path string
	ast  hujson.Value

	Providers  map[string]json.RawMessage
	MCPServers map[string]json.RawMessage

	hasProvider bool
	hasMCP      bool

	origProviders map[string]json.RawMessage
	origMCP       map[string]json.RawMessage

	dirty bool
}

// DefaultPath returns the global opencode config path.
func DefaultPath() string {
	if p := os.Getenv("OPENCODE_CONFIG"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "opencode.json"
	}
	return filepath.Join(home, ".config", "opencode", "opencode.json")
}

// DefaultSkillsDir returns the global opencode skills directory.
func DefaultSkillsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "skills"
	}
	return filepath.Join(home, ".config", "opencode", "skills")
}

// Load reads path, tolerating JSONC (comments and trailing commas). A missing
// file yields an empty config rooted at "{}".
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		data = []byte("{}")
	}
	ast, err := hujson.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("解析 %s 失败: %w", path, err)
	}
	c := &Config{path: path, ast: ast}
	if err := c.decode(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) decode() error {
	std := c.ast.Clone()
	std.Standardize()
	root := map[string]json.RawMessage{}
	if err := json.Unmarshal(std.Pack(), &root); err != nil {
		return fmt.Errorf("解析配置内容失败: %w", err)
	}
	_, c.hasProvider = root[sectionProvider]
	_, c.hasMCP = root[sectionMCP]
	c.Providers = decodeObject(root[sectionProvider])
	c.MCPServers = decodeObject(root[sectionMCP])
	c.origProviders = cloneRawMap(c.Providers)
	c.origMCP = cloneRawMap(c.MCPServers)
	c.dirty = false
	return nil
}

func decodeObject(raw json.RawMessage) map[string]json.RawMessage {
	if len(raw) == 0 {
		return map[string]json.RawMessage{}
	}
	m := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return map[string]json.RawMessage{}
	}
	return m
}

// Path reports the config file path.
func (c *Config) Path() string { return c.path }

// Dirty reports whether there are unsaved in-memory edits.
func (c *Config) Dirty() bool { return c.dirty }

// Reload re-reads the config file from disk, discarding in-memory edits.
func (c *Config) Reload() error {
	data, err := os.ReadFile(c.path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		data = []byte("{}")
	}
	ast, err := hujson.Parse(data)
	if err != nil {
		return fmt.Errorf("解析 %s 失败: %w", c.path, err)
	}
	c.ast = ast
	return c.decode()
}

// ProviderIDs returns the sorted provider IDs.
func (c *Config) ProviderIDs() []string { return sortedKeys(c.Providers) }

// MCPIDs returns the sorted MCP server names.
func (c *Config) MCPIDs() []string { return sortedKeys(c.MCPServers) }

func sortedKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Provider returns the raw provider entry.
func (c *Config) Provider(id string) json.RawMessage { return c.Providers[id] }

// SetProvider adds or replaces a provider entry.
func (c *Config) SetProvider(id string, raw json.RawMessage) {
	c.Providers[id] = raw
	c.dirty = true
}

// DeleteProvider removes a provider entry.
func (c *Config) DeleteProvider(id string) {
	delete(c.Providers, id)
	c.dirty = true
}

// MCP returns the raw MCP entry.
func (c *Config) MCP(name string) json.RawMessage { return c.MCPServers[name] }

// SetMCP adds or replaces an MCP entry.
func (c *Config) SetMCP(name string, raw json.RawMessage) {
	c.MCPServers[name] = raw
	c.dirty = true
}

// DeleteMCP removes an MCP entry.
func (c *Config) DeleteMCP(name string) {
	delete(c.MCPServers, name)
	c.dirty = true
}

// Save applies the in-memory edits to the AST directly so that comments, key
// order and unrelated keys are preserved, then writes the file atomically
// (keeping a .bak of the previous content).
func (c *Config) Save() error {
	changed := c.dirty || !rawMapEqual(c.origProviders, c.Providers) || !rawMapEqual(c.origMCP, c.MCPServers)
	if !changed {
		c.dirty = false
		return nil
	}

	ast := c.ast.Clone()
	root, ok := ast.Value.(*hujson.Object)
	if !ok {
		return errors.New("配置根节点不是 JSON 对象")
	}
	if err := applySection(root, sectionProvider, c.origProviders, c.Providers, c.hasProvider); err != nil {
		return err
	}
	if err := applySection(root, sectionMCP, c.origMCP, c.MCPServers, c.hasMCP); err != nil {
		return err
	}
	ast.Format()
	out := ast.Pack()

	if err := validateJSON(out); err != nil {
		return err
	}

	if prev, err := os.ReadFile(c.path); err == nil {
		_ = os.WriteFile(c.path+".bak", prev, 0600)
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0700); err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, out, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, c.path); err != nil {
		return err
	}

	ast2, err := hujson.Parse(out)
	if err != nil {
		return err
	}
	c.ast = ast2
	return c.decode()
}

func applySection(root *hujson.Object, section string, orig, cur map[string]json.RawMessage, existed bool) error {
	member := findMember(root, section)

	if !existed {
		if len(cur) == 0 {
			return nil
		}
		obj, err := buildObject(cur)
		if err != nil {
			return err
		}
		root.Members = append(root.Members, newMember(section, obj, "\n"))
		return nil
	}

	if len(cur) == 0 {
		removeMember(root, section)
		return nil
	}

	obj, ok := member.Value.Value.(*hujson.Object)
	if !ok {
		newObj, err := buildObject(cur)
		if err != nil {
			return err
		}
		member.Value.Value = newObj
		return nil
	}

	for _, key := range unionKeys(orig, cur) {
		o, okOrig := orig[key]
		v, okCur := cur[key]
		switch {
		case okCur && (!okOrig || !bytesEqual(o, v)):
			parsed, err := hujson.Parse(v)
			if err != nil {
				return err
			}
			if m := findMember(obj, key); m != nil {
				m.Value.Value = parsed.Value
			} else {
				obj.Members = append(obj.Members, newMember(key, parsed.Value, "\n"))
			}
		case !okCur && okOrig:
			removeMember(obj, key)
		}
	}
	return nil
}

func newMember(name string, value hujson.ValueTrimmed, trailing string) hujson.ObjectMember {
	return hujson.ObjectMember{
		Name: hujson.Value{
			BeforeExtra: hujson.Extra("\n"),
			Value:       hujson.String(name),
		},
		Value: hujson.Value{
			BeforeExtra: hujson.Extra(" "),
			Value:       value,
			AfterExtra:  hujson.Extra(trailing),
		},
	}
}

func buildObject(m map[string]json.RawMessage) (*hujson.Object, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	v, err := hujson.Parse(b)
	if err != nil {
		return nil, err
	}
	obj, ok := v.Value.(*hujson.Object)
	if !ok {
		return nil, errors.New("内部错误: 期望 JSON 对象")
	}
	return obj, nil
}

func validateJSON(b []byte) error {
	// hujson.Standardize aliases and mutates its input buffer, so operate on a
	// copy to avoid corrupting the bytes that are about to be written.
	cp := make([]byte, len(b))
	copy(cp, b)
	std, err := hujson.Standardize(cp)
	if err != nil {
		return fmt.Errorf("生成的配置不是合法 JSON: %w", err)
	}
	if !json.Valid(std) {
		return errors.New("生成的配置不是合法 JSON")
	}
	return nil
}
