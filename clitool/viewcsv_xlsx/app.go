package main

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"viewcsv_xlsx/internal/tableview"
)

type model struct {
	files    []*tableview.File
	noHeader bool

	fileIdx  int
	sheetIdx int
	rowOff   int
	colOff   int

	width  int
	height int
}

func newModel(files []*tableview.File, noHeader bool) model {
	return model{
		files:    files,
		noHeader: noHeader,
		width:    100,
		height:   30,
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m *model) file() *tableview.File {
	if m.fileIdx < 0 || m.fileIdx >= len(m.files) {
		return nil
	}
	return m.files[m.fileIdx]
}

func (m *model) sheet() *tableview.Sheet {
	f := m.file()
	if f == nil || m.sheetIdx < 0 || m.sheetIdx >= len(f.Sheets) {
		return nil
	}
	return &f.Sheets[m.sheetIdx]
}

func (m *model) clamp() {
	if len(m.files) == 0 {
		m.fileIdx, m.sheetIdx, m.rowOff, m.colOff = 0, 0, 0, 0
		return
	}
	if m.fileIdx < 0 {
		m.fileIdx = 0
	}
	if m.fileIdx >= len(m.files) {
		m.fileIdx = len(m.files) - 1
	}
	f := m.file()
	n := len(f.Sheets)
	if n == 0 {
		m.sheetIdx, m.rowOff, m.colOff = 0, 0, 0
		return
	}
	if m.sheetIdx < 0 {
		m.sheetIdx = 0
	}
	if m.sheetIdx >= n {
		m.sheetIdx = n - 1
	}
	if m.rowOff < 0 {
		m.rowOff = 0
	}
	if m.colOff < 0 {
		m.colOff = 0
	}
}

func (m *model) nextFile(delta int) {
	if len(m.files) < 2 {
		return
	}
	n := len(m.files)
	m.fileIdx = (m.fileIdx + delta) % n
	if m.fileIdx < 0 {
		m.fileIdx += n
	}
	m.sheetIdx, m.rowOff, m.colOff = 0, 0, 0
}

func (m *model) nextSheet(delta int) {
	f := m.file()
	if f == nil || len(f.Sheets) < 2 {
		return
	}
	n := len(f.Sheets)
	m.sheetIdx = (m.sheetIdx + delta) % n
	if m.sheetIdx < 0 {
		m.sheetIdx += n
	}
	m.rowOff, m.colOff = 0, 0
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.nextFile(1)
		case "shift+tab":
			m.nextFile(-1)
		case "[":
			m.nextSheet(-1)
		case "]":
			m.nextSheet(1)
		case "up", "k":
			if m.rowOff > 0 {
				m.rowOff--
			}
		case "down", "j":
			m.rowOff++
			m.clampRow()
		case "left":
			if m.colOff > 0 {
				m.colOff--
			}
		case "right":
			m.colOff++
			m.clampCol()
		case "g", "home":
			m.rowOff = 0
		case "G", "end":
			m.rowOff = m.maxRowOff()
		case "pgup":
			m.rowOff -= m.bodyRows()
			if m.rowOff < 0 {
				m.rowOff = 0
			}
		case "pgdown":
			m.rowOff += m.bodyRows()
			m.clampRow()
		}
		m.clamp()
		m.clampRow()
		m.clampCol()
	}
	return m, nil
}

func (m *model) bodyRows() int {
	n := m.height - 8
	if f := m.file(); f != nil && len(f.Sheets) > 1 {
		n--
	}
	if n < 3 {
		n = 3
	}
	return n
}

func (m *model) dataRows() int {
	sh := m.sheet()
	if sh == nil {
		return 0
	}
	n := len(sh.Rows)
	if !m.noHeader && n > 0 {
		n--
	}
	if n < 0 {
		n = 0
	}
	return n
}

func (m *model) maxRowOff() int {
	max := m.dataRows() - m.bodyRows()
	if max < 0 {
		return 0
	}
	return max
}

func (m *model) clampRow() {
	if m.rowOff > m.maxRowOff() {
		m.rowOff = m.maxRowOff()
	}
	if m.rowOff < 0 {
		m.rowOff = 0
	}
}

func (m *model) colCount() int {
	sh := m.sheet()
	if sh == nil {
		return 0
	}
	n := 0
	for _, row := range sh.Rows {
		if len(row) > n {
			n = len(row)
		}
	}
	return n
}

func (m *model) clampCol() {
	n := m.colCount()
	if n <= 1 {
		m.colOff = 0
		return
	}
	if m.colOff >= n {
		m.colOff = n - 1
	}
}

func (m model) View() string {
	m.clamp()
	m.clampRow()
	m.clampCol()
	var b strings.Builder
	b.WriteString(m.headerView())
	b.WriteString("\n\n")
	b.WriteString(m.bodyView())
	b.WriteString(helpStyle.Render(m.helpLine()))
	out := b.String()
	if m.height > 0 {
		lines := strings.Split(out, "\n")
		if len(lines) > m.height {
			out = strings.Join(lines[:m.height], "\n")
		}
	}
	return out
}

