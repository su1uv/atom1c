package ui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/su1uv/atom1c/internal"
)

func NewProgram(s *internal.State) *tea.Program {
	return tea.NewProgram(newModel(s))
}

func newModel(s *internal.State) tea.Model {
	styles := newStyles(false)
	keys := newListKeyMap()

	return model{
		state:   s,
		styles:  styles,
		keys:    keys,
		focus:   focusFeeds,
		feeds:   initialFeedsModel(styles),
		posts:   initialPostsModel(styles),
		help:    help.New(),
		addFeed: initialAddFeedModel(styles.modal),
	}
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

// model owns application-level focus and routes each key to one owner.
type model struct {
	state     *internal.State
	styles    Styles
	keys      *listKeyMap
	width     int
	height    int
	focus     focusState
	modalOpen bool
	feeds     listPane
	posts     listPane
	help      help.Model
	addFeed   addFeedModel
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if result, ok := msg.(paneMessage); ok {
		return m, m.pane(result.pane).Update(result.msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.feeds.setSize(msg.Width, msg.Height)
		m.posts.setSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyPressMsg:
		if m.modalOpen {
			return m.updateModal(msg)
		}

		// A filter owns keyboard input until filter mode ends. Printable global
		// shortcuts therefore remain filter text.
		if m.activePane().list.FilterState() == list.Filtering {
			return m, m.updateActivePane(msg)
		}

		switch {
		case key.Matches(msg, m.keys.addFeed):
			m.modalOpen = true
			return m, m.addFeed.open()
		case key.Matches(msg, m.keys.selectItem) && m.focus == focusFeeds:
			m.focus = focusPosts
			return m, nil
		case key.Matches(msg, m.keys.deselectItem) && m.focus == focusPosts:
			m.focus = focusFeeds
			return m, nil
		case key.Matches(msg, m.keys.togglePagination):
			pane := m.activePane()
			pane.list.SetShowPagination(!pane.list.ShowPagination())
			return m, nil
		}

		return m, m.updateActivePane(msg)
	}

	if m.modalOpen {
		return m.updateModal(msg)
	}

	// Non-key messages may be results of work started by either pane. Updating
	// both lets each list consume only messages relevant to its own state.
	feedsCmd := m.feeds.Update(msg)
	postsCmd := m.posts.Update(msg)
	return m, tea.Batch(feedsCmd, postsCmd)
}

func (m model) updateModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	var action addFeedAction
	var cmd tea.Cmd
	m.addFeed, cmd, action = m.addFeed.Update(msg)
	if action != addFeedNoAction {
		m.modalOpen = false
	}
	return m, cmd
}

func (m *model) activePane() *listPane {
	return m.pane(m.focus)
}

func (m *model) pane(id focusState) *listPane {
	if id == focusPosts {
		return &m.posts
	}
	return &m.feeds
}

func (m *model) updateActivePane(msg tea.Msg) tea.Cmd {
	return m.activePane().Update(msg)
}

func (m model) View() tea.View {
	help := m.help.ShortHelpView([]key.Binding{
		m.keys.addFeed,
		m.keys.next,
		m.keys.prev,
		m.keys.nextPage,
		m.keys.prevPage,
		m.keys.filter,
		m.keys.selectItem,
		m.keys.deselectItem,
		m.keys.togglePagination,
		m.keys.quit,
	})

	feedsContent := m.feeds.View().Content
	postsContent := m.posts.View().Content
	content := m.styles.app.Render(lipgloss.JoinHorizontal(lipgloss.Top, feedsContent, postsContent) + "\n\n" + help)

	if m.modalOpen {
		content = overlayModal(content, m.addFeed.View().Content, m.width, m.height)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}
