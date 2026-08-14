package ui

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
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

	delegateKeys := newDelegateKeyMap()
	keys := newListKeyMap()

	common.styles = newStyles(false)

	const numMocks = 25
	mockFeeds := make([]list.Item, numMocks)
	mockPosts := make([]list.Item, numMocks)
	for i := range numMocks {
		mockFeeds[i] = item{
			name: fmt.Sprintf("Feed %v", i),
			url:  "https://random.com",
		}
		mockPosts[i] = item{
			name: fmt.Sprintf("Posts %v", i),
			url:  "https://random-post.com",
		}
	}

	delegateFeeds := newItemDelegate(delegateKeys, &common.styles)
	delegatePosts := newItemDelegate(delegateKeys, &common.styles)

	feedList := list.New(mockFeeds, delegateFeeds, 0, 0)
	feedList.Title = "Feeds"
	feedList.Styles.Title = common.styles.title
	feedList.SetShowHelp(false)

	postsList := list.New(mockPosts, delegatePosts, 0, 0)
	postsList.Title = "Posts"
	postsList.Styles.Title = common.styles.title
	postsList.SetShowHelp(false)

	common.state = s
	common.keys = keys
	common.delegateKeys = delegateKeys
	common.focus = focusFeeds

	feeds := feedsModel{
		common: &common,
		list:   feedList,
	}

	posts := postsModel{
		common: &common,
		list:   postsList,
	}

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
	state        *internal.State
	width        int
	height       int
	isDarkBG     bool
	styles       Styles
	keys         *listKeyMap
	delegateKeys *delegateKeyMap

	focus focusState
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
	postsContent := m.posts.View().Content
	v := tea.NewView(lipgloss.JoinHorizontal(lipgloss.Top, feedsContent, postsContent))
	v.AltScreen = true
	return v
}
