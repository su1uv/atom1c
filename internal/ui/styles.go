package ui

import "charm.land/lipgloss/v2"

type Styles struct {
	some lipgloss.Style
}

func newStyles() Styles {
	return Styles{
		some: lipgloss.NewStyle(),
	}
}
