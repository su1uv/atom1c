package ui

import (
	"fmt"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/database"
)

func NewProgram(s *internal.State) *tea.Program {
	return tea.NewProgram(newModel(s))
}

func newModel(s *internal.State) tea.Model {
	styles := newStyles(false)
	keys := newListKeyMap()
	var store feedStore
	if s != nil && s.Db != nil && s.SQLDB != nil {
		store = stateFeedStore{state: s}
	}

	return model{
		styles:             styles,
		keys:               keys,
		focus:              focusFeeds,
		feeds:              initialFeedsModel(styles),
		posts:              initialPostsModel(styles),
		help:               help.New(),
		addFeed:            initialAddFeedModel(styles.modal),
		feedStore:          store,
		feedSearch:         initialFeedSearch(),
		showFeedPagination: true,
		showPostPagination: true,
		refreshing:         make(map[int64]bool),
		refreshRequests:    make(map[int64]uint64),
		refreshErrors:      make(map[int64]string),
		refreshCompleted:   make(map[int64]bool),
	}
}

type focusState int

const (
	focusFeeds focusState = iota
	focusPosts
)

const helpHeight = 3

type item struct {
	id   int64
	name string
	url  string
}

func (f item) Title() string       { return f.name }
func (f item) Description() string { return f.url }
func (f item) FilterValue() string { return f.name }

// model owns application-level focus and routes each key to one owner.
type model struct {
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

	feedStore          feedStore
	feedSearch         textinput.Model
	feedSearching      bool
	feedQuery          string
	feedPage           int
	feedPageSize       int
	feedCursor         int
	feedTotal          int64
	feedLoading        bool
	feedErr            string
	feedRequest        uint64
	saveRequest        uint64
	pendingSavedFeedID int64
	showFeedPagination bool
	showPostPagination bool

	openFeedID       int64
	openFeedName     string
	openFeedURL      string
	postLoading      bool
	postErr          string
	postErrorAction  postErrorAction
	postFailedFeed   database.Feed
	postRequest      uint64
	refreshing       map[int64]bool
	refreshRequests  map[int64]uint64
	refreshErrors    map[int64]string
	refreshCompleted map[int64]bool
	refreshRequest   uint64
	lastRefreshFeed  database.Feed
}

