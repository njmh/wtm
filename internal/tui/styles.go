package tui

import "charm.land/lipgloss/v2"

var (
	colorAccent = lipgloss.Color("99")
	colorMuted  = lipgloss.Color("244")
	colorDanger = lipgloss.Color("203")
	colorWarn   = lipgloss.Color("214")
	colorOK     = lipgloss.Color("78")
	colorInfo   = lipgloss.Color("75")

	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	mutedStyle    = lipgloss.NewStyle().Foreground(colorMuted)
	headerStyle   = lipgloss.NewStyle().Foreground(colorMuted).Bold(true)
	selectedStyle = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color("231")).Background(lipgloss.Color("238"))
	keyStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	okStyle    = lipgloss.NewStyle().Foreground(colorOK)
	busyStyle  = lipgloss.NewStyle().Foreground(colorWarn)
	errorStyle = lipgloss.NewStyle().Foreground(colorDanger)
	warnStyle  = lipgloss.NewStyle().Foreground(colorWarn)
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(1, 2)
	labelStyle = lipgloss.NewStyle().Bold(true)
)