func (m model) headerView() string {
	var parts []string
	for i, f := range m.files {
		label := f.Name
		if i == m.fileIdx {
			parts = append(parts, activeTabStyle.Render(label))
		} else {
			parts = append(parts, inactiveTabStyle.Render(label))
		}
	}
	path := ""
	if f := m.file(); f != nil {
		path = f.Path
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	}
	title := titleStyle.Render("viewcsv_xlsx")
	if len(m.files) > 1 {
		title += dimStyle.Render(fmt.Sprintf("  %d/%d", m.fileIdx+1, len(m.files)))
	}
	return title + "  " + strings.Join(parts, "") + dimStyle.Render("  "+path)
}

func (m model) bodyView() string {
	sh := m.sheet()
	if sh == nil {
		return dimStyle.Render("空文件") + "\n"
	}
	if len(sh.Rows) == 0 {
		return dimStyle.Render("空表") + "\n"
	}
	var b strings.Builder
	f := m.file()
	if f != nil && len(f.Sheets) > 1 {
		var sheets []string
		for i, s := range f.Sheets {
			label := s.Name
			if i == m.sheetIdx {
				sheets = append(sheets, selectedStyle.Render("["+label+"]"))
			} else {
				sheets = append(sheets, dimStyle.Render(label))
			}
		}
		b.WriteString(strings.Join(sheets, " "))
		b.WriteString("\n")
	}

	rows := sh.Rows
	header := []string(nil)
	data := rows
	if !m.noHeader && len(rows) > 0 {
		header = rows[0]
		data = rows[1:]
	}
	start := m.rowOff
	if start > len(data) {
		start = len(data)
	}
	end := start + m.bodyRows()
	if end > len(data) {
		end = len(data)
	}
	if end < start {
		end = start
	}
	visible := data[start:end]
	show := visible
	if header != nil {
		show = append([][]string{header}, visible...)
	}
	if len(show) == 0 {
		b.WriteString(dimStyle.Render("暂无数据") + "\n")
		return b.String()
	}

	colOff := m.colOff
	trimmed := trimCols(show, colOff)
	avail := m.width - 2
	if avail < 20 {
		avail = 20
	}
	widths := fitWidths(trimmed, avail)
	b.WriteString(renderSep(widths, "┌", "┬", "┐"))
	b.WriteByte('\n')
	for i, row := range trimmed {
		line := renderRow(row, widths)
		if i == 0 && header != nil {
			b.WriteString(headerStyle.Render(line))
			b.WriteByte('\n')
			b.WriteString(renderSep(widths, "├", "┼", "┤"))
			b.WriteByte('\n')
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteString(renderSep(widths, "└", "┴", "┘"))
	b.WriteByte('\n')
	total := m.dataRows()
	shown := end - start
	if total > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("行 %d-%d / %d", start+1, start+shown, total)))
		if colOff > 0 {
			b.WriteString(dimStyle.Render(fmt.Sprintf("  · 列偏移 %d", colOff)))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func trimCols(rows [][]string, off int) [][]string {
	if off <= 0 {
		return rows
	}
	out := make([][]string, len(rows))
	for i, row := range rows {
		if off >= len(row) {
			out[i] = nil
			continue
		}
		out[i] = row[off:]
	}
	return out
}

func fitWidths(rows [][]string, avail int) []int {
	widths := tableview.ColWidths(rows, 40)
	if len(widths) == 0 {
		return widths
	}
	need := 1
	for _, w := range widths {
		need += w + 3
	}
	for need > avail && len(widths) > 1 {
		need -= widths[len(widths)-1] + 3
		widths = widths[:len(widths)-1]
	}
	return widths
}

func renderRow(row []string, widths []int) string {
	var b strings.Builder
	b.WriteString("│")
	for i, w := range widths {
		cell := ""
		if i < len(row) {
			cell = row[i]
		}
		b.WriteByte(' ')
		b.WriteString(tableview.PadCell(cell, w))
		b.WriteString(" │")
	}
	return b.String()
}

func renderSep(widths []int, left, mid, right string) string {
	var b strings.Builder
	b.WriteString(left)
	for i, w := range widths {
		if i > 0 {
			b.WriteString(mid)
		}
		b.WriteString(strings.Repeat("─", w+2))
	}
	b.WriteString(right)
	return b.String()
}

func (m model) helpLine() string {
	if len(m.files) > 1 {
		return "tab/shift+tab 切换文件 · [/] 切换工作表 · ↑↓←→ 滚动 · q 退出"
	}
	f := m.file()
	if f != nil && len(f.Sheets) > 1 {
		return "[/] 切换工作表 · ↑↓←→ 滚动 · q 退出"
	}
	return "↑↓←→ 滚动 · q 退出"
}
