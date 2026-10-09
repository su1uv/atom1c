package ui

import (
	"strings"
	"sync/atomic"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/su1uv/atom1c/internal/database"
)

func readerFixture() model {
	m := updateModel(testModel(), tea.WindowSizeMsg{Width: 80, Height: 20})
	m.focus = focusPosts
	posts := make([]database.Post, 30)
	for i := range posts {
		posts[i] = database.Post{ID: int64(i + 1), FeedID: 1, Title: "Article", Link: "https://example.test/article", ContentKind: "text", Content: strings.Repeat("Long body line\n", 100)}
	}
	return updateModel(m, postPageResult{request: m.postRequest, feedID: 1, posts: posts})
}

func TestReaderNavigationWhileReflowIsPending(t *testing.T) {
	for _, navigation := range []tea.KeyPressMsg{press("end", tea.KeyEnd), press("pgdown", tea.KeyPgDown), press("j", 'j')} {
		m := openReader(t, readerFixture())
		m, cmd := applyMessage(m, tea.WindowSizeMsg{Width: 40, Height: 20})
		m = updateModel(m, navigation)
		m = runUICommands(t, m, cmd)
		if m.reader.viewport.YOffset() == 0 {
			t.Fatalf("reflow undid %s", navigation.String())
		}
		if navigation.String() == "end" && !m.reader.viewport.AtBottom() {
			t.Fatal("reflow undid explicit bottom navigation")
		}
	}
}

func TestReaderKeysDuringInitialRender(t *testing.T) {
	for _, tt := range []struct {
		key    tea.KeyPressMsg
		bottom bool
	}{
		{press("a", 'a'), false}, {press("home", tea.KeyHome), false},
		{press("down", tea.KeyDown), false}, {press("end", tea.KeyEnd), true},
	} {
		m := readerFixture()
		m, cmd := applyMessage(m, press("enter", tea.KeyEnter))
		m = updateModel(m, tt.key)
		m = runUICommands(t, m, cmd)
		if tt.bottom && !m.reader.viewport.AtBottom() {
			t.Fatal("pending End lost bottom intent")
		}
		if !tt.bottom && !m.reader.viewport.AtTop() {
			t.Fatalf("pending %s jumped down", tt.key.String())
		}
	}
}

func TestArticleRendererCoalescesWorkAndCachesDocument(t *testing.T) {
	m := readerFixture()
	started, release := make(chan struct{}), make(chan struct{})
	var builds atomic.Int32
	m.articleRenderer.build = func(article articleSnapshot) string {
		builds.Add(1)
		close(started)
		<-release
		return renderArticleDocument(article)
	}
	m, first := applyMessage(m, press("enter", tea.KeyEnter))
	done := make(chan tea.Msg, 1)
	go func() { done <- first() }()
	<-started
	var commands []tea.Cmd
	for width := 40; width <= 50; width++ {
		var cmd tea.Cmd
		m, cmd = applyMessage(m, tea.WindowSizeMsg{Width: width, Height: 20})
		commands = append(commands, cmd)
	}
	close(release)
	if result := <-done; result != nil {
		t.Fatal("obsolete active render returned a result")
	}
	for _, cmd := range commands[:len(commands)-1] {
		if result := cmd(); result != nil {
			t.Fatal("obsolete queued render did work")
		}
	}
	m = runUICommands(t, m, commands[len(commands)-1])
	if builds.Load() != 1 || m.reader.viewport.Width() != readerColumnWidth(50) || m.reader.loading {
		t.Fatalf("builds %d, reader %#v", builds.Load(), m.reader)
	}
}

func openReader(t *testing.T, m model) model {
	t.Helper()
	m, cmd := applyMessage(m, press("enter", tea.KeyEnter))
	if !m.readerOpen || cmd == nil {
		t.Fatal("Enter did not schedule reader rendering")
	}
	return runUICommands(t, m, cmd)
}

func TestReaderOpeningRoutingAndReturnPreserveList(t *testing.T) {
	m := readerFixture()
	m.posts.list.SetFilterText("Article")
	m.posts.list.Select(m.posts.list.Paginator.PerPage + 1)
	index, page, query := m.posts.list.Index(), m.posts.list.Paginator.Page, m.posts.list.FilterInput.Value()
	selected, ok := m.selectedArticle()
	if !ok {
		t.Fatal("selected post has no persisted record")
	}
	m = openReader(t, m)
	if m.reader.article.post.ID != selected.post.ID || !strings.Contains(ansi.Strip(m.View().Content), "Source: Feed 0") {
		t.Fatalf("wrong article: %#v", m.reader.article)
	}
	for _, msg := range []tea.KeyPressMsg{press("a", 'a'), press("R", 'R'), press("r", 'r'), press("/", '/'), press("P", 'P'), shiftTab(), press("tab", tea.KeyTab)} {
		var cmd tea.Cmd
		m, cmd = applyMessage(m, msg)
		if cmd != nil || !m.readerOpen || m.modalOpen || m.focus != focusPosts || m.posts.list.Index() != index {
			t.Fatalf("reader leaked key %q", msg.String())
		}
	}
	m = updateModel(m, press("esc", tea.KeyEscape))
	if m.readerOpen || m.focus != focusPosts || m.posts.list.Index() != index || m.posts.list.Paginator.Page != page || m.posts.list.FilterInput.Value() != query {
		t.Fatal("return changed list state")
	}
}

