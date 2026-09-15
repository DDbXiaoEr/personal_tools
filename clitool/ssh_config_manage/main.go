package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"sshconfig-manage/internal/sshconfig"
)

func main() {
	path := flag.String("file", sshconfig.DefaultPath(), "ssh config 文件路径")
	flag.Parse()

	cfg, err := sshconfig.ParseFile(*path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取配置失败: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(newModel(*path, cfg), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "运行失败: %v\n", err)
		os.Exit(1)
	}
}
