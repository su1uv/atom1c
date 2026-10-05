package database

import (
	"context"
	"database/sql"
	"testing"
)

func TestPostsSchemaSupportsFeedScopedIdentityAndSourceMetadata(t *testing.T) {
	db, queries, dbPath := openTimestampTestDB(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}

	firstFeed, err := queries.CreateFeed(ctx, CreateFeedParams{Name: "First", Url: "https://example.test/first"})
	if err != nil {
		t.Fatalf("create first feed: %v", err)
	}
	secondFeed, err := queries.CreateFeed(ctx, CreateFeedParams{Name: "Second", Url: "https://example.test/second"})
	if err != nil {
		t.Fatalf("create second feed: %v", err)
	}

	insert := `INSERT INTO posts (
		feed_id, identity_key, source_id, guid_is_permalink, title, link,
		content, content_kind, published_raw, published_at, updated_raw, source_updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	values := []any{firstFeed.ID, "id:entry-1", "entry-1", nil, "Title", "https://example.test/post",
		"<p>Body</p>", "html", "Thu, 01 Oct 2026 12:00:00 +0000", "2026-10-01 12:00:00", "", nil}
	if _, err := db.ExecContext(ctx, insert, values...); err != nil {
		t.Fatalf("insert post: %v", err)
	}

	if _, err := db.ExecContext(ctx, insert, values...); err == nil {
		t.Fatal("duplicate identity in one feed inserted without error")
	}
	values[0] = secondFeed.ID
	if _, err := db.ExecContext(ctx, insert, values...); err != nil {
		t.Fatalf("same identity in another feed: %v", err)
	}
	values[0] = int64(999999)
	if _, err := db.ExecContext(ctx, insert, values...); err == nil {
		t.Fatal("post with nonexistent feed inserted without error")
	}

	var storedTitle, storedIdentity, publishedRaw, publishedAt string
	var guid sql.NullInt64
	err = db.QueryRowContext(ctx, `SELECT title, identity_key, guid_is_permalink, published_raw, published_at
		FROM posts WHERE feed_id = ? ORDER BY id LIMIT 1`, firstFeed.ID).Scan(
		&storedTitle, &storedIdentity, &guid, &publishedRaw, &publishedAt)
	if err != nil {
		t.Fatalf("read persisted post metadata: %v", err)
	}
	if storedTitle != "Title" || storedIdentity != "id:entry-1" || guid.Valid || publishedRaw != values[8] || publishedAt != "2026-10-01 12:00:00" {
		t.Fatalf("persisted post = (%q, %q, %v, %q, %q), unexpected values", storedTitle, storedIdentity, guid, publishedRaw, publishedAt)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}
	reopened, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	defer reopened.Close()
	reopened.SetMaxOpenConns(1)
	if _, err := reopened.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		t.Fatalf("enable foreign keys after reopening: %v", err)
	}
	var reopenedContent string
	if err := reopened.QueryRowContext(ctx, `SELECT content FROM posts WHERE feed_id = ? AND identity_key = ?`, firstFeed.ID, "id:entry-1").Scan(&reopenedContent); err != nil {
		t.Fatalf("read post after reopening: %v", err)
	}
	if reopenedContent != "<p>Body</p>" {
		t.Fatalf("reopened content = %q, want stored markup", reopenedContent)
	}
}
