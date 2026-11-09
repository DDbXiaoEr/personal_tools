package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"viewcsv_xlsx/internal/tableview"
)

func testModel() model {
	files := []*tableview.File{{
		Path: "a.csv",
		Name: "a.csv",
		Sheets: []tableview.Sheet{{
			Name: "a.csv",
			Rows: [][]string{
				{"name", "city"},
				{"Alice", "北京"},
				{"Bob", "上海"},
			},
		}},
	}}
	return newModel(files, false)
}

func send(m model, keys ...tea.KeyMsg) model {
	var tm tea.Model = m
	for _, k := range keys {
		tm, _ = tm.Update(k)
	}
	return tm.(model)
}

func TestSearchFiltersRows(t *testing.T) {
	m := testModel()
	if len(m.data) != 2 {
		t.Fatalf("initial rows = %d", len(m.data))
	}

	m = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !m.searching {
		t.Fatal("search mode not entered")
	}

	m = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("北京")})
	if m.query != "北京" {
		t.Fatalf("query = %q", m.query)
	}
	if len(m.data) != 1 || m.data[0][0] != "Alice" {
		t.Fatalf("filtered = %v", m.data)
	}

	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.searching || m.query != "北京" || len(m.data) != 1 {
		t.Fatalf("confirmed state = %+v", m)
	}

	m = send(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.query != "" || len(m.data) != 2 {
		t.Fatalf("cleared state = %+v", m)
	}
}

func TestSearchEscCancels(t *testing.T) {
	m := send(testModel(),
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("bob")},
		tea.KeyMsg{Type: tea.KeyEsc},
	)
	if m.searching || m.query != "" || len(m.data) != 2 {
		t.Fatalf("state = %+v", m)
	}
}

func TestHighlight(t *testing.T) {
	got := highlight("Alice  ", "ali")
	if !strings.Contains(got, "Ali") {
		t.Fatalf("highlight = %q", got)
	}
	if plain := stripANSI(got); plain != "Alice  " {
		t.Fatalf("plain = %q", plain)
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
