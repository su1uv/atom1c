package ui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/database"
	feedpkg "github.com/su1uv/atom1c/internal/feed"
	"github.com/su1uv/atom1c/internal/handlers"
)

func postStoreWithFeeds(feeds ...database.Feed) *memoryFeedStore {
	return &memoryFeedStore{feeds: feeds, posts: make(map[int64][]database.Post)}
}

func TestOpeningFeedLoadsPostsAndFeedCursorDoesNotChangeOpenPane(t *testing.T) {
	store := postStoreWithFeeds(
		database.Feed{ID: 1, Name: "One", Url: "https://example.test/one"},
		database.Feed{ID: 2, Name: "Two", Url: "https://example.test/two"},
	)
	store.posts[1] = []database.Post{
		{ID: 12, FeedID: 1, Title: "Newest", Link: "https://example.test/newest"},
		{ID: 10, FeedID: 1, Title: "Older", Link: "https://example.test/older"},
	}
	m := loadedFeedModel(t, store)

	m, cmd := applyMessage(m, press("tab", tea.KeyTab))
	if m.openFeedID != 1 || !m.postLoading {
		t.Fatalf("opened feed state = id %d, loading %v; want feed 1 loading", m.openFeedID, m.postLoading)
	}
	if len(store.postCalls) != 0 {
		t.Fatal("post read ran synchronously instead of through a command")
	}
	m, _ = applyMessage(m, runCommand(t, cmd))
	if got := len(m.posts.list.Items()); got != 2 {
		t.Fatalf("loaded %d posts, want 2", got)
	}
	if got := m.posts.list.Items()[0].(item).name; got != "Newest" {
		t.Fatalf("first post = %q, want newest database row", got)
	}

	m, _ = applyMessage(m, shiftTab())
	m = updateModel(m, press("down", tea.KeyDown))
	if got := m.posts.list.Items()[0].(item).name; got != "Newest" {
		t.Fatalf("moving feed cursor changed opened posts: got %q", got)
	}
	if m.openFeedID != 1 {
		t.Fatalf("open feed changed to %d while moving cursor", m.openFeedID)
	}
}

func TestOpeningAnotherFeedRejectsLatePostRead(t *testing.T) {
	store := postStoreWithFeeds(
		database.Feed{ID: 1, Name: "One", Url: "https://example.test/one"},
		database.Feed{ID: 2, Name: "Two", Url: "https://example.test/two"},
	)
	store.posts[1] = []database.Post{{ID: 11, FeedID: 1, Title: "One post"}}
	store.posts[2] = []database.Post{{ID: 22, FeedID: 2, Title: "Two post"}}
	m := loadedFeedModel(t, store)

	m, oldCmd := applyMessage(m, press("tab", tea.KeyTab))
	oldResult := runCommand(t, oldCmd)
	m, _ = applyMessage(m, shiftTab())
	m = updateModel(m, press("down", tea.KeyDown))
	m, newCmd := applyMessage(m, press("tab", tea.KeyTab))
	if m.openFeedID != 2 {
		t.Fatalf("opened feed = %d, want 2", m.openFeedID)
	}
	m, _ = applyMessage(m, runCommand(t, newCmd))
	m, _ = applyMessage(m, oldResult)
	if got := m.posts.list.Items()[0].(item).name; got != "Two post" {
		t.Fatalf("late response replaced current posts with %q", got)
	}
}

