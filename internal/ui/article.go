package ui

import "github.com/su1uv/atom1c/internal/database"

// articleSnapshot owns value copies so a post reload cannot change an open reader.
type articleSnapshot struct {
	post   database.Post
	source string
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
