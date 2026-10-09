package ui

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/database"
	"github.com/su1uv/atom1c/reader"
)

type fakeArticleFetcher struct {
	article reader.Article
	err     error
	calls   []string
	started chan struct{}
	release <-chan struct{}
	block   bool
}

func (f *fakeArticleFetcher) Fetch(ctx context.Context, rawURL string) (reader.Article, error) {
	f.calls = append(f.calls, rawURL)
	if f.started != nil {
		close(f.started)
	}
	if f.block || f.release != nil {
		select {
		case <-f.release:
		case <-ctx.Done():
			return reader.Article{}, ctx.Err()
		}
	}
	return f.article, f.err
}

type fakeArticleCache struct {
	rows    map[int64]database.ArticleCache
	err     error
	saves   int
	saveErr error
}

func (s *fakeArticleCache) Get(_ context.Context, postID int64, sourceURL string) (database.ArticleCache, error) {
	if s.err != nil {
		return database.ArticleCache{}, s.err
	}
	row, ok := s.rows[postID]
	if !ok || row.SourceUrl != sourceURL {
		return database.ArticleCache{}, sql.ErrNoRows
	}
	return row, nil
}
func (s *fakeArticleCache) Save(_ context.Context, params database.UpsertArticleCacheParams) (database.ArticleCache, error) {
	s.saves++
	if s.saveErr != nil {
		return database.ArticleCache{}, s.saveErr
	}
	row := database.ArticleCache{PostID: params.ID, SourceUrl: params.Link, FinalUrl: params.FinalUrl, Markdown: params.Markdown, Title: params.Title, Author: params.Author, SiteName: params.SiteName, PublishedAt: params.PublishedAt}
	if s.rows == nil {
		s.rows = map[int64]database.ArticleCache{}
	}
	s.rows[params.ID] = row
	return row, nil
}

func TestArticleFetchInheritsSessionCancellation(t *testing.T) {
	sessionCtx, cancel := context.WithCancel(context.Background())
	fetcher := &fakeArticleFetcher{block: true, started: make(chan struct{})}
	m := testModel()
	m.ctx = sessionCtx
	m.readerOpen = true
	m.reader.fetchSession = 1
	m.reader.article.post = database.Post{ID: 7, Link: "https://example.test/article"}
	m.articleFetcher = fetcher
	cmd := m.beginArticleFetch()
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	<-fetcher.started
	cancel()
	result := (<-done).(articleFetchResult)
	if !errors.Is(result.err, context.Canceled) {
		t.Fatalf("article fetch error = %v, want context.Canceled", result.err)
	}
}

func TestArticleOpenShowsPreviewThenFetchesAndCachesFullArticle(t *testing.T) {
	m := readerFixture()
	fetcher := &fakeArticleFetcher{article: reader.Article{URL: "https://example.test/article", Title: "Website title", Author: "Writer", SiteName: "Publisher", Markdown: "# Full body\n\n- Extracted list"}}
	cache := &fakeArticleCache{}
	m.articleFetcher, m.articleCache = fetcher, cache
	m, cmd := applyMessage(m, press("enter", tea.KeyEnter))
	result := runCommand(t, cmd)
	m, cmd = applyMessage(m, result)
	if cmd == nil {
		t.Fatal("cache miss did not start preview rendering")
	}
	result = runCommand(t, cmd)
	m, cmd = applyMessage(m, result)
	if !strings.Contains(ansi.Strip(m.reader.viewport.GetContent()), "Long body line") || !m.reader.fetching || cmd == nil {
		t.Fatal("feed preview not shown while website fetch starts")
	}
	fetchResult := runCommand(t, cmd)
	m, cmd = applyMessage(m, fetchResult)
	if cmd == nil || !m.reader.article.showFull {
		t.Fatal("successful extraction did not select full article")
	}
	m = runUICommands(t, m, cmd)
	plain := ansi.Strip(m.reader.viewport.GetContent())
	for _, want := range []string{"Website title", "Publisher", "Writer", "Full body", "Extracted list"} {
		if !strings.Contains(plain, want) {
			t.Errorf("full article missing %q: %s", want, plain)
		}
	}
	if strings.Contains(plain, "Long body line") || cache.saves != 1 {
		t.Fatal("preview retained or full article was not cached")
	}
}

