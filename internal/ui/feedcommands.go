package ui

import (
	"context"
	"errors"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/su1uv/atom1c/internal/database"
	"github.com/su1uv/atom1c/internal/handlers"
)

const feedOperationTimeout = 15 * time.Second
const feedSearchDebounce = 250 * time.Millisecond

var errFeedDatabaseUnavailable = errors.New("feed database is not configured")

type feedPageResult struct {
	request uint64
	page    int
	result  handlers.FeedPage
	err     error
}

type feedSearchDebounceMsg struct {
	request uint64
}

type savedFeedPageResult struct {
	request uint64
	page    int
	cursor  int
	result  handlers.FeedPage
	err     error
}

type feedSaveResult struct {
	request  uint64
	pageSize int
	feed     database.Feed
	page     handlers.FeedPage
	pageIdx  int
	cursor   int
	saveErr  error
	loadErr  error
}

func (m *model) beginFeedPageLoad() tea.Cmd {
	m.feedRequest++
	m.feedLoading = true
	m.feedErr = ""
	return m.feedPageCommand(m.feedRequest)
}

func (m *model) beginSavedFeedReload() tea.Cmd {
	m.feedRequest++
	m.feedLoading = true
	m.feedErr = ""
	request := m.feedRequest
	store := m.feedStore
	sessionCtx := m.sessionContext()
	feedID := m.pendingSavedFeedID
	pageSize := max(m.feedPageSize, 1)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(sessionCtx, feedOperationTimeout)
		defer cancel()
		if store == nil {
			return savedFeedPageResult{request: request, err: errFeedDatabaseUnavailable}
		}
		position, err := store.Position(ctx, feedID, "")
		if err != nil {
			return savedFeedPageResult{request: request, err: err}
		}
		page := int(position / int64(pageSize))
		cursor := int(position % int64(pageSize))
		result, err := store.GetPage(ctx, handlers.FeedPageParams{
			Limit:  int64(pageSize),
			Offset: int64(page * pageSize),
		})
		return savedFeedPageResult{request: request, page: page, cursor: cursor, result: result, err: err}
	}
}

func (m model) feedPageCommand(request uint64) tea.Cmd {
	if m.feedStore == nil {
		return func() tea.Msg {
			return feedPageResult{request: request, page: m.feedPage, err: errFeedDatabaseUnavailable}
		}
	}
	store := m.feedStore
	page := m.feedPage
	pageSize := max(m.feedPageSize, 1)
	search := m.feedQuery
	sessionCtx := m.sessionContext()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(sessionCtx, feedOperationTimeout)
		defer cancel()
		result, err := store.GetPage(ctx, handlers.FeedPageParams{
			Search: search,
			Limit:  int64(pageSize),
			Offset: int64(page * pageSize),
		})
		return feedPageResult{request: request, page: page, result: result, err: err}
	}
}

func (m model) debounceFeedSearch(request uint64) tea.Cmd {
	return tea.Tick(feedSearchDebounce, func(time.Time) tea.Msg {
		return feedSearchDebounceMsg{request: request}
	})
}

func (m *model) beginFeedSave(params handlers.AddFeedParams) tea.Cmd {
	m.saveRequest++
	request := m.saveRequest
	m.feedRequest++ // Invalidate any page load started before the insert.
	m.feedLoading = false
	m.addFeed.saving = true
	m.addFeed.err = ""
	pageSize := max(m.feedPageSize, 1)
	store := m.feedStore
	sessionCtx := m.sessionContext()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(sessionCtx, feedOperationTimeout)
		defer cancel()
		if store == nil {
			return feedSaveResult{request: request, pageSize: pageSize, saveErr: errFeedDatabaseUnavailable}
		}

		feed, err := store.Add(ctx, params)
		if err != nil {
			return feedSaveResult{request: request, pageSize: pageSize, saveErr: err}
		}
		position, err := store.Position(ctx, feed.ID, "")
		if err != nil {
			return feedSaveResult{request: request, pageSize: pageSize, feed: feed, loadErr: err}
		}
		pageIdx := int(position / int64(pageSize))
		cursor := int(position % int64(pageSize))
		page, err := store.GetPage(ctx, handlers.FeedPageParams{
			Limit:  int64(pageSize),
			Offset: int64(pageIdx * pageSize),
		})
		return feedSaveResult{
			request:  request,
			pageSize: pageSize,
			feed:     feed,
			page:     page,
			pageIdx:  pageIdx,
			cursor:   cursor,
			loadErr:  err,
		}
	}
}

