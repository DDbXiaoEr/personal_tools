package opencode

import (
	"encoding/json"
	"sort"
	"strings"
)

// Provider is an editable view over a provider config entry. Unknown keys are
// preserved on Encode.
type Provider struct {
	ID      string
	Name    string
	NPM     string
	BaseURL string
	APIKey  string
	Models  []*Model

	raw     map[string]json.RawMessage
	options map[string]json.RawMessage
}

// Model is an editable view over provider.models[id].
type Model struct {
	ID       string
	Name     string
	Variants []*Variant

	raw map[string]json.RawMessage
}

// Variant is an editable view over provider.models[id].variants[name].
type Variant struct {
	ID       string
	Disabled bool
	Options  []KV

	raw map[string]json.RawMessage
}

// VariantRef points at a variant inside a model.
type VariantRef struct {
	ModelID string
	Variant *Variant
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
	if v, ok := p.raw["models"]; ok {
		p.Models = parseModels(v)
	}
	return p
}

func parseModels(raw json.RawMessage) []*Model {
	m := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	ids := sortedKeys(m)
	out := make([]*Model, 0, len(ids))
	for _, id := range ids {
		out = append(out, parseModel(id, m[id]))
	}
	return out
}

func parseModel(id string, raw json.RawMessage) *Model {
	md := &Model{ID: id, raw: map[string]json.RawMessage{}}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &md.raw)
	}
	if v, ok := md.raw["name"]; ok {
		md.Name = asString(v)
	}
	if v, ok := md.raw["variants"]; ok {
		md.Variants = parseVariants(v)
	}
	return md
}

func parseVariants(raw json.RawMessage) []*Variant {
	m := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	ids := sortedKeys(m)
	out := make([]*Variant, 0, len(ids))
	for _, id := range ids {
		out = append(out, parseVariant(id, m[id]))
	}
	return out
}

func parseVariant(id string, raw json.RawMessage) *Variant {
	v := &Variant{ID: id, raw: map[string]json.RawMessage{}}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &v.raw)
	}
	if d, ok := v.raw["disabled"]; ok {
		v.Disabled = asBool(d)
	}
	keys := make([]string, 0, len(v.raw))
	for k := range v.raw {
		if k == "disabled" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v.Options = append(v.Options, KV{Key: k, Value: rawToOption(v.raw[k])})
	}
	return v
}

func rawToOption(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return strings.TrimSpace(string(raw))
}

func optionToRaw(s string) json.RawMessage {
	s = strings.TrimSpace(s)
	if s == "" {
		return mustRaw("")
	}
	if json.Valid([]byte(s)) {
		return json.RawMessage(s)
	}
	return mustRaw(s)
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

	models := map[string]json.RawMessage{}
	for _, md := range p.Models {
		if md.isEmpty() {
			continue
		}
		b, err := md.Encode()
		if err != nil {
			return nil, err
		}
		models[md.ID] = b
	}
	if len(models) == 0 {
		delete(out, "models")
	} else {
		b, err := json.Marshal(models)
		if err != nil {
			return nil, err
		}
		out["models"] = b
	}
	return json.Marshal(out)
}

func (m *Model) Encode() (json.RawMessage, error) {
	out := cloneRawMap(m.raw)
	setOrDelete(out, "name", m.Name)
	if len(m.Variants) == 0 {
		delete(out, "variants")
	} else {
		vars := map[string]json.RawMessage{}
		for _, v := range m.Variants {
			b, err := v.Encode()
			if err != nil {
				return nil, err
			}
			vars[v.ID] = b
		}
		b, err := json.Marshal(vars)
		if err != nil {
			return nil, err
		}
		out["variants"] = b
	}
	return json.Marshal(out)
}

func (v *Variant) Clone() *Variant {
	out := &Variant{
		ID:       v.ID,
		Disabled: v.Disabled,
		raw:      cloneRawMap(v.raw),
	}
	if len(v.Options) > 0 {
		out.Options = append([]KV(nil), v.Options...)
	}
	return out
}

func (v *Variant) Encode() (json.RawMessage, error) {
	out := cloneRawMap(v.raw)
	seen := map[string]struct{}{"disabled": {}}
	if v.Disabled {
		out["disabled"] = json.RawMessage("true")
	} else {
		delete(out, "disabled")
	}
	for _, kv := range v.Options {
		if kv.Key == "" || kv.Key == "disabled" {
			continue
		}
		seen[kv.Key] = struct{}{}
		out[kv.Key] = optionToRaw(kv.Value)
	}
	for k := range out {
		if _, ok := seen[k]; !ok {
			delete(out, k)
		}
	}
	return json.Marshal(out)
}

func (m *Model) isEmpty() bool {
	if m.Name != "" || len(m.Variants) > 0 {
		return false
	}
	for k := range m.raw {
		if k != "name" && k != "variants" {
			return false
		}
	}
	return true
}

// ExtraKeys lists keys that are preserved but not editable in the UI.
func (p *Provider) ExtraKeys() []string {
	var keys []string
	for k := range p.raw {
		if k != "name" && k != "npm" && k != "options" && k != "models" {
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

// VariantRefs returns models that have variants, in sorted order.
func (p *Provider) VariantRefs() []VariantRef {
	var out []VariantRef
	for _, md := range p.Models {
		for _, v := range md.Variants {
			out = append(out, VariantRef{ModelID: md.ID, Variant: v})
		}
	}
	return out
}

// NumVariants reports how many variants are configured.
func (p *Provider) NumVariants() int {
	n := 0
	for _, md := range p.Models {
		n += len(md.Variants)
	}
	return n
}

// FindModel returns the model with the given id.
func (p *Provider) FindModel(id string) *Model {
	for _, md := range p.Models {
		if md.ID == id {
			return md
		}
	}
	return nil
}

// FindVariant returns the variant under modelID.
func (p *Provider) FindVariant(modelID, variantID string) *Variant {
	md := p.FindModel(modelID)
	if md == nil {
		return nil
	}
	for _, v := range md.Variants {
		if v.ID == variantID {
			return v
		}
	}
	return nil
}

func (p *Provider) ensureModel(id string) *Model {
	if md := p.FindModel(id); md != nil {
		return md
	}
	md := &Model{ID: id, raw: map[string]json.RawMessage{}}
	p.Models = append(p.Models, md)
	return md
}

// SetVariant adds or replaces a variant under modelID.
func (p *Provider) SetVariant(modelID string, v *Variant) {
	md := p.ensureModel(modelID)
	for i, existing := range md.Variants {
		if existing.ID == v.ID {
			md.Variants[i] = v
			return
		}
	}
	md.Variants = append(md.Variants, v)
}

// DeleteVariant removes a variant. Empty models with no other fields are dropped on Encode.
func (p *Provider) DeleteVariant(modelID, variantID string) {
	md := p.FindModel(modelID)
	if md == nil {
		return
	}
	out := md.Variants[:0]
	for _, v := range md.Variants {
		if v.ID != variantID {
			out = append(out, v)
		}
	}
	md.Variants = out
}

// ToggleVariantDisabled flips the disabled flag. Returns false if not found.
func (p *Provider) ToggleVariantDisabled(modelID, variantID string) bool {
	v := p.FindVariant(modelID, variantID)
	if v == nil {
		return false
	}
	v.Disabled = !v.Disabled
	return true
}
