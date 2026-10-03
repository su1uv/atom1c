package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

const timestampLayout = "2006-01-02 15:04:05"

func TestTimestampRoundTrips(t *testing.T) {
	db, queries, dbPath, _ := openTimestampTestDB(t, 0)
	ctx := context.Background()

	neverFetched, err := queries.CreateFeed(ctx, CreateFeedParams{
		Name: "Never fetched",
		Url:  "https://example.test/never.atom",
	})
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}
	assertCanonicalTimestamp(t, neverFetched.CreatedAt)
	assertCanonicalTimestamp(t, neverFetched.UpdatedAt)
	if neverFetched.LastFetchedAt.Valid {
		t.Fatalf("new feed LastFetchedAt = %q, want NULL", neverFetched.LastFetchedAt.String)
	}

	oldest, err := queries.CreateFeed(ctx, CreateFeedParams{
		Name: "Oldest",
		Url:  "https://example.test/oldest.atom",
	})
	if err != nil {
		t.Fatalf("create oldest feed: %v", err)
	}
	recent, err := queries.CreateFeed(ctx, CreateFeedParams{
		Name: "Recent",
		Url:  "https://example.test/recent.atom",
	})
	if err != nil {
		t.Fatalf("create recent feed: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE feeds SET last_fetched_at = ? WHERE id = ?`, "2001-01-01 00:00:00", oldest.ID); err != nil {
		t.Fatalf("set oldest fixture timestamp: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE feeds SET last_fetched_at = ? WHERE id = ?`, "2002-01-01 00:00:00", recent.ID); err != nil {
		t.Fatalf("set recent fixture timestamp: %v", err)
	}

	next, err := queries.GetNextFeedToFetch(ctx)
	if err != nil {
		t.Fatalf("get never-fetched feed: %v", err)
	}
	if next.ID != neverFetched.ID || next.LastFetchedAt.Valid {
		t.Fatalf("next feed = (%d, %v), want never-fetched feed %d", next.ID, next.LastFetchedAt, neverFetched.ID)
	}
	if err := queries.MarkFeedAsFetched(ctx, neverFetched.ID); err != nil {
		t.Fatalf("mark never-fetched feed: %v", err)
	}
	updated, err := queries.GetNextFeedToFetch(ctx)
	if err != nil {
		t.Fatalf("get oldest fetched feed: %v", err)
	}
	if updated.ID != oldest.ID {
		t.Fatalf("next feed ID = %d, want oldest fetched feed %d", updated.ID, oldest.ID)
	}
	if neverFetched.LastFetchedAt.Valid {
		t.Fatal("initial feed result unexpectedly changed after update")
	}

	feeds, err := queries.GetFeeds(ctx)
	if err != nil {
		t.Fatalf("list feeds: %v", err)
	}
	if len(feeds) != 3 {
		t.Fatalf("GetFeeds returned %d feeds, want 3", len(feeds))
	}
	var fetched Feed
	for _, feed := range feeds {
		if feed.ID == neverFetched.ID {
			fetched = feed
			break
		}
	}
	if !fetched.LastFetchedAt.Valid {
		t.Fatal("marked feed has NULL LastFetchedAt")
	}
	assertCanonicalTimestamp(t, fetched.LastFetchedAt.String)
	assertCanonicalTimestamp(t, fetched.UpdatedAt)

	if _, err := db.ExecContext(ctx, `INSERT INTO users (username) VALUES (?)`, "owner"); err != nil {
		t.Fatalf("insert user fixture: %v", err)
	}
	user, err := queries.GetUserByUsername(ctx, "owner")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	assertCanonicalTimestamp(t, user.CreatedAt)
	assertCanonicalTimestamp(t, user.UpdatedAt)

	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}
	reopened, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	defer reopened.Close()
	reopenedQueries := New(reopened)
	gotFeed, err := reopenedQueries.GetNextFeedToFetch(ctx)
	if err != nil {
		t.Fatalf("read feed after reopening: %v", err)
	}
	if gotFeed.ID != oldest.ID || !gotFeed.LastFetchedAt.Valid || gotFeed.LastFetchedAt.String != "2001-01-01 00:00:00" {
		t.Fatalf("reopened next feed = (%d, %q), want oldest feed with persisted timestamp", gotFeed.ID, gotFeed.LastFetchedAt.String)
	}
	if gotFeed.CreatedAt != oldest.CreatedAt || gotFeed.UpdatedAt != oldest.UpdatedAt {
		t.Fatalf("reopened feed timestamps = (%q, %q), want (%q, %q)", gotFeed.CreatedAt, gotFeed.UpdatedAt, oldest.CreatedAt, oldest.UpdatedAt)
	}
	gotUser, err := reopenedQueries.GetUserByUsername(ctx, "owner")
	if err != nil {
		t.Fatalf("read user after reopening: %v", err)
	}
	if gotUser.Username != "owner" {
		t.Fatalf("reopened username = %q, want owner", gotUser.Username)
	}
	if gotUser.CreatedAt != user.CreatedAt || gotUser.UpdatedAt != user.UpdatedAt {
		t.Fatalf("reopened user timestamps = (%q, %q), want (%q, %q)", gotUser.CreatedAt, gotUser.UpdatedAt, user.CreatedAt, user.UpdatedAt)
	}
}

