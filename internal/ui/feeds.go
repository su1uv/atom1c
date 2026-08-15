package ui

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

type item struct {
	name string
	url  string
}

func (f item) Title() string       { return f.name }
func (f item) Description() string { return f.url }
func (f item) FilterValue() string { return f.name }

func initialFeedsModel(c *commonModel) feedsModel {
	const numMocks = 25
	mockFeeds := make([]list.Item, numMocks)
	for i := range numMocks {
		mockFeeds[i] = item{
			name: fmt.Sprintf("Feed %v", i),
			url:  "https://random.com",
		}
	}

	feedList := list.New(mockFeeds, list.NewDefaultDelegate(), 0, 0)
	feedList.Title = "Feeds"
	feedList.Styles.Title = c.styles.title
	feedList.SetShowHelp(false)

	return feedsModel{
		common: c,
		list:   feedList,
	}
}

type feedsModel struct {
	common *commonModel
	list   list.Model
}

func (m *feedsModel) setSize(w, h int) {
	x, y := m.common.styles.list.GetFrameSize()
	m.list.SetSize((w-x)/2, h-y-helpHeight)
	m.common.styles.list = m.common.styles.list.Width((w) / 2)
}

func (m feedsModel) Init() tea.Cmd {
	return nil
}

func (m feedsModel) Update(msg tea.Msg) (feedsModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, m.common.keys.togglePagination):
			m.list.SetShowPagination(!m.list.ShowPagination())

		case key.Matches(msg, m.common.keys.selectItem):
			m.common.focus = focusPosts
			return m, nil
		}
	}
	if m.common.focus != focusFeeds {
		return m, nil
	}

	newListModel, cmd := m.list.Update(msg)
	m.list = newListModel
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m feedsModel) View() tea.View {
	v := tea.NewView(m.common.styles.list.Render(m.list.View()))
	return v
}
