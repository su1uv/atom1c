package feed

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"
)

var feedHTTPClient = &http.Client{Timeout: 15 * time.Second}

func fetchWithClient(ctx context.Context, feedURL string, client *http.Client) (*Feed, error) {
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
	parsed, err := parseFeed(content)
	if err != nil {
		return nil, fmt.Errorf("parse feed %q: %w", feedURL, err)
	}
	return parsed, nil
}

func parseFeed(content []byte) (*Feed, error) {
	var root struct {
		XMLName xml.Name
		Version string `xml:"version,attr"`
	}
	if err := xml.Unmarshal(content, &root); err != nil {
		return nil, fmt.Errorf("invalid XML: %w", err)
	}

	switch {
	case root.XMLName.Local == "feed" && root.XMLName.Space == atomNamespace:
		return parseAtomFeed(content)
	case root.XMLName.Local == "rss" && root.XMLName.Space == "":
		if root.Version != "2.0" {
			return nil, fmt.Errorf("unsupported RSS version %q; only RSS 2.0 is supported", root.Version)
		}
		return parseRSSFeed(content)
	default:
		return nil, fmt.Errorf("unsupported feed root {%s}%s", root.XMLName.Space, root.XMLName.Local)
	}
}
