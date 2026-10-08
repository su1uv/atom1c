package ui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/su1uv/atom1c/internal/database"
)

func TestRepeatedPostPageKeysStayInsideTerminal(t *testing.T) {
	for _, width := range []int{60, 79, 80, 100, 120} {
		t.Run(fmt.Sprintf("width_%d", width), func(t *testing.T) {
			const height = 24
			m := updateModel(testModel(), tea.WindowSizeMsg{Width: width, Height: height})
			m.focus = focusPosts
			posts := make([]database.Post, 100)
			for i := range posts {
				posts[i] = database.Post{ID: int64(i + 1), Title: strings.Repeat("Long title ", 10), Link: "https://example.test/" + strings.Repeat("path/", 20)}
			}
			m = updateModel(m, postPageResult{request: m.postRequest, feedID: m.openFeedID, posts: posts})
			for step := 0; step < 100; step++ {
				view := m.View().Content
				if got := lipgloss.Width(view); got > width {
					t.Fatalf("page key %d: rendered width=%d exceeds terminal=%d (panes=%d help=%d)", step, got, width,
						lipgloss.Width(m.feedView())+lipgloss.Width(m.postsView()), m.help.Width())
				}
				if got := lipgloss.Height(view); got > height {
					t.Fatalf("page key %d: rendered height=%d exceeds terminal=%d", step, got, height)
				}
				if got, want := lipgloss.Height(m.postsView()), lipgloss.Height(m.feedView()); got != want {
					t.Fatalf("page key %d: post height=%d, feed height=%d", step, got, want)
				}
				if step < 50 {
					m = updateModel(m, press("right", tea.KeyRight))
				} else {
					m = updateModel(m, press("left", tea.KeyLeft))
				}
			}
		})
	}
}

func TestFeedContainerDoesNotWrapLongListItems(t *testing.T) {
	for _, width := range []int{60, 80, 100} {
		t.Run(fmt.Sprintf("width_%d", width), func(t *testing.T) {
			m := updateModel(testModel(), tea.WindowSizeMsg{Width: width, Height: 24})
			items := make([]list.Item, m.feedPageSize)
			for i := range items {
				items[i] = item{id: int64(i + 1), name: strings.Repeat("Long feed title ", 8), url: "https://example.test/" + strings.Repeat("path/", 20)}
			}
			_ = m.feeds.list.SetItems(items)
			if got, want := lipgloss.Height(m.feeds.View().Content), m.feeds.list.Height()+m.feeds.style.GetVerticalFrameSize(); got != want {
				t.Fatalf("feed container grew beyond its size: height=%d, allocated=%d", got, want)
			}
		})
	}
}
