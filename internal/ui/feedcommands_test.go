package ui

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/pressly/goose/v3"
	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/database"
	"github.com/su1uv/atom1c/internal/handlers"
	_ "modernc.org/sqlite"
)

type memoryFeedStore struct {
	feeds        []database.Feed
	posts        map[int64][]database.Post
	addErr       error
	getPageErr   error
	positionErr  error
	postsErr     error
	refreshErr   error
	refreshHook  func(database.Feed)
	addCalls     int
	pageCalls    []handlers.FeedPageParams
	postCalls    []int64
	refreshCalls []database.Feed
}

type contextCheckingFeedStore struct{}

func (contextCheckingFeedStore) GetPage(ctx context.Context, _ handlers.FeedPageParams) (handlers.FeedPage, error) {
	return handlers.FeedPage{}, ctx.Err()
}
func (contextCheckingFeedStore) Add(context.Context, handlers.AddFeedParams) (database.Feed, error) {
	return database.Feed{}, nil
}
func (contextCheckingFeedStore) Position(context.Context, int64, string) (int64, error) {
	return 0, nil
}
func (contextCheckingFeedStore) GetPosts(context.Context, int64) ([]database.Post, error) {
	return nil, nil
}
func (contextCheckingFeedStore) Refresh(context.Context, database.Feed) error { return nil }

func TestFeedPageCommandInheritsSessionCancellation(t *testing.T) {
	sessionCtx, cancel := context.WithCancel(context.Background())
	m := testModel()
	m.ctx = sessionCtx
	m.feedStore = contextCheckingFeedStore{}
	m.feedPageSize = 10
	cmd := m.beginFeedPageLoad()
	cancel()
	result := runCommand(t, cmd).(feedPageResult)
	if !errors.Is(result.err, context.Canceled) {
		t.Fatalf("feed load error = %v, want context.Canceled", result.err)
	}
}

func (s *memoryFeedStore) GetPage(_ context.Context, params handlers.FeedPageParams) (handlers.FeedPage, error) {
	s.pageCalls = append(s.pageCalls, params)
	if s.getPageErr != nil {
		return handlers.FeedPage{}, s.getPageErr
	}
	matches := make([]database.Feed, 0, len(s.feeds))
	for _, feed := range s.feeds {
		if strings.Contains(strings.ToLower(feed.Name), strings.ToLower(params.Search)) {
			matches = append(matches, feed)
		}
	}
	start := min(int(params.Offset), len(matches))
	end := min(start+int(params.Limit), len(matches))
	return handlers.FeedPage{Feeds: matches[start:end], Total: int64(len(matches))}, nil
}

func (s *memoryFeedStore) Add(_ context.Context, params handlers.AddFeedParams) (database.Feed, error) {
	s.addCalls++
	if s.addErr != nil {
		return database.Feed{}, s.addErr
	}
	for _, feed := range s.feeds {
		if feed.Url == params.URL {
			return database.Feed{}, handlers.ErrFeedURLExists
		}
	}
	feed := database.Feed{
		ID:        int64(len(s.feeds) + 1),
		CreatedAt: fmt.Sprintf("2026-10-07 10:%02d:00", len(s.feeds)),
		Name:      params.Name,
		Url:       params.URL,
	}
	s.feeds = append(s.feeds, feed)
	return feed, nil
}

func (s *memoryFeedStore) Position(_ context.Context, id int64, search string) (int64, error) {
	if s.positionErr != nil {
		return 0, s.positionErr
	}
	var position int64
	for _, feed := range s.feeds {
		if !strings.Contains(strings.ToLower(feed.Name), strings.ToLower(search)) {
			continue
		}
		if feed.ID == id {
			return position, nil
		}
		position++
	}
	return 0, errors.New("feed not found")
}

func (s *memoryFeedStore) GetPosts(_ context.Context, feedID int64) ([]database.Post, error) {
	s.postCalls = append(s.postCalls, feedID)
	if s.postsErr != nil {
		return nil, s.postsErr
	}
	return append([]database.Post(nil), s.posts[feedID]...), nil
}