func (m model) Init() tea.Cmd {
	if m.feedPageSize > 0 && m.feedStore != nil {
		return m.beginFeedPageLoad()
	}
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case feedPageResult:
		return m.updateFeedPageResult(msg)
	case postPageResult:
		return m.updatePostPageResult(msg)
	case postFilterResult:
		return m.updatePostFilterResult(msg)
	case feedRefreshResult:
		return m.updateFeedRefreshResult(msg)
	case savedFeedPageResult:
		return m.updateSavedFeedPageResult(msg)
	case feedSearchDebounceMsg:
		if msg.request == m.feedRequest {
			return m, m.feedPageCommand(msg.request)
		}
		return m, nil
	case feedSaveResult:
		return m.updateFeedSaveResult(msg)
	}

	if result, ok := msg.(paneMessage); ok {
		return m, m.pane(result.pane).Update(result.msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.SetWidth(max(msg.Width-m.styles.app.GetHorizontalFrameSize(), 1))
		oldPageSize := m.feedPageSize
		absoluteIndex := m.feedPage*max(oldPageSize, 1) + m.feedCursor
		paneWidth := max(msg.Width-m.styles.app.GetHorizontalFrameSize(), 0)
		m.feeds.setSize(paneWidth, max(msg.Height-2, 0))
		m.posts.setSize(paneWidth, max(msg.Height-2, 0))
		m.feedPageSize = max(m.feeds.list.Paginator.PerPage, 1)
		m.feedSearch.SetWidth(max(msg.Width/2-4, 1))
		if oldPageSize == 0 {
			return m, m.beginFeedPageLoad()
		}
		if oldPageSize != m.feedPageSize {
			m.feedPage = absoluteIndex / m.feedPageSize
			m.feedCursor = absoluteIndex % m.feedPageSize
			if m.pendingSavedFeedID != 0 {
				return m, m.beginSavedFeedReload()
			}
			return m, m.beginFeedPageLoad()
		}
		return m, nil

	case tea.KeyPressMsg:
		if m.modalOpen {
			return m.updateModal(msg)
		}
		if m.focus == focusFeeds && m.feedSearching {
			return m.updateFeedSearch(msg)
		}

		// A filter owns keyboard input until filter mode ends. Printable global
		// shortcuts therefore remain filter text.
		if m.activePane().list.FilterState() == list.Filtering {
			return m, m.updateActivePane(msg)
		}

		switch {
		case m.focus == focusFeeds && key.Matches(msg, m.keys.filter):
			m.feedSearching = true
			return m, m.feedSearch.Focus()
		case key.Matches(msg, m.keys.addFeed):
			m.modalOpen = true
			return m, m.addFeed.open()
		case key.Matches(msg, m.keys.selectItem) && m.focus == focusFeeds:
			selected, ok := m.feeds.list.SelectedItem().(item)
			if !ok || selected.id == 0 {
				return m, nil
			}
			m.focus = focusPosts
			return m, m.openFeed(selected)
		case key.Matches(msg, m.keys.deselectItem) && m.focus == focusPosts:
			m.focus = focusFeeds
			return m, nil
		case key.Matches(msg, m.keys.refresh):
			return m, m.beginSelectedFeedRefresh()
		case key.Matches(msg, m.keys.togglePagination):
			if m.focus == focusFeeds {
				m.showFeedPagination = !m.showFeedPagination
				return m, nil
			}
			m.showPostPagination = !m.showPostPagination
			return m, nil
		case m.focus == focusFeeds && key.Matches(msg, m.keys.nextPage):
			if m.canGoToFeedPage(m.feedPage + 1) {
				m.feedPage++
				m.feedCursor = 0
				m.pendingSavedFeedID = 0
				return m, m.beginFeedPageLoad()
			}
			return m, nil
		case m.focus == focusFeeds && key.Matches(msg, m.keys.prevPage):
			if m.feedPage > 0 {
				m.feedPage--
				m.feedCursor = 0
				m.pendingSavedFeedID = 0
				return m, m.beginFeedPageLoad()
			}
			return m, nil
		case m.focus == focusPosts && key.Matches(msg, m.keys.nextPage):
			if m.posts.list.Paginator.Page < m.posts.list.Paginator.TotalPages-1 {
				m.posts.list.NextPage()
				m.posts.list.Select(m.posts.list.Paginator.Page * m.posts.list.Paginator.PerPage)
			}
			return m, nil
		case m.focus == focusPosts && key.Matches(msg, m.keys.prevPage):
			if m.posts.list.Paginator.Page > 0 {
				m.posts.list.PrevPage()
				m.posts.list.Select(m.posts.list.Paginator.Page * m.posts.list.Paginator.PerPage)
			}
			return m, nil
		case key.Matches(msg, m.keys.retry):
			if m.focus == focusFeeds {
				if m.feedErr != "" {
					if m.pendingSavedFeedID != 0 {
						return m, m.beginSavedFeedReload()
					}
					return m, m.beginFeedPageLoad()
				}
				if selected, ok := m.feeds.list.SelectedItem().(item); ok && m.refreshErrors[selected.id] != "" {
					return m, m.beginFeedRefresh(database.Feed{ID: selected.id, Name: selected.name, Url: selected.url})
				}
				if m.lastRefreshFeed.ID != 0 && m.refreshErrors[m.lastRefreshFeed.ID] != "" {
					return m, m.beginFeedRefresh(m.lastRefreshFeed)
				}
			} else if m.postErr != "" {
				if m.postErrorAction == postRetryRefresh {
					return m, m.beginFeedRefresh(m.postFailedFeed)
				}
				if m.postErrorAction == postRetryRead {
					return m, m.beginPostLoad(m.currentPostSelectionID())
				}
			}
			return m, nil
		}

		cmd := m.updateActivePane(msg)
		if m.focus == focusFeeds {
			m.feedCursor = m.feeds.list.Index()
		}
		return m, cmd
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
	if action == addFeedClose {
		m.modalOpen = false
	}
	if action == addFeedSubmit {
		params := m.addFeed.submission()
		return m, m.beginFeedSave(params)
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
		m.keys.retry,
		m.keys.refresh,
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
	if m.width > 0 {
		help = lipgloss.NewStyle().MaxWidth(max(m.width-m.styles.app.GetHorizontalFrameSize(), 1)).MaxHeight(1).Render(help)
	}

	feedsContent := m.feedView()
	postsContent := m.postsView()
	content := m.styles.app.Render(lipgloss.JoinHorizontal(lipgloss.Top, feedsContent, postsContent) + "\n" + help)
	var cursor *tea.Cursor

	if m.modalOpen {
		modal := m.addFeed.View()
		content = overlayModal(content, modal.Content, m.width, m.height)
		if modal.Cursor != nil {
			cursor = modal.Cursor
			cursor.X += max((m.width-lipgloss.Width(modal.Content))/2, 0)
			cursor.Y += max((m.height-lipgloss.Height(modal.Content))/2, 0)
		}
	}

	v := tea.NewView(content)
	v.AltScreen = true
	if m.modalOpen {
		v.Cursor = cursor
	} else if m.feedSearching {
		if cursor := m.feedSearch.Cursor(); cursor != nil {
			cursor.X++ // Account for the application's left margin.
			v.Cursor = cursor
		}
	}
	return v
}

func initialFeedSearch() textinput.Model {
	input := textinput.New()
	input.Prompt = "/ "
	input.Placeholder = "search all feeds"
	input.CharLimit = 250
	input.SetVirtualCursor(false)
	input.Blur()
	return input
}

func (m model) canGoToFeedPage(page int) bool {
	if m.feedPageSize < 1 {
		return false
	}
	return page >= 0 && int64(page*m.feedPageSize) < m.feedTotal
}

func (m model) feedView() string {
	status := ""
	switch {
	case m.feedLoading:
		status = "Loading feeds…"
	case m.feedErr != "":
		status = "Error: " + m.feedErr + " (r to retry)"
	case m.feedTotal == 0 && m.feedQuery == "":
		status = "No feeds yet — press a to add one"
	case m.feedTotal == 0:
		status = "No feeds match " + fmt.Sprintf("%q", m.feedQuery)
	default:
		status = fmt.Sprintf("%d feeds", m.feedTotal)
		if m.showFeedPagination {
			pages := (int(m.feedTotal) + max(m.feedPageSize, 1) - 1) / max(m.feedPageSize, 1)
			status += fmt.Sprintf(" • page %d/%d", m.feedPage+1, max(pages, 1))
		}
	}
	if m.feedQuery != "" {
		status = "Search " + fmt.Sprintf("%q", m.feedQuery) + " • " + status
	}
	refreshStatus := ""
	if selected, ok := m.feeds.list.SelectedItem().(item); ok && m.refreshing[selected.id] {
		refreshStatus = "Refreshing " + selected.name + "…"
	} else if selected, ok := m.feeds.list.SelectedItem().(item); ok && m.refreshErrors[selected.id] != "" {
		refreshStatus = "Refresh failed: " + m.refreshErrors[selected.id] + " (r to retry)"
	} else if selected, ok := m.feeds.list.SelectedItem().(item); ok && m.refreshCompleted[selected.id] {
		refreshStatus = "Refreshed " + selected.name
	} else if m.lastRefreshFeed.ID != 0 && m.refreshing[m.lastRefreshFeed.ID] {
		refreshStatus = "Refreshing " + m.lastRefreshFeed.Name + "…"
	} else if m.lastRefreshFeed.ID != 0 && m.refreshErrors[m.lastRefreshFeed.ID] != "" {
		refreshStatus = "Refresh failed for " + m.lastRefreshFeed.Name + ": " + m.refreshErrors[m.lastRefreshFeed.ID] + " (r to retry)"
	} else if m.lastRefreshFeed.ID != 0 && m.refreshCompleted[m.lastRefreshFeed.ID] {
		refreshStatus = "Refreshed " + m.lastRefreshFeed.Name
	}
	if refreshStatus != "" {
		status += " • " + refreshStatus
	}
	parts := []string{m.feedSearch.View(), m.paneHeader(status), m.feeds.View().Content}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m model) postsView() string {
	feedHeader := "No feed open"
	if m.openFeedID != 0 {
		feedHeader = "Feed: " + m.openFeedName
	}
	status := "Select a feed and press tab"
	switch {
	case m.openFeedID != 0 && m.postLoading:
		status = "Loading posts for " + m.openFeedName + "…"
	case m.openFeedID != 0 && m.postErr != "":
		status = m.postErr + " (r to retry)"
	case m.openFeedID != 0 && len(m.posts.list.Items()) == 0:
		status = "No posts in " + m.openFeedName + " — press R to refresh"
	case m.openFeedID != 0:
		status = fmt.Sprintf("%d posts", len(m.posts.list.Items()))
	}
	if m.openFeedID != 0 && m.showPostPagination && !m.postLoading && m.postErr == "" {
		status += fmt.Sprintf(" • page %d/%d", m.posts.list.Paginator.Page+1, max(m.posts.list.Paginator.TotalPages, 1))
	}
	if m.openFeedID != 0 && m.refreshing[m.openFeedID] {
		status += " • refreshing feed…"
	} else if m.openFeedID != 0 && m.refreshCompleted[m.openFeedID] && !m.postLoading && m.postErr == "" {
		status += " • refreshed"
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.paneHeader(feedHeader), m.paneHeader(status), m.posts.View().Content)
}

func (m model) paneHeader(text string) string {
	return lipgloss.NewStyle().MaxWidth(max(m.width/2-4, 1)).MaxHeight(1).Render(text)
}
