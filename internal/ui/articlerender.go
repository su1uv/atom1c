package ui

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/net/html"
)

// renderArticle hides content-kind handling, HTML parsing, metadata formatting,
// styling and terminal-cell wrapping behind one deterministic interface.
func renderArticle(article articleSnapshot, width int) string {
	return wrapArticleDocument(renderArticleDocument(article), width)
}

func renderArticleDocument(article articleSnapshot) string {
	p := article.post
	body := ""
	switch p.ContentKind {
	case "text":
		body = terminalText(p.Content)
	case "html", "xhtml":
		root, err := parseArticleHTML(p.Content, p.ContentKind)
		if err != nil {
			body = "Could not render feed-provided content"
		} else {
			var out htmlText
			out.children(root, false, 0)
			body = strings.Trim(out.String(), "\n")
			if out.Len() >= maxRenderedBytes {
				body += "\n[Content truncated for terminal rendering]"
			}
		}
	default:
		if strings.TrimSpace(p.Content) != "" {
			body = "Unsupported content type: " + terminalText(p.ContentKind)
		}
	}
	if strings.TrimSpace(ansi.Strip(body)) == "" {
		body = "No feed-provided content"
	}
	title := styleLines(lipgloss.NewStyle().Bold(true), terminalText(p.Title))
	return title + "\nSource: " + terminalText(article.source) + "\nDate: " + articleDate(article) + "\nLink: " + terminalText(p.Link) + "\n\n" + body
}

func wrapArticleDocument(doc string, width int) string {
	width = max(width, 1)
	lines := strings.Split(ansi.Wrap(doc, width, ""), "\n")
	for i, line := range lines {
		// Wrap cannot split a grapheme wider than the entire terminal.
		lines[i] = ansi.Truncate(line, width, "")
	}
	return strings.Join(lines, "\n")
}

func parseArticleHTML(content, kind string) (*html.Node, error) {
	if kind != "xhtml" {
		return html.ParseWithOptions(strings.NewReader(content), html.ParseOptionEnableScripting(false))
	}
	// Atom XHTML preserves inner XML. RawToken keeps namespace-local names even
	// when a prefix declaration was on the enclosing Atom content element.
	root := &html.Node{Type: html.DocumentNode}
	stack := []*html.Node{root}
	decoder := xml.NewDecoder(strings.NewReader(content))
	for {
		token, err := decoder.RawToken()
		if err == io.EOF {
			if len(stack) != 1 {
				return nil, fmt.Errorf("unbalanced XHTML")
			}
			return root, nil
		}
		if err != nil {
			return nil, err
		}
		parent := stack[len(stack)-1]
		switch token := token.(type) {
		case xml.StartElement:
			n := &html.Node{Type: html.ElementNode, Data: token.Name.Local}
			for _, attr := range token.Attr {
				n.Attr = append(n.Attr, html.Attribute{Key: attr.Name.Local, Val: attr.Value})
			}
			parent.AppendChild(n)
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) <= 1 || parent.Data != token.Name.Local {
				return nil, fmt.Errorf("unbalanced XHTML")
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			parent.AppendChild(&html.Node{Type: html.TextNode, Data: string(token)})
		}
	}
}

// Styling individual lines avoids Lipgloss's rectangular multiline padding.
func styleLines(style lipgloss.Style, text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = style.Render(line)
	}
	return strings.Join(lines, "\n")
}

func articleDate(article articleSnapshot) string {
	p := article.post
	if p.PublishedAt.Valid {
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05"} {
			if date, err := time.Parse(layout, p.PublishedAt.String); err == nil {
				return date.UTC().Format("2006-01-02 15:04:05 UTC")
			}
		}
	}
	if raw := strings.TrimSpace(terminalText(p.PublishedRaw)); raw != "" {
		return raw
	}
	return "Unknown"
}

// Feed text is data, not terminal commands. Only renderer-generated ANSI remains.
func terminalText(s string) string {
	s = ansi.Strip(s)
	return strings.Map(func(r rune) rune {
		if r == '\f' {
			return ' '
		}
		if r == '\n' || r == '\t' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.ReplaceAll(s, "\t", "    "))
}

const maxRenderedBytes = 20 << 20

type htmlText struct {
	strings.Builder
	pendingSpace bool
	leadingSpace bool
}

func (out *htmlText) WriteString(s string) (int, error) {
	if out.Len()+len(s) > maxRenderedBytes {
		s = s[:max(maxRenderedBytes-out.Len(), 0)]
	}
	return out.Builder.WriteString(s)
}

func (out *htmlText) WriteByte(b byte) error {
	if out.Len() >= maxRenderedBytes {
		return nil
	}
	return out.Builder.WriteByte(b)
}

func (out *htmlText) gap(n int) {
	out.pendingSpace = false
	s := out.String()
	if s == "" {
		return
	}
	trailing := len(s) - len(strings.TrimRight(s, "\n"))
	if trailing < n {
		out.WriteString(strings.Repeat("\n", n-trailing))
	}
}

func (out *htmlText) children(n *html.Node, pre bool, depth int) {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		out.node(child, pre, depth)
	}
}

