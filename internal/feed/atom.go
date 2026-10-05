package feed

import (
	"encoding/xml"
	"fmt"
)

const atomNamespace = "http://www.w3.org/2005/Atom"

type atomFeedDocument struct {
	XMLName xml.Name
	Title   atomTextConstruct   `xml:"http://www.w3.org/2005/Atom title"`
	Updated string              `xml:"http://www.w3.org/2005/Atom updated"`
	Links   []atomLink          `xml:"http://www.w3.org/2005/Atom link"`
	Entries []atomEntryDocument `xml:"http://www.w3.org/2005/Atom entry"`
}

type atomEntryDocument struct {
	ID        string             `xml:"http://www.w3.org/2005/Atom id"`
	Title     atomTextConstruct  `xml:"http://www.w3.org/2005/Atom title"`
	Links     []atomLink         `xml:"http://www.w3.org/2005/Atom link"`
	Content   *atomTextConstruct `xml:"http://www.w3.org/2005/Atom content"`
	Summary   *atomTextConstruct `xml:"http://www.w3.org/2005/Atom summary"`
	Published string             `xml:"http://www.w3.org/2005/Atom published"`
	Updated   string             `xml:"http://www.w3.org/2005/Atom updated"`
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

func parseAtomFeed(content []byte) (*Feed, error) {
	var document atomFeedDocument
	if err := xml.Unmarshal(content, &document); err != nil {
		return nil, fmt.Errorf("invalid Atom XML: %w", err)
	}
	if document.XMLName.Local != "feed" || document.XMLName.Space != atomNamespace {
		return nil, fmt.Errorf("Atom root must be {%s}feed, got {%s}%s", atomNamespace, document.XMLName.Space, document.XMLName.Local)
	}

	result := &Feed{
		Title:   document.Title.text(),
		Link:    alternateLink(document.Links),
		Updated: atomDate(document.Updated),
		Entries: make([]Entry, 0, len(document.Entries)),
	}
	for _, source := range document.Entries {
		construct := source.Content
		if construct == nil {
			construct = source.Summary
		}
		entry := Entry{
			ID:        source.ID,
			Title:     source.Title.text(),
			Published: atomDate(source.Published),
			Updated:   atomDate(source.Updated),
		}
		entry.Link = alternateLink(source.Links)
		if construct != nil {
			entry.Content = construct.text()
			entry.ContentKind = atomContentKind(construct.Type)
		}
		result.Entries = append(result.Entries, entry)
	}
	return result, nil
}

func (c atomTextConstruct) text() string {
	if c.Type == "xhtml" {
		return c.Inner
	}
	return c.Value
}

func atomContentKind(kind string) ContentKind {
	switch kind {
	case "", "text":
		return ContentText
	case "html":
		return ContentHTML
	case "xhtml":
		return ContentXHTML
	default:
		return ContentKind(kind)
	}
}

func alternateLink(links []atomLink) string {
	for _, link := range links {
		if link.Href != "" && (link.Rel == "" || link.Rel == "alternate") {
			return link.Href
		}
	}
	return ""
}