func TestRefreshUsesFeedCapturedFromFocusedPane(t *testing.T) {
	store := postStoreWithFeeds(
		database.Feed{ID: 1, Name: "One", Url: "https://example.test/one"},
		database.Feed{ID: 2, Name: "Two", Url: "https://example.test/two"},
	)
	m := loadedFeedModel(t, store)

	m, cmd := applyMessage(m, press("R", 'R'))
	if len(store.refreshCalls) != 0 {
		t.Fatal("refresh ran synchronously")
	}
	if len(m.refreshing) != 1 || !m.refreshing[1] {
		t.Fatalf("refreshing state = %#v, want feed 1 pending", m.refreshing)
	}
	result := runCommand(t, cmd)
	if got := result.(feedRefreshResult).feed.ID; got != 1 {
		t.Fatalf("captured feed ID = %d, want 1", got)
	}
	m = updateModel(m, press("down", tea.KeyDown))
	m, _ = applyMessage(m, result)
	if len(store.refreshCalls) != 1 || store.refreshCalls[0].ID != 1 {
		t.Fatalf("refresh targets = %#v, want captured feed 1", store.refreshCalls)
	}

	m, _ = applyMessage(m, press("tab", tea.KeyTab))
	if m.openFeedID != 2 {
		t.Fatalf("opening selected feed = %d, want 2", m.openFeedID)
	}
	m, postRefreshCmd := applyMessage(m, press("R", 'R'))
	if postRefreshCmd == nil {
		t.Fatal("posts-pane refresh did not return an async command")
	}
	if !m.refreshing[2] {
		t.Fatalf("posts-pane refresh state = %#v, want feed 2 pending", m.refreshing)
	}
	m, duplicateCmd := applyMessage(m, press("R", 'R'))
	if duplicateCmd != nil {
		t.Fatal("duplicate refresh for the same feed was scheduled")
	}
	postResult := runCommand(t, postRefreshCmd)
	if got := postResult.(feedRefreshResult).feed.ID; got != 2 {
		t.Fatalf("posts-pane refresh target = %d, want feed 2", got)
	}
	m, _ = applyMessage(m, postResult)
	if got := len(store.refreshCalls); got != 2 || store.refreshCalls[1].ID != 2 {
		t.Fatalf("refresh calls = %#v, want feed 1 then feed 2", store.refreshCalls)
	}
}

func TestFeedPaneRefreshStatusAndRetryAreFeedScoped(t *testing.T) {
	store := postStoreWithFeeds(database.Feed{ID: 1, Name: "One", Url: "https://example.test/one"})
	store.refreshErr = errors.New("network failed")
	m := loadedFeedModel(t, store)
	m, cmd := applyMessage(m, press("R", 'R'))
	m, _ = applyMessage(m, runCommand(t, cmd))
	if !strings.Contains(m.feedView(), "Refresh failed") {
		t.Fatalf("feed pane did not show refresh failure: %q", m.feedView())
	}
	store.refreshErr = nil
	m, retryCmd := applyMessage(m, press("r", 'r'))
	if retryCmd == nil {
		t.Fatal("feed-pane retry did not schedule the failed refresh")
	}
	m, _ = applyMessage(m, runCommand(t, retryCmd))
	if got := len(store.refreshCalls); got != 2 {
		t.Fatalf("refresh calls after retry = %d, want 2", got)
	}
	if !strings.Contains(m.feedView(), "Refreshed One") {
		t.Fatalf("feed pane did not show refresh completion: %q", m.feedView())
	}
}

func TestPostLoadAndRefreshFailuresHaveOperationSpecificRetry(t *testing.T) {
	store := postStoreWithFeeds(database.Feed{ID: 1, Name: "One", Url: "https://example.test/one"})
	store.postsErr = errors.New("read failed")
	store.posts[1] = []database.Post{{ID: 10, FeedID: 1, Title: "Cached post"}}
	m := loadedFeedModel(t, store)
	m, cmd := applyMessage(m, press("tab", tea.KeyTab))
	m, _ = applyMessage(m, runCommand(t, cmd))
	if m.postErrorAction != postRetryRead || m.postErr == "" {
		t.Fatalf("post read error state = (%v, %q)", m.postErrorAction, m.postErr)
	}
	store.postsErr = nil
	m, retryRead := applyMessage(m, press("r", 'r'))
	m, _ = applyMessage(m, runCommand(t, retryRead))
	if len(store.postCalls) != 2 {
		t.Fatalf("post read calls after retry = %d, want 2", len(store.postCalls))
	}

	store.refreshErr = errors.New("network failed")
	m, refresh := applyMessage(m, press("R", 'R'))
	m, _ = applyMessage(m, runCommand(t, refresh))
	if m.postErrorAction != postRetryRefresh || m.postErr == "" {
		t.Fatalf("refresh error state = (%v, %q)", m.postErrorAction, m.postErr)
	}
	if got := m.posts.list.Items()[0].(item).name; got != "Cached post" {
		t.Fatalf("refresh failure replaced cached post with %q", got)
	}
	store.refreshErr = nil
	m, retryRefresh := applyMessage(m, press("r", 'r'))
	if retryRefresh == nil {
		t.Fatal("r did not schedule the failed refresh")
	}
	if got := retryRefresh().(feedRefreshResult).err; got != nil {
		t.Fatalf("retried refresh error = %v", got)
	}
	if got := len(store.refreshCalls); got != 2 {
		t.Fatalf("refresh calls after retry = %d, want the failed call and one retry", got)
	}
}

