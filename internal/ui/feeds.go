package ui

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

type feed struct {
	name string
	url  string
}

func (f feed) Title() string       { return f.name }
func (f feed) Description() string { return f.url }
func (f feed) FilterValue() string { return f.name }

type feedsModel struct {
	common *commonModel
	list   list.Model
}

func (m feedsModel) Init() tea.Cmd {
	return nil
}

func (m *feedsModel) updateListProperties() {
	h, v := m.common.styles.app.GetFrameSize()
	m.list.SetSize(m.common.width-h, m.common.height-v)

	m.common.styles = newStyles(m.common.isDarkBG)
	m.list.Styles.Title = m.common.styles.title
}

func (m feedsModel) Update(msg tea.Msg) (feedsModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.common.isDarkBG = msg.IsDark()
		m.updateListProperties()
		return m, nil

	case tea.WindowSizeMsg:
		m.common.width, m.common.height = msg.Width, msg.Height
		m.updateListProperties()
		return m, nil

	case tea.KeyPressMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, m.common.keys.togglePagination):
			m.list.SetShowPagination(!m.list.ShowPagination())
		}
	}

	newListModel, cmd := m.list.Update(msg)
	m.list = newListModel
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m feedsModel) View() tea.View {
	v := tea.NewView(m.common.styles.app.Render(m.list.View()))
	return v
}
