package opencode

import (
	"encoding/json"
	"sort"
)

// Provider is an editable view over a provider config entry. Unknown keys are
// preserved on Encode.
type Provider struct {
	ID      string
	Name    string
	NPM     string
	BaseURL string
	APIKey  string

	raw     map[string]json.RawMessage
	options map[string]json.RawMessage
}

// ParseProvider decodes a raw provider entry.
func ParseProvider(id string, raw json.RawMessage) *Provider {
	p := &Provider{
		ID:      id,
		raw:     map[string]json.RawMessage{},
		options: map[string]json.RawMessage{},
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &p.raw)
	}
	if o, ok := p.raw["options"]; ok {
		_ = json.Unmarshal(o, &p.options)
	}
	if v, ok := p.raw["name"]; ok {
		p.Name = asString(v)
	}
	if v, ok := p.raw["npm"]; ok {
		p.NPM = asString(v)
	}
	if v, ok := p.options["baseURL"]; ok {
		p.BaseURL = asString(v)
	}
	if v, ok := p.options["apiKey"]; ok {
		p.APIKey = asString(v)
	}
	return p
}

// Encode serializes the provider entry, keeping unknown keys.
func (p *Provider) Encode() (json.RawMessage, error) {
	out := cloneRawMap(p.raw)
	setOrDelete(out, "name", p.Name)
	setOrDelete(out, "npm", p.NPM)

	opts := cloneRawMap(p.options)
	setOrDelete(opts, "baseURL", p.BaseURL)
	setOrDelete(opts, "apiKey", p.APIKey)
	if len(opts) == 0 {
		delete(out, "options")
	} else {
		b, err := json.Marshal(opts)
		if err != nil {
			return nil, err
		}
		out["options"] = b
	}
	return json.Marshal(out)
}

// ExtraKeys lists keys that are preserved but not editable in the UI.
func (p *Provider) ExtraKeys() []string {
	var keys []string
	for k := range p.raw {
		if k != "name" && k != "npm" && k != "options" {
			keys = append(keys, k)
		}
	}
	for k := range p.options {
		if k != "baseURL" && k != "apiKey" {
			keys = append(keys, "options."+k)
		}
	}
	sort.Strings(keys)
	return keys
}
