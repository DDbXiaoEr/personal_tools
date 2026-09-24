package inventory

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

func ParseYAML(src string) (*Inventory, error) {
	src = strings.TrimSpace(src)
	inv := &Inventory{Format: FormatYAML}
	if src == "" || src == "---" {
		return inv, nil
	}
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(src), &root); err != nil {
		return nil, errf("解析 YAML 失败: %w", err)
	}
	doc := root
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		doc = *root.Content[0]
	}
	if doc.Kind != yaml.MappingNode {
		return nil, errf("YAML 清单根节点必须是映射")
	}
	for i := 0; i+1 < len(doc.Content); i += 2 {
		name := strings.TrimSpace(doc.Content[i].Value)
		if name == "" {
			continue
		}
		parseYAMLGroup(inv, name, doc.Content[i+1])
	}
	return inv, nil
}

func parseYAMLGroup(inv *Inventory, name string, n *yaml.Node) {
	g := inv.EnsureGroup(name)
	if n == nil {
		return
	}
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			key := n.Content[i].Value
			val := n.Content[i+1]
			switch key {
			case "hosts":
				parseYAMLHosts(g, val)
			case "vars":
				g.Vars = mergeKV(g.Vars, mappingToKV(val))
			case "children":
				parseYAMLChildren(inv, g, val)
			default:
				g.SetVar(key, scalarString(val))
			}
		}
	case yaml.SequenceNode:
		parseYAMLHosts(g, n)
	}
}

func parseYAMLHosts(g *Group, n *yaml.Node) {
	if n == nil {
		return
	}
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			name := strings.TrimSpace(n.Content[i].Value)
			if name == "" {
				continue
			}
			h := g.Host(name)
			if h == nil {
				h = &Host{Name: name}
				g.Hosts = append(g.Hosts, h)
			}
			for _, kv := range mappingToKV(n.Content[i+1]) {
				h.Set(kv.Key, kv.Value)
			}
		}
	case yaml.SequenceNode:
		for _, item := range n.Content {
			switch item.Kind {
			case yaml.ScalarNode:
				name := strings.TrimSpace(item.Value)
				if name == "" {
					continue
				}
				if g.Host(name) == nil {
					g.Hosts = append(g.Hosts, &Host{Name: name})
				}
			case yaml.MappingNode:
				for i := 0; i+1 < len(item.Content); i += 2 {
					name := strings.TrimSpace(item.Content[i].Value)
					if name == "" {
						continue
					}
					h := g.Host(name)
					if h == nil {
						h = &Host{Name: name}
						g.Hosts = append(g.Hosts, h)
					}
					for _, kv := range mappingToKV(item.Content[i+1]) {
						h.Set(kv.Key, kv.Value)
					}
				}
			}
		}
	}
}

func parseYAMLChildren(inv *Inventory, g *Group, n *yaml.Node) {
	if n == nil {
		return
	}
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			name := strings.TrimSpace(n.Content[i].Value)
			if name == "" {
				continue
			}
			g.Children = appendUnique(g.Children, name)
			parseYAMLGroup(inv, name, n.Content[i+1])
		}
	case yaml.SequenceNode:
		for _, item := range n.Content {
			name := strings.TrimSpace(item.Value)
			if name == "" {
				continue
			}
			g.Children = appendUnique(g.Children, name)
			inv.EnsureGroup(name)
		}
	}
}

func mappingToKV(n *yaml.Node) []KV {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	var out []KV
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := strings.TrimSpace(n.Content[i].Value)
		if k == "" {
			continue
		}
		out = append(out, KV{Key: k, Value: scalarString(n.Content[i+1])})
	}
	return out
}

func mergeKV(dst, extra []KV) []KV {
	for _, kv := range extra {
		found := false
		for i, d := range dst {
			if d.Key == kv.Key {
				dst[i].Value = kv.Value
				found = true
				break
			}
		}
		if !found {
			dst = append(dst, kv)
		}
	}
	return dst
}

func scalarString(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	switch n.Kind {
	case yaml.ScalarNode:
		return n.Value
	case yaml.SequenceNode, yaml.MappingNode:
		b, err := yaml.Marshal(n)
		if err != nil {
			return n.Value
		}
		return strings.TrimSpace(string(b))
	default:
		return n.Value
	}
}

func (inv *Inventory) EncodeYAML() (string, error) {
	root := &yaml.Node{Kind: yaml.MappingNode}
	for _, g := range inv.Groups {
		if g.Name == GroupAll && g.empty() {
			continue
		}
		if g.Name == GroupUngrouped && g.empty() {
			continue
		}
		root.Content = append(root.Content, yamlScalar(g.Name), encodeYAMLGroup(g))
	}
	doc := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}
	b, err := yaml.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func encodeYAMLGroup(g *Group) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode}
	if len(g.Hosts) > 0 {
		hosts := &yaml.Node{Kind: yaml.MappingNode}
		for _, h := range g.Hosts {
			hosts.Content = append(hosts.Content, yamlScalar(h.Name), encodeYAMLVars(h.Vars, true))
		}
		n.Content = append(n.Content, yamlScalar("hosts"), hosts)
	}
	if len(g.Vars) > 0 {
		n.Content = append(n.Content, yamlScalar("vars"), encodeYAMLVars(g.Vars, false))
	}
	if len(g.Children) > 0 {
		children := &yaml.Node{Kind: yaml.MappingNode}
		for _, c := range g.Children {
			children.Content = append(children.Content, yamlScalar(c), &yaml.Node{Kind: yaml.MappingNode})
		}
		n.Content = append(n.Content, yamlScalar("children"), children)
	}
	return n
}

func encodeYAMLVars(kvs []KV, allowNull bool) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode}
	if len(kvs) == 0 && allowNull {
		return n
	}
	for _, kv := range kvs {
		n.Content = append(n.Content, yamlScalar(kv.Key), yamlValue(kv.Value))
	}
	return n
}

func yamlScalar(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
}

func yamlValue(s string) *yaml.Node {
	if s == "" {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: ""}
	}
	if i, err := strconv.Atoi(s); err == nil && fmt.Sprint(i) == s {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: s}
	}
	if s == "true" || s == "false" {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: s}
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
}