func (s *memoryFeedStore) Refresh(_ context.Context, feed database.Feed) error {
	s.refreshCalls = append(s.refreshCalls, feed)
	if s.refreshErr != nil {
		return s.refreshErr
	}
	if s.refreshHook != nil {
		s.refreshHook(feed)
	}
	return nil
}

func modelWithFeedStore(store feedStore) model {
	m := testModel()
	m.feedStore = store
	return m
}

func runCommand(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected Bubble Tea command")
	}
	return cmd()
}

func applyMessage(m model, msg tea.Msg) (model, tea.Cmd) {
	updated, cmd := m.Update(msg)
	return updated.(model), cmd
}

func loadedFeedModel(t *testing.T, store feedStore) model {
	t.Helper()
	m := modelWithFeedStore(store)
	updated, cmd := applyMessage(m, tea.WindowSizeMsg{Width: 100, Height: 20})
	m = updated
	msg := runCommand(t, cmd)
	m, _ = applyMessage(m, msg)
	if m.feedPageSize < 1 {
		t.Fatal("feed page size was not calculated from the pane")
	}
	return m
}

func TestFeedBrowsingLoadsAndPagesPastTwentyResults(t *testing.T) {
	store := &memoryFeedStore{}
	for i := 0; i < 25; i++ {
		store.feeds = append(store.feeds, database.Feed{
			ID:        int64(i + 1),
			CreatedAt: fmt.Sprintf("2026-10-07 09:%02d:00", i),
			Name:      fmt.Sprintf("Feed %02d", i),
			Url:       fmt.Sprintf("https://example.test/%02d", i),
		})
	}
	m := loadedFeedModel(t, store)
	if m.feedTotal != 25 {
		t.Fatalf("feed total = %d, want 25", m.feedTotal)
	}
	if len(m.feeds.list.Items()) != m.feedPageSize {
		t.Fatalf("first page has %d items, want pane-sized page of %d", len(m.feeds.list.Items()), m.feedPageSize)
	}

	var cmd tea.Cmd
	for pages := 0; m.canGoToFeedPage(m.feedPage + 1); pages++ {
		if pages > 100 {
			t.Fatalf("feed pagination did not reach the final page; current page %d, page size %d", m.feedPage, m.feedPageSize)
		}
		previousPage := m.feedPage
		m, cmd = applyMessage(m, press("l", 'l'))
		if m.feedPage != previousPage+1 {
			t.Fatalf("next-page key left page at %d, want %d", m.feedPage, previousPage+1)
		}
		msg := runCommand(t, cmd)
		m, _ = applyMessage(m, msg)
	}
	if m.feedTotal != 25 {
		t.Fatalf("last-page total = %d, want 25", m.feedTotal)
	}
	lastPageStart := m.feedPage * m.feedPageSize
	if got, want := m.feeds.list.Items()[0].(item).name, fmt.Sprintf("Feed %02d", lastPageStart); got != want {
		t.Fatalf("last page starts with %q, want %q", got, want)
	}
	if m.canGoToFeedPage(m.feedPage + 1) {
		t.Fatal("feed page navigation passed the last page")
	}
}

func TestResizePreservesFeedSelectionByCollectionPosition(t *testing.T) {
	store := &memoryFeedStore{}
	for i := 0; i < 25; i++ {
		store.feeds = append(store.feeds, database.Feed{
			ID:        int64(i + 1),
			CreatedAt: fmt.Sprintf("2026-10-07 09:%02d:00", i),
			Name:      fmt.Sprintf("Feed %02d", i),
			Url:       fmt.Sprintf("https://example.test/%02d", i),
		})
	}
	m := loadedFeedModel(t, store)
	oldPageSize := m.feedPageSize
	for page := 0; page < 2; page++ {
		var cmd tea.Cmd
		m, cmd = applyMessage(m, press("l", 'l'))
		m, _ = applyMessage(m, runCommand(t, cmd))
	}
	m = updateModel(m, press("down", tea.KeyDown))
	if m.feeds.list.Index() != 1 || m.feedCursor != 1 {
		t.Fatalf("pre-resize selected cursor = list %d / model %d, want 1 / 1", m.feeds.list.Index(), m.feedCursor)
	}
	m, cmd := applyMessage(m, tea.WindowSizeMsg{Width: 100, Height: 32})
	if m.feedPageSize == oldPageSize {
		t.Fatalf("test resize did not change page size from %d", oldPageSize)
	}
	m, _ = applyMessage(m, runCommand(t, cmd))
	selected, ok := m.feeds.list.SelectedItem().(item)
	if !ok || selected.id != 8 {
		t.Fatalf("selected feed after resize = %#v, want original collection item ID 8", m.feeds.list.SelectedItem())
	}
	wantPage := 7 / m.feedPageSize
	if m.feedPage != wantPage {
		t.Fatalf("resized page = %d, want %d", m.feedPage, wantPage)
	}
}

