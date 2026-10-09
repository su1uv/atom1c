package ui

import (
	"context"
	"errors"

	tea "charm.land/bubbletea/v2"
	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/database"
)

type postErrorAction int

const (
	postNoRetry postErrorAction = iota
	postRetryRead
	postRetryRefresh
)

var errPostDatabaseUnavailable = errors.New("post database is not configured")

type postPageResult struct {
	request        uint64
	feedID         int64
	selectedPostID int64
	posts          []database.Post
	err            error
}

type postFilterResult struct {
	request        uint64
	selectedPostID int64
	result         tea.Msg
}

type feedRefreshResult struct {
	request uint64
	feed    database.Feed
	err     error
}

type feedRefreshNotification struct {
	notification internal.FeedRefreshNotification
	err          error
}

func (m model) feedRefreshSubscriptionCommand() tea.Cmd {
	if m.refreshSubscription == nil {
		return nil
	}
	subscription := m.refreshSubscription
	ctx := m.sessionContext()
	return func() tea.Msg {
		notification, err := subscription.Next(ctx)
		return feedRefreshNotification{notification: notification, err: err}
	}
}

func (m model) updateFeedRefreshNotification(msg feedRefreshNotification) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, nil
	}
	commands := []tea.Cmd{m.feedRefreshSubscriptionCommand()}
	reloadOpenFeed := msg.notification.Reconcile && m.openFeedID != 0
	for _, feedID := range msg.notification.FeedIDs {
		if feedID == m.openFeedID && m.openFeedID != 0 {
			reloadOpenFeed = true
			break
		}
	}
	if reloadOpenFeed {
		commands = append(commands, m.beginPostLoad(m.currentPostSelectionID()))
	}
	return m, tea.Batch(commands...)
}

func (m *model) openFeed(selected item) tea.Cmd {
	if selected.id != m.openFeedID {
		m.postRecords = nil
		m.posts.list.ResetFilter()
		_ = m.posts.list.SetItems(nil)
		m.posts.list.ResetSelected()
	}
	m.openFeedID = selected.id
	m.openFeedName = selected.name
	m.openFeedURL = selected.url
	m.postErr = ""
	m.postErrorAction = postNoRetry
	return m.beginPostLoad(m.currentPostSelectionID())
}

func (m *model) currentPostSelectionID() int64 {
	selected, ok := m.posts.list.SelectedItem().(item)
	if !ok {
		return 0
	}
	return selected.id
}

func (m *model) beginPostLoad(selectedPostID int64) tea.Cmd {
	if m.openFeedID == 0 {
		return nil
	}
	m.postRequest++
	request := m.postRequest
	feedID := m.openFeedID
	m.postLoading = true
	m.postErr = ""
	m.postErrorAction = postNoRetry
	store := m.feedStore
	sessionCtx := m.sessionContext()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(sessionCtx, feedOperationTimeout)
		defer cancel()
		if store == nil {
			return postPageResult{request: request, feedID: feedID, selectedPostID: selectedPostID, err: errPostDatabaseUnavailable}
		}
		posts, err := store.GetPosts(ctx, feedID)
		return postPageResult{
			request:        request,
			feedID:         feedID,
			selectedPostID: selectedPostID,
			posts:          posts,
			err:            err,
		}
	}
}

func (m model) updatePostPageResult(msg postPageResult) (tea.Model, tea.Cmd) {
	if msg.request != m.postRequest || msg.feedID != m.openFeedID {
		return m, nil
	}
	m.postLoading = false
	if msg.err != nil {
		m.postErr = "Could not load posts: " + msg.err.Error()
		m.postErrorAction = postRetryRead
		return m, nil
	}
	m.postRecords = make(map[int64]database.Post, len(msg.posts))
	for _, post := range msg.posts {
		m.postRecords[post.ID] = post
	}
	items := postItems(msg.posts)
	if filterCmd := m.posts.list.SetItems(items); filterCmd != nil {
		request := msg.request
		selectedPostID := msg.selectedPostID
		return m, func() tea.Msg {
			return postFilterResult{request: request, selectedPostID: selectedPostID, result: filterCmd()}
		}
	}
	m.selectPostByID(msg.selectedPostID)
	m.postErr = ""
	m.postErrorAction = postNoRetry
	return m, nil
}

func (m model) updatePostFilterResult(msg postFilterResult) (tea.Model, tea.Cmd) {
	if msg.request != m.postRequest {
		return m, nil
	}
	updated, cmd := m.posts.list.Update(msg.result)
	m.posts.list = updated
	m.selectPostByID(msg.selectedPostID)
	return m, tagPaneCommand(cmd, focusPosts)
}

func (m *model) selectPostByID(postID int64) {
	items := m.posts.list.VisibleItems()
	if len(items) == 0 {
		m.posts.list.ResetSelected()
		return
	}
	for i, listItem := range items {
		if listItem.(item).id == postID && postID != 0 {
			m.posts.list.Select(i)
			return
		}
	}
	m.posts.list.Select(0)
}

func (m *model) beginSelectedFeedRefresh() tea.Cmd {
	if m.focus == focusPosts {
		if m.openFeedID == 0 {
			return nil
		}
		return m.beginFeedRefresh(database.Feed{ID: m.openFeedID, Name: m.openFeedName, Url: m.openFeedURL})
	}
	selected, ok := m.feeds.list.SelectedItem().(item)
	if !ok || selected.id == 0 {
		return nil
	}
	return m.beginFeedRefresh(database.Feed{ID: selected.id, Name: selected.name, Url: selected.url})
}

func (m *model) beginFeedRefresh(storedFeed database.Feed) tea.Cmd {
	if storedFeed.ID == 0 || m.refreshing[storedFeed.ID] {
		return nil
	}
	m.refreshRequest++
	request := m.refreshRequest
	m.lastRefreshFeed = storedFeed
	m.refreshing[storedFeed.ID] = true
	m.refreshRequests[storedFeed.ID] = request
	delete(m.refreshErrors, storedFeed.ID)
	delete(m.refreshCompleted, storedFeed.ID)
	if m.openFeedID == storedFeed.ID {
		m.postErr = ""
		m.postErrorAction = postNoRetry
	}
	store := m.feedStore
	sessionCtx := m.sessionContext()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(sessionCtx, feedOperationTimeout)
		defer cancel()
		if store == nil {
			return feedRefreshResult{request: request, feed: storedFeed, err: errPostDatabaseUnavailable}
		}
		return feedRefreshResult{request: request, feed: storedFeed, err: store.Refresh(ctx, storedFeed)}
	}
}

func (m model) updateFeedRefreshResult(msg feedRefreshResult) (tea.Model, tea.Cmd) {
	if !m.refreshing[msg.feed.ID] || m.refreshRequests[msg.feed.ID] != msg.request {
		return m, nil
	}
	delete(m.refreshing, msg.feed.ID)
	if msg.err != nil {
		m.refreshErrors[msg.feed.ID] = msg.err.Error()
		if m.openFeedID == msg.feed.ID {
			m.postErr = "Refresh failed: " + msg.err.Error()
			m.postErrorAction = postRetryRefresh
			m.postFailedFeed = msg.feed
		}
		return m, nil
	}
	delete(m.refreshErrors, msg.feed.ID)
	m.refreshCompleted[msg.feed.ID] = true
	if m.openFeedID == msg.feed.ID {
		if m.refreshSubscription != nil {
			return m, nil
		}
		return m, m.beginPostLoad(m.currentPostSelectionID())
	}
	return m, nil
}
