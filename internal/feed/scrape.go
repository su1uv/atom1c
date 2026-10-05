package feed

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"

	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/database"
)

const postTimestampLayout = "2006-01-02 15:04:05"

var feedRefreshLocks sync.Map

// RefreshFeed fetches and persists a feed's entries, then marks the feed fetched
// in the same transaction as the post changes. Invalid entries and any database
// failure leave both posts and the fetch timestamp unchanged. Refreshes for the
// same feed are serialized so an older response cannot overwrite a newer one.
func RefreshFeed(ctx context.Context, db *sql.DB, storedFeed database.Feed) error {
	unlock, err := lockFeedRefresh(ctx, storedFeed.ID)
	if err != nil {
		return err
	}
	defer unlock()

	fetched, err := Fetch(ctx, storedFeed.Url)
	if err != nil {
		return fmt.Errorf("fetch feed %q: %w", storedFeed.Url, err)
	}

	posts := make([]database.UpsertPostParams, 0, len(fetched.Entries))
	for i, entry := range fetched.Entries {
		identityKey, err := postIdentity(entry)
		if err != nil {
			return fmt.Errorf("feed %q entry %d: %w", storedFeed.Url, i+1, err)
		}
		posts = append(posts, database.UpsertPostParams{
			FeedID:          storedFeed.ID,
			IdentityKey:     identityKey,
			SourceID:        entry.ID,
			GuidIsPermalink: nullableBool(entry.GUIDIsPermaLink),
			Title:           entry.Title,
			Link:            entry.Link,
			Content:         entry.Content,
			ContentKind:     string(entry.ContentKind),
			PublishedRaw:    entry.Published.Raw,
			PublishedAt:     nullableSourceDate(entry.Published),
			UpdatedRaw:      entry.Updated.Raw,
			SourceUpdatedAt: nullableSourceDate(entry.Updated),
		})
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin feed refresh transaction: %w", err)
	}
	defer tx.Rollback()

	queries := database.New(tx)
	for _, post := range posts {
		if _, err := queries.UpsertPost(ctx, post); err != nil {
			return fmt.Errorf("persist entry %q for feed %q: %w", post.IdentityKey, storedFeed.Url, err)
		}
	}
	updated, err := queries.MarkFeedAsFetched(ctx, storedFeed.ID)
	if err != nil {
		return fmt.Errorf("mark feed %q fetched: %w", storedFeed.Url, err)
	}
	if updated != 1 {
		return fmt.Errorf("mark feed %q fetched: feed %d no longer exists", storedFeed.Url, storedFeed.ID)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit feed refresh for %q: %w", storedFeed.Url, err)
	}
	return nil
}

func lockFeedRefresh(ctx context.Context, feedID int64) (func(), error) {
	value, _ := feedRefreshLocks.LoadOrStore(feedID, make(chan struct{}, 1))
	lock := value.(chan struct{})
	select {
	case lock <- struct{}{}:
		return func() { <-lock }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ScrapeFeeds fetches the next due feed and persists all entries atomically.
func ScrapeFeeds(ctx context.Context, s *internal.State) error {
	nextFeed, err := s.Db.GetNextFeedToFetch(ctx)
	if err != nil {
		return err
	}
	return RefreshFeed(ctx, s.SQLDB, nextFeed)
}

func postIdentity(entry Entry) (string, error) {
	if strings.TrimSpace(entry.ID) != "" {
		return "id:" + entry.ID, nil
	}
	if strings.TrimSpace(entry.Link) != "" {
		return "link:" + entry.Link, nil
	}
	return "", fmt.Errorf("entry has neither a source ID nor a link")
}

func nullableBool(value *bool) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	if *value {
		return sql.NullInt64{Int64: 1, Valid: true}
	}
	return sql.NullInt64{Int64: 0, Valid: true}
}

func nullableSourceDate(date SourceDate) sql.NullString {
	if date.Normalized == nil {
		return sql.NullString{}
	}
	return sql.NullString{
		String: date.Normalized.UTC().Format(postTimestampLayout),
		Valid:  true,
	}
}