func TestCachedArticleIsImmediateOfflineAndFeedToggleIsLocal(t *testing.T) {
	m := readerFixture()
	cache := &fakeArticleCache{rows: map[int64]database.ArticleCache{1: {PostID: 1, SourceUrl: "https://example.test/article", FinalUrl: "https://example.test/article", Markdown: "# Cached full article", Title: "Cached title", Author: "Cached author", SiteName: "Cached site"}}}
	fetcher := &fakeArticleFetcher{err: errors.New("offline")}
	m.articleCache, m.articleFetcher = cache, fetcher
	m = openReader(t, m)
	if len(fetcher.calls) != 0 || !m.reader.article.showFull || !strings.Contains(ansi.Strip(m.reader.viewport.GetContent()), "Cached full article") {
		t.Fatal("cached article was not used without network")
	}
	m, cmd := applyMessage(m, press("f", 'f'))
	m = runUICommands(t, m, cmd)
	if m.reader.article.showFull || !strings.Contains(ansi.Strip(m.reader.viewport.GetContent()), "Long body line") {
		t.Fatal("f did not switch to feed preview")
	}
	m, cmd = applyMessage(m, press("f", 'f'))
	m = runUICommands(t, m, cmd)
	if !m.reader.article.showFull || !strings.Contains(ansi.Strip(m.reader.viewport.GetContent()), "Cached full article") {
		t.Fatal("f did not restore cached article")
	}
	m, reload := applyMessage(m, press("R", 'R'))
	m = runUICommands(t, m, reload)
	if len(fetcher.calls) != 1 || !strings.Contains(ansi.Strip(m.reader.viewport.GetContent()), "Cached full article") {
		t.Fatal("failed reload lost cached article")
	}
	m, retry := applyMessage(m, press("r", 'r'))
	if retry == nil {
		t.Fatal("r did not retry failed fetch")
	}
	m = runUICommands(t, m, retry)
	if len(fetcher.calls) != 2 || !strings.Contains(ansi.Strip(m.reader.viewport.GetContent()), "Cached full article") {
		t.Fatal("failed retry lost cached article")
	}
}

func TestArticleFailureKeepsFeedPreviewAndRetryIsScoped(t *testing.T) {
	m := readerFixture()
	fetcher := &fakeArticleFetcher{err: errors.New("site rejected request")}
	m.articleCache, m.articleFetcher = &fakeArticleCache{}, fetcher
	m, cmd := applyMessage(m, press("enter", tea.KeyEnter))
	m = runUICommands(t, m, cmd)
	if !strings.Contains(ansi.Strip(m.reader.viewport.GetContent()), "Long body line") || !strings.Contains(m.readerView().Content, "r retry") {
		t.Fatal("fetch failure removed feed preview or retry hint")
	}
	_, retry := applyMessage(m, press("r", 'r'))
	m = runUICommands(t, m, retry)
	if len(fetcher.calls) != 2 || !strings.Contains(ansi.Strip(m.reader.viewport.GetContent()), "Long body line") {
		t.Fatal("retry did not repeat only website retrieval")
	}
}

func TestChangedPostLinkRejectsLateFetchedArticle(t *testing.T) {
	m := readerFixture()
	cache := &fakeArticleCache{rows: map[int64]database.ArticleCache{1: {PostID: 1, SourceUrl: "https://example.test/article", FinalUrl: "https://example.test/article", Markdown: "# Current cached article", Title: "Current"}}}
	fetcher := &fakeArticleFetcher{article: reader.Article{URL: "https://example.test/article", Title: "Stale fetch", Markdown: "stale body"}}
	m.articleCache, m.articleFetcher = cache, fetcher
	m = openReader(t, m)
	cache.saveErr = sql.ErrNoRows
	m, cmd := applyMessage(m, press("R", 'R'))
	m = runUICommands(t, m, cmd)
	if m.reader.article.full == nil || m.reader.article.full.Title != "Current" || !strings.Contains(m.reader.fetchError, "link changed") || strings.Contains(ansi.Strip(m.reader.viewport.GetContent()), "stale body") {
		t.Fatal("fetch result for a changed post link replaced the current article")
	}
}

