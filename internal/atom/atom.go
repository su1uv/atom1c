package atom

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"time"

	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/db"
)

type AtomFeed struct {
	Title   string      `xml:"title"`
	Updated string      `xml:"updated"`
	Link    string      `xml:"link"`
	Entries []AtomEntry `xml:"entry"`
}

type AtomEntry struct {
	Title     string `xml:"title"`
	Link      string `xml:"link"`
	Content   string `xml:"content"`
	Published string `xml:"published"`
}

func ScrapeFeeds(s *internal.State) error {
	nextFeed, err := s.Db.GetNextFeedToFetch(context.Background())
	if err != nil {
		return err
	}

	feed, err := fetchFeed(context.Background(), nextFeed.Url)
	if err != nil {
		return err
	}

	if err = s.Db.MarkFeedAsFetched(context.Background(), db.MarkFeedAsFetchedParams{
		ID:            nextFeed.ID,
		UpdatedAt:     time.Now(),
		LastFetchedAt: sql.NullTime{Time: time.Now(), Valid: true},
	}); err != nil {
		return err
	}

	fmt.Printf("Feed fetched: %v", feed.Title)
	// TODO: Create posts with the feed's entries

	return nil
}

func fetchFeed(ctx context.Context, feedURL string) (*AtomFeed, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return &AtomFeed{}, err
	}

	req.Header.Set("User-Agent", "atom1c")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return &AtomFeed{}, err
	}
	defer res.Body.Close()

	content, err := io.ReadAll(res.Body)
	if err != nil {
		return &AtomFeed{}, err
	}

	var atomFeed AtomFeed
	if err := xml.Unmarshal(content, &atomFeed); err != nil {
		return &AtomFeed{}, err
	}

	atomFeed.Title = html.UnescapeString(atomFeed.Title)
	for _, entry := range atomFeed.Entries {
		entry.Title = html.UnescapeString(entry.Title)
		entry.Content = html.UnescapeString(entry.Content)
	}

	return &atomFeed, nil
}

func (f AtomFeed) String() string {
	return fmt.Sprintf("Title: %v\nUpdated: %v\nLink: %v\nEntries: %v\n", f.Title, f.Updated, f.Link, f.Entries)
}

func (e AtomEntry) String() string {
	return fmt.Sprintf("[\nTitle: %v\nLink: %v\nContent: %v\nPublished: %v\n]", e.Title, e.Link, e.Content, e.Published)
}
