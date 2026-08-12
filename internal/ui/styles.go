package ui

import "charm.land/lipgloss/v2"

type Styles struct {
	App           lipgloss.Style
	Title         lipgloss.Style
	StatusMessage lipgloss.Style
}

func NewStyles(darkBG bool) Styles {
	lightDark := lipgloss.LightDark(darkBG)

	return Styles{
		App: lipgloss.NewStyle().
			Padding(1, 2),
		Title: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1),
		StatusMessage: lipgloss.NewStyle().
			Foreground(lightDark(lipgloss.Color("#04B575"), lipgloss.Color("#04B575"))),
	}
}

type TabStyles struct {
	Doc         lipgloss.Style
	Highlight   lipgloss.Style
	InactiveTab lipgloss.Style
	ActiveTab   lipgloss.Style
	Window      lipgloss.Style
}

func NewTabStyles(darkBG bool) *TabStyles {
	lightDark := lipgloss.LightDark(darkBG)

	inactiveTabBorder := tabBorderWithBottom("┴", "─", "┴")
	activeTabBorder := tabBorderWithBottom("┘", " ", "└")
	highlightColor := lightDark(lipgloss.Color("#874BFD"), lipgloss.Color("#7D56F4"))

	s := new(TabStyles)
	s.Doc = lipgloss.NewStyle().Padding(1, 2, 1, 2)
	s.InactiveTab = lipgloss.NewStyle().
		Border(inactiveTabBorder, true).
		BorderForeground(highlightColor).
		Padding(0, 1)
	s.ActiveTab = s.InactiveTab.
		Border(activeTabBorder, true)
	s.Window = lipgloss.NewStyle().
		BorderForeground(highlightColor).
		Padding(2, 0).
		Align(lipgloss.Center).
		Border(lipgloss.NormalBorder()).
		UnsetBorderTop()
	return s
}

func tabBorderWithBottom(left, middle, right string) lipgloss.Border {
	border := lipgloss.RoundedBorder()
	border.BottomLeft = left
	border.Bottom = middle
	border.BottomRight = right
	return border
}