func TestFeedToggleDuringCachedReloadDoesNotStrandFetch(t *testing.T) {
	m := readerFixture()
	cache := &fakeArticleCache{rows: map[int64]database.ArticleCache{1: {PostID: 1, SourceUrl: "https://example.test/article", FinalUrl: "https://example.test/article", Markdown: "# Cached", Title: "Cached"}}}
	started, release := make(chan struct{}), make(chan struct{})
	fetcher := &fakeArticleFetcher{article: reader.Article{URL: "https://example.test/article", Title: "Reloaded", Markdown: "# Reloaded body"}, started: started, release: release}
	m.articleCache, m.articleFetcher = cache, fetcher
	m = openReader(t, m)
	m, cmd := applyMessage(m, press("R", 'R'))
	resultCh := make(chan tea.Msg, 1)
	go func() { resultCh <- cmd() }()
	<-started
	m, renderCmd := applyMessage(m, press("f", 'f'))
	m = runUICommands(t, m, renderCmd)
	if m.reader.fetching != true || m.reader.article.showFull {
		t.Fatal("f did not keep preview selected during reload")
	}
	close(release)
	m, renderCmd = applyMessage(m, <-resultCh)
	if m.reader.fetching || !m.reader.article.showFull || m.reader.article.full.Title != "Reloaded" || renderCmd == nil {
		t.Fatal("reload result was lost after preview toggle")
	}
	m = runUICommands(t, m, renderCmd)
}

func TestClosingReaderCancelsWebsiteFetchAndRejectsLateResult(t *testing.T) {
	m := readerFixture()
	started := make(chan struct{})
	fetcher := &fakeArticleFetcher{article: reader.Article{Title: "Late", Markdown: "late body"}, started: started, block: true}
	cache := &fakeArticleCache{}
	m.articleFetcher, m.articleCache = fetcher, cache
	m, cmd := applyMessage(m, press("enter", tea.KeyEnter))
	m, cmd = applyMessage(m, runCommand(t, cmd))
	m, cmd = applyMessage(m, runCommand(t, cmd))
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	<-started
	m = updateModel(m, press("esc", tea.KeyEscape))
	late := <-done
	m = updateModel(m, late)
	if m.readerOpen || cache.saves != 0 {
		t.Fatalf("closed reader accepted late result: open=%v saves=%d result=%#v", m.readerOpen, cache.saves, late)
	}
}