func TestSuccessfulRefreshReloadsPostsAndPreservesSelection(t *testing.T) {
	store := postStoreWithFeeds(database.Feed{ID: 1, Name: "One", Url: "https://example.test/one"})
	store.posts[1] = []database.Post{
		{ID: 12, FeedID: 1, Title: "Newest", Link: "https://example.test/newest"},
		{ID: 10, FeedID: 1, Title: "Keep selected", Link: "https://example.test/selected"},
	}
	m := loadedFeedModel(t, store)
	m, loadCmd := applyMessage(m, press("tab", tea.KeyTab))
	m, _ = applyMessage(m, runCommand(t, loadCmd))
	m.posts.list.SetFilterText("keep")
	if m.posts.list.FilterInput.Value() != "keep" {
		t.Fatalf("test filter = %q, want keep", m.posts.list.FilterInput.Value())
	}
	m.posts.list.Select(0)
	store.refreshHook = func(database.Feed) {
		store.posts[1] = []database.Post{
			{ID: 15, FeedID: 1, Title: "Inserted", Link: "https://example.test/inserted"},
			{ID: 12, FeedID: 1, Title: "Updated newest", Link: "https://example.test/newest"},
			{ID: 10, FeedID: 1, Title: "Keep selected", Link: "https://example.test/selected"},
		}
	}
	m, refreshCmd := applyMessage(m, press("R", 'R'))
	m = runUICommands(t, m, refreshCmd)
	if got := len(m.posts.list.Items()); got != 3 {
		t.Fatalf("reloaded %d posts, want 3", got)
	}
	selected, ok := m.posts.list.SelectedItem().(item)
	if !ok || selected.id != 10 {
		t.Fatalf("selected post after refresh = %#v, want post ID 10", m.posts.list.SelectedItem())
	}
	if got := m.posts.list.Items()[1].(item).name; got != "Updated newest" {
		t.Fatalf("updated post title = %q", got)
	}
	if m.posts.list.FilterInput.Value() != "keep" {
		t.Fatalf("post filter after refresh = %q, want preserved query keep", m.posts.list.FilterInput.Value())
	}
}

func TestSuccessfulSharedRefreshNotificationReloadsOpenFeedAndPreservesReaderState(t *testing.T) {
	store := postStoreWithFeeds(database.Feed{ID: 1, Name: "One", Url: "https://example.test/one"})
	store.posts[1] = []database.Post{
		{ID: 12, FeedID: 1, Title: "Newest", Link: "https://example.test/newest"},
		{ID: 10, FeedID: 1, Title: "Keep selected", Link: "https://example.test/selected"},
	}
	m := loadedFeedModel(t, store)
	m, loadCmd := applyMessage(m, press("tab", tea.KeyTab))
	m, _ = applyMessage(m, runCommand(t, loadCmd))
	m.posts.list.SetFilterText("keep")
	m.posts.list.Select(0)
	m.refreshSubscription = testRefreshSubscription{}
	m.readerOpen = true
	m.reader.article = articleSnapshot{source: "open snapshot"}
	store.posts[1] = []database.Post{
		{ID: 15, FeedID: 1, Title: "Inserted", Link: "https://example.test/inserted"},
		{ID: 12, FeedID: 1, Title: "Updated newest", Link: "https://example.test/newest"},
		{ID: 10, FeedID: 1, Title: "Keep selected", Link: "https://example.test/selected"},
	}

	priorRequest := m.postRequest
	m, cmd := applyMessage(m, feedRefreshNotification{feedIDs: []int64{1}})
	if m.postRequest != priorRequest+1 || !m.postLoading {
		t.Fatalf("notification post state = request %d, loading %v; want request %d and loading", m.postRequest, m.postLoading, priorRequest+1)
	}
	if cmd == nil {
		t.Fatal("matching refresh notification did not schedule the watcher/reload")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("notification command = %#v, want watcher and post reload", batch)
	}
	page, ok := batch[1]().(postPageResult)
	if !ok {
		t.Fatalf("post reload result = %#v, want postPageResult", page)
	}
	m, postFilterCmd := applyMessage(m, page)
	if postFilterCmd != nil {
		m, _ = applyMessage(m, runCommand(t, postFilterCmd))
	}
	selected, ok := m.posts.list.SelectedItem().(item)
	if !ok || selected.id != 10 {
		t.Fatalf("selected post = %#v, want post ID 10", m.posts.list.SelectedItem())
	}
	if got := m.posts.list.FilterInput.Value(); got != "keep" {
		t.Fatalf("filter after notification reload = %q, want keep", got)
	}
	if got := m.postRecords[12].Title; got != "Updated newest" {
		t.Fatalf("updated persisted post title = %q, want Updated newest", got)
	}
	if got := m.reader.article.source; got != "open snapshot" {
		t.Fatalf("open article snapshot source changed to %q", got)
	}
}

