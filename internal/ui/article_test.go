package ui

import (
	"errors"
	"testing"

	"github.com/su1uv/atom1c/internal/database"
)

func TestArticleSnapshotTracksAcceptedRecordsOnly(t *testing.T) {
	m := testModel()
	m.openFeedID, m.openFeedName = 1, "Source"
	m.postRequest = 3
	post := database.Post{ID: 10, FeedID: 1, Title: "Title", Content: "Original", ContentKind: "text"}
	m = updateModel(m, postPageResult{request: 3, feedID: 1, posts: []database.Post{post}})
	snapshot, ok := m.selectedArticle()
	if !ok || snapshot.post != post || snapshot.source != "Source" {
		t.Fatalf("snapshot = %#v, %v", snapshot, ok)
	}
	for _, result := range []postPageResult{
		{request: 2, feedID: 1, posts: []database.Post{{ID: 10, Content: "Stale"}}},
		{request: 3, feedID: 2, posts: []database.Post{{ID: 10, Content: "Wrong feed"}}},
		{request: 3, feedID: 1, err: errors.New("read failed")},
	} {
		m = updateModel(m, result)
		got, ok := m.selectedArticle()
		if !ok || got != snapshot {
			t.Fatalf("rejected read replaced article: %#v, %v", got, ok)
		}
	}
	post.Content = "Updated"
	m = updateModel(m, postPageResult{request: 3, feedID: 1, posts: []database.Post{post}})
	got, _ := m.selectedArticle()
	if got.post.Content != "Updated" || snapshot.post.Content != "Original" {
		t.Fatalf("new article = %#v; original snapshot = %#v", got, snapshot)
	}
	m.openFeed(item{id: 2, name: "Other"})
	if got, ok := m.selectedArticle(); ok {
		t.Fatalf("article survived feed switch: %#v", got)
	}
}

func TestArticleSnapshotUsesSelectedIDAndRejectsEmptySelection(t *testing.T) {
	m := testModel()
	m.openFeedID = 1
	if _, ok := m.selectedArticle(); ok {
		t.Fatal("empty list produced article")
	}
	m = updateModel(m, postPageResult{feedID: 1, posts: []database.Post{
		{ID: 20, FeedID: 1, Content: "First"}, {ID: 10, FeedID: 1, Content: "Second"},
	}})
	m.posts.list.Select(1)
	got, ok := m.selectedArticle()
	if !ok || got.post.ID != 10 || got.post.Content != "Second" {
		t.Fatalf("selected article = %#v, %v", got, ok)
	}
	m = updateModel(m, postPageResult{feedID: 1})
	if _, ok := m.selectedArticle(); ok {
		t.Fatal("empty reload retained article")
	}
}
