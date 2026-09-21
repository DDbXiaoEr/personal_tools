package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"shellrc-manage/internal/shellrc"
)

func main() {
	dir := flag.String("dir", shellrc.DefaultDir(), "配置目录（默认 $HOME）")
	shell := flag.String("shell", shellrc.DetectShell(), "shell 名称：zsh 或 bash")
	flag.Parse()

	store, err := shellrc.Load(*dir, *shell)
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
