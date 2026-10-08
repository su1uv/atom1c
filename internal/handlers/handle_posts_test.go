package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/su1uv/atom1c/internal/database"
)

func TestHandleGetPostsByFeedScopesAndOrdersResults(t *testing.T) {
	state := testFeedState(t)
	ctx := context.Background()

	first, err := HandleAddFeed(ctx, state, AddFeedParams{Name: "First", URL: "https://example.test/first"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := HandleAddFeed(ctx, state, AddFeedParams{Name: "Second", URL: "https://example.test/second"})
	if err != nil {
		t.Fatal(err)
	}

	for _, params := range []database.UpsertPostParams{
		{FeedID: first.ID, IdentityKey: "id:old", SourceID: "old", Title: "Older"},
		{FeedID: second.ID, IdentityKey: "id:other", SourceID: "other", Title: "Other feed"},
		{FeedID: first.ID, IdentityKey: "id:new", SourceID: "new", Title: "Newer"},
	} {
		if _, err := state.Db.UpsertPost(ctx, params); err != nil {
			t.Fatalf("insert %q: %v", params.Title, err)
		}
	}

	posts, err := HandleGetPostsByFeed(ctx, state, first.ID)
	if err != nil {
		t.Fatalf("get posts: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("got %d posts, want 2: %#v", len(posts), posts)
	}
	if posts[0].Title != "Newer" || posts[1].Title != "Older" {
		t.Fatalf("post order = %q, %q; want descending ID order Newer, Older", posts[0].Title, posts[1].Title)
	}
}

func TestHandleGetPostsByFeedReturnsEmptyAndPropagatesErrors(t *testing.T) {
	t.Run("empty feed", func(t *testing.T) {
		state := testFeedState(t)
		feed, err := HandleAddFeed(context.Background(), state, AddFeedParams{Name: "Empty", URL: "https://example.test/empty"})
		if err != nil {
			t.Fatal(err)
		}
		posts, err := HandleGetPostsByFeed(context.Background(), state, feed.ID)
		if err != nil {
			t.Fatalf("get empty feed: %v", err)
		}
		if len(posts) != 0 {
			t.Fatalf("empty feed returned %d posts", len(posts))
		}
	})

	t.Run("unavailable database", func(t *testing.T) {
		if _, err := HandleGetPostsByFeed(context.Background(), nil, 1); err == nil {
			t.Fatal("missing state was accepted")
		}
	})

	t.Run("canceled context", func(t *testing.T) {
		state := testFeedState(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := HandleGetPostsByFeed(ctx, state, 1); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled read error = %v, want context.Canceled", err)
		}
	})
}