func TestFeedSearchOwnsShortcutKeysAndSearchesBeyondCurrentPage(t *testing.T) {
	store := &memoryFeedStore{}
	for i := 0; i < 12; i++ {
		name := fmt.Sprintf("Feed %02d", i)
		if i == 11 {
			name = "Needle source"
		}
		store.feeds = append(store.feeds, database.Feed{ID: int64(i + 1), Name: name, Url: fmt.Sprintf("https://example.test/%d", i)})
	}
	m := loadedFeedModel(t, store)
	m = updateModel(m, press("/", '/'))
	if !m.feedSearching {
		t.Fatal("slash did not open global feed search")
	}
	m, _ = applyMessage(m, press("n", 'n'))
	m, _ = applyMessage(m, press("e", 'e'))
	m, _ = applyMessage(m, press("e", 'e'))
	m, _ = applyMessage(m, press("d", 'd'))
	m, _ = applyMessage(m, press("l", 'l'))
	m, _ = applyMessage(m, press("e", 'e'))
	if got := m.feedSearch.Value(); got != "needle" {
		t.Fatalf("typed search = %q, want needle", got)
	}
	if m.modalOpen || m.feedPage != 0 {
		t.Fatal("search keystrokes triggered a global shortcut or kept the old page")
	}
	request := m.feedRequest
	m, cmd := applyMessage(m, feedSearchDebounceMsg{request: request})
	if cmd == nil {
		t.Fatal("valid search debounce did not request database results")
	}
	m, _ = applyMessage(m, runCommand(t, cmd))
	if m.feedTotal != 1 || len(m.feeds.list.Items()) != 1 || m.feeds.list.Items()[0].(item).name != "Needle source" {
		t.Fatalf("global search results = %d feeds / %#v, want only the later Needle source", m.feedTotal, m.feeds.list.Items())
	}
	if len(store.pageCalls) < 2 || store.pageCalls[len(store.pageCalls)-1].Search != "needle" {
		t.Fatalf("database search calls = %#v, want global search needle", store.pageCalls)
	}
}

func TestStaleFeedPageResultDoesNotReplaceNewSearch(t *testing.T) {
	store := &memoryFeedStore{feeds: []database.Feed{{ID: 1, Name: "Fresh", Url: "https://example.test/fresh"}}}
	m := loadedFeedModel(t, store)
	oldRequest := m.feedRequest
	m, _ = applyMessage(m, press("/", '/'))
	m, _ = applyMessage(m, press("f", 'f'))
	before := m.feeds.list.Items()[0].(item).name
	m, _ = applyMessage(m, feedPageResult{
		request: oldRequest,
		page:    0,
		result:  handlers.FeedPage{Feeds: []database.Feed{{ID: 999, Name: "Obsolete", Url: "https://example.test/old"}}, Total: 1},
	})
	if got := m.feeds.list.Items()[0].(item).name; got != before {
		t.Fatalf("stale result replaced current list with %q", got)
	}
}

