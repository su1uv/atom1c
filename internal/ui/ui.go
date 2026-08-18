package ui

import (
	"charm.land/bubbles/v2/help"
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
	addFeed := initialAddFeedModel(&common)

	m := model{
		common:  &common,
		feeds:   feeds,
		posts:   posts,
		help:    help.New(),
		addFeed: addFeed,
	}

	return m
}

type focusState int

const (
	focusFeeds focusState = iota
	focusPosts
)
const helpHeight = 3

type item struct {
	name string
	url  string
}

func (f item) Title() string       { return f.name }
func (f item) Description() string { return f.url }
func (f item) FilterValue() string { return f.name }

type commonModel struct {
	state       *internal.State
	isDarkBG    bool
	styles      Styles
	keys        *listKeyMap
	width       int
	height      int
	focus       focusState
	isOpenModal bool
}

type model struct {
	common  *commonModel
	feeds   feedsModel
	posts   postsModel
	help    help.Model
	addFeed addFeedModel
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

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.common.keys.addFeed):
			m.common.isOpenModal = true
		}
	}

	if m.common.isOpenModal {
		addFeedModel, addFeedCmd := m.addFeed.Update(msg)
		return addFeedModel, addFeedCmd
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
	help := m.help.ShortHelpView([]key.Binding{
		m.common.keys.addFeed,
		m.common.keys.next,
		m.common.keys.prev,
		m.common.keys.nextPage,
		m.common.keys.prevPage,
		m.common.keys.filter,
		m.common.keys.selectItem,
		m.common.keys.deselectItem,
		m.common.keys.togglePagination,
		m.common.keys.quit,
	})

	feedsContent := m.feeds.View().Content
	postsContent := m.posts.View().Content
	content := m.common.styles.app.Render(lipgloss.JoinHorizontal(lipgloss.Top, feedsContent, postsContent) + "\n\n" + help)

	if m.common.isOpenModal {
		content = overlayModal(content, m.addFeed.View().Content, m.common.width, m.common.height)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}
