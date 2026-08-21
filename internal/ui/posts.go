package ui

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

func initialPostsModel(c *commonModel) postsModel {
	// TODO: remove mocks get actual data
	mockPosts := make([]list.Item, len(postsMock))
	for i, post := range postsMock {
		mockPosts[i] = post
	}

	postsList := list.New(mockPosts, list.NewDefaultDelegate(), 0, 0)
	postsList.Title = "Posts"
	postsList.Styles.Title = c.styles.title
	postsList.SetShowHelp(false)

	return postsModel{
		common: c,
		list:   postsList,
	}
}

type postsModel struct {
	common *commonModel
	list   list.Model
}

func (m *postsModel) setSize(w, h int) {
	x, y := m.common.styles.list.GetFrameSize()
	m.list.SetSize((w-x)/2, h-y-helpHeight)
	m.common.styles.list = m.common.styles.list.Width((w - x) / 2)
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

func (m postsModel) View() tea.View {
	v := tea.NewView(m.common.styles.list.Render(m.list.View()))
	return v
}