func TestSavedFeedIsLoadedOnItsPageAndSelected(t *testing.T) {
	store := &memoryFeedStore{}
	for i := 0; i < 8; i++ {
		store.feeds = append(store.feeds, database.Feed{
			ID: int64(i + 1), Name: fmt.Sprintf("Feed %02d", i), Url: fmt.Sprintf("https://example.test/%d", i),
		})
	}
	m := loadedFeedModel(t, store)
	m = updateModel(m, press("tab", tea.KeyTab))
	m = updateModel(m, press("a", 'a'))
	m.addFeed.inputs[0].SetValue("  New feed  ")
	m.addFeed.inputs[1].SetValue("  https://example.test/new  ")
	m.addFeed.focusIndex = len(m.addFeed.inputs)
	m, cmd := applyMessage(m, press("enter", tea.KeyEnter))
	if !m.modalOpen || !m.addFeed.saving {
		t.Fatal("modal did not remain open in saving state")
	}
	if got := m.addFeed.inputs[0].Value(); got != "New feed" {
		t.Fatalf("submitted name was not trimmed: %q", got)
	}
	oldPageSize := m.feedPageSize
	var resizeCmd tea.Cmd
	m, resizeCmd = applyMessage(m, tea.WindowSizeMsg{Width: 100, Height: 32})
	if m.feedPageSize == oldPageSize || resizeCmd == nil {
		t.Fatal("test resize did not start a page-size reload while the save was in flight")
	}
	msg := runCommand(t, cmd)
	var reloadCmd tea.Cmd
	m, reloadCmd = applyMessage(m, msg)
	if reloadCmd != nil {
		m, _ = applyMessage(m, runCommand(t, reloadCmd))
	}
	newID := int64(9)
	if m.modalOpen || m.addFeed.inputs[0].Value() != "" {
		t.Fatal("successful save did not close and reset the modal")
	}
	if m.focus != focusFeeds {
		t.Fatal("successful save did not return focus to the feeds pane")
	}
	if m.feedPage != (len(store.feeds)-1)/m.feedPageSize {
		t.Fatalf("new feed page = %d, want last page %d", m.feedPage, (len(store.feeds)-1)/m.feedPageSize)
	}
	selected, ok := m.feeds.list.SelectedItem().(item)
	if !ok || selected.id != newID {
		t.Fatalf("selected item = %#v, want newly saved feed ID %d", m.feeds.list.SelectedItem(), newID)
	}
}

func TestFeedSaveFailurePreservesDraftAndPreventsDuplicateSubmit(t *testing.T) {
	store := &memoryFeedStore{addErr: handlers.ErrFeedURLExists}
	m := modelWithFeedStore(store)
	m = updateModel(m, press("a", 'a'))
	m.addFeed.inputs[0].SetValue("Existing")
	m.addFeed.inputs[1].SetValue("https://example.test/existing")
	m.addFeed.focusIndex = len(m.addFeed.inputs)
	m, cmd := applyMessage(m, press("enter", tea.KeyEnter))
	if !m.addFeed.saving {
		t.Fatal("valid submission did not enter saving state")
	}
	m, duplicateCmd := applyMessage(m, press("enter", tea.KeyEnter))
	if duplicateCmd != nil {
		t.Fatal("saving modal accepted a second submission")
	}
	if store.addCalls != 0 {
		t.Fatal("save command ran synchronously")
	}
	msg := runCommand(t, cmd)
	m, _ = applyMessage(m, msg)
	if !m.modalOpen || m.addFeed.saving {
		t.Fatal("failed save should leave modal open and restore editing")
	}
	if store.addCalls != 1 {
		t.Fatalf("save calls = %d, want exactly one", store.addCalls)
	}
	if m.addFeed.inputs[0].Value() != "Existing" || m.addFeed.inputs[1].Value() != "https://example.test/existing" {
		t.Fatal("failed save did not preserve the draft")
	}
	if !strings.Contains(m.addFeed.err, "already exists") {
		t.Fatalf("duplicate URL error = %q", m.addFeed.err)
	}
	if !m.addFeed.inputs[1].Focused() {
		t.Fatal("duplicate URL failure did not return focus to the URL field")
	}
}

