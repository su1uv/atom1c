package atom

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/su1uv/atom1c/internal"
)

const atomNamespace = "http://www.w3.org/2005/Atom"

var feedHTTPClient = &http.Client{Timeout: 15 * time.Second}

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

type atomFeedDocument struct {
	XMLName xml.Name
	Title   atomTextConstruct   `xml:"http://www.w3.org/2005/Atom title"`
	Updated string              `xml:"http://www.w3.org/2005/Atom updated"`
	Links   []atomLink          `xml:"http://www.w3.org/2005/Atom link"`
	Entries []atomEntryDocument `xml:"http://www.w3.org/2005/Atom entry"`
}

type atomEntryDocument struct {
	Title     atomTextConstruct  `xml:"http://www.w3.org/2005/Atom title"`
	Links     []atomLink         `xml:"http://www.w3.org/2005/Atom link"`
	Content   *atomTextConstruct `xml:"http://www.w3.org/2005/Atom content"`
	Summary   *atomTextConstruct `xml:"http://www.w3.org/2005/Atom summary"`
	Published string             `xml:"http://www.w3.org/2005/Atom published"`
}

type atomLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
}

type atomTextConstruct struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
	Inner string `xml:",innerxml"`
}

func (c atomTextConstruct) text() string {
	if c.Type == "xhtml" {
		return c.Inner
	}
	return c.Value
}

func alternateLink(links []atomLink) string {
	for _, link := range links {
		if link.Href != "" && (link.Rel == "" || link.Rel == "alternate") {
			return link.Href
		}
	}
	return ""
}

func ScrapeFeeds(ctx context.Context, s *internal.State) error {
	nextFeed, err := s.Db.GetNextFeedToFetch(ctx)
	if err != nil {
		return err
	}

	feed, err := fetchFeed(ctx, nextFeed.Url)
	if err != nil {
		return err
	}

	if err = s.Db.MarkFeedAsFetched(ctx, nextFeed.ID); err != nil {
		return err
	}

	fmt.Printf("Feed fetched: %v", feed.Title)
	// TODO: Create posts with the feed's entries

	return nil
}

func fetchFeed(ctx context.Context, feedURL string) (*AtomFeed, error) {
	return fetchFeedWithClient(ctx, feedURL, feedHTTPClient)
}

func fetchFeedWithClient(ctx context.Context, feedURL string, client *http.Client) (*AtomFeed, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request for feed %q: %w", feedURL, err)
	}

	req.Header.Set("User-Agent", "atom1c")
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch feed %q: %w", feedURL, err)
	}
	defer res.Body.Close()
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("fetch feed %q: unexpected HTTP status %s", feedURL, res.Status)
	}

	content, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("read feed %q response: %w", feedURL, err)
	}

	var document atomFeedDocument
	if err := xml.Unmarshal(content, &document); err != nil {
		return nil, fmt.Errorf("parse Atom feed %q: %w", feedURL, err)
	}
	if document.XMLName.Local != "feed" || document.XMLName.Space != atomNamespace {
		return nil, fmt.Errorf("parse Atom feed %q: root element must be {"+atomNamespace+"}feed, got {%s}%s", feedURL, document.XMLName.Space, document.XMLName.Local)
	}

	atomFeed := &AtomFeed{
		Title:   document.Title.text(),
		Updated: document.Updated,
		Link:    alternateLink(document.Links),
		Entries: make([]AtomEntry, 0, len(document.Entries)),
	}
	for _, entry := range document.Entries {
		content := ""
		if entry.Content != nil {
			content = entry.Content.text()
		} else if entry.Summary != nil {
			content = entry.Summary.text()
		}
		atomFeed.Entries = append(atomFeed.Entries, AtomEntry{
			Title:     entry.Title.text(),
			Link:      alternateLink(entry.Links),
			Content:   content,
			Published: entry.Published,
		})
	}
	return atomFeed, nil
}

func (f AtomFeed) String() string {
	return fmt.Sprintf("Title: %v\nUpdated: %v\nLink: %v\nEntries: %v\n", f.Title, f.Updated, f.Link, f.Entries)
}

func (e AtomEntry) String() string {
	return fmt.Sprintf("[\nTitle: %v\nLink: %v\nContent: %v\nPublished: %v\n]", e.Title, e.Link, e.Content, e.Published)
}
