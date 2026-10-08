package ui

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/su1uv/atom1c/internal/database"
	"github.com/su1uv/atom1c/reader"
)

func TestBuildArticleMarkdownUsesFeedPreviewAndMetadata(t *testing.T) {
	post := database.Post{Title: "Feed title", Link: "https://example.test/post", ContentKind: "html", Content: `<h2>Preview section</h2><ul><li>First</li><li>Second</li></ul>`, PublishedAt: sql.NullString{String: "2026-06-01T12:00:00Z", Valid: true}}
	article := articleSnapshot{post: post, source: "Journal"}
	doc := buildArticleMarkdown(article)
	for _, want := range []string{"# Feed title", "**Source:** Journal", "**Published:** 2026\\-06\\-01", "[Open original post](https://example.test/post)", "Preview section", "First", "Second"} {
		if !strings.Contains(doc, want) {
			t.Errorf("preview markdown missing %q: %s", want, doc)
		}
	}
	rendered, err := reader.RenderMarkdown(doc, 60, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Feed title", "Preview section", "First", "Second"} {
		if !strings.Contains(ansi.Strip(rendered), want) {
			t.Errorf("reader missing %q: %s", want, rendered)
		}
	}
}

func TestBuildArticleMarkdownPrefersWebsiteMetadataAndContent(t *testing.T) {
	published := time.Date(2026, 6, 1, 12, 0, 0, 0, time.FixedZone("offset", 3600))
	article := articleSnapshot{
		post:     database.Post{Title: "Feed headline", Link: "https://example.test/post", ContentKind: "text", Content: "Feed summary", PublishedAt: sql.NullString{String: "2026-05-31T00:00:00Z", Valid: true}},
		source:   "Journal",
		full:     &reader.Article{Title: "Website headline", Author: "Ada", SiteName: "Example", Published: &published, Markdown: "# Website body\n\n- Extracted point"},
		showFull: true,
	}
	doc := buildArticleMarkdown(article)
	for _, want := range []string{"# Website headline", "**Source:** Journal", "**Site:** Example", "**Author:** Ada", "2026-06-01 11:00 UTC", "Website body", "Extracted point", "Feed link", "Open original post"} {
		if !strings.Contains(doc, want) {
			t.Errorf("full article missing %q: %s", want, doc)
		}
	}
	if strings.Contains(doc, "Feed summary") || strings.Contains(doc, "Feed headline") {
		t.Fatalf("full article contains preview: %s", doc)
	}
	article.showFull = false
	doc = buildArticleMarkdown(article)
	if !strings.Contains(doc, "Feed headline") || !strings.Contains(doc, "Feed summary") || strings.Contains(doc, "Website body") {
		t.Fatalf("feed toggle markdown: %s", doc)
	}
}

func TestEmptyAndUnsupportedFeedContentHaveReaderFallback(t *testing.T) {
	for _, kind := range []string{"text", "html", "application/pdf"} {
		doc := buildArticleMarkdown(articleSnapshot{post: database.Post{Title: "Post", ContentKind: kind, Content: "  \n "}, source: "Feed"})
		if !strings.Contains(doc, "No feed-provided content") {
			t.Errorf("kind %q missing empty-content notice: %s", kind, doc)
		}
	}
}

func TestFullArticleOmitsDuplicateLeadingTitle(t *testing.T) {
	for _, test := range []struct{ markdown, want string }{
		{"# Title\n\nBody", "Body"},
		{"\n## Title ##\n\nBody", "Body"},
		{"# Different\n\nBody", "# Different\n\nBody"},
		{"Title\n\nBody", "Title\n\nBody"},
	} {
		if got := removeLeadingTitle(test.markdown, "Title"); got != test.want {
			t.Errorf("removeLeadingTitle(%q) = %q, want %q", test.markdown, got, test.want)
		}
	}
}

func TestArticleMetadataFallbackAndResponsiveWidth(t *testing.T) {
	post := database.Post{Title: strings.Repeat("A long heading ", 12), Link: "https://example.test/post", ContentKind: "text", Content: strings.Repeat("Readable article text with Unicode 界 and line wrapping. ", 100)}
	doc := renderArticle(articleSnapshot{post: post, source: "A source"}, 32)
	if width := lipgloss.Width(doc); width > 32 {
		t.Fatalf("reader width %d exceeds terminal width 32", width)
	}
	for _, tt := range []struct {
		published sql.NullString
		raw, want string
	}{
		{sql.NullString{String: "2026-06-01T12:00:00+02:00", Valid: true}, "raw", "2026-06-01 10:00:00 UTC"},
		{sql.NullString{}, "unparsed source date", "unparsed source date"},
		{sql.NullString{}, "", "Unknown"},
	} {
		if got := articleDate(articleSnapshot{post: database.Post{PublishedAt: tt.published, PublishedRaw: tt.raw}}); got != tt.want {
			t.Errorf("date %q, want %q", got, tt.want)
		}
	}
}
