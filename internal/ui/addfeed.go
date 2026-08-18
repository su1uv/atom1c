package ui

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

type addFeedModel struct {
	common      *commonModel
	modalKeyMap *modalKeyMap
}

func (m addFeedModel) Init() tea.Cmd {
	return nil
}

func (m addFeedModel) Update(msg tea.Msg) (addFeedModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.modalKeyMap.close):
			fmt.Printf("close modal")
			return m, nil
		}
	}

	return m, tea.Batch(cmds...)
}

func (m addFeedModel) View() tea.View {
	return tea.NewView("This is a modal")
}
