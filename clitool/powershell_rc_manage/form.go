package main

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type fieldKind int

const (
	kindText fieldKind = iota
	kindArea
	kindChoice
	kindBool
	kindButton
)

type field struct {
	label    string
	kind     fieldKind
	input    textinput.Model
	area     textarea.Model
	options  []string
	choice   int
	boolean  bool
	action   string
	readOnly bool
	hidden   bool
	onChoice func(*form)
}

func newText(label, placeholder string) *field {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Prompt = ""
	ti.CharLimit = 4096
	return &field{label: label, kind: kindText, input: ti}
}

func newArea(label, placeholder string, height int) *field {
	ta := textarea.New()
	ta.Placeholder = placeholder
	ta.ShowLineNumbers = false
	ta.SetHeight(height)
	ta.CharLimit = 0
	return &field{label: label, kind: kindArea, area: ta}
}

func newChoice(label string, options []string, idx int) *field {
	return &field{label: label, kind: kindChoice, options: options, choice: idx}
}

func (f *form) focused() *field {
	if f.focus < 0 || f.focus >= len(f.fields) {
		return nil
	}
	return f.fields[f.focus]
}

func (f *field) setText(v string) {
	if f.kind == kindText {
		f.input.SetValue(v)
	}
}

func (f *field) setArea(v string) {
	if f.kind == kindArea {
		f.area.SetValue(v)
	}
}

func (f *field) text() string {
	return strings.TrimSpace(f.input.Value())
}

func (f *field) areaText() string {
	return strings.TrimRight(f.area.Value(), "\n")
}

func (f *field) choiceValue() string {
	if f.choice < 0 || f.choice >= len(f.options) {
		return ""
	}
	return f.options[f.choice]
}

type form struct {
	title  string
	fields []*field
	focus  int
}

func (f *form) add(fs ...*field) *form {
	f.fields = append(f.fields, fs...)
	return f
}

func (f *form) visibleIndexes() []int {
	var out []int
	for i, fld := range f.fields {
		if !fld.hidden {
			out = append(out, i)
		}
	}
	return out
}

func (f *form) focusFirst() {
	vis := f.visibleIndexes()
	if len(vis) == 0 {
		f.focus = 0
		f.applyFocus()
		return
	}
	f.focus = vis[0]
	f.applyFocus()
}

func (f *form) applyFocus() {
	if len(f.fields) == 0 {
		return
	}
	if f.focus < 0 || f.focus >= len(f.fields) || f.fields[f.focus].hidden {
		vis := f.visibleIndexes()
		if len(vis) == 0 {
			return
		}
		f.focus = vis[0]
	}
	for i, fld := range f.fields {
		switch fld.kind {
		case kindText:
			if i == f.focus {
				fld.input.Focus()
			} else {
				fld.input.Blur()
			}
		case kindArea:
			if i == f.focus {
				fld.area.Focus()
			} else {
				fld.area.Blur()
			}
		}
	}
}

func (f *form) next() {
	vis := f.visibleIndexes()
	if len(vis) == 0 {
		return
	}
	cur := 0
	for i, idx := range vis {
		if idx == f.focus {
			cur = i
			break
		}
	}
	f.focus = vis[(cur+1)%len(vis)]
	f.applyFocus()
}

func (f *form) prev() {
	vis := f.visibleIndexes()
	if len(vis) == 0 {
		return
	}
	cur := 0
	for i, idx := range vis {
		if idx == f.focus {
			cur = i
			break
		}
	}
	f.focus = vis[(cur-1+len(vis))%len(vis)]
	f.applyFocus()
}

func (f *form) moveUp() bool {
	if fld := f.focused(); fld != nil && fld.kind == kindArea && fld.area.Line() > 0 {
		return false
	}
	f.prev()
	return true
}

func (f *form) moveDown() bool {
	if fld := f.focused(); fld != nil && fld.kind == kindArea && fld.area.Line() < fld.area.LineCount()-1 {
		return false
	}
	f.next()
	return true
}

func (f *form) updateFocused(msg tea.KeyMsg) tea.Cmd {
	if len(f.fields) == 0 {
		return nil
	}
	fld := f.fields[f.focus]
	if fld.hidden || fld.readOnly {
		return nil
	}
	switch fld.kind {
	case kindText:
		var cmd tea.Cmd
		fld.input, cmd = fld.input.Update(msg)
		return cmd
	case kindArea:
		var cmd tea.Cmd
		fld.area, cmd = fld.area.Update(msg)
		return cmd
	case kindChoice:
		switch msg.String() {
		case "left", "h":
			if fld.choice > 0 {
				fld.choice--
			}
		case "right", "l", " ", "enter":
			if fld.choice < len(fld.options)-1 {
				fld.choice++
			} else {
				fld.choice = 0
			}
		default:
			return nil
		}
		if fld.onChoice != nil {
			fld.onChoice(f)
			f.applyFocus()
		}
	case kindBool:
		switch msg.String() {
		case " ", "enter", "left", "right", "h", "l":
			fld.boolean = !fld.boolean
		}
	}
	return nil
}

func (f *form) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(f.title))
	b.WriteString("\n\n")
	for i, fld := range f.fields {
		if fld.hidden {
			continue
		}
		if fld.kind == kindButton {
			style := buttonStyle
			if i == f.focus {
				style = buttonFocusedStyle
			}
			b.WriteString("  ")
			b.WriteString(style.Render("▸ " + fld.label))
			b.WriteString("\n")
			continue
		}
		label := labelStyle
		if i == f.focus {
			label = focusedLabel
		}
		b.WriteString(label.Render(fld.label))
		switch fld.kind {
		case kindText:
			if fld.readOnly {
				b.WriteString(dimStyle.Render(fld.input.Value()))
			} else {
				b.WriteString(fld.input.View())
			}
			b.WriteString("\n")
		case kindArea:
			b.WriteString("\n")
			b.WriteString(fld.area.View())
			b.WriteString("\n")
		case kindChoice:
			var parts []string
			for j, opt := range fld.options {
				if j == fld.choice {
					parts = append(parts, selectedStyle.Render("["+opt+"]"))
				} else {
					parts = append(parts, dimStyle.Render(opt))
				}
			}
			b.WriteString(strings.Join(parts, " "))
			b.WriteString("\n")
		case kindBool:
			mark := "[ ]"
			if fld.boolean {
				mark = "[x]"
			}
			b.WriteString(mark)
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("tab/shift+tab/↑↓ 切换字段 · ←→/空格 切换选项 · ctrl+s 应用 · esc 取消"))
	return b.String()
}
