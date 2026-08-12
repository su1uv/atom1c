package ui

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/joho/godotenv"
	"github.com/su1uv/atom1c/internal/db"
)

type feed struct {
	title       string
	description string
}

func GenerateFeeds(dbFeed db.Feed) feed {
	return feed{
		title:       dbFeed.Name,
		description: dbFeed.Url,
	}
}

func (f feed) Title() string       { return f.title }
func (f feed) Description() string { return f.description }
func (f feed) FilterValue() string { return f.title }

type keyMap struct {
	toggleSpinner    key.Binding
	toggleTitleBar   key.Binding
	toggleStatusBar  key.Binding
	togglePagination key.Binding
	toggleHelpMenu   key.Binding
	insertItem       key.Binding
}

func newKeyMap() *keyMap {
	return &keyMap{
		toggleSpinner: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "toggle spinner"),
		),
		insertItem: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "insert item"),
		),
		toggleTitleBar: key.NewBinding(
			key.WithKeys("T"),
			key.WithHelp("T", "toggle title bar"),
		),
		toggleStatusBar: key.NewBinding(
			key.WithKeys("S"),
			key.WithHelp("S", "toggle status bar"),
		),
		togglePagination: key.NewBinding(
			key.WithKeys("P"),
			key.WithHelp("P", "toggle pagination"),
		),
		toggleHelpMenu: key.NewBinding(
			key.WithKeys("H"),
			key.WithHelp("H", "toggle help"),
		),
	}
}

type model struct {
	styles        Styles
	darkBG        bool
	width, height int
	list          list.Model
	keys          *keyMap
	delegateKeys  *DelegateKeyMap
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.RequestBackgroundColor,
	)
}

func (m *model) updateListProps() {
	h, v := m.styles.App.GetFrameSize()
	m.list.SetSize(m.width-h, m.height-v)

	m.styles = NewStyles(m.darkBG)
	m.list.Styles.Title = m.styles.Title
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.darkBG = msg.IsDark()
		m.updateListProps()
		return m, nil
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.updateListProps()
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, m.keys.toggleSpinner):
			cmd := m.list.ToggleSpinner()
			return m, cmd
		case key.Matches(msg, m.keys.toggleTitleBar):
			v := !m.list.ShowTitle()
			m.list.SetShowTitle(v)
			m.list.SetShowFilter(v)
			m.list.SetFilteringEnabled(v)
			return m, nil
		case key.Matches(msg, m.keys.toggleStatusBar):
			m.list.SetShowStatusBar(!m.list.ShowStatusBar())
			return m, nil

		case key.Matches(msg, m.keys.togglePagination):
			m.list.SetShowPagination(!m.list.ShowPagination())
			return m, nil

		case key.Matches(msg, m.keys.toggleHelpMenu):
			m.list.SetShowHelp(!m.list.ShowHelp())
			return m, nil

		case key.Matches(msg, m.keys.insertItem):
			m.delegateKeys.Remove.SetEnabled(true)
			return m, nil
		}
	}

	newListModel, cmd := m.list.Update(msg)
	m.list = newListModel
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() tea.View {
	v := tea.NewView(m.styles.App.Render(m.list.View()))
	v.AltScreen = true
	return v
}

func initialModel() model {
	godotenv.Load()

	m := model{}
	m.styles = NewStyles(false)

	delegateKeys := NewDelegateKeyMap()
	keys := newKeyMap()

	items := make([]list.Item, 5)
	// for i, feed := range feeds {
	// 	items[i] = GenerateFeeds(feed)
	// }

	delegate := NewItemDelegate(delegateKeys, &m.styles)
	feedList := list.New(items, delegate, 0, 0)
	feedList.Title = "Feeds"
	feedList.Styles.Title = m.styles.Title
	feedList.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			keys.toggleSpinner,
			keys.insertItem,
			keys.toggleHelpMenu,
			keys.togglePagination,
			keys.toggleStatusBar,
			keys.toggleTitleBar,
		}
	}

	m.list = feedList
	m.keys = keys
	m.delegateKeys = delegateKeys

	return m
}
