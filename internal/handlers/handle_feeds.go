package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/db"
)

type AddFeedParams struct {
	Name string
	URL  string
}

func HandleAddFeed(s *internal.State, params AddFeedParams) error {
	feed, err := s.Db.CreateFeed(context.Background(), db.CreateFeedParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      params.Name,
		Url:       params.URL,
	})
	if err != nil {
		return err
	}

	fmt.Printf("\n- %v feed added.\n", feed.Name)
	return nil
}

func HandleGetFeeds(s *internal.State) ([]db.Feed, error) {
	feeds, err := s.Db.GetFeeds(context.Background())
	if err != nil {
		return []db.Feed{}, err
	}

	return feeds, nil
}
