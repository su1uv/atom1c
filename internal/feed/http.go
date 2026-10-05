package feed

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var feedHTTPClient = &http.Client{Timeout: 15 * time.Second}

const maxFeedResponseBytes = 10 * 1024 * 1024

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

	content, err := io.ReadAll(io.LimitReader(res.Body, maxFeedResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read feed %q response: %w", feedURL, err)
	}
	if len(content) > maxFeedResponseBytes {
		return nil, fmt.Errorf("read feed %q response: decoded body exceeds %d-byte (10 MiB) limit", feedURL, maxFeedResponseBytes)
	}
	parsed, err := parseFeed(content)
	if err != nil {
		return nil, fmt.Errorf("parse feed %q: %w", feedURL, err)
	}
	return parsed, nil
}

func parseFeed(content []byte) (*Feed, error) {
	root, err := validateFeedDocument(content)
	if err != nil {
		return nil, fmt.Errorf("invalid XML: %w", err)
	}

	switch {
	case root.Name.Local == "feed" && root.Name.Space == atomNamespace:
		return parseAtomFeed(content)
	case root.Name.Local == "rss" && root.Name.Space == "":
		var version string
		for _, attr := range root.Attr {
			if attr.Name == (xml.Name{Local: "version"}) {
				version = attr.Value
			}
		}
		if version != "2.0" {
			return nil, fmt.Errorf("unsupported RSS version %q; only RSS 2.0 is supported", version)
		}
		return parseRSSFeed(content)
	default:
		return nil, fmt.Errorf("unsupported feed root {%s}%s", root.Name.Space, root.Name.Local)
	}
}

// Decode through EOF: xml.Unmarshal alone ignores everything after its root.
func validateFeedDocument(content []byte) (xml.StartElement, error) {
	d := xml.NewDecoder(bytes.NewReader(content))
	var root xml.StartElement
	for {
		token, err := d.Token()
		if err == io.EOF {
			if root.Name.Local == "" {
				return root, fmt.Errorf("document has no root element")
			}
			return root, nil
		}
		if err != nil {
			return root, err
		}
		switch token := token.(type) {
		case xml.StartElement:
			if root.Name.Local != "" {
				return root, fmt.Errorf("document has multiple root elements")
			}
			root = token
			if err := d.Skip(); err != nil {
				return root, err
			}
		case xml.CharData:
			if strings.Trim(string(token), " \t\r\n") != "" {
				return root, fmt.Errorf("non-whitespace text outside root element")
			}
		}
	}
}