func TestUnrelatedSharedRefreshNotificationDoesNotReloadOpenFeed(t *testing.T) {
	store := postStoreWithFeeds(database.Feed{ID: 1, Name: "Open", Url: "https://example.test/open"})
	m := modelWithFeedStore(store)
	m.refreshSubscription = testRefreshSubscription{}
	priorRequest := m.postRequest
	m, cmd := applyMessage(m, feedRefreshNotification{feedIDs: []int64{2, 3}})
	if m.postRequest != priorRequest || m.postLoading {
		t.Fatalf("unrelated notification changed post load state: request %d loading %v", m.postRequest, m.postLoading)
	}
	if cmd == nil {
		t.Fatal("unrelated notification did not re-arm the subscription watcher")
	}
}

func TestManualRefreshReloadsBothConnectedModelsThroughSharedCoordinator(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = fmt.Fprint(w, `<feed xmlns="http://www.w3.org/2005/Atom"><entry><id>shared-entry</id><title>Shared update</title></entry></feed>`)
	}))
	defer server.Close()
	dbPath := filepath.Join(t.TempDir(), "shared-refresh.db")
	db := openUIWorkflowDB(t, dbPath)
	defer db.Close()
	queries := database.New(db)
	storedFeed, err := queries.CreateFeed(context.Background(), database.CreateFeedParams{Name: "Shared", Url: server.URL})
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}
	manager, err := feedpkg.NewRefreshCoordinator(context.Background(), db)
	if err != nil {
		t.Fatalf("create refresh coordinator: %v", err)
	}
	defer manager.Close()
	state := &internal.State{Db: queries, SQLDB: db, FeedRefresh: manager}
	first, cancelFirst := newSharedReaderModel(t, state, storedFeed)
	defer cancelFirst()
	second, cancelSecond := newSharedReaderModel(t, state, storedFeed)
	defer cancelSecond()

	first, refreshCmd := applyMessage(first, press("R", 'R'))
	if refreshCmd == nil {
		t.Fatal("manual refresh did not start")
	}
	refreshResult := runCommand(t, refreshCmd)
	first, _ = applyMessage(first, refreshResult)
	if got := requests.Load(); got != 1 {
		t.Fatalf("feed requests = %d, want one successful manual fetch", got)
	}
	first = applySharedRefreshNotification(t, first)
	second = applySharedRefreshNotification(t, second)

	for session, model := range map[string]model{"first": first, "second": second} {
		if got := model.postRecords[1].Title; got != "Shared update" {
			t.Errorf("%s session post title = %q, want Shared update", session, got)
		}
	}
}

