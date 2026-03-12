package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	White     = lipgloss.Color("255")
	BrightWhite = lipgloss.Color("255")
	Gray      = lipgloss.Color("250")
	Magenta   = lipgloss.Color("201")
	Pink      = lipgloss.Color("205")
	Blue      = lipgloss.Color("39")
	Green     = lipgloss.Color("42")
	Red       = lipgloss.Color("203")
	DarkGray  = lipgloss.Color("240")

	// Styles
	HeaderStyle = lipgloss.NewStyle().
			Foreground(Pink).
			Bold(true).
			Padding(0, 1)

	SelectedStyle = lipgloss.NewStyle().
			Foreground(Magenta).
			Bold(true)

	InfoStyle = lipgloss.NewStyle().
			Foreground(White)

	InactiveStyle = lipgloss.NewStyle().
			Foreground(Gray)

	PathStyle = lipgloss.NewStyle().
			Foreground(Blue).
			Bold(true)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(Green).
			Bold(true)

	WarningStyle = lipgloss.NewStyle().
			Foreground(Red).
			Bold(true)

	// Box Styles
	MainBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Gray).
			Padding(0, 1)

	HeaderBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(Gray)

	FooterBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(Gray)

	// Custom UI Elements
	CursorStyle = lipgloss.NewStyle().
			Background(Magenta).
			Foreground(White).
			Bold(true)

	ErrorMessageStyle = lipgloss.NewStyle().
			Background(Red).
			Foreground(White).
			Bold(true).
			Padding(0, 1)

	SuccessMessageStyle = lipgloss.NewStyle().
			Background(Green).
			Foreground(White).
			Bold(true).
			Padding(0, 1)
)