func TestSavedFeedReloadRetryDoesNotRepeatInsert(t *testing.T) {
	store := &memoryFeedStore{getPageErr: errors.New("temporary read failure")}
	m := modelWithFeedStore(store)
	m.feedPageSize = 2
	m = updateModel(m, press("a", 'a'))
	m.addFeed.inputs[0].SetValue("New feed")
	m.addFeed.inputs[1].SetValue("https://example.test/new")
	m.addFeed.focusIndex = len(m.addFeed.inputs)
	m, cmd := applyMessage(m, press("enter", tea.KeyEnter))
	m, _ = applyMessage(m, runCommand(t, cmd))
	if m.modalOpen || m.pendingSavedFeedID == 0 || !strings.Contains(m.feedErr, "saved feed") {
		t.Fatalf("saved-but-not-loaded state = modal %v, pending ID %d, error %q", m.modalOpen, m.pendingSavedFeedID, m.feedErr)
	}
	store.getPageErr = nil
	m, cmd = applyMessage(m, press("r", 'r'))
	m, _ = applyMessage(m, runCommand(t, cmd))
	if store.addCalls != 1 {
		t.Fatalf("retry repeated insertion %d times", store.addCalls)
	}
	if m.pendingSavedFeedID != 0 || m.feedErr != "" {
		t.Fatalf("successful retry state = pending ID %d, error %q", m.pendingSavedFeedID, m.feedErr)
	}
	selected, ok := m.feeds.list.SelectedItem().(item)
	if !ok || selected.name != "New feed" {
		t.Fatalf("retry selected %v, want saved feed", m.feeds.list.SelectedItem())
	}
}

func TestAddFeedWorkflowPersistsAcrossDatabaseReopen(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ui-feeds.db")
	db := openUIWorkflowDB(t, dbPath)
	state := &internal.State{Db: database.New(db), SQLDB: db}
	m := newModel(state).(model)
	m, cmd := applyMessage(m, tea.WindowSizeMsg{Width: 100, Height: 20})
	m, _ = applyMessage(m, runCommand(t, cmd))
	m = updateModel(m, press("a", 'a'))
	m.addFeed.inputs[0].SetValue("Persistent feed")
	m.addFeed.inputs[1].SetValue("https://example.test/persistent")
	m.addFeed.focusIndex = len(m.addFeed.inputs)
	m, cmd = applyMessage(m, press("enter", tea.KeyEnter))
	m, _ = applyMessage(m, runCommand(t, cmd))
	if m.feedTotal != 1 || len(m.feeds.list.Items()) != 1 {
		t.Fatalf("feed not visible after save: total %d, items %d", m.feedTotal, len(m.feeds.list.Items()))
	}
	selected, ok := m.feeds.list.SelectedItem().(item)
	if !ok || selected.name != "Persistent feed" {
		t.Fatalf("selected after save = %#v, want Persistent feed", m.feeds.list.SelectedItem())
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	reopened := openUIWorkflowDB(t, dbPath)
	defer reopened.Close()
	restarted := newModel(&internal.State{Db: database.New(reopened), SQLDB: reopened}).(model)
	restarted, cmd = applyMessage(restarted, tea.WindowSizeMsg{Width: 100, Height: 20})
	restarted, _ = applyMessage(restarted, runCommand(t, cmd))
	if restarted.feedTotal != 1 || len(restarted.feeds.list.Items()) != 1 {
		t.Fatalf("feed missing after database reopen: total %d, items %d", restarted.feedTotal, len(restarted.feeds.list.Items()))
	}
	if got := restarted.feeds.list.Items()[0].(item).name; got != "Persistent feed" {
		t.Fatalf("feed after restart = %q, want Persistent feed", got)
	}
}

func TestSeparateSessionsReloadSharedDatabaseChanges(t *testing.T) {
	db := openUIWorkflowDB(t, filepath.Join(t.TempDir(), "shared-sessions.db"))
	defer db.Close()
	state := &internal.State{Db: database.New(db), SQLDB: db}
	first := newModel(state).(model)
	second := newModel(state).(model)
	for _, session := range []*model{&first, &second} {
		var cmd tea.Cmd
		*session, cmd = applyMessage(*session, tea.WindowSizeMsg{Width: 100, Height: 20})
		*session, _ = applyMessage(*session, runCommand(t, cmd))
	}

	first = updateModel(first, press("a", 'a'))
	first.addFeed.inputs[0].SetValue("Shared feed")
	first.addFeed.inputs[1].SetValue("https://example.test/shared")
	first.addFeed.focusIndex = len(first.addFeed.inputs)
	var cmd tea.Cmd
	first, cmd = applyMessage(first, press("enter", tea.KeyEnter))
	first, _ = applyMessage(first, runCommand(t, cmd))
	if first.feedTotal != 1 {
		t.Fatalf("writer session feed total = %d, want 1", first.feedTotal)
	}
	if second.feedTotal != 0 {
		t.Fatalf("other session changed before reload: total = %d", second.feedTotal)
	}

	cmd = second.beginFeedPageLoad()
	second, _ = applyMessage(second, runCommand(t, cmd))
	if second.feedTotal != 1 || len(second.feeds.list.Items()) != 1 {
		t.Fatalf("reloaded session did not observe shared feed: total=%d items=%d", second.feedTotal, len(second.feeds.list.Items()))
	}
	if got := second.feeds.list.Items()[0].(item).name; got != "Shared feed" {
		t.Fatalf("reloaded session feed name = %q, want Shared feed", got)
	}
}

func openUIWorkflowDB(t *testing.T, dbPath string) *sql.DB {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	migrationDir := filepath.Join(filepath.Dir(sourceFile), "..", "..", "sql", "schema")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open SQLite database: %v", err)
	}
	db.SetMaxOpenConns(1)
	if err := goose.SetDialect("sqlite"); err != nil {
		t.Fatalf("set migration dialect: %v", err)
	}
	if err := goose.Up(db, migrationDir); err != nil {
		_ = db.Close()
		t.Fatalf("apply migrations: %v", err)
	}
	return db
}