func (out *htmlText) node(n *html.Node, pre bool, depth int) {
	if depth >= 64 {
		// Flatten unusually deep markup without recursive styling/copy growth.
		for current := n; current != nil; {
			if current.Type == html.TextNode {
				out.WriteString(terminalText(current.Data))
			}
			if current.FirstChild != nil {
				current = current.FirstChild
				continue
			}
			for current != n && current.NextSibling == nil {
				current = current.Parent
			}
			if current == n {
				break
			}
			current = current.NextSibling
		}
		return
	}
	if n.Type == html.TextNode {
		text := terminalText(n.Data)
		if !pre {
			for _, r := range text {
				if r == ' ' || r == '\n' || r == '\t' || r == '\r' || r == '\f' {
					out.pendingSpace = true
					continue
				}
				if out.pendingSpace && out.Len() > 0 && !strings.HasSuffix(out.String(), "\n") {
					out.WriteByte(' ')
				}
				if out.pendingSpace && out.Len() == 0 {
					out.leadingSpace = true
				}
				out.pendingSpace = false
				out.WriteString(string(r))
			}
			return
		}
		out.WriteString(text)
		return
	}
	if n.Type != html.ElementNode {
		out.children(n, pre, depth)
		return
	}
	switch n.Data {
	case "script", "style", "head", "template":
		return
	case "br":
		out.WriteByte('\n')
	case "hr":
		out.gap(2)
		out.WriteString("────────")
		out.gap(2)
	case "img":
		label := attribute(n, "alt")
		if label == "" {
			label = attribute(n, "title")
		}
		if label == "" {
			label = "Image"
		}
		if out.pendingSpace && out.Len() > 0 && !strings.HasSuffix(out.String(), "\n") {
			out.WriteByte(' ')
		}
		out.pendingSpace = false
		out.WriteString("[Image: " + label + "]")
	case "a":
		out.children(n, pre, depth)
		if link := attribute(n, "href"); link != "" {
			out.WriteString(" (" + link + ")")
		}
	case "ul", "ol":
		out.gap(2)
		index := 0
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			if child.Type != html.ElementNode || child.Data != "li" {
				continue
			}
			index++
			out.gap(1)
			prefix := "• "
			if n.Data == "ol" {
				prefix = fmt.Sprintf("%d. ", index)
			}
			var entry htmlText
			entry.children(child, pre, depth+1)
			lines := strings.Split(strings.TrimSpace(entry.String()), "\n")
			out.WriteString(prefix + lines[0])
			for _, line := range lines[1:] {
				out.WriteString("\n  " + line)
			}
		}
		out.gap(2)
	case "blockquote":
		if depth >= 8 {
			out.children(n, pre, depth+1)
			return
		}
		var quoted htmlText
		quoted.children(n, pre, depth+1)
		out.gap(2)
		for i, line := range strings.Split(strings.TrimSpace(quoted.String()), "\n") {
			if i > 0 {
				out.WriteByte('\n')
			}
			out.WriteString("> " + line)
		}
		out.gap(2)
	case "pre":
		var code htmlText
		code.children(n, true, depth+1)
		out.gap(2)
		out.WriteString(styleLines(lipgloss.NewStyle().Foreground(lipgloss.Color("#A8BFA0")), strings.Trim(code.String(), "\n")))
		out.gap(2)
	case "h1", "h2", "h3", "h4", "h5", "h6", "strong", "b", "em", "i", "code":
		var text htmlText
		text.children(n, pre, depth+1)
		style := lipgloss.NewStyle()
		block := strings.HasPrefix(n.Data, "h")
		if block {
			out.gap(2)
		}
		if block || n.Data == "strong" || n.Data == "b" {
			style = style.Bold(true)
		}
		if n.Data == "em" || n.Data == "i" {
			style = style.Italic(true)
		}
		if n.Data == "code" {
			style = style.Foreground(lipgloss.Color("#A8BFA0"))
		}
		value := text.String()
		if !pre && !block && (out.pendingSpace || text.leadingSpace) && value != "" && out.Len() > 0 && !strings.HasSuffix(out.String(), "\n") {
			out.WriteByte(' ')
		}
		if text.leadingSpace && out.Len() == 0 {
			out.leadingSpace = true
		}
		out.WriteString(styleLines(style, value))
		out.pendingSpace = text.pendingSpace
		if block {
			out.gap(2)
		}
	case "p", "div", "section", "article", "header", "footer", "figure", "figcaption", "table", "tr":
		out.gap(2)
		out.children(n, pre, depth+1)
		out.gap(2)
	case "td", "th":
		out.children(n, pre, depth+1)
		out.WriteString(" | ")
	default:
		out.children(n, pre, depth+1)
	}
}

func attribute(n *html.Node, name string) string {
	for _, attr := range n.Attr {
		if attr.Key == name {
			return terminalText(attr.Val)
		}
	}
	return ""
}
