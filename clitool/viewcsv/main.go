package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

func main() {
	input := flag.String("i", "", "input CSV file")
	delimiter := flag.String("d", ",", "delimiter character")
	noHeader := flag.Bool("no-header", false, "treat first row as data, not header")
	flag.Parse()

	if *input == "" {
		if flag.NArg() > 0 {
			*input = flag.Arg(0)
		} else {
			fmt.Fprintln(os.Stderr, "usage: viewcsv -i input.csv [-d delimiter] [--no-header]")
			os.Exit(1)
		}
	}

	if err := viewCSV(*input, *delimiter, *noHeader); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
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