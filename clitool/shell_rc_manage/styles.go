package main

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	activeTabStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Padding(0, 1)
	inactiveTabStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(0, 1)
	selectedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57"))
	dimStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	catStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	labelStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Width(16)
	focusedLabel     = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Width(16)
	helpStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	okStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("76"))
	boxStyle         = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
)
