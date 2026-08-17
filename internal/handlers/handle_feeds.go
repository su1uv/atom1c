package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/database"
)

type AddFeedParams struct {
	Name string
	URL  string
}

func HandleAddFeed(s *internal.State, params AddFeedParams) error {
	feed, err := s.Db.CreateFeed(context.Background(), database.CreateFeedParams{
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

func HandleGetFeeds(s *internal.State) ([]database.Feed, error) {
	feeds, err := s.Db.GetFeeds(context.Background())
	if err != nil {
		return []database.Feed{}, err
	}

	return feeds, nil
}
