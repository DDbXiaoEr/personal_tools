package main

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	activeTabStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Padding(0, 1)
	inactiveTabStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(0, 1)
	selectedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57"))
	dimStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	labelStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Width(24)
	focusedLabel       = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Width(24)
	helpStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	okStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("76"))
	warnStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	boxStyle           = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	buttonStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Border(lipgloss.RoundedBorder()).Padding(0, 1)
	buttonFocusedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Border(lipgloss.RoundedBorder()).Padding(0, 1)
)
