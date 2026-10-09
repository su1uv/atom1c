package ui

import (
	"context"

	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/database"
	"github.com/su1uv/atom1c/internal/feed"
	"github.com/su1uv/atom1c/internal/handlers"
)

type feedStore interface {
	GetPage(context.Context, handlers.FeedPageParams) (handlers.FeedPage, error)
	Add(context.Context, handlers.AddFeedParams) (database.Feed, error)
	Position(context.Context, int64, string) (int64, error)
	GetPosts(context.Context, int64) ([]database.Post, error)
	Refresh(context.Context, database.Feed) error
}

type stateFeedStore struct {
	state *internal.State
}

func (s stateFeedStore) GetPage(ctx context.Context, params handlers.FeedPageParams) (handlers.FeedPage, error) {
	return handlers.HandleGetFeedsPage(ctx, s.state, params)
}

func (s stateFeedStore) Add(ctx context.Context, params handlers.AddFeedParams) (database.Feed, error) {
	return handlers.HandleAddFeed(ctx, s.state, params)
}

func (s stateFeedStore) Position(ctx context.Context, id int64, search string) (int64, error) {
	return handlers.HandleGetFeedPosition(ctx, s.state, id, search)
}

func (s stateFeedStore) GetPosts(ctx context.Context, feedID int64) ([]database.Post, error) {
	return handlers.HandleGetPostsByFeed(ctx, s.state, feedID)
}

func (s stateFeedStore) Refresh(ctx context.Context, storedFeed database.Feed) error {
	if s.state.FeedRefresh != nil {
		return s.state.FeedRefresh.Refresh(ctx, storedFeed)
	}
	return feed.RefreshFeed(ctx, s.state.SQLDB, storedFeed)
}
