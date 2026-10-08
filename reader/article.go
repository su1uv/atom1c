// Package reader retrieves readable public web articles and presents their
// content as Markdown and styled terminal text. It has no Atom1c dependencies.
package reader

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"
	"unicode"

	markdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"golang.org/x/net/html"
)

// Article is the portable document returned by extraction and consumed by
// terminal views or other applications.
type Article struct {
	URL       string
	Title     string
	Author    string
	SiteName  string
	Published *time.Time
	Markdown  string
}

// MaxMarkdownBytes bounds converted content and terminal rendering work.
const MaxMarkdownBytes = 20 << 20

// FeedContentMarkdown converts persisted feed content to safe Markdown for the
// terminal reader. Plain-text content is escaped so feed text cannot become
// headings, links, or formatting by accident.
func FeedContentMarkdown(content, kind, baseURL string) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", nil
	}
	switch kind {
	case "text":
		converted := escapeMarkdownText(content)
		if len(converted) > MaxMarkdownBytes {
			return "", fmt.Errorf("feed content exceeds %d-byte Markdown limit", MaxMarkdownBytes)
		}
		return converted, nil
	case "html", "xhtml":
		if kind == "xhtml" {
			if normalized, err := normalizeXHTML(content); err == nil {
				content = normalized
			}
		}
		content, err := sanitizeHTML(content, baseURL)
		if err != nil {
			return "", fmt.Errorf("sanitize feed HTML: %w", err)
		}
		options := make([]converter.ConvertOptionFunc, 0, 1)
		if parsed, err := url.Parse(baseURL); err == nil && parsed.IsAbs() {
			options = append(options, converter.WithDomain(parsed.String()))
		}
		converted, err := markdown.ConvertString(content, options...)
		if err != nil {
			return "", fmt.Errorf("convert feed HTML to Markdown: %w", err)
		}
		if len(converted) > MaxMarkdownBytes {
			return "", fmt.Errorf("feed content exceeds %d-byte Markdown limit", MaxMarkdownBytes)
		}
		return strings.TrimSpace(converted), nil
	default:
		return "", fmt.Errorf("unsupported feed content type %q", kind)
	}
}

func normalizeXHTML(content string) (string, error) {
	root := &html.Node{Type: html.DocumentNode}
	stack := []*html.Node{root}
	decoder := xml.NewDecoder(strings.NewReader(content))
	for {
		token, err := decoder.RawToken()
		if err == io.EOF {
			if len(stack) != 1 {
				return "", fmt.Errorf("unbalanced XHTML")
			}
			var out strings.Builder
			if err := html.Render(&out, root); err != nil {
				return "", err
			}
			return out.String(), nil
		}
		if err != nil {
			return "", err
		}
		parent := stack[len(stack)-1]
		switch token := token.(type) {
		case xml.StartElement:
			node := &html.Node{Type: html.ElementNode, Data: localXMLName(token.Name.Local)}
			for _, attr := range token.Attr {
				node.Attr = append(node.Attr, html.Attribute{Key: localXMLName(attr.Name.Local), Val: attr.Value})
			}
			parent.AppendChild(node)
			stack = append(stack, node)
		case xml.EndElement:
			if len(stack) <= 1 || parent.Data != localXMLName(token.Name.Local) {
				return "", fmt.Errorf("unbalanced XHTML")
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			parent.AppendChild(&html.Node{Type: html.TextNode, Data: string(token)})
		}
	}
}

func localXMLName(name string) string {
	if prefix := strings.LastIndexByte(name, ':'); prefix >= 0 {
		return name[prefix+1:]
	}
	return name
}

// SafeLink resolves a link against baseURL and returns it only when it targets
// HTTP(S). It is suitable for terminal hyperlinks originating in feed content.
func SafeLink(raw, baseURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.User != nil {
		return ""
	}
	if baseURL != "" {
		base, err := url.Parse(baseURL)
		if err != nil {
			return ""
		}
		parsed = base.ResolveReference(parsed)
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return ""
	}
	return parsed.String()
}

func sanitizeHTML(input, baseURL string) (string, error) {
	root, err := html.Parse(strings.NewReader(input))
	if err != nil {
		return "", err
	}
	stack := []*html.Node{root}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node.Type == html.ElementNode {
			for i := 0; i < len(node.Attr); {
				attr := &node.Attr[i]
				if strings.HasPrefix(strings.ToLower(attr.Key), "on") {
					node.Attr = append(node.Attr[:i], node.Attr[i+1:]...)
					continue
				}
				if (node.Data == "a" && attr.Key == "href") || (node.Data == "img" && attr.Key == "src") {
					if safe := SafeLink(attr.Val, baseURL); safe != "" {
						attr.Val = safe
					} else {
						node.Attr = append(node.Attr[:i], node.Attr[i+1:]...)
						continue
					}
				}
				i++
			}
		}
		for child := node.LastChild; child != nil; child = child.PrevSibling {
			stack = append(stack, child)
		}
	}
	var out strings.Builder
	if err := html.Render(&out, root); err != nil {
		return "", err
	}
	return out.String(), nil
}

func sanitizeArticleNode(node *html.Node) {
	stack := []*html.Node{node}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if current.Type == html.ElementNode {
			for i := 0; i < len(current.Attr); {
				attr := &current.Attr[i]
				if strings.HasPrefix(strings.ToLower(attr.Key), "on") {
					current.Attr = append(current.Attr[:i], current.Attr[i+1:]...)
					continue
				}
				if (current.Data == "a" && attr.Key == "href") || (current.Data == "img" && attr.Key == "src") {
					if safe := SafeLink(attr.Val, ""); safe != "" {
						attr.Val = safe
					} else {
						current.Attr = append(current.Attr[:i], current.Attr[i+1:]...)
						continue
					}
				}
				i++
			}
		}
		for child := current.LastChild; child != nil; child = child.PrevSibling {
			stack = append(stack, child)
		}
	}
}

func cleanMarkdown(text string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, text)
}

func escapeMarkdownText(content string) string {
	var out strings.Builder
	for _, r := range content {
		if strings.ContainsRune("\\`*_{}[]<>()#+-.!|", r) {
			out.WriteRune('\\')
		}
		out.WriteRune(r)
	}
	return out.String()
}
