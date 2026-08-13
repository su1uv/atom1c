package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/su1uv/atom1c/internal"
)

func NewProgram(s *internal.State) *tea.Program {
	m := newModel(s)
	p := tea.NewProgram(m)
	return p
}

type commonModel struct {
	state  *internal.State
	width  int
	height int
	styles Styles
}

type model struct {
	common *commonModel
}

func newModel(s *internal.State) tea.Model {
	common := commonModel{
		state:  s,
		styles: newStyles(),
	}

	m := model{
		common: &common,
	}

	return m
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m model) View() tea.View {
	v := tea.NewView("this is a new view")
	return v
}