func TestScheduledRefreshReloadsBothConnectedModelsThroughSharedCoordinator(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = fmt.Fprint(w, `<feed xmlns="http://www.w3.org/2005/Atom"><entry><id>scheduled-entry</id><title>Scheduled update</title></entry></feed>`)
	}))
	defer server.Close()
	dbPath := filepath.Join(t.TempDir(), "scheduled-refresh.db")
	db := openUIWorkflowDB(t, dbPath)
	defer db.Close()
	queries := database.New(db)
	storedFeed, err := queries.CreateFeed(context.Background(), database.CreateFeedParams{Name: "Shared", Url: server.URL})
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}
	manager, err := feedpkg.NewRefreshCoordinator(context.Background(), db)
	if err != nil {
		t.Fatalf("create refresh coordinator: %v", err)
	}
	schedulerCtx, cancelScheduler := context.WithCancel(context.Background())
	schedulerDone := make(chan error, 1)
	schedulerFinished := false
	defer func() {
		cancelScheduler()
		if !schedulerFinished {
			<-schedulerDone
		}
		manager.Close()
	}()
	state := &internal.State{Db: queries, SQLDB: db, FeedRefresh: manager}
	firstCtx, cancelFirst := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelFirst()
	secondCtx, cancelSecond := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelSecond()
	first := newSharedReaderModelForContext(t, state, storedFeed, firstCtx)
	second := newSharedReaderModelForContext(t, state, storedFeed, secondCtx)
	go func() {
		schedulerDone <- feedpkg.RunRefreshScheduler(schedulerCtx, queries, manager, time.Hour, nil)
	}()

	firstNotification := runCommand(t, first.feedRefreshSubscriptionCommand()).(feedRefreshNotification)
	secondNotification := runCommand(t, second.feedRefreshSubscriptionCommand()).(feedRefreshNotification)
	first = applyRefreshNotificationResult(t, first, firstNotification)
	second = applyRefreshNotificationResult(t, second, secondNotification)
	if got := requests.Load(); got != 1 {
		t.Fatalf("feed requests = %d, want one scheduled fetch", got)
	}
	for session, model := range map[string]model{"first": first, "second": second} {
		if got := model.postRecords[1].Title; got != "Scheduled update" {
			t.Errorf("%s session post title = %q, want Scheduled update", session, got)
		}
	}
	cancelScheduler()
	schedulerErr := <-schedulerDone
	schedulerFinished = true
	if schedulerErr != nil {
		t.Fatalf("stop refresh scheduler: %v", schedulerErr)
	}
}

func newSharedReaderModel(t *testing.T, state *internal.State, storedFeed database.Feed) (model, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	return newSharedReaderModelForContext(t, state, storedFeed, ctx), cancel
}

func newSharedReaderModelForContext(t *testing.T, state *internal.State, storedFeed database.Feed, ctx context.Context) model {
	t.Helper()
	m := newModelWithContext(state, ctx).(model)
	m.focus = focusPosts
	loadCmd := m.openFeed(item{id: storedFeed.ID, name: storedFeed.Name, url: storedFeed.Url})
	m, _ = applyMessage(m, runCommand(t, loadCmd))
	return m
}

func applySharedRefreshNotification(t *testing.T, m model) model {
	t.Helper()
	watchCmd := m.feedRefreshSubscriptionCommand()
	if watchCmd == nil {
		t.Fatal("model has no refresh notification subscription")
	}
	notification, ok := runCommand(t, watchCmd).(feedRefreshNotification)
	if !ok || notification.err != nil {
		t.Fatalf("refresh notification = %#v, want successful notification", notification)
	}
	return applyRefreshNotificationResult(t, m, notification)
}

func applyRefreshNotificationResult(t *testing.T, m model, notification feedRefreshNotification) model {
	t.Helper()
	if notification.err != nil {
		t.Fatalf("refresh notification error = %v", notification.err)
	}
	m, reload := applyMessage(m, notification)
	if reload == nil {
		t.Fatal("matching refresh notification did not schedule a reload")
	}
	batch, ok := reload().(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("notification commands = %#v, want watcher and reload", batch)
	}
	page, ok := batch[1]().(postPageResult)
	if !ok {
		t.Fatalf("notification reload result = %#v, want postPageResult", page)
	}
	m, filterCmd := applyMessage(m, page)
	if filterCmd != nil {
		m, _ = applyMessage(m, runCommand(t, filterCmd))
	}
	return m
}

type testRefreshSubscription struct{}

