package feed

import (
	"encoding/xml"
	"fmt"
	"strings"
)

type rssDocument struct {
	XMLName xml.Name
	Version string
	Channel *rssChannel
}

type rssChannel struct {
	Title         string
	Link          string
	PubDate       string
	LastBuildDate string
	Items         []rssItem
}

type rssItem struct {
	Title       string
	Link        string
	GUID        *rssGUID
	Description *string
	Encoded     *string
	PubDate     string
}

type rssGUID struct {
	IsPermaLink *string
	Value       string
}

// RSS core names are explicitly unnamespaced. encoding/xml's unqualified
// struct tags match any namespace, so dispatch children by their full XML name.
func decodeRSSChildren(d *xml.Decoder, fields map[xml.Name]any) error {
	for {
		token, err := d.Token()
		if err != nil {
			return err
		}
		switch token := token.(type) {
		case xml.StartElement:
			if field, ok := fields[token.Name]; ok {
				if err := d.DecodeElement(field, &token); err != nil {
					return err
				}
			} else if err := d.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			return nil
		}
	}
}

func (r *rssDocument) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	r.XMLName = start.Name
	for _, attr := range start.Attr {
		if attr.Name == (xml.Name{Local: "version"}) {
			r.Version = attr.Value
		}
	}
	return decodeRSSChildren(d, map[xml.Name]any{{Local: "channel"}: &r.Channel})
}

func (r *rssChannel) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return decodeRSSChildren(d, map[xml.Name]any{
		{Local: "title"}: &r.Title, {Local: "link"}: &r.Link,
		{Local: "pubDate"}: &r.PubDate, {Local: "lastBuildDate"}: &r.LastBuildDate,
		{Local: "item"}: &r.Items,
	})
}

func (r *rssItem) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return decodeRSSChildren(d, map[xml.Name]any{
		{Local: "title"}: &r.Title, {Local: "link"}: &r.Link,
		{Local: "guid"}: &r.GUID, {Local: "description"}: &r.Description,
		{Local: "pubDate"}: &r.PubDate,
		{Space: "http://purl.org/rss/1.0/modules/content/", Local: "encoded"}: &r.Encoded,
	})
}

func (r *rssGUID) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name == (xml.Name{Local: "isPermaLink"}) {
			value := attr.Value
			r.IsPermaLink = &value
		}
	}
	return d.DecodeElement(&r.Value, &start)
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
			Link:        source.Link,
			Published:   rssDate(source.PubDate),
			ContentKind: ContentHTML,
		}
		if source.GUID != nil {
			entry.ID = source.GUID.Value
			permalink, err := rssGUIDIsPermalink(source.GUID.IsPermaLink)
			if err != nil {
				return nil, fmt.Errorf("RSS item %q: %w", entry.Title, err)
			}
			entry.GUIDIsPermaLink = &permalink
			if strings.TrimSpace(entry.Link) == "" && permalink {
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
