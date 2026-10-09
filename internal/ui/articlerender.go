package ui

import (
	"strings"
	"time"

	"github.com/su1uv/atom1c/reader"
)

func renderArticle(article articleSnapshot, width int) string {
	content, err := reader.RenderMarkdown(buildArticleMarkdown(article), width, true)
	if err != nil {
		return "Could not render article: " + err.Error()
	}
	return content
}

func renderArticleDocument(article articleSnapshot) string {
	return buildArticleMarkdown(article)
}

func buildArticleMarkdown(article articleSnapshot) string {
	title := article.post.Title
	body := ""
	metadata := make([]string, 0, 5)
	if article.showFull && article.full != nil {
		if article.full.Title != "" {
			title = article.full.Title
		}
		if article.full.SiteName != "" {
			metadata = append(metadata, "**Site:** "+plainMarkdown(article.full.SiteName))
		}
		if article.full.Author != "" {
			metadata = append(metadata, "**Author:** "+plainMarkdown(article.full.Author))
		}
		if article.full.Published != nil {
			metadata = append(metadata, "**Published:** "+article.full.Published.UTC().Format("2006-01-02 15:04 UTC"))
		} else if date := articleDate(article); date != "Unknown" {
			metadata = append(metadata, "**Published:** "+plainMarkdown(date))
		}
		body = article.full.Markdown
	} else {
		var err error
		body, err = reader.FeedContentMarkdown(article.post.Content, article.post.ContentKind, article.post.Link)
		if err != nil {
			body = "Feed content could not be converted: " + plainMarkdown(err.Error())
		}
		if date := articleDate(article); date != "Unknown" {
			metadata = append(metadata, "**Published:** "+plainMarkdown(date))
		}
	}
	if strings.TrimSpace(body) == "" {
		body = "No feed-provided content"
	}
	if title == "" {
		title = "Untitled article"
	}
	if article.showFull && article.full != nil {
		body = removeLeadingTitle(body, title)
	}
	lines := []string{"# " + plainMarkdown(title), "", "**Source:** " + plainMarkdown(article.source)}
	for _, line := range metadata {
		lines = append(lines, "", line)
	}
	if link := reader.SafeLink(article.post.Link, ""); link != "" {
		lines = append(lines, "", "**Feed link:** [Open original post]("+link+")")
	}
	lines = append(lines, "", "---", "", strings.TrimSpace(body))
	return strings.Join(lines, "\n")
}

func removeLeadingTitle(markdown, title string) string {
	lines := strings.Split(markdown, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		trimmed := strings.TrimSpace(line)
		hashes := len(trimmed) - len(strings.TrimLeft(trimmed, "#"))
		if hashes < 1 || hashes > 6 {
			return markdown
		}
		heading := strings.TrimSpace(strings.Trim(trimmed[hashes:], "#"))
		if heading != title {
			return markdown
		}
		return strings.TrimSpace(strings.Join(append(lines[:i], lines[i+1:]...), "\n"))
	}
	return markdown
}

func plainMarkdown(value string) string {
	converted, err := reader.FeedContentMarkdown(value, "text", "")
	if err != nil {
		return ""
	}
	return strings.ReplaceAll(converted, "\n", " ")
}

func articleDate(article articleSnapshot) string {
	post := article.post
	if post.PublishedAt.Valid {
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05"} {
			if date, err := time.Parse(layout, post.PublishedAt.String); err == nil {
				return date.UTC().Format("2006-01-02 15:04:05 UTC")
			}
		}
	}
	if raw := strings.TrimSpace(post.PublishedRaw); raw != "" {
		return raw
	}
	return "Unknown"
}
