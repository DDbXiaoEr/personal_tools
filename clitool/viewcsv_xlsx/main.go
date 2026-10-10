package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"viewcsv_xlsx/internal/tableview"
)

func main() {
	delimiter := flag.String("d", ",", "分隔符（仅 CSV）")
	noHeader := flag.Bool("no-header", false, "第一行当数据，不当表头")
	sheet := flag.String("sheet", "", "工作表名称或从 1 开始的序号（仅 XLSX，默认全部）")
	flag.Parse()

	patterns := flag.Args()
	if len(patterns) == 0 {
		fmt.Fprintln(os.Stderr, "usage: viewcsv_xlsx file.csv|file.xlsx [file ...] [-d delimiter] [--no-header] [--sheet name]")
		fmt.Fprintln(os.Stderr, "       viewcsv_xlsx *.xlsx")
		os.Exit(1)
	}

	paths, err := tableview.Expand(patterns)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	files := make([]*tableview.File, 0, len(paths))
	for _, path := range paths {
		f, err := tableview.Load(path, *delimiter, *sheet)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", path+":", err)
			os.Exit(1)
		}
		files = append(files, f)
	}

	if len(files) < 2 || !term.IsTerminal(int(os.Stdout.Fd())) {
		printFiles(files, *noHeader)
		return
	}

	p := tea.NewProgram(newModel(files, *noHeader), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "运行失败:", err)
		os.Exit(1)
	}
}