func TestFeedSearchDoesNotAcceptLateResultsForInactivePage(t *testing.T) {
	store := &memoryFeedStore{feeds: []database.Feed{{ID: 1, Name: "Only", Url: "https://example.test/only"}}}
	m := loadedFeedModel(t, store)
	m.feedPage = 1
	m.feedRequest++
	request := m.feedRequest
	m.feedPage, m.feedCursor = 0, 0
	m, _ = applyMessage(m, feedPageResult{request: request - 1, page: 1, result: handlers.FeedPage{Total: 1}})
	if m.feedPage != 0 || m.feedTotal != 1 {
		t.Fatal("late page response changed active page state")
	}
	if m.feeds.list.FilterState() != list.Unfiltered {
		t.Fatal("database search unexpectedly changed the list's local fuzzy-filter state")
	}
}

func TestValidateFeedDraft(t *testing.T) {
	for _, tc := range []struct {
		name      string
		feedName  string
		feedURL   string
		wantError string
		wantFocus int
	}{
		{name: "valid HTTP URL", feedName: "News", feedURL: "http://example.test/feed"},
		{name: "valid HTTPS URL", feedName: "News", feedURL: "https://example.test/feed"},
		{name: "scheme matching is case insensitive", feedName: "News", feedURL: "HTTPS://example.test/feed"},
		{name: "blank name", feedURL: "https://example.test/feed", wantError: "Feed name is required.", wantFocus: 0},
		{name: "relative URL", feedName: "News", feedURL: "/feed", wantError: "Enter an absolute HTTP or HTTPS URL.", wantFocus: 1},
		{name: "unsupported scheme", feedName: "News", feedURL: "ftp://example.test/feed", wantError: "Enter an absolute HTTP or HTTPS URL.", wantFocus: 1},
		{name: "missing host", feedName: "News", feedURL: "https:/feed", wantError: "Enter an absolute HTTP or HTTPS URL.", wantFocus: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotError, gotFocus := validateFeedDraft(tc.feedName, tc.feedURL)
			if gotError != tc.wantError || gotFocus != tc.wantFocus {
				t.Fatalf("validation = (%q, %d), want (%q, %d)", gotError, gotFocus, tc.wantError, tc.wantFocus)
			}
		})
	}
}
