package sshconfig

import (
	"fmt"
	"strings"
)

const templatePrefix = "# sshman-template:"

type Template struct {
	Name         string
	User         string
	Port         string
	IdentityFile string
}

func (t *Template) Clone() *Template {
	if t == nil {
		return &Template{}
	}
	c := *t
	return &c
}

func (t *Template) ApplyTo(h *Host) {
	if t.User != "" {
		h.Set("User", t.User)
	}
	if t.Port != "" {
		h.Set("Port", t.Port)
	}
	if t.IdentityFile != "" {
		h.Set("IdentityFile", t.IdentityFile)
	}
}

func ParseTemplateLine(line string) (*Template, bool) {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) < len(templatePrefix) || !strings.EqualFold(trimmed[:len(templatePrefix)], templatePrefix) {
		return nil, false
	}
	rest := strings.TrimLeft(trimmed[len(templatePrefix):], " \t")
	if rest == "" {
		return nil, false
	}
	fields := strings.SplitN(rest, "\t", 4)
	t := &Template{Name: strings.TrimSpace(fields[0])}
	if t.Name == "" {
		return nil, false
	}
	if len(fields) > 1 {
		t.User = strings.TrimSpace(fields[1])
	}
	if len(fields) > 2 {
		t.Port = strings.TrimSpace(fields[2])
	}
	if len(fields) > 3 {
		t.IdentityFile = strings.TrimSpace(fields[3])
	}
	return t, true
}

func (t *Template) Line() string {
	return fmt.Sprintf("%s %s\t%s\t%s\t%s", templatePrefix, t.Name, t.User, t.Port, t.IdentityFile)
}

func (c *Config) HasTemplate(name string) bool {
	for _, t := range c.Templates {
		if t.Name == name {
			return true
		}
	}
	return false
}

func (c *Config) TemplateAt(index int) *Template {
	if index < 0 || index >= len(c.Templates) {
		return nil
	}
	return c.Templates[index]
}

func UniqueTemplateName(templates []*Template, base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "template"
	}
	used := make(map[string]bool, len(templates))
	for _, t := range templates {
		used[t.Name] = true
	}
	if !used[base] {
		return base
	}
	for i := 2; ; i++ {
		cand := fmt.Sprintf("%s-%d", base, i)
		if !used[cand] {
			return cand
		}
	}
}
