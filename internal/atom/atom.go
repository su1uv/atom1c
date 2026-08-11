package atom

import (
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
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
