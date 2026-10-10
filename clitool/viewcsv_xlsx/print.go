package main

import (
	"fmt"
	"strings"

	"viewcsv_xlsx/internal/tableview"
)

func printFiles(files []*tableview.File, noHeader bool) {
	for i, f := range files {
		if i > 0 {
			fmt.Println()
		}
		if len(files) > 1 {
			fmt.Println("=== " + f.Path + " ===")
		}
		printFile(f, noHeader)
	}
}

func printFile(f *tableview.File, noHeader bool) {
	if len(f.Sheets) == 0 {
		fmt.Println("空文件")
		return
	}
	multi := len(f.Sheets) > 1
	for i, sh := range f.Sheets {
		if multi {
			if i > 0 {
				fmt.Println()
			}
			fmt.Println("--- sheet: " + sh.Name + " ---")
		}
		if len(sh.Rows) == 0 {
			fmt.Println("空表")
			continue
		}
		printTable(sh.Rows, noHeader)
	}
}

func printTable(records [][]string, noHeader bool) {
	widths := tableview.ColWidths(records, 0)
	printSep(widths, "┌", "┬", "┐")
	for i, row := range records {
		printRow(row, widths)
		if i == 0 && !noHeader {
			printSep(widths, "├", "┼", "┤")
		}
	}
	printSep(widths, "└", "┴", "┘")
}

func printRow(row []string, widths []int) {
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
	fmt.Println(b.String())
}

func printSep(widths []int, left, mid, right string) {
	var b strings.Builder
	b.WriteString(left)
	for i, w := range widths {
		if i > 0 {
			b.WriteString(mid)
		}
		b.WriteString(strings.Repeat("─", w+2))
	}
	b.WriteString(right)
	fmt.Println(b.String())
}
