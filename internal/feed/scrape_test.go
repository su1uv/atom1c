package feed

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/su1uv/atom1c/internal/database"
	_ "modernc.org/sqlite"
)

func TestRefreshFeedUpsertsEntriesByFeedScopedIdentity(t *testing.T) {
	ctx := context.Background()
	db, queries := openRefreshTestDB(t)
	var mu sync.RWMutex
	body := atomDocument(
		atomEntry("tag:example.test,2026:one", "Initial", "https://example.test/one", `<content type="html">&lt;p&gt;initial&lt;/p&gt;</content><published>2026-10-01T12:00:00Z</published>`),
		atomEntry("", "Link identified", "https://example.test/link-only", `<content>first</content>`),
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.RLock()
		defer mu.RUnlock()
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	storedFeed := createRefreshFeed(t, queries, server.URL)

	if err := RefreshFeed(ctx, db, storedFeed); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	initialPosts, err := queries.GetPostsByFeed(ctx, storedFeed.ID)
	if err != nil {
		t.Fatalf("list initial posts: %v", err)
	}
	if len(initialPosts) != 2 {
		t.Fatalf("initial post count = %d, want 2", len(initialPosts))
	}
	initialByIdentity := postsByIdentity(initialPosts)
	initialOne := initialByIdentity["id:tag:example.test,2026:one"]
	initialLink := initialByIdentity["link:https://example.test/link-only"]
	if initialOne.ID == 0 || initialLink.ID == 0 {
		t.Fatalf("posts lack expected identities: %#v", initialByIdentity)
	}
	if initialOne.Content != "<p>initial</p>" || !initialOne.PublishedAt.Valid || initialOne.PublishedAt.String != "2026-10-01 12:00:00" {
		t.Fatalf("stored content/date = (%q, %v), want HTML and normalized UTC date", initialOne.Content, initialOne.PublishedAt)
	}
	if initialOne.GuidIsPermalink.Valid {
		t.Fatalf("Atom GUID metadata = %v, want NULL", initialOne.GuidIsPermalink)
	}

	mu.Lock()
	body = atomDocument(
		atomEntry("tag:example.test,2026:one", "Changed", "https://example.test/one", `<content type="html">&lt;p&gt;updated&lt;/p&gt;</content><published>2026-10-02T13:14:15+02:00</published><updated>2026-10-03T01:02:03Z</updated>`),
		atomEntry("tag:example.test,2026:two", "Duplicate first", "https://example.test/two", `<content>first duplicate</content>`),
		atomEntry("tag:example.test,2026:two", "Duplicate last", "https://example.test/two", `<content>last duplicate</content>`),
	)
	mu.Unlock()
	if err := RefreshFeed(ctx, db, storedFeed); err != nil {
		t.Fatalf("second refresh: %v", err)
	}
	updatedPosts, err := queries.GetPostsByFeed(ctx, storedFeed.ID)
	if err != nil {
		t.Fatalf("list updated posts: %v", err)
	}
	if len(updatedPosts) != 3 {
		t.Fatalf("updated post count = %d, want 3 (including retained missing entry)", len(updatedPosts))
	}
	updatedByIdentity := postsByIdentity(updatedPosts)
	updatedOne := updatedByIdentity["id:tag:example.test,2026:one"]
	if updatedOne.ID != initialOne.ID || updatedOne.CreatedAt != initialOne.CreatedAt {
		t.Fatalf("updated post identity = (%d, %q), want preserved (%d, %q)", updatedOne.ID, updatedOne.CreatedAt, initialOne.ID, initialOne.CreatedAt)
	}
	if updatedOne.Title != "Changed" || updatedOne.Content != "<p>updated</p>" || updatedOne.PublishedRaw != "2026-10-02T13:14:15+02:00" || updatedOne.PublishedAt.String != "2026-10-02 11:14:15" || updatedOne.UpdatedRaw != "2026-10-03T01:02:03Z" || !updatedOne.SourceUpdatedAt.Valid || updatedOne.SourceUpdatedAt.String != "2026-10-03 01:02:03" {
		t.Fatalf("updated post = %#v, want latest feed-provided fields", updatedOne)
	}
	updatedTwo := updatedByIdentity["id:tag:example.test,2026:two"]
	if updatedTwo.Title != "Duplicate last" || updatedTwo.Content != "last duplicate" {
		t.Fatalf("duplicate identity fields = (%q, %q), want last occurrence", updatedTwo.Title, updatedTwo.Content)
	}
	if got := updatedByIdentity["link:https://example.test/link-only"].ID; got != initialLink.ID {
		t.Fatalf("unchanged link identity ID = %d, want %d", got, initialLink.ID)
	}
	markedFeed, err := queries.GetNextFeedToFetch(ctx)
	if err != nil {
		t.Fatalf("read fetched feed: %v", err)
	}
	if !markedFeed.LastFetchedAt.Valid {
		t.Fatal("successful refresh did not mark feed fetched")
	}
}

func TestRefreshFeedRollsBackPostsWhenPersistenceOrFetchMarkFails(t *testing.T) {
	tests := []struct {
		name    string
		trigger string
	}{
		{
			name: "later post upsert fails",
			trigger: `CREATE TRIGGER reject_second_post BEFORE INSERT ON posts
				WHEN NEW.source_id = 'new-two'
				BEGIN SELECT RAISE(ABORT, 'injected post failure'); END`,
		},
		{
			name: "marking feed fetched fails",
			trigger: `CREATE TRIGGER reject_fetch_update BEFORE UPDATE OF last_fetched_at ON feeds
				BEGIN SELECT RAISE(ABORT, 'injected fetch update failure'); END`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			db, queries := openRefreshTestDB(t)
			var mu sync.RWMutex
			body := atomDocument(atomEntry("initial", "Original", "https://example.test/original", `<content>original</content>`))
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				mu.RLock()
				defer mu.RUnlock()
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			storedFeed := createRefreshFeed(t, queries, server.URL)
			if err := RefreshFeed(ctx, db, storedFeed); err != nil {
				t.Fatalf("initial refresh: %v", err)
			}
			if _, err := db.ExecContext(ctx, `UPDATE feeds SET last_fetched_at = '2000-01-01 00:00:00' WHERE id = ?`, storedFeed.ID); err != nil {
				t.Fatalf("set fetch timestamp sentinel: %v", err)
			}
			before, err := queries.GetPostsByFeed(ctx, storedFeed.ID)
			if err != nil {
				t.Fatalf("read original posts: %v", err)
			}

			mu.Lock()
			body = atomDocument(
				atomEntry("initial", "Should roll back", "https://example.test/original", `<content>changed</content>`),
				atomEntry("new-one", "New one", "https://example.test/new-one", `<content>new</content>`),
				atomEntry("new-two", "New two", "https://example.test/new-two", `<content>new</content>`),
			)
			mu.Unlock()
			if _, err := db.ExecContext(ctx, tt.trigger); err != nil {
				t.Fatalf("install failure trigger: %v", err)
			}
			if err := RefreshFeed(ctx, db, storedFeed); err == nil {
				t.Fatal("refresh succeeded despite injected database failure")
			}

			after, err := queries.GetPostsByFeed(ctx, storedFeed.ID)
			if err != nil {
				t.Fatalf("read posts after rollback: %v", err)
			}
			if len(after) != len(before) || after[0].ID != before[0].ID || after[0].Title != before[0].Title || after[0].Content != before[0].Content {
				t.Fatalf("posts after failed refresh = %#v, original = %#v", after, before)
			}
			var lastFetched sql.NullString
			if err := db.QueryRowContext(ctx, `SELECT last_fetched_at FROM feeds WHERE id = ?`, storedFeed.ID).Scan(&lastFetched); err != nil {
				t.Fatalf("read fetch timestamp after rollback: %v", err)
			}
			if !lastFetched.Valid || lastFetched.String != "2000-01-01 00:00:00" {
				t.Fatalf("last_fetched_at after rollback = %v, want sentinel", lastFetched)
			}
		})
	}
}

