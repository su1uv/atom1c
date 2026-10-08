package ui

import (
	"database/sql"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/su1uv/atom1c/internal/database"
)

func TestRenderArticleContent(t *testing.T) {
	for _, tt := range []struct {
		name, kind, content string
		want, absent        []string
	}{
		{"literal text", "text", "<p>literal</p> &amp;\n  code", []string{"<p>literal</p> &amp;", "  code"}, nil},
		{"html", "html", `<h1>Heading</h1><p>A &amp; B <strong>bold</strong></p><ul><li>One</li><li>Two</li></ul><blockquote>Quote</blockquote><pre>  x\n  y</pre><a href="https://example.test">site</a><img alt="Diagram" src="img.png"><script>bad()</script><style>badcss</style>`, []string{"Heading", "A & B bold", "• One", "• Two", "Quote", "  x", "site (https://example.test)", "[Image: Diagram]"}, []string{"<h1>", "bad()", "badcss"}},
		{"xhtml", "xhtml", `<div xmlns="http://www.w3.org/1999/xhtml"><p>XHTML<br/>second</p></div>`, []string{"XHTML\nsecond"}, []string{"xmlns"}},
		{"xhtml cdata", "xhtml", `<div xmlns="http://www.w3.org/1999/xhtml"><p><![CDATA[A < B]]></p></div>`, []string{"A < B"}, nil},
		{"blank styled", "html", `<p><strong> </strong><em> </em><code> </code></p>`, []string{"No feed-provided content"}, nil},
		{"noscript", "html", `<noscript><p>Readable fallback</p></noscript>`, []string{"Readable fallback"}, nil},
		{"malformed", "html", "<p>first<p>second &copy;", []string{"first", "second ©"}, nil},
		{"ordered", "html", "<ol><li>First</li><li>Second</li></ol>", []string{"1. First", "2. Second"}, nil},
		{"empty", "text", " \n\t", []string{"No feed-provided content"}, nil},
		{"markup empty", "html", "<p> </p>", []string{"No feed-provided content"}, nil},
		{"unsupported", "application/pdf", "raw payload", []string{"Unsupported content type: application/pdf"}, []string{"raw payload"}},
		{"terminal controls", "text", "hello\x1b[2J\x1b]52;c;evil\a\rworld", []string{"hello"}, []string{"\x1b", "\a", "\r"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			doc := renderArticle(articleSnapshot{post: database.Post{ContentKind: tt.kind, Content: tt.content}}, 100)
			plain := ansi.Strip(doc)
			for _, want := range tt.want {
				if !strings.Contains(plain, want) {
					t.Errorf("missing %q in %q", want, plain)
				}
			}
			for _, absent := range tt.absent {
				if strings.Contains(plain, absent) {
					t.Errorf("unexpected %q in %q", absent, plain)
				}
			}
		})
	}
}

func TestRenderHTMLStructureAndLinearSize(t *testing.T) {
	for _, tt := range []struct{ content, want string }{
		{"<p>A <em> B </em> C</p>", "A B C"},
		{"<p>A</p>\n<p>B</p>", "A\n\nB"},
		{"<ul><li><p>One</p><p>More</p></li><li>Two</li></ul>", "• One\n  \n  More\n• Two"},
		{"<ul><li>One<ul><li>Nested</li></ul></li></ul>", "• One\n  \n  • Nested"},
	} {
		root, err := parseArticleHTML(tt.content, "html")
		if err != nil {
			t.Fatal(err)
		}
		var out htmlText
		out.children(root, false, 0)
		if got := strings.TrimSpace(ansi.Strip(out.String())); got != tt.want {
			t.Errorf("%s: got %q want %q", tt.content, got, tt.want)
		}
	}
	for _, content := range []string{
		"<pre>" + strings.Repeat("x", 4096) + "\n" + strings.Repeat("y\n", 2048) + "</pre>",
		strings.Repeat("<blockquote>", 128) + strings.Repeat("line\n", 2048) + strings.Repeat("</blockquote>", 128),
	} {
		doc := renderArticle(articleSnapshot{post: database.Post{ContentKind: "html", Content: content}}, 80)
		if len(doc) > 20*len(content) {
			t.Fatalf("rendering grew from %d to %d bytes", len(content), len(doc))
		}
	}
}

func TestRenderArticleMetadataAndWidth(t *testing.T) {
	for _, tt := range []struct {
		name      string
		published sql.NullString
		raw, want string
	}{
		{"utc", sql.NullString{String: "2026-10-08T12:30:00+02:00", Valid: true}, "original", "2026-10-08 10:30:00 UTC"},
		{"database", sql.NullString{String: "2026-10-08 12:30:00", Valid: true}, "original", "2026-10-08 12:30:00 UTC"},
		{"raw", sql.NullString{}, "unknown format", "unknown format"},
		{"unknown", sql.NullString{}, "", "Unknown"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			doc := renderArticle(articleSnapshot{source: "Feed", post: database.Post{Title: "Title", Link: "https://example.test", PublishedAt: tt.published, PublishedRaw: tt.raw, ContentKind: "text", Content: "Body"}}, 80)
			for _, want := range []string{"Title", "Source: Feed", "Date: " + tt.want, "Link: https://example.test", "Body"} {
				if !strings.Contains(ansi.Strip(doc), want) {
					t.Fatalf("missing %q in %q", want, doc)
				}
			}
		})
	}
	for _, width := range []int{1, 7, 20, 80} {
		doc := renderArticle(articleSnapshot{source: strings.Repeat("源", 40), post: database.Post{Title: strings.Repeat("Title", 40), Link: strings.Repeat("path", 40), ContentKind: "html", Content: "<pre>" + strings.Repeat("界", 80) + "</pre><p>" + strings.Repeat("abcdef", 80) + "</p>"}}, width)
		// A two-cell grapheme cannot fit into a one-cell terminal; it must be omitted.
		if got := lipgloss.Width(doc); got > width {
			t.Fatalf("width %d rendered %d", width, got)
		}
	}
}
