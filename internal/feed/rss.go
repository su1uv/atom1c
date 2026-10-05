package feed

import (
	"encoding/xml"
	"fmt"
	"strings"
)

type rssDocument struct {
	XMLName xml.Name
	Version string      `xml:"version,attr"`
	Channel *rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	PubDate       string    `xml:"pubDate"`
	LastBuildDate string    `xml:"lastBuildDate"`
	Items         []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	GUID        *rssGUID `xml:"guid"`
	Description *string  `xml:"description"`
	Encoded     *string  `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
	PubDate     string   `xml:"pubDate"`
}

type rssGUID struct {
	IsPermaLink *string `xml:"isPermaLink,attr"`
	Value       string  `xml:",chardata"`
}

func parseRSSFeed(content []byte) (*Feed, error) {
	var document rssDocument
	if err := xml.Unmarshal(content, &document); err != nil {
		return nil, fmt.Errorf("invalid RSS XML: %w", err)
	}
	if document.XMLName.Local != "rss" || document.XMLName.Space != "" || document.Version != "2.0" {
		return nil, fmt.Errorf("RSS root must be unnamespaced <rss version=\"2.0\">")
	}
	if document.Channel == nil {
		return nil, fmt.Errorf("RSS 2.0 document has no channel")
	}

	channel := document.Channel
	updatedRaw := channel.LastBuildDate
	if strings.TrimSpace(updatedRaw) == "" {
		updatedRaw = channel.PubDate
	}
	result := &Feed{
		Title:   strings.TrimSpace(channel.Title),
		Link:    strings.TrimSpace(channel.Link),
		Updated: rssDate(updatedRaw),
		Entries: make([]Entry, 0, len(channel.Items)),
	}
	for _, source := range channel.Items {
		entry := Entry{
			Title:       strings.TrimSpace(source.Title),
			Link:        strings.TrimSpace(source.Link),
			Published:   rssDate(source.PubDate),
			ContentKind: ContentHTML,
		}
		if source.GUID != nil {
			entry.ID = strings.TrimSpace(source.GUID.Value)
			permalink, err := rssGUIDIsPermalink(source.GUID.IsPermaLink)
			if err != nil {
				return nil, fmt.Errorf("RSS item %q: %w", entry.Title, err)
			}
			entry.GUIDIsPermaLink = &permalink
			if entry.Link == "" && permalink {
				entry.Link = entry.ID
			}
		}
		if source.Encoded != nil {
			entry.Content = *source.Encoded
		} else if source.Description != nil {
			entry.Content = *source.Description
		}
		result.Entries = append(result.Entries, entry)
	}
	return result, nil
}

func rssGUIDIsPermalink(value *string) (bool, error) {
	if value == nil {
		return true, nil
	}
	switch strings.TrimSpace(*value) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("invalid guid isPermaLink value %q", *value)
	}
}
