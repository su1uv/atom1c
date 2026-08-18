package ui

import "charm.land/bubbles/v2/key"

type listKeyMap struct {
	next             key.Binding
	prev             key.Binding
	nextPage         key.Binding
	prevPage         key.Binding
	filter           key.Binding
	togglePagination key.Binding
	selectItem       key.Binding
	deselectItem     key.Binding
	quit             key.Binding
	addFeed          key.Binding
}

func newListKeyMap() *listKeyMap {
	return &listKeyMap{
		addFeed: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add feed"),
		),
		next: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j", "next"),
		),
		prev: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k", "prev"),
		),
		nextPage: key.NewBinding(
			key.WithKeys("l", "right"),
			key.WithHelp("l", "next page"),
		),
		prevPage: key.NewBinding(
			key.WithKeys("h", "left"),
			key.WithHelp("h", "prev page"),
		),
		filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter search"),
		),
		togglePagination: key.NewBinding(
			key.WithKeys("P"),
			key.WithHelp("P", "toggle pagination"),
		),
		selectItem: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "select"),
		),
		deselectItem: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "deselect"),
		),
		quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

type modalKeyMap struct {
	close key.Binding
}

func newModalKeyMap() *modalKeyMap {
	return &modalKeyMap{
		close: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "close"),
		),
	}
}