func TestRefreshFeedRejectsEntriesWithoutIdentityBeforeWriting(t *testing.T) {
	ctx := context.Background()
	db, queries := openRefreshTestDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, atomDocument(
			atomEntry("valid", "Valid", "https://example.test/valid", `<content>valid</content>`),
			atomEntry("", "No identity", "", `<content>invalid</content>`),
		))
	}))
	defer server.Close()
	storedFeed := createRefreshFeed(t, queries, server.URL)

	if err := RefreshFeed(ctx, db, storedFeed); err == nil {
		t.Fatal("refresh accepted an entry without an ID or link")
	}
	posts, err := queries.GetPostsByFeed(ctx, storedFeed.ID)
	if err != nil {
		t.Fatalf("list posts after validation failure: %v", err)
	}
	if len(posts) != 0 {
		t.Fatalf("posts after invalid response = %d, want 0", len(posts))
	}
	if stored, err := queries.GetNextFeedToFetch(ctx); err != nil {
		t.Fatalf("read feed after validation failure: %v", err)
	} else if stored.LastFetchedAt.Valid {
		t.Fatalf("invalid response marked feed fetched at %q", stored.LastFetchedAt.String)
	}
}

func TestRefreshFeedPersistsRSSGUIDAndDateMetadata(t *testing.T) {
	ctx := context.Background()
	db, queries := openRefreshTestDB(t)
	body := `<rss version="2.0"><channel><item><title>RSS post</title><guid isPermaLink="false">opaque-guid</guid><description>&lt;p&gt;RSS body&lt;/p&gt;</description><pubDate>Thu, 01 Oct 2026 12:00:00 -0400</pubDate></item></channel></rss>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, body)
	}))
	defer server.Close()
	storedFeed := createRefreshFeed(t, queries, server.URL)

	if err := RefreshFeed(ctx, db, storedFeed); err != nil {
		t.Fatalf("refresh RSS feed: %v", err)
	}
	posts, err := queries.GetPostsByFeed(ctx, storedFeed.ID)
	if err != nil {
		t.Fatalf("list RSS posts: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("RSS post count = %d, want 1", len(posts))
	}
	post := posts[0]
	if post.IdentityKey != "id:opaque-guid" || post.SourceID != "opaque-guid" || post.Title != "RSS post" || post.Content != "<p>RSS body</p>" || post.ContentKind != string(ContentHTML) {
		t.Fatalf("RSS post = %#v, want normalized RSS fields", post)
	}
	if !post.GuidIsPermalink.Valid || post.GuidIsPermalink.Int64 != 0 {
		t.Fatalf("RSS GUID permalink value = %v, want false", post.GuidIsPermalink)
	}
	if post.PublishedRaw != "Thu, 01 Oct 2026 12:00:00 -0400" || !post.PublishedAt.Valid || post.PublishedAt.String != "2026-10-01 16:00:00" {
		t.Fatalf("RSS publication date = (%q, %v), want raw source and normalized UTC", post.PublishedRaw, post.PublishedAt)
	}
}

func TestPostIdentitySelection(t *testing.T) {
	tests := []struct {
		name    string
		entry   Entry
		want    string
		wantErr bool
	}{
		{name: "source ID takes precedence", entry: Entry{ID: "id-value", Link: "link-value"}, want: "id:id-value"},
		{name: "link fallback", entry: Entry{Link: "link-value"}, want: "link:link-value"},
		{name: "whitespace ID falls back verbatim to link", entry: Entry{ID: " \t", Link: " link-value "}, want: "link: link-value "},
		{name: "whitespace identity values rejected", entry: Entry{ID: " \t", Link: " \n"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := postIdentity(tt.entry)
			if (err != nil) != tt.wantErr {
				t.Fatalf("postIdentity() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("postIdentity() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFeedRefreshLockSerializesSameFeedAndHonorsCancellation(t *testing.T) {
	firstUnlock, err := lockFeedRefresh(context.Background(), 41)
	if err != nil {
		t.Fatalf("lock first refresh: %v", err)
	}
	defer firstUnlock()

	secondContext, cancelSecond := context.WithCancel(context.Background())
	secondResult := make(chan error, 1)
	secondAcquired := make(chan struct{})
	go func() {
		unlock, err := lockFeedRefresh(secondContext, 41)
		if err == nil {
			close(secondAcquired)
			unlock()
		}
		secondResult <- err
	}()
	select {
	case <-secondAcquired:
		t.Fatal("second refresh acquired a lock held for the same feed")
	case <-time.After(20 * time.Millisecond):
	}

	otherUnlock, err := lockFeedRefresh(context.Background(), 42)
	if err != nil {
		t.Fatalf("lock a different feed while first is held: %v", err)
	}
	otherUnlock()

	cancelSecond()
	select {
	case err := <-secondResult:
		if err != context.Canceled {
			t.Fatalf("waiting refresh error = %v, want context canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("waiting refresh did not observe cancellation")
	}
}

func TestRefreshFeedEmptyAndFailedFetches(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		cancel     bool
		wantErr    bool
	}{
		{name: "empty feed succeeds", body: atomDocument(), wantErr: false},
		{name: "HTTP error", statusCode: http.StatusBadGateway, body: "unavailable", wantErr: true},
		{name: "malformed XML", body: `<feed xmlns="http://www.w3.org/2005/Atom"><entry>`, wantErr: true},
		{name: "canceled context", body: atomDocument(), cancel: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tt.cancel {
				cancel()
			}
			db, queries := openRefreshTestDB(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tt.statusCode != 0 {
					w.WriteHeader(tt.statusCode)
				}
				_, _ = fmt.Fprint(w, tt.body)
			}))
			defer server.Close()
			storedFeed := createRefreshFeed(t, queries, server.URL)

			err := RefreshFeed(ctx, db, storedFeed)
			if (err != nil) != tt.wantErr {
				t.Fatalf("RefreshFeed() error = %v, wantErr %v", err, tt.wantErr)
			}
			posts, err := queries.GetPostsByFeed(context.Background(), storedFeed.ID)
			if err != nil {
				t.Fatalf("list posts: %v", err)
			}
			if len(posts) != 0 {
				t.Fatalf("posts = %d, want 0", len(posts))
			}
			feedAfter, err := queries.GetNextFeedToFetch(context.Background())
			if err != nil {
				t.Fatalf("read feed: %v", err)
			}
			if feedAfter.LastFetchedAt.Valid == tt.wantErr {
				t.Fatalf("LastFetchedAt = %v for wantErr %v", feedAfter.LastFetchedAt, tt.wantErr)
			}
		})
	}
}

func openRefreshTestDB(t *testing.T) (*sql.DB, *database.Queries) {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	migrationDir := filepath.Join(filepath.Dir(sourceFile), "..", "..", "sql", "schema")
	dbPath := filepath.Join(t.TempDir(), "refresh.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := goose.SetDialect("sqlite"); err != nil {
		t.Fatalf("set sqlite migration dialect: %v", err)
	}
	if err := goose.Up(db, migrationDir); err != nil {
		t.Fatalf("apply test migrations: %v", err)
	}
	var foreignKeys int
	if err := db.QueryRowContext(context.Background(), `PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatalf("read foreign key setting: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign key setting = %d, want enabled", foreignKeys)
	}
	return db, database.New(db)
}

func createRefreshFeed(t *testing.T, queries *database.Queries, url string) database.Feed {
	t.Helper()
	stored, err := queries.CreateFeed(context.Background(), database.CreateFeedParams{Name: "Test", Url: url})
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}
	return stored
}

func atomDocument(entries ...string) string {
	return `<feed xmlns="http://www.w3.org/2005/Atom">` + strings.Join(entries, "") + `</feed>`
}

func atomEntry(id, title, link, fields string) string {
	return `<entry><id>` + id + `</id><title>` + title + `</title>` +
		func() string {
			if link == "" {
				return ""
			}
			return `<link href="` + link + `"/>`
		}() + fields + `</entry>`
}

func postsByIdentity(posts []database.Post) map[string]database.Post {
	byIdentity := make(map[string]database.Post, len(posts))
	for _, post := range posts {
		byIdentity[post.IdentityKey] = post
	}
	return byIdentity
}
