package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"occonfig/internal/opencode"
)

func main() {
	configPath := flag.String("config", opencode.DefaultPath(), "opencode 配置文件路径")
	skillsDir := flag.String("skills-dir", opencode.DefaultSkillsDir(), "全局技能目录")
	flag.Parse()

	cfg, err := opencode.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取配置失败: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(newModel(cfg, *skillsDir), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "运行失败: %v\n", err)
		os.Exit(1)
	}
}
