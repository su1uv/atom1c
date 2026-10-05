package feed

import (
	"context"
	"time"
)

// ContentKind describes how an entry's feed-provided content should be treated.
type ContentKind string

const (
	ContentText  ContentKind = "text"
	ContentHTML  ContentKind = "html"
	ContentXHTML ContentKind = "xhtml"
)

// SourceDate retains the date text supplied by a feed and its parsed UTC value,
// when the source text uses a supported date format.
type SourceDate struct {
	Raw        string
	Normalized *time.Time
}

// Feed is a normalized Atom or RSS 2.0 feed.
type Feed struct {
	Title   string
	Link    string
	Updated SourceDate
	Entries []Entry
}

// Entry is a normalized Atom entry or RSS item.
type Entry struct {
	ID              string
	GUIDIsPermaLink *bool
	Title           string
	Link            string
	Content         string
	ContentKind     ContentKind
	Published       SourceDate
	Updated         SourceDate
}

// Fetch retrieves and parses an Atom or RSS 2.0 feed.
func Fetch(ctx context.Context, feedURL string) (*Feed, error) {
	return fetchWithClient(ctx, feedURL, feedHTTPClient)
}
