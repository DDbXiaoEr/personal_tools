package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"ansiman/internal/inventory"
)

func main() {
	path := flag.String("file", inventory.DefaultPath(), "清单文件路径")
	flag.Parse()

	inv, err := inventory.Load(*path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取清单失败: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(newModel(*path, inv), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "运行失败: %v\n", err)
		os.Exit(1)
	}
}
