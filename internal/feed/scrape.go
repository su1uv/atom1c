package feed

import (
	"context"
	"fmt"

	"github.com/su1uv/atom1c/internal"
)

// ScrapeFeeds fetches the next due feed and marks it fetched after successful
// parsing.
func ScrapeFeeds(ctx context.Context, s *internal.State) error {
	nextFeed, err := s.Db.GetNextFeedToFetch(ctx)
	if err != nil {
		return err
	}

	fetched, err := Fetch(ctx, nextFeed.Url)
	if err != nil {
		return err
	}
	if err := s.Db.MarkFeedAsFetched(ctx, nextFeed.ID); err != nil {
		return err
	}

	fmt.Printf("Feed fetched: %v", fetched.Title)
	// TODO: Persist the feed's entries in the posts step.
	return nil
}
