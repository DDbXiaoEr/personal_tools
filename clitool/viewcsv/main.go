package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

func main() {
	delimiter := flag.String("d", ",", "delimiter character")
	noHeader := flag.Bool("no-header", false, "treat first row as data, not header")
	flag.Parse()

	patterns := flag.Args()
	if len(patterns) == 0 {
		fmt.Fprintln(os.Stderr, "usage: viewcsv file.csv [file.csv ...] [-d delimiter] [--no-header]")
		fmt.Fprintln(os.Stderr, "       viewcsv *.csv")
		os.Exit(1)
	}

	files, err := expandFiles(patterns)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	for i, filename := range files {
		if len(files) > 1 {
			if i > 0 {
				fmt.Println()
			}
			fmt.Println("=== " + filename + " ===")
		}
		if err := viewCSV(filename, *delimiter, *noHeader); err != nil {
			fmt.Fprintln(os.Stderr, "error:", filename+":", err)
			os.Exit(1)
		}
	}
}

func expandFiles(patterns []string) ([]string, error) {
	seen := make(map[string]bool)
	var files []string
	for _, pattern := range patterns {
		if hasGlob(pattern) {
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return nil, err
			}
			if len(matches) == 0 {
				return nil, fmt.Errorf("no files matching %q", pattern)
			}
			sort.Strings(matches)
			for _, m := range matches {
				if !seen[m] {
					seen[m] = true
					files = append(files, m)
				}
			}
			continue
		}
		if !seen[pattern] {
			seen[pattern] = true
			files = append(files, pattern)
		}
	}
	return files, nil
}

func hasGlob(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

func viewCSV(filename, delimiter string, noHeader bool) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	if len(delimiter) > 0 {
		reader.Comma = rune(delimiter[0])
	}

	records, err := reader.ReadAll()
	if err != nil {
		return err
	}

	if len(records) == 0 {
		fmt.Println("empty CSV file")
		return nil
	}

	printTable(records, noHeader)
	return nil
}

func printTable(records [][]string, noHeader bool) {
	if len(records) == 0 {
		return
	}

	numCols := len(records[0])
	for _, row := range records {
		if len(row) > numCols {
			numCols = len(row)
		}
	}

	colWidths := make([]int, numCols)
	for _, row := range records {
		for i, cell := range row {
			if i < numCols {
				width := stringWidth(cell)
				if width > colWidths[i] {
					colWidths[i] = width
				}
			}
		}
	}

	printSeparator(colWidths, true, false, false)
	for i, row := range records {
		printRow(row, colWidths)
		if i == 0 && !noHeader {
			printSeparator(colWidths, false, false, true)
		}
	}
	printSeparator(colWidths, false, true, false)
}

func printRow(row []string, colWidths []int) {
	fmt.Print("│")
	for i, width := range colWidths {
		cell := ""
		if i < len(row) {
			cell = row[i]
		}
		fmt.Printf(" %-*s │", width, cell)
	}
	fmt.Println()
}

func printSeparator(colWidths []int, isTop, isBottom, isHeader bool) {
	left := "├"
	right := "┤"
	mid := "┼"
	
	if isTop {
		left = "┌"
		right = "┐"
		mid = "┬"
	} else if isBottom {
		left = "└"
		right = "┘"
		mid = "┴"
	} else if isHeader {
		left = "├"
		right = "┤"
		mid = "┼"
	}
	
	fmt.Print(left)
	for i, width := range colWidths {
		if i > 0 {
			fmt.Print(mid)
		}
		fmt.Print(strings.Repeat("─", width+2))
	}
	fmt.Println(right)
}

func stringWidth(s string) int {
	width := 0
	for _, r := range s {
		if isWideChar(r) {
			width += 2
		} else {
			width += 1
		}
	}
	return width
}

func isWideChar(r rune) bool {
	return utf8.RuneLen(r) > 1
}