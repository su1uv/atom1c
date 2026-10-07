package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/database"
	_ "modernc.org/sqlite"
)

func TestHandleGetFeedsPageReturnsConsistentPageAndCount(t *testing.T) {
	state := testFeedState(t)
	ctx := context.Background()
	for i := 0; i < 25; i++ {
		if _, err := HandleAddFeed(ctx, state, AddFeedParams{
			Name: fmt.Sprintf("Feed %02d", i),
			URL:  fmt.Sprintf("https://example.test/%02d", i),
		}); err != nil {
			t.Fatalf("add feed %d: %v", i, err)
		}
	}

	page, err := HandleGetFeedsPage(ctx, state, FeedPageParams{Limit: 10, Offset: 20})
	if err != nil {
		t.Fatalf("get third page: %v", err)
	}
	if len(page.Feeds) != 5 || page.Total != 25 {
		t.Fatalf("third page = (%d feeds, total %d), want (5, 25)", len(page.Feeds), page.Total)
	}
	if page.Feeds[0].Name != "Feed 20" || page.Feeds[4].Name != "Feed 24" {
		t.Fatalf("third page names = %q through %q, want Feed 20 through Feed 24", page.Feeds[0].Name, page.Feeds[4].Name)
	}

	search, err := HandleGetFeedsPage(ctx, state, FeedPageParams{Search: "fEeD 2", Limit: 5})
	if err != nil {
		t.Fatalf("search feeds: %v", err)
	}
	if search.Total != 5 || len(search.Feeds) != 5 {
		t.Fatalf("search result = (%d feeds, total %d), want (5, 5)", len(search.Feeds), search.Total)
	}

	if _, err := HandleGetFeedsPage(ctx, state, FeedPageParams{Limit: 0}); err == nil {
		t.Fatal("zero page limit was accepted")
	}
	if _, err := HandleGetFeedsPage(ctx, state, FeedPageParams{Limit: 1, Offset: -1}); err == nil {
		t.Fatal("negative page offset was accepted")
	}
}

func TestHandleAddFeedReturnsDuplicateURLSentinel(t *testing.T) {
	state := testFeedState(t)
	ctx := context.Background()
	params := AddFeedParams{Name: "First", URL: "https://example.test/feed"}
	first, err := HandleAddFeed(ctx, state, params)
	if err != nil {
		t.Fatalf("add first feed: %v", err)
	}
	if first.ID == 0 {
		t.Fatal("created feed has no database ID")
	}

	_, err = HandleAddFeed(ctx, state, AddFeedParams{Name: "Second", URL: params.URL})
	if !errors.Is(err, ErrFeedURLExists) {
		t.Fatalf("duplicate add error = %v, want ErrFeedURLExists", err)
	}
}

func TestHandleGetFeedPositionUsesFilteredStableOrder(t *testing.T) {
	state := testFeedState(t)
	ctx := context.Background()
	first, err := HandleAddFeed(ctx, state, AddFeedParams{Name: "News first", URL: "https://example.test/first"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := HandleAddFeed(ctx, state, AddFeedParams{Name: "Other", URL: "https://example.test/other"}); err != nil {
		t.Fatal(err)
	}
	last, err := HandleAddFeed(ctx, state, AddFeedParams{Name: "News last", URL: "https://example.test/last"})
	if err != nil {
		t.Fatal(err)
	}

	position, err := HandleGetFeedPosition(ctx, state, last.ID, "news")
	if err != nil {
		t.Fatalf("get feed position: %v", err)
	}
	if position != 1 {
		t.Fatalf("filtered position = %d, want 1", position)
	}
	position, err = HandleGetFeedPosition(ctx, state, first.ID, "")
	if err != nil {
		t.Fatalf("get first feed position: %v", err)
	}
	if position != 0 {
		t.Fatalf("first position = %d, want 0", position)
	}
}

func testFeedState(t *testing.T) *internal.State {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	migrationDir := filepath.Join(filepath.Dir(sourceFile), "..", "..", "sql", "schema")
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "feeds.db")+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := goose.SetDialect("sqlite"); err != nil {
		t.Fatalf("set migration dialect: %v", err)
	}
	if err := goose.Up(db, migrationDir); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	return &internal.State{Db: database.New(db), SQLDB: db}
}
