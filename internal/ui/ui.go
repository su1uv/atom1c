package ui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/su1uv/atom1c/internal"
)

func NewProgram(s *internal.State) *tea.Program {
	m := newModel(s)
	p := tea.NewProgram(m)
	return p
}

func newModel(s *internal.State) tea.Model {
	common := commonModel{}

	keys := newListKeyMap()

	common.styles = newStyles(false)
	common.state = s
	common.keys = keys
	common.focus = focusFeeds

	feeds := initialFeedsModel(&common)
	posts := initialPostsModel(&common)

	m := model{
		common: &common,
		feeds:  feeds,
		posts:  posts,
	}

	return m
}

type focusState int

const (
	focusFeeds focusState = iota
	focusPosts
)

type commonModel struct {
	state    *internal.State
	isDarkBG bool
	styles   Styles
	keys     *listKeyMap
	width    int
	height   int
	focus    focusState
}

type model struct {
	common *commonModel
	feeds  feedsModel
	posts  postsModel
}

type listKeyMap struct {
	togglePagination key.Binding
	selectItem       key.Binding
	deselectItem     key.Binding
}

func newListKeyMap() *listKeyMap {
	return &listKeyMap{
		togglePagination: key.NewBinding(
			key.WithKeys("P"),
			key.WithHelp("P", "toggle pagination"),
		),
		selectItem: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "select item"),
		),
		deselectItem: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "go back"),
		),
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.common.height = msg.Height
		m.common.width = msg.Width
		m.feeds.setSize(msg.Width, msg.Height)
		m.posts.setSize(msg.Width, msg.Height)
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}

	// propagate to feeds model
	feedsModel, feedsCmd := m.feeds.Update(msg)
	m.feeds = feedsModel
	cmds = append(cmds, feedsCmd)
	// propagate to posts model
	postsModel, postsCmd := m.posts.Update(msg)
	m.posts = postsModel
	cmds = append(cmds, postsCmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() tea.View {
	feedsContent := m.feeds.View().Content
	postsContent := m.posts.View()
	v := tea.NewView(lipgloss.JoinHorizontal(lipgloss.Top, feedsContent, postsContent))
	v.AltScreen = true
	return v
}
