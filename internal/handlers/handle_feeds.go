package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/database"
)

var ErrFeedURLExists = errors.New("a feed with this URL already exists")

type AddFeedParams struct {
	Name string
	URL  string
}

type FeedPageParams struct {
	Search string
	Limit  int64
	Offset int64
}

type FeedPage struct {
	Feeds []database.Feed
	Total int64
}

func HandleAddFeed(ctx context.Context, s *internal.State, params AddFeedParams) (database.Feed, error) {
	if s == nil || s.Db == nil {
		return database.Feed{}, errors.New("feed database is not configured")
	}
	feed, err := s.Db.CreateFeed(ctx, database.CreateFeedParams{
		Name: params.Name,
		Url:  params.URL,
	})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed: feeds.url") {
			return database.Feed{}, ErrFeedURLExists
		}
		return database.Feed{}, fmt.Errorf("create feed: %w", err)
	}
	return feed, nil
}

func HandleGetFeedsPage(ctx context.Context, s *internal.State, params FeedPageParams) (FeedPage, error) {
	if s == nil || s.Db == nil || s.SQLDB == nil {
		return FeedPage{}, errors.New("feed database is not configured")
	}
	if params.Limit < 1 || params.Offset < 0 {
		return FeedPage{}, fmt.Errorf("invalid feed page limit or offset")
	}

	tx, err := s.SQLDB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return FeedPage{}, fmt.Errorf("begin feed page read: %w", err)
	}
	defer tx.Rollback()

	queries := s.Db.WithTx(tx)
	feeds, err := queries.GetFeedsPage(ctx, database.GetFeedsPageParams{
		Search: params.Search,
		Limit:  params.Limit,
		Offset: params.Offset,
	})
	if err != nil {
		return FeedPage{}, fmt.Errorf("get feed page: %w", err)
	}
	total, err := queries.CountFeeds(ctx, params.Search)
	if err != nil {
		return FeedPage{}, fmt.Errorf("count feeds: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return FeedPage{}, fmt.Errorf("commit feed page read: %w", err)
	}
	return FeedPage{Feeds: feeds, Total: total}, nil
}

func HandleGetFeedPosition(ctx context.Context, s *internal.State, id int64, search string) (int64, error) {
	if s == nil || s.Db == nil {
		return 0, errors.New("feed database is not configured")
	}
	position, err := s.Db.GetFeedPosition(ctx, database.GetFeedPositionParams{ID: id, Search: search})
	if err != nil {
		return 0, fmt.Errorf("get feed position: %w", err)
	}
	return position, nil
}