func (m model) updateFeedPageResult(msg feedPageResult) (tea.Model, tea.Cmd) {
	if msg.request != m.feedRequest {
		return m, nil
	}
	m.feedLoading = false
	if msg.err != nil {
		m.feedErr = msg.err.Error()
		return m, nil
	}
	if m.feedPageSize > 0 && msg.result.Total > 0 {
		lastPage := int((msg.result.Total - 1) / int64(m.feedPageSize))
		if msg.page > lastPage {
			m.feedPage = lastPage
			m.feedCursor = 0
			return m, m.beginFeedPageLoad()
		}
	} else if msg.result.Total == 0 {
		m.feedPage = 0
		m.feedCursor = 0
	}
	m.feedPage = msg.page
	m.feedTotal = msg.result.Total
	m.applyFeedItems(msg.result.Feeds, m.feedCursor)
	m.feedErr = ""
	return m, nil
}

func (m model) updateSavedFeedPageResult(msg savedFeedPageResult) (tea.Model, tea.Cmd) {
	if msg.request != m.feedRequest {
		return m, nil
	}
	m.feedLoading = false
	if msg.err != nil {
		m.feedErr = "saved feed, but reload failed: " + msg.err.Error()
		return m, nil
	}
	m.feedPage = msg.page
	m.feedCursor = msg.cursor
	m.feedTotal = msg.result.Total
	m.applyFeedItems(msg.result.Feeds, msg.cursor)
	m.feedErr = ""
	m.pendingSavedFeedID = 0
	return m, nil
}

func (m model) updateFeedSaveResult(msg feedSaveResult) (tea.Model, tea.Cmd) {
	if msg.request != m.saveRequest {
		return m, nil
	}
	if msg.saveErr != nil {
		m.addFeed.saving = false
		if errors.Is(msg.saveErr, handlers.ErrFeedURLExists) {
			m.addFeed.err = "A feed with this URL already exists."
		} else {
			m.addFeed.err = "Could not save feed: " + msg.saveErr.Error()
		}
		m.addFeed.focusIndex = 1
		return m, m.addFeed.focusInput()
	}

	m.modalOpen = false
	m.addFeed.reset()
	m.focus = focusFeeds
	m.feedSearching = false
	m.feedSearch.Blur()
	m.feedSearch.SetValue("")
	m.feedQuery = ""
	m.feedPage = msg.pageIdx
	m.feedCursor = msg.cursor
	m.pendingSavedFeedID = msg.feed.ID
	if msg.pageSize != max(m.feedPageSize, 1) {
		m.feedLoading = false
		return m, m.beginSavedFeedReload()
	}
	m.feedTotal = msg.page.Total
	if msg.loadErr != nil {
		m.feedLoading = false
		m.feedErr = "saved feed, but reload failed: " + msg.loadErr.Error()
		return m, nil
	}
	m.feedLoading = false
	m.feedErr = ""
	m.pendingSavedFeedID = 0
	m.applyFeedItems(msg.page.Feeds, msg.cursor)
	return m, nil
}

func (m *model) applyFeedItems(feeds []database.Feed, cursor int) {
	items := feedItems(feeds)
	_ = m.feeds.list.SetItems(items)
	if len(items) == 0 {
		m.feedCursor = 0
		m.feeds.list.ResetSelected()
		return
	}
	cursor = min(max(cursor, 0), len(items)-1)
	m.feedCursor = cursor
	m.feeds.list.Select(cursor)
}

func (m model) updateFeedSearch(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.feedSearching = false
		m.feedSearch.Blur()
		return m, nil
	case "enter":
		m.feedSearching = false
		m.feedSearch.Blur()
		return m, m.beginFeedPageLoad()
	}

	oldValue := m.feedSearch.Value()
	input, inputCmd := m.feedSearch.Update(msg)
	m.feedSearch = input
	if input.Value() == oldValue {
		return m, inputCmd
	}
	m.feedQuery = input.Value()
	m.feedPage = 0
	m.feedCursor = 0
	m.pendingSavedFeedID = 0
	m.feedLoading = true
	m.feedErr = ""
	m.feedRequest++
	request := m.feedRequest
	return m, tea.Batch(inputCmd, m.debounceFeedSearch(request))
}
