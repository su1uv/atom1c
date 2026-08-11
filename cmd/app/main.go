package main

import (
	"fmt"
	"os"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	_ "github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/ui"
)

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
	state         *internal.State
	styles        ui.Styles
	darkBG        bool
	width, height int
	list          list.Model
	keys          *keyMap
	delegateKeys  *ui.DelegateKeyMap
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.RequestBackgroundColor,
	)
}

func (m *model) updateListProps() {
	h, v := m.styles.App.GetFrameSize()
	m.list.SetSize(m.width-h, m.height-v)

	m.styles = ui.NewStyles(m.darkBG)
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
			newItem := ui.GenerateItem()
			insCmd := m.list.InsertItem(0, newItem)
			statusCmd := m.list.NewStatusMessage(m.styles.StatusMessage.Render("Added " + newItem.Title()))
			return m, tea.Batch(insCmd, statusCmd)
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
	m.styles = ui.NewStyles(false)

	delegateKeys := ui.NewDelegateKeyMap()
	keys := newKeyMap()
	cfg := internal.Config{
		CurrentUsername: "su1uv",
		DbURL:           os.Getenv("GOOSE_DBSTRING"),
	}
	state := internal.State{
		Cfg: &cfg,
	}

	const numItems = 5
	items := make([]list.Item, numItems)
	for i := range numItems {
		items[i] = ui.GenerateItem()
	}

	delegate := ui.NewItemDelegate(delegateKeys, &m.styles)
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
	m.state = &state

	return m
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Something went wrong: %v\n", err)
		os.Exit(1)
	}
}
