package inventory

import "strings"

const (
	GroupUngrouped = "ungrouped"
	GroupAll       = "all"
)

type Format int

const (
	FormatINI Format = iota
	FormatYAML
)

func (f Format) String() string {
	if f == FormatYAML {
		return "yaml"
	}
	return "ini"
}

type KV struct {
	Key   string
	Value string
}

type Host struct {
	Name string
	Vars []KV
}

type Group struct {
	Name     string
	Hosts    []*Host
	Vars     []KV
	Children []string
}

type Inventory struct {
	Path   string
	Format Format
	Groups []*Group
	Dirty  bool
}

func (h *Host) Get(key string) string {
	for _, kv := range h.Vars {
		if kv.Key == key {
			return kv.Value
		}
	}
	return ""
}

func (h *Host) Set(key, value string) {
	key = trim(key)
	if key == "" {
		return
	}
	if value == "" {
		h.Delete(key)
		return
	}
	for i, kv := range h.Vars {
		if kv.Key == key {
			h.Vars[i].Value = value
			return
		}
	}
	h.Vars = append(h.Vars, KV{Key: key, Value: value})
}

func (h *Host) Delete(key string) {
	out := h.Vars[:0]
	for _, kv := range h.Vars {
		if kv.Key != key {
			out = append(out, kv)
		}
	}
	h.Vars = out
}

func (h *Host) Summary() string {
	var parts []string
	if v := h.Get("ansible_host"); v != "" {
		parts = append(parts, v)
	}
	user := h.Get("ansible_user")
	port := h.Get("ansible_port")
	switch {
	case user != "" && port != "":
		parts = append(parts, user+":"+port)
	case user != "":
		parts = append(parts, user)
	case port != "":
		parts = append(parts, ":"+port)
	}
	if c := h.Get("ansible_connection"); c != "" && c != "ssh" {
		parts = append(parts, c)
	}
	return strings.Join(parts, "  ")
}

func (h *Host) ExtraVars() []KV {
	var out []KV
	for _, kv := range h.Vars {
		if isKnownHostKey(kv.Key) {
			continue
		}
		out = append(out, kv)
	}
	return out
}

func isKnownHostKey(k string) bool {
	switch k {
	case "ansible_host", "ansible_user", "ansible_port", "ansible_connection":
		return true
	}
	return false
}

func (g *Group) Host(name string) *Host {
	for _, h := range g.Hosts {
		if h.Name == name {
			return h
		}
	}
	return nil
}

func (g *Group) GetVar(key string) string {
	for _, kv := range g.Vars {
		if kv.Key == key {
			return kv.Value
		}
	}
	return ""
}

func (g *Group) SetVar(key, value string) {
	key = trim(key)
	if key == "" {
		return
	}
	if value == "" {
		g.DeleteVar(key)
		return
	}
	for i, kv := range g.Vars {
		if kv.Key == key {
			g.Vars[i].Value = value
			return
		}
	}
	g.Vars = append(g.Vars, KV{Key: key, Value: value})
}

func (g *Group) DeleteVar(key string) {
	out := g.Vars[:0]
	for _, kv := range g.Vars {
		if kv.Key != key {
			out = append(out, kv)
		}
	}
	g.Vars = out
}

func (g *Group) empty() bool {
	return len(g.Hosts) == 0 && len(g.Vars) == 0 && len(g.Children) == 0
}

func (inv *Inventory) Group(name string) *Group {
	for _, g := range inv.Groups {
		if g.Name == name {
			return g
		}
	}
	return nil
}

func (inv *Inventory) EnsureGroup(name string) *Group {
	name = trim(name)
	if name == "" {
		name = GroupUngrouped
	}
	if g := inv.Group(name); g != nil {
		return g
	}
	g := &Group{Name: name}
	inv.Groups = append(inv.Groups, g)
	return g
}

type HostRef struct {
	GroupIdx int
	HostIdx  int
}

func (inv *Inventory) HostRefs() []HostRef {
	var out []HostRef
	for gi, g := range inv.Groups {
		for hi := range g.Hosts {
			out = append(out, HostRef{GroupIdx: gi, HostIdx: hi})
		}
	}
	return out
}

func (inv *Inventory) HostAt(ref HostRef) (*Group, *Host) {
	if ref.GroupIdx < 0 || ref.GroupIdx >= len(inv.Groups) {
		return nil, nil
	}
	g := inv.Groups[ref.GroupIdx]
	if ref.HostIdx < 0 || ref.HostIdx >= len(g.Hosts) {
		return nil, nil
	}
	return g, g.Hosts[ref.HostIdx]
}

type VarRef struct {
	GroupIdx int
	VarIdx   int
}

func (inv *Inventory) VarRefs() []VarRef {
	var out []VarRef
	for gi, g := range inv.Groups {
		for vi := range g.Vars {
			out = append(out, VarRef{GroupIdx: gi, VarIdx: vi})
		}
	}
	return out
}

