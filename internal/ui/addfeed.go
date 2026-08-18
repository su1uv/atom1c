package ui

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func initialAddFeedModel(common *commonModel) addFeedModel {

	return addFeedModel{
		common:      common,
		modalKeyMap: newModalKeyMap(),
	}
}

type addFeedModel struct {
	common      *commonModel
	modalKeyMap *modalKeyMap
}

func (m addFeedModel) Init() tea.Cmd {
	return nil
}

func (m addFeedModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func overlayModal(bg, modal string, w, h int) string {
	mw, mh := lipgloss.Width(modal), lipgloss.Height(modal)
	x := max((w-mw)/2, 0)
	y := max((h-mh)/2, 0)

	baseLayer := lipgloss.NewLayer(bg).X(0).Y(0).Z(0)
	modalLayer := lipgloss.NewLayer(modal).X(x).Y(y).Z(1)

	return lipgloss.NewCompositor(baseLayer, modalLayer).Render()
}
