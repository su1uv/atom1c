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

func (m *postsModel) updateListProperties() {
	h, v := m.common.styles.app.GetFrameSize()
	m.list.SetSize(m.common.width-h, m.common.height-v)

	m.common.styles = newStyles(m.common.isDarkBG)
	m.list.Styles.Title = m.common.styles.title
}

func (m postsModel) Update(msg tea.Msg) (postsModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.common.width, m.common.height = msg.Width, msg.Height
		m.updateListProperties()

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

func (m postsModel) View() tea.View {
	v := tea.NewView(m.common.styles.app.Render(m.list.View()))
	return v
}
