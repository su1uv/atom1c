package ui

import "charm.land/lipgloss/v2"

type Styles struct {
	app           lipgloss.Style
	title         lipgloss.Style
	statusMessage lipgloss.Style
	list          lipgloss.Style
}

func newStyles(isDarkBG bool) Styles {
	lightDark := lipgloss.LightDark(isDarkBG)

	return Styles{
		app: lipgloss.NewStyle().
			Margin(0, 1, 1, 1),
		list: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#9C7CA5")),
		title: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#372248")),
		statusMessage: lipgloss.NewStyle().
			Foreground(lightDark(lipgloss.Color("#04B575"), lipgloss.Color("#04B575"))),
	}
}
