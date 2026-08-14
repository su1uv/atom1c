package ui

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

type postsModel struct {
	common *commonModel
	list   list.Model
}

func (m *postsModel) setSize(w, h int) {
	x, y := m.common.styles.app.GetFrameSize()
	m.list.SetSize((w-x)/2, h-y)
}

func (m postsModel) Update(msg tea.Msg) (postsModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, m.common.keys.togglePagination):
			m.list.SetShowPagination(!m.list.ShowPagination())

		case key.Matches(msg, m.common.keys.deselectItem):
			m.common.focus = focusFeeds
			return m, nil
		}
	}
	if m.common.focus != focusPosts {
		return m, nil
	}
	newListModel, cmd := m.list.Update(msg)
	m.list = newListModel
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m postsModel) View() string {
	v := m.common.styles.app.Render(m.list.View())
	return v
}
