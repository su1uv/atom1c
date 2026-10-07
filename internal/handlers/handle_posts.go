package handlers

import (
	"context"
	"errors"
	"fmt"

	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/database"
)

func HandleGetPostsByFeed(ctx context.Context, s *internal.State, feedID int64) ([]database.Post, error) {
	if s == nil || s.Db == nil {
		return nil, errors.New("post database is not configured")
	}
	posts, err := s.Db.GetPostsByFeed(ctx, feedID)
	if err != nil {
		return nil, fmt.Errorf("get posts for feed %d: %w", feedID, err)
	}
	return posts, nil
}