func (inv *Inventory) VarAt(ref VarRef) (*Group, KV) {
	if ref.GroupIdx < 0 || ref.GroupIdx >= len(inv.Groups) {
		return nil, KV{}
	}
	g := inv.Groups[ref.GroupIdx]
	if ref.VarIdx < 0 || ref.VarIdx >= len(g.Vars) {
		return nil, KV{}
	}
	return g, g.Vars[ref.VarIdx]
}

func (inv *Inventory) AddHost(group, name string, vars []KV) error {
	name = trim(name)
	if name == "" {
		return errf("主机名不能为空")
	}
	g := inv.EnsureGroup(group)
	if g.Host(name) != nil {
		return errf("组 %s 中已存在主机 %s", g.Name, name)
	}
	g.Hosts = append(g.Hosts, &Host{Name: name, Vars: cloneKV(vars)})
	inv.Dirty = true
	return nil
}

func (inv *Inventory) UpdateHost(ref HostRef, vars []KV) error {
	_, h := inv.HostAt(ref)
	if h == nil {
		return errf("主机不存在")
	}
	h.Vars = cloneKV(vars)
	inv.Dirty = true
	return nil
}

func (inv *Inventory) DeleteHost(ref HostRef) error {
	g, h := inv.HostAt(ref)
	if h == nil {
		return errf("主机不存在")
	}
	g.Hosts = append(g.Hosts[:ref.HostIdx], g.Hosts[ref.HostIdx+1:]...)
	inv.Dirty = true
	return nil
}

func (inv *Inventory) AddGroup(name string, children []string) error {
	name = trim(name)
	if name == "" {
		return errf("组名不能为空")
	}
	if !validGroupName(name) {
		return errf("组名不能包含 ':'")
	}
	if inv.Group(name) != nil {
		return errf("组 %s 已存在", name)
	}
	inv.Groups = append(inv.Groups, &Group{Name: name, Children: cleanNames(children)})
	inv.Dirty = true
	return nil
}

func (inv *Inventory) UpdateGroup(name string, children []string) error {
	g := inv.Group(name)
	if g == nil {
		return errf("组不存在")
	}
	g.Children = cleanNames(children)
	inv.Dirty = true
	return nil
}

func (inv *Inventory) DeleteGroup(name string) error {
	idx := -1
	for i, g := range inv.Groups {
		if g.Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return errf("组不存在")
	}
	g := inv.Groups[idx]
	if name != GroupUngrouped && len(g.Hosts) > 0 {
		u := inv.EnsureGroup(GroupUngrouped)
		u.Hosts = append(u.Hosts, g.Hosts...)
		if u != g {
			g.Hosts = nil
		} else {
			g.Hosts = u.Hosts
		}
		idx = -1
		for i, x := range inv.Groups {
			if x.Name == name {
				idx = i
				break
			}
		}
	}
	if idx >= 0 {
		inv.Groups = append(inv.Groups[:idx], inv.Groups[idx+1:]...)
	}
	for _, x := range inv.Groups {
		x.Children = removeName(x.Children, name)
	}
	inv.Dirty = true
	return nil
}

func (inv *Inventory) AddVar(group, key, value string) error {
	key = trim(key)
	if key == "" {
		return errf("变量名不能为空")
	}
	g := inv.EnsureGroup(group)
	if g.GetVar(key) != "" {
		return errf("组 %s 中已存在变量 %s", g.Name, key)
	}
	g.SetVar(key, value)
	inv.Dirty = true
	return nil
}

func (inv *Inventory) UpdateVar(ref VarRef, value string) error {
	g, kv := inv.VarAt(ref)
	if g == nil || kv.Key == "" {
		return errf("变量不存在")
	}
	g.SetVar(kv.Key, value)
	inv.Dirty = true
	return nil
}

func (inv *Inventory) DeleteVar(ref VarRef) error {
	g, kv := inv.VarAt(ref)
	if g == nil || kv.Key == "" {
		return errf("变量不存在")
	}
	g.DeleteVar(kv.Key)
	inv.Dirty = true
	return nil
}

func validGroupName(name string) bool {
	return name != "" && !containsRune(name, ':')
}

func cleanNames(names []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, n := range names {
		n = trim(n)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out
}

func removeName(names []string, name string) []string {
	out := names[:0]
	for _, n := range names {
		if n != name {
			out = append(out, n)
		}
	}
	return out
}

func cloneKV(in []KV) []KV {
	if len(in) == 0 {
		return nil
	}
	out := make([]KV, len(in))
	copy(out, in)
	return out
}

func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}

func trim(s string) string {
	i, j := 0, len(s)
	for i < j && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
		i++
	}
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t' || s[j-1] == '\n' || s[j-1] == '\r') {
		j--
	}
	return s[i:j]
}
