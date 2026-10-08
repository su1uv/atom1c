package ui

import (
	"time"

	"github.com/su1uv/atom1c/internal/database"
	"github.com/su1uv/atom1c/reader"
)

// articleSnapshot owns value copies so a post reload cannot change an open reader.
type articleSnapshot struct {
	post     database.Post
	source   string
	full     *reader.Article
	showFull bool
}

func cachedArticle(cache database.ArticleCache) reader.Article {
	article := reader.Article{URL: cache.FinalUrl, Title: cache.Title, Author: cache.Author, SiteName: cache.SiteName, Markdown: cache.Markdown}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05"} {
		if published, err := time.Parse(layout, cache.PublishedAt); err == nil {
			published = published.UTC()
			article.Published = &published
			break
		}
	}
	return article
}

func cacheParams(postID int64, sourceURL string, article reader.Article) database.UpsertArticleCacheParams {
	published := ""
	if article.Published != nil {
		published = article.Published.UTC().Format(time.RFC3339)
	}
	return database.UpsertArticleCacheParams{FinalUrl: article.URL, Markdown: article.Markdown, Title: article.Title, Author: article.Author, SiteName: article.SiteName, PublishedAt: published, ID: postID, Link: sourceURL}
}

func (m model) selectedArticle() (articleSnapshot, bool) {
	selected, ok := m.posts.list.SelectedItem().(item)
	if !ok {
		return articleSnapshot{}, false
	}
	post, ok := m.postRecords[selected.id]
	if !ok || post.FeedID != m.openFeedID {
		return articleSnapshot{}, false
	}
	return articleSnapshot{post: post, source: m.openFeedName}, true
}
