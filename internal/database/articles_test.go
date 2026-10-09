package database

import (
	"context"
	"database/sql"
	"testing"
)

func TestArticleCacheIsSourceScopedPersistentAndCascades(t *testing.T) {
	db, queries, dbPath := openTimestampTestDB(t)
	ctx := context.Background()
	feed, err := queries.CreateFeed(ctx, CreateFeedParams{Name: "Feed", Url: "https://example.test/feed"})
	if err != nil {
		t.Fatal(err)
	}
	postID := insertCacheTestPost(t, db, feed.ID, "https://example.test/article")
	params := UpsertArticleCacheParams{FinalUrl: "https://example.test/final", Markdown: "# Original", Title: "Title", Author: "Author", SiteName: "Site", PublishedAt: "2026-06-01T12:00:00Z", ID: postID, Link: "https://example.test/article"}
	stored, err := queries.UpsertArticleCache(ctx, params)
	if err != nil {
		t.Fatalf("save cache: %v", err)
	}
	got, err := queries.GetArticleCache(ctx, GetArticleCacheParams{PostID: postID, SourceUrl: params.Link})
	if err != nil || got.Markdown != stored.Markdown || got.SourceUrl != params.Link {
		t.Fatalf("get cache = %#v, %v", got, err)
	}
	if _, err := queries.GetArticleCache(ctx, GetArticleCacheParams{PostID: postID, SourceUrl: "https://example.test/changed"}); err != sql.ErrNoRows {
		t.Fatalf("changed source returned cache, error = %v", err)
	}
	if _, err := queries.UpsertArticleCache(ctx, UpsertArticleCacheParams{FinalUrl: "https://example.test/stale", Markdown: "stale", ID: postID, Link: "https://example.test/changed"}); err != sql.ErrNoRows {
		t.Fatalf("stale save error = %v, want no rows", err)
	}
	stillStored, err := queries.GetArticleCache(ctx, GetArticleCacheParams{PostID: postID, SourceUrl: params.Link})
	if err != nil || stillStored.Markdown != "# Original" {
		t.Fatalf("failed save replaced cache = %#v, %v", stillStored, err)
	}
	params.Markdown = "# Refreshed"
	if _, err := queries.UpsertArticleCache(ctx, params); err != nil {
		t.Fatal(err)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := reopened.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		t.Fatal(err)
	}
	reopenedQueries := New(reopened)
	got, err = reopenedQueries.GetArticleCache(ctx, GetArticleCacheParams{PostID: postID, SourceUrl: params.Link})
	if err != nil || got.Markdown != "# Refreshed" {
		t.Fatalf("reopened cache = %#v, %v", got, err)
	}
	if _, err := reopened.ExecContext(ctx, "DELETE FROM feeds WHERE id = ?", feed.ID); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := reopened.QueryRowContext(ctx, "SELECT COUNT(*) FROM article_cache WHERE post_id = ?", postID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("cache after feed delete count=%d err=%v", count, err)
	}
}

func insertCacheTestPost(t *testing.T, db *sql.DB, feedID int64, link string) int64 {
	t.Helper()
	result, err := db.Exec(`INSERT INTO posts (feed_id, identity_key, source_id, title, link, content, content_kind, published_raw, updated_raw) VALUES (?, 'entry', 'entry', 'Title', ?, '', 'text', '', '')`, feedID, link)
	if err != nil {
		t.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
