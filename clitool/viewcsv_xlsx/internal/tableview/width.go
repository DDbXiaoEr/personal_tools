package tableview

import (
	"strings"
	"unicode"

	"github.com/mattn/go-runewidth"
)

func Sanitize(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\t':
			b.WriteString("    ")
		case r == '\n' || r == '\r':
			b.WriteByte(' ')
		case unicode.IsControl(r):
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func CellWidth(s string) int {
	return runewidth.StringWidth(Sanitize(s))
}

func PadCell(s string, width int) string {
	s = Sanitize(s)
	w := runewidth.StringWidth(s)
	if w >= width {
		return runewidth.Truncate(s, width, "")
	}
	return s + spaces(width-w)
}

func ColWidths(rows [][]string, maxCol int) []int {
	n := 0
	for _, row := range rows {
		if len(row) > n {
			n = len(row)
		}
	}
	widths := make([]int, n)
	for _, row := range rows {
		for i, cell := range row {
			w := CellWidth(cell)
			if w > widths[i] {
				widths[i] = w
			}
		}
	}
	if maxCol > 0 {
		for i := range widths {
			if widths[i] > maxCol {
				widths[i] = maxCol
			}
		}
	}
	return widths
}

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}
