package ui

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
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

	const numFeeds = 5
	mockFeeds := make([]list.Item, numFeeds)
	for i := range mockFeeds {
		mockFeeds[i] = feed{
			name: fmt.Sprintf("Feed %v", i),
			url:  "https://random.com",
		}
	}

	delegate := newItemDelegate(delegateKeys, &common.styles)
	feedList := list.New(mockFeeds, delegate, 0, 0)
	feedList.Title = "Feeds"
	feedList.Styles.Title = common.styles.title
	feedList.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			keys.togglePagination,
		}
	}

	common.state = s
	common.delegateKeys = delegateKeys
	common.keys = keys

	feeds := feedsModel{
		common: &common,
		list:   feedList,
	}

	m := model{
		common: &common,
		feeds:  feeds,
	}

	return m
}

type commonModel struct {
	state        *internal.State
	width        int
	height       int
	isDarkBG     bool
	styles       Styles
	keys         *listKeyMap
	delegateKeys *delegateKeyMap
}

type model struct {
	common *commonModel
	feeds  feedsModel
}

type listKeyMap struct {
	togglePagination key.Binding
}

func newListKeyMap() *listKeyMap {
	return &listKeyMap{
		togglePagination: key.NewBinding(
			key.WithKeys("P"),
			key.WithHelp("P", "toggle pagination"),
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

	return m, tea.Batch(cmds...)
}

func (m model) View() tea.View {
	content := m.feeds.View().Content
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}
