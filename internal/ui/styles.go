package ui

import "charm.land/lipgloss/v2"

type Styles struct {
	app           lipgloss.Style
	title         lipgloss.Style
	statusMessage lipgloss.Style
}

func newStyles(isDarkBG bool) Styles {
	lightDark := lipgloss.LightDark(isDarkBG)

	return Styles{
		app: lipgloss.NewStyle().
			Padding(1, 2),
		title: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1),
		statusMessage: lipgloss.NewStyle().
			Foreground(lightDark(lipgloss.Color("#04B575"), lipgloss.Color("#04B575"))),
	}
}
