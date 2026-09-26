package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"powershell-rc-manage/internal/psrc"
)

func main() {
	dir := flag.String("dir", psrc.DefaultDir(), "配置目录（默认 $HOME）")
	flag.Parse()

	store, err := psrc.Load(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取配置失败: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(newModel(*dir, store), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "运行失败: %v\n", err)
		os.Exit(1)
	}
}