func (testRefreshSubscription) Next(ctx context.Context) ([]int64, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (testRefreshSubscription) Close() {}

func TestRetryAfterRefreshReloadFailureDoesNotRepeatRefresh(t *testing.T) {
	store := postStoreWithFeeds(database.Feed{ID: 1, Name: "One", Url: "https://example.test/one"})
	store.posts[1] = []database.Post{{ID: 10, FeedID: 1, Title: "Cached"}}
	m := loadedFeedModel(t, store)
	m, loadCmd := applyMessage(m, press("tab", tea.KeyTab))
	m, _ = applyMessage(m, runCommand(t, loadCmd))
	m, refreshCmd := applyMessage(m, press("R", 'R'))
	refreshResult := runCommand(t, refreshCmd)
	m, postsCmd := applyMessage(m, refreshResult)
	store.postsErr = errors.New("temporary read failure")
	m, _ = applyMessage(m, runCommand(t, postsCmd))
	if m.postErrorAction != postRetryRead {
		t.Fatalf("follow-up read failure action = %v, want retry read", m.postErrorAction)
	}
	if got := len(m.posts.list.Items()); got != 1 {
		t.Fatalf("cached posts after read failure = %d, want 1", got)
	}
	store.postsErr = nil
	m, retryCmd := applyMessage(m, press("r", 'r'))
	m, _ = applyMessage(m, runCommand(t, retryCmd))
	if got := len(store.refreshCalls); got != 1 {
		t.Fatalf("refresh calls after post-read retry = %d, want 1", got)
	}
	if got := len(store.postCalls); got != 3 {
		t.Fatalf("post reads after retry = %d, want initial, refresh reload, and retry", got)
	}
}

func TestRefreshAndReadWorkflowPersistsAtomAndRSSPosts(t *testing.T) {
	for _, tc := range []struct {
		name    string
		body    func(firstTitle string, includeThird bool) string
		updated string
	}{
		{
			name: "Atom",
			body: func(firstTitle string, includeThird bool) string {
				entries := `<entry><id>one</id><title>` + firstTitle + `</title><link href="https://example.test/one"/><content>first body</content></entry>` +
					`<entry><id>two</id><title>Second</title><link href="https://example.test/two"/><content>second body</content></entry>`
				if includeThird {
					entries += `<entry><id>three</id><title>Third</title><link href="https://example.test/three"/><content>third body</content></entry>`
				}
				return `<feed xmlns="http://www.w3.org/2005/Atom"><title>Atom feed</title>` + entries + `</feed>`
			},
			updated: "Atom updated",
		},
		{
			name: "RSS 2.0",
			body: func(firstTitle string, includeThird bool) string {
				entries := `<item><guid>one</guid><title>` + firstTitle + `</title><link>https://example.test/one</link><description>first body</description></item>` +
					`<item><guid>two</guid><title>Second</title><link>https://example.test/two</link><description>second body</description></item>`
				if includeThird {
					entries += `<item><guid>three</guid><title>Third</title><link>https://example.test/three</link><description>third body</description></item>`
				}
				return `<rss version="2.0"><channel><title>RSS feed</title><link>https://example.test/</link>` + entries + `</channel></rss>`
			},
			updated: "RSS updated",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.RWMutex
			body := tc.body("Original", false)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				mu.RLock()
				defer mu.RUnlock()
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()

			dbPath := filepath.Join(t.TempDir(), "workflow.db")
			db := openUIWorkflowDB(t, dbPath)
			state := &internal.State{Db: database.New(db), SQLDB: db}
			storedFeed, err := handlers.HandleAddFeed(t.Context(), state, handlers.AddFeedParams{Name: tc.name, URL: server.URL})
			if err != nil {
				t.Fatalf("add feed: %v", err)
			}

			m := newModel(state).(model)
			m.articleCache, m.articleFetcher = nil, nil
			m, cmd := applyMessage(m, tea.WindowSizeMsg{Width: 100, Height: 20})
			m, _ = applyMessage(m, runCommand(t, cmd))
			if selected, ok := m.feeds.list.SelectedItem().(item); !ok || selected.id != storedFeed.ID {
				t.Fatalf("selected feed = %#v, want ID %d", m.feeds.list.SelectedItem(), storedFeed.ID)
			}

			m, cmd = applyMessage(m, press("tab", tea.KeyTab))
			m, _ = applyMessage(m, runCommand(t, cmd))
			if len(m.posts.list.Items()) != 0 || !strings.Contains(m.postsView(), "No posts") {
				t.Fatalf("initial empty post state = %q", m.postsView())
			}

			m = refreshAndReload(t, m)
			if got := len(m.posts.list.Items()); got != 2 {
				t.Fatalf("initial refreshed posts = %d, want 2", got)
			}
			if got := m.posts.list.Items()[0].(item).name; got != "Second" {
				t.Fatalf("initial newest post = %q, want Second", got)
			}
			m.posts.list.Select(1)
			m = openReader(t, m)
			plainView := ansi.Strip(m.View().Content)
			if !strings.Contains(plainView, "Original") || !strings.Contains(plainView, "first body") || !strings.Contains(plainView, "Source: "+tc.name) {
				t.Fatalf("persisted article view = %q", m.View().Content)
			}
			m = updateModel(m, press("esc", tea.KeyEscape))

			mu.Lock()
			body = tc.body(tc.updated, true)
			mu.Unlock()
			m, refreshCmd := applyMessage(m, press("R", 'R'))
			m = openReader(t, m)
			m = runUICommands(t, m, refreshCmd)
			if !strings.Contains(m.reader.viewport.GetContent(), "Original") {
				t.Fatal("refresh disturbed persisted article snapshot")
			}
			m = updateModel(m, press("esc", tea.KeyEscape))
			m = openReader(t, m)
			if !strings.Contains(ansi.Strip(m.reader.viewport.GetContent()), tc.updated) {
				t.Fatal("reopening article did not show persisted update")
			}
			m = updateModel(m, press("esc", tea.KeyEscape))
			if got := len(m.posts.list.Items()); got != 3 {
				t.Fatalf("updated post count = %d, want 3 without duplicates", got)
			}
			if got := m.posts.list.Items()[0].(item).name; got != "Third" {
				t.Fatalf("updated newest post = %q, want Third", got)
			}
			foundUpdated := false
			for _, listItem := range m.posts.list.Items() {
				if listItem.(item).name == tc.updated {
					foundUpdated = true
				}
			}
			if !foundUpdated {
				t.Fatalf("updated title %q not loaded after refresh", tc.updated)
			}

			if err := db.Close(); err != nil {
				t.Fatalf("close database: %v", err)
			}
			reopened := openUIWorkflowDB(t, dbPath)
			defer reopened.Close()
			restarted := newModel(&internal.State{Db: database.New(reopened), SQLDB: reopened}).(model)
			restarted.articleCache, restarted.articleFetcher = nil, nil
			restarted, cmd = applyMessage(restarted, tea.WindowSizeMsg{Width: 100, Height: 20})
			restarted, _ = applyMessage(restarted, runCommand(t, cmd))
			restarted, cmd = applyMessage(restarted, press("tab", tea.KeyTab))
			restarted, _ = applyMessage(restarted, runCommand(t, cmd))
			if len(restarted.posts.list.Items()) != 3 || restarted.posts.list.Items()[0].(item).name != "Third" {
				t.Fatalf("posts after reopening database = %#v", restarted.posts.list.Items())
			}
			restarted.posts.list.Select(2)
			restarted = openReader(t, restarted)
			plainView = ansi.Strip(restarted.View().Content)
			if !strings.Contains(plainView, tc.updated) || !strings.Contains(plainView, "first body") {
				t.Fatalf("article after database reopen = %q", restarted.View().Content)
			}
		})
	}
}

func refreshAndReload(t *testing.T, m model) model {
	t.Helper()
	m, cmd := applyMessage(m, press("R", 'R'))
	return runUICommands(t, m, cmd)
}

func runUICommands(t *testing.T, m model, cmd tea.Cmd) model {
	t.Helper()
	for steps := 0; cmd != nil && steps < 8; steps++ {
		m, cmd = applyMessage(m, runCommand(t, cmd))
	}
	if cmd != nil {
		t.Fatal("asynchronous UI command chain did not settle")
	}
	return m
}

var _ feedStore = (*memoryFeedStore)(nil)
