package ui

import "charm.land/lipgloss/v2"

type Styles struct {
	app   lipgloss.Style
	title lipgloss.Style
	list  lipgloss.Style
	modal lipgloss.Style
}

func newStyles(isDarkBG bool) Styles {
	return Styles{
		app: lipgloss.NewStyle().
			Margin(0, 1, 1, 1),
		list: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#9C7CA5")),
		title: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#372248")),
		modal: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#9C7CA5")),
	}
}