func TestReaderScrollAndQuit(t *testing.T) {
	for _, quit := range []tea.KeyPressMsg{press("q", 'q'), tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl})} {
		m := openReader(t, readerFixture())
		m = updateModel(m, press("j", 'j'))
		if m.reader.viewport.YOffset() != 1 {
			t.Fatal("j did not scroll one line")
		}
		m = updateModel(m, press("pgdown", tea.KeyPgDown))
		if m.reader.viewport.YOffset() <= 1 {
			t.Fatal("PgDown did not scroll a page")
		}
		m = updateModel(m, press("end", tea.KeyEnd))
		if !m.reader.viewport.AtBottom() {
			t.Fatal("End did not reach bottom")
		}
		m = updateModel(m, press("down", tea.KeyDown))
		if m.reader.viewport.PastBottom() {
			t.Fatal("scroll passed bottom")
		}
		m = updateModel(m, press("home", tea.KeyHome))
		m = updateModel(m, press("k", 'k'))
		if !m.reader.viewport.AtTop() {
			t.Fatal("Home/up did not stay at top")
		}
		_, cmd := applyMessage(m, quit)
		if _, ok := runCommand(t, cmd).(tea.QuitMsg); !ok {
			t.Fatal("reader did not quit application")
		}
	}
}

func TestReaderResizesAndRejectsObsoleteRenders(t *testing.T) {
	m := readerFixture()
	m, first := applyMessage(m, press("enter", tea.KeyEnter))
	oldResult := runCommand(t, first)
	m, resize := applyMessage(m, tea.WindowSizeMsg{Width: 40, Height: 10})
	m = runUICommands(t, m, resize)
	m = updateModel(m, oldResult)
	if m.reader.viewport.Width() != readerColumnWidth(40) || lipgloss.Width(m.reader.viewport.GetContent()) > readerColumnWidth(40) {
		t.Fatal("stale render replaced resized document")
	}
	m = updateModel(m, press("end", tea.KeyEnd))
	for _, size := range []tea.WindowSizeMsg{{Width: 1, Height: 1}, {Width: 7, Height: 4}, {Width: 100, Height: 24}, {Width: 40, Height: 10}} {
		var cmd tea.Cmd
		m, cmd = applyMessage(m, size)
		m = runUICommands(t, m, cmd)
		view := m.View().Content
		if lipgloss.Width(view) > size.Width || lipgloss.Height(view) > size.Height {
			t.Fatalf("size %dx%d rendered %dx%d", size.Width, size.Height, lipgloss.Width(view), lipgloss.Height(view))
		}
		if size.Height > 1 && !m.reader.viewport.AtBottom() {
			t.Fatal("resize lost bottom reading position")
		}
	}
	m, pending := applyMessage(m, tea.WindowSizeMsg{Width: 60, Height: 20})
	late := runCommand(t, pending)
	m = updateModel(m, press("esc", tea.KeyEscape))
	m = updateModel(m, late)
	if m.readerOpen {
		t.Fatal("late render reopened closed reader")
	}
	m = openReader(t, m)
	current := m.reader.viewport.GetContent()
	m = updateModel(m, late)
	if m.reader.viewport.GetContent() != current {
		t.Fatal("previous reader result replaced reopened reader")
	}
}

func TestReaderSnapshotSurvivesRefreshAndReopensCurrentContent(t *testing.T) {
	store := postStoreWithFeeds(database.Feed{ID: 1, Name: "Feed", Url: "https://example.test"})
	store.posts[1] = []database.Post{{ID: 10, FeedID: 1, Title: "Article", Content: "Original", ContentKind: "text"}}
	m := loadedFeedModel(t, store)
	m, cmd := applyMessage(m, press("tab", tea.KeyTab))
	m = runUICommands(t, m, cmd)
	m, refresh := applyMessage(m, press("R", 'R'))
	m = openReader(t, m)
	store.posts[1][0].Content = "Updated"
	m = runUICommands(t, m, refresh)
	if m.reader.article.post.Content != "Original" || !strings.Contains(m.reader.viewport.GetContent(), "Original") {
		t.Fatal("refresh changed open snapshot")
	}
	m = updateModel(m, press("esc", tea.KeyEscape))
	m = openReader(t, m)
	if !strings.Contains(m.reader.viewport.GetContent(), "Updated") {
		t.Fatal("reopening did not display updated content")
	}
}

func TestReaderRespectsExistingInputOwners(t *testing.T) {
	for _, owner := range []string{"feeds", "modal", "filter", "empty"} {
		t.Run(owner, func(t *testing.T) {
			m := readerFixture()
			switch owner {
			case "feeds":
				m.focus = focusFeeds
			case "modal":
				m = updateModel(m, press("a", 'a'))
			case "filter":
				m = updateModel(m, press("/", '/'))
				if m.posts.list.FilterState() != list.Filtering {
					t.Fatal("filter not active")
				}
			case "empty":
				m = updateModel(m, postPageResult{request: m.postRequest, feedID: 1})
			}
			m = updateModel(m, press("enter", tea.KeyEnter))
			if m.readerOpen {
				t.Fatalf("Enter opened reader from %s", owner)
			}
		})
	}
}
