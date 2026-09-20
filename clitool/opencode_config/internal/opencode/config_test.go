package opencode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleJSONC = `{
  // top comment
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    // anthropic comment
    "anthropic": {
      "options": {
        "timeout": 600000
      },
      "models": {
        "claude": { "name": "Claude" }
      }
    },
  },
  "mcp": {
    "playwright": {
      "type": "local",
      "command": ["/bin/pw"],
      "enabled": true
    }
  },
  "custom_top": { "keep": true }
}
`

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "opencode.json")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSavePreservesCommentsAndUnknownKeys(t *testing.T) {
	path := writeTemp(t, sampleJSONC)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	p := ParseProvider("anthropic", cfg.Provider("anthropic"))
	p.BaseURL = "https://proxy.example.com/v1"
	raw, err := p.Encode()
	if err != nil {
		t.Fatal(err)
	}
	cfg.SetProvider("anthropic", raw)

	np := &Provider{ID: "local", Name: "Local", NPM: "@ai-sdk/openai-compatible", BaseURL: "http://localhost:1234/v1"}
	nraw, err := np.Encode()
	if err != nil {
		t.Fatal(err)
	}
	cfg.SetProvider("local", nraw)

	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if cfg.Dirty() {
		t.Fatal("expected clean after save")
	}

	out := readFile(t, path)
	for _, want := range []string{
		"// top comment",
		"// anthropic comment",
		`"custom_top"`,
		`"models"`,
		`"claude"`,
		"https://proxy.example.com/v1",
		`"local"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("saved output missing %q\n%s", want, out)
		}
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	got := ParseProvider("anthropic", reloaded.Provider("anthropic"))
	if got.BaseURL != "https://proxy.example.com/v1" {
		t.Errorf("baseURL = %q", got.BaseURL)
	}
	if got.ExtraKeys()[0] != "models" {
		t.Errorf("expected models preserved, extras=%v", got.ExtraKeys())
	}
	if _, ok := reloaded.Providers["local"]; !ok {
		t.Error("new provider missing after reload")
	}
}

func TestMCPAddEditRemove(t *testing.T) {
	path := writeTemp(t, sampleJSONC)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	cfg.DeleteMCP("playwright")

	remote := &MCP{
		Name:    "context7",
		Type:    "remote",
		URL:     "https://mcp.context7.com/mcp",
		Headers: []KV{{Key: "CONTEXT7_API_KEY", Value: "{env:CONTEXT7_API_KEY}"}},
		OAuth:   true,
		Enabled: true,
	}
	raw, err := remote.Encode()
	if err != nil {
		t.Fatal(err)
	}
	cfg.SetMCP("context7", raw)

	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	out := readFile(t, path)
	if strings.Contains(out, "playwright") {
		t.Errorf("removed mcp still present:\n%s", out)
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reloaded.MCPServers["playwright"]; ok {
		t.Error("playwright not removed")
	}
	mc := ParseMCP("context7", reloaded.MCP("context7"))
	if mc.URL != "https://mcp.context7.com/mcp" {
		t.Errorf("url = %q", mc.URL)
	}
	if !mc.OAuth || !mc.Enabled {
		t.Errorf("oauth/enabled = %v/%v", mc.OAuth, mc.Enabled)
	}
}

func TestCreateMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "opencode.json")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	p := &Provider{ID: "x", Name: "X", NPM: "@ai-sdk/openai-compatible"}
	raw, err := p.Encode()
	if err != nil {
		t.Fatal(err)
	}
	cfg.SetProvider("x", raw)
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	out := readFile(t, path)
	if !strings.Contains(out, `"provider"`) {
		t.Errorf("provider section missing: %s", out)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if _, ok := reloaded.Providers["x"]; !ok {
		t.Errorf("provider x missing after reload: %s", out)
	}
}

func TestPointerEscaping(t *testing.T) {
	path := writeTemp(t, sampleJSONC)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	weird := &Provider{ID: "a/b~c", Name: "Weird"}
	raw, err := weird.Encode()
	if err != nil {
		t.Fatal(err)
	}
	cfg.SetProvider("a/b~c", raw)

	ids := cfg.ProviderIDs()
	if len(ids) != 2 || ids[0] != "a/b~c" || ids[1] != "anthropic" {
		t.Fatalf("ids = %v", ids)
	}

	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reloaded.Providers["a/b~c"]; !ok {
		t.Fatalf("weird provider missing after reload: %s", readFile(t, path))
	}
	if _, ok := reloaded.Providers["anthropic"]; !ok {
		t.Error("anthropic provider lost")
	}
}

func TestTrailingCommaAndCommentsLoad(t *testing.T) {
	path := writeTemp(t, sampleJSONC)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("JSONC with comments/trailing comma failed: %v", err)
	}
	if len(cfg.ProviderIDs()) != 1 || len(cfg.MCPIDs()) != 1 {
		t.Fatalf("providers=%v mcp=%v", cfg.ProviderIDs(), cfg.MCPIDs())
	}
}
