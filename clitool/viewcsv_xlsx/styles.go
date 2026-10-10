package main

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	activeTabStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Padding(0, 1)
	inactiveTabStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(0, 1)
	selectedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57"))
	dimStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	helpStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	headerStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
)