func TestTimestampMigrationNormalizesGoTimeStrings(t *testing.T) {
	db, queries, _, migrationDir := openTimestampTestDB(t, 2)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `INSERT INTO feeds (created_at, updated_at, name, url, last_fetched_at)
		VALUES (?, ?, ?, ?, ?)`,
		"2025-01-02 03:04:05.987654321 -0600 CST m=+12.345",
		"2025-01-02 04:05:06 +0000 UTC",
		"Legacy feed",
		"https://example.test/legacy.atom",
		"2025-01-02 05:06:07.123456789 +0530 IST m=+0.1",
	)
	if err != nil {
		t.Fatalf("insert legacy feed: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO users (created_at, updated_at, username)
		VALUES (?, ?, ?)`,
		"2025-01-02 03:04:05.987654321 -0600 CST m=+12.345",
		"2025-01-02 04:05:06 +0000 UTC",
		"legacy-owner",
	); err != nil {
		t.Fatalf("insert legacy user: %v", err)
	}

	if err := goose.Up(db, migrationDir); err != nil {
		t.Fatalf("apply timestamp migration: %v", err)
	}

	feed, err := queries.GetFeeds(ctx)
	if err != nil {
		t.Fatalf("read migrated feed: %v", err)
	}
	if len(feed) != 1 {
		t.Fatalf("migrated feeds = %d, want 1", len(feed))
	}
	if feed[0].Name != "Legacy feed" || feed[0].Url != "https://example.test/legacy.atom" {
		t.Fatalf("feed data changed during migration: %+v", feed[0])
	}
	assertTimestampEqual(t, feed[0].CreatedAt, "2025-01-02 09:04:05")
	assertTimestampEqual(t, feed[0].UpdatedAt, "2025-01-02 04:05:06")
	if !feed[0].LastFetchedAt.Valid {
		t.Fatal("legacy non-null fetch timestamp became NULL")
	}
	assertTimestampEqual(t, feed[0].LastFetchedAt.String, "2025-01-01 23:36:07")

	user, err := queries.GetUserByUsername(ctx, "legacy-owner")
	if err != nil {
		t.Fatalf("read migrated user: %v", err)
	}
	assertTimestampEqual(t, user.CreatedAt, "2025-01-02 09:04:05")
	assertTimestampEqual(t, user.UpdatedAt, "2025-01-02 04:05:06")
}

func TestTimestampMigrationRejectsUnknownFormat(t *testing.T) {
	db, _, _, migrationDir := openTimestampTestDB(t, 2)
	if _, err := db.Exec(`INSERT INTO feeds (created_at, updated_at, name, url)
		VALUES (?, ?, ?, ?)`, "not a timestamp", "not a timestamp", "Unknown", "https://example.test/unknown.atom"); err != nil {
		t.Fatalf("insert unknown-format fixture: %v", err)
	}
	if err := goose.Up(db, migrationDir); err == nil {
		t.Fatal("timestamp migration succeeded with an unknown format; want failure")
	}
	version, err := goose.GetDBVersion(db)
	if err != nil {
		t.Fatalf("read migration version: %v", err)
	}
	if version != 2 {
		t.Fatalf("database version = %d after failed migration, want 2", version)
	}
	var timestamp string
	if err := db.QueryRow(`SELECT created_at FROM feeds WHERE name = 'Unknown'`).Scan(&timestamp); err != nil {
		t.Fatalf("read fixture after rollback: %v", err)
	}
	if timestamp != "not a timestamp" {
		t.Fatalf("timestamp after failed migration = %q, want original value", timestamp)
	}
}

func openTimestampTestDB(t *testing.T, upTo int64) (*sql.DB, *Queries, string, string) {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	migrationDir := filepath.Join(filepath.Dir(sourceFile), "..", "..", "sql", "schema")
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := goose.SetDialect("sqlite"); err != nil {
		t.Fatalf("set sqlite migration dialect: %v", err)
	}
	if upTo == 0 {
		err = goose.Up(db, migrationDir)
	} else {
		err = goose.UpTo(db, migrationDir, upTo)
	}
	if err != nil {
		t.Fatalf("apply test migrations: %v", err)
	}
	return db, New(db), dbPath, migrationDir
}

func assertCanonicalTimestamp(t *testing.T, value string) {
	t.Helper()
	parsed, err := time.ParseInLocation(timestampLayout, value, time.UTC)
	if err != nil {
		t.Fatalf("timestamp %q is not canonical UTC: %v", value, err)
	}
	if got := parsed.Format(timestampLayout); got != value {
		t.Fatalf("timestamp %q does not round-trip as canonical UTC (got %q)", value, got)
	}
}

func assertTimestampEqual(t *testing.T, got, want string) {
	t.Helper()
	assertCanonicalTimestamp(t, got)
	if got != want {
		t.Fatalf("timestamp = %q, want %q", got, want)
	}
}