func TestPersistedFullArticleWorkflowSurvivesRestartAndFailedReload(t *testing.T) {
	var requests atomic.Int32
	var fail atomic.Bool
	var html strings.Builder
	for i := 0; i < 10; i++ {
		fmt.Fprintf(&html, "<p>Website paragraph %d contains a complete story with enough words to qualify as readable article content, including context, details, and a clear explanation for people reading it.</p>", i)
	}
	page := `<!doctype html><html><head><title>Website Feature</title><meta name="author" content="Reporter"><meta property="og:site_name" content="Daily News"></head><body><nav>Navigation Sign in</nav><article><h1>Website Feature</h1>` + html.String() + `<h2>Important section</h2><ul><li>First detail</li><li>Second detail</li></ul></article><footer>Footer ads</footer></body></html>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		if fail.Load() {
			http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page))
	}))
	defer server.Close()
	dbPath := filepath.Join(t.TempDir(), "articles.db")
	db := openUIWorkflowDB(t, dbPath)
	queries := database.New(db)
	feed, err := queries.CreateFeed(t.Context(), database.CreateFeedParams{Name: "Daily News", Url: server.URL + "/feed"})
	if err != nil {
		t.Fatal(err)
	}
	postResult, err := db.ExecContext(t.Context(), `INSERT INTO posts (feed_id, identity_key, source_id, title, link, content, content_kind, published_raw, updated_raw) VALUES (?, 'entry', 'entry', 'Feed headline', ?, 'Short feed summary', 'text', '', '')`, feed.ID, server.URL+"/story")
	if err != nil {
		t.Fatal(err)
	}
	postID, err := postResult.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	newModelWithServer := func(databaseHandle *sql.DB) model {
		m := newModel(&internal.State{Db: database.New(databaseHandle), SQLDB: databaseHandle}).(model)
		m.articleFetcher = reader.NewFetcher(server.Client())
		return m
	}
	m := newModelWithServer(db)
	m, cmd := applyMessage(m, tea.WindowSizeMsg{Width: 100, Height: 22})
	m, _ = applyMessage(m, runCommand(t, cmd))
	m, cmd = applyMessage(m, press("tab", tea.KeyTab))
	m, _ = applyMessage(m, runCommand(t, cmd))
	m = openReader(t, m)
	plain := ansi.Strip(m.View().Content)
	for _, want := range []string{"Website Feature", "Reporter", "Daily News", "Website paragraph 0"} {
		if !strings.Contains(plain, want) {
			t.Errorf("full article missing %q: %s", want, plain)
		}
	}
	for _, unwanted := range []string{"Navigation Sign in", "Footer ads", "Short feed summary"} {
		if strings.Contains(plain, unwanted) {
			t.Errorf("article retained page/feed clutter %q", unwanted)
		}
	}
	if requests.Load() != 1 {
		t.Fatalf("initial fetches = %d, want one", requests.Load())
	}
	cache, err := queries.GetArticleCache(t.Context(), database.GetArticleCacheParams{PostID: postID, SourceUrl: server.URL + "/story"})
	if err != nil || !strings.Contains(cache.Markdown, "Website paragraph 0") || !strings.Contains(cache.Markdown, "Important section") || !strings.Contains(cache.Markdown, "First detail") || !strings.Contains(cache.Markdown, "Second detail") {
		t.Fatalf("persisted cache = %#v, %v", cache, err)
	}
	m.reader.viewport.GotoBottom()
	plain = ansi.Strip(m.View().Content)
	for _, want := range []string{"Important section", "First detail", "Second detail"} {
		if !strings.Contains(plain, want) {
			t.Errorf("article end missing %q: %s", want, plain)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db = openUIWorkflowDB(t, dbPath)
	defer db.Close()
	m = newModelWithServer(db)
	m, cmd = applyMessage(m, tea.WindowSizeMsg{Width: 100, Height: 22})
	m, _ = applyMessage(m, runCommand(t, cmd))
	m, cmd = applyMessage(m, press("tab", tea.KeyTab))
	m, _ = applyMessage(m, runCommand(t, cmd))
	m = openReader(t, m)
	if requests.Load() != 1 || !strings.Contains(ansi.Strip(m.View().Content), "Website paragraph 0") {
		t.Fatal("cached article was fetched again or unavailable after restart")
	}
	fail.Store(true)
	m, cmd = applyMessage(m, press("R", 'R'))
	m = runUICommands(t, m, cmd)
	if !strings.Contains(ansi.Strip(m.View().Content), "Website paragraph 0") || !strings.Contains(m.View().Content, "r retry") {
		t.Fatal("failed reload lost cached article or did not show retry")
	}
	if requests.Load() != 2 {
		t.Fatalf("reload request count = %d, want 2", requests.Load())
	}
	if _, err := db.ExecContext(t.Context(), "UPDATE posts SET link = ? WHERE id = ?", server.URL+"/changed", postID); err != nil {
		t.Fatal(err)
	}
	if _, err := queries.GetArticleCache(t.Context(), database.GetArticleCacheParams{PostID: postID, SourceUrl: server.URL + "/changed"}); err == nil {
		t.Fatal("cache matched changed source URL")
	}
}
