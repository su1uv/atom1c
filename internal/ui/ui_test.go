package ui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/cursor"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/database"
)

func testModel() model {
	m := newModel(&internal.State{}).(model)
	items := make([]list.Item, 5)
	for i := range items {
		items[i] = item{id: int64(i + 1), name: fmt.Sprintf("Feed %d", i), url: fmt.Sprintf("https://example.test/%d", i)}
	}
	_ = m.feeds.list.SetItems(items)
	postFixtures := []item{
		{name: "Test post one", url: "https://example.test/post-one"},
		{name: "Test post two", url: "https://example.test/post-two"},
	}
	postItems := make([]list.Item, len(postFixtures))
	for i := range postFixtures {
		postItems[i] = postFixtures[i]
	}
	_ = m.posts.list.SetItems(postItems)
	m.openFeedID = 1
	m.openFeedName = "Feed 0"
	m.openFeedURL = "https://example.test/0"
	return m
}

func shiftTab() tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift})
}

func press(text string, code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Text: text, Code: code})
}

func updateModel(m model, msg tea.Msg) model {
	updated, _ := m.Update(msg)
	return updated.(model)
}

func TestOpeningAddFeedModalFocusesFirstInputAndConsumesShortcut(t *testing.T) {
	m := updateModel(testModel(), press("a", 'a'))

	if !m.modalOpen {
		t.Fatal("add-feed modal did not open")
	}
	if !m.addFeed.inputs[0].Focused() {
		t.Fatal("first input is not focused after opening")
	}
	if got := m.addFeed.inputs[0].Value(); got != "" {
		t.Fatalf("opening shortcut entered modal input: got %q", got)
	}
	if got := m.addFeed.inputs[1].Value(); got != "" {
		t.Fatalf("opening shortcut entered second modal input: got %q", got)
	}
	if m.View().Cursor == nil {
		t.Fatal("add-feed input cursor was not exposed by the root view")
	}
}

func TestReopeningAddFeedModalPreservesDraftAndFocusesFirstInput(t *testing.T) {
	m := testModel()
	m = updateModel(m, press("a", 'a'))
	m.addFeed.inputs[0].SetValue("draft name")
	m.addFeed.inputs[1].SetValue("https://example.com/feed")
	m = updateModel(m, press("esc", tea.KeyEscape))
	if m.modalOpen {
		t.Fatal("modal did not close")
	}
	if m.addFeed.inputs[0].Focused() || m.addFeed.inputs[1].Focused() {
		t.Fatal("modal inputs remained focused after closing")
	}
	m = updateModel(m, press("a", 'a'))

	if !m.addFeed.inputs[0].Focused() {
		t.Fatal("first input is not focused after reopening")
	}
	if got := m.addFeed.inputs[0].Value(); got != "draft name" {
		t.Fatalf("draft name was not preserved: got %q", got)
	}
	if got := m.addFeed.inputs[1].Value(); got != "https://example.com/feed" {
		t.Fatalf("draft URL was not preserved: got %q", got)
	}
	if m.addFeed.inputs[1].Focused() {
		t.Fatal("second input remains focused after reopening")
	}
}

func TestModalOwnsKeyboardAndKeepsDraftNavigationLocal(t *testing.T) {
	m := updateModel(testModel(), press("a", 'a'))
	feedIndex := m.feeds.list.Index()
	postIndex := m.posts.list.Index()
	feedsPagination := m.feeds.list.ShowPagination()
	postsPagination := m.posts.list.ShowPagination()

	m = updateModel(m, press("q", 'q'))
	m = updateModel(m, press("tab", tea.KeyTab))
	m = updateModel(m, press("P", 'P'))

	if !m.modalOpen {
		t.Fatal("modal closed while typing or navigating")
	}
	if got := m.addFeed.inputs[0].Value(); got != "q" {
		t.Fatalf("modal did not retain first-field input: got %q", got)
	}
	if !m.addFeed.inputs[1].Focused() {
		t.Fatal("tab did not focus the second input")
	}
	if got := m.addFeed.inputs[1].Value(); got != "P" {
		t.Fatalf("pagination shortcut was not treated as modal text: got %q", got)
	}
	if m.feeds.list.Index() != feedIndex || m.posts.list.Index() != postIndex {
		t.Fatal("modal keys changed a background list selection")
	}
	if m.feeds.list.ShowPagination() != feedsPagination || m.posts.list.ShowPagination() != postsPagination {
		t.Fatal("modal key toggled background pagination")
	}
}

func TestAddFeedSubmitRequiresSubmitButtonFocus(t *testing.T) {
	m := updateModel(testModel(), press("a", 'a'))
	m.addFeed.inputs[0].SetValue("name")

	m = updateModel(m, press("enter", tea.KeyEnter))
	if !m.modalOpen {
		t.Fatal("enter on an input submitted the modal")
	}
	if !m.addFeed.inputs[1].Focused() {
		t.Fatal("enter on the first input did not move focus to the next field")
	}

	m = updateModel(m, press("tab", tea.KeyTab))
	if m.addFeed.focusIndex != len(m.addFeed.inputs) {
		t.Fatal("tab navigation did not focus Submit")
	}
	m = updateModel(m, press("enter", tea.KeyEnter))
	if !m.modalOpen {
		t.Fatal("invalid submission closed the modal")
	}
	if !strings.Contains(m.addFeed.err, "absolute HTTP") || !m.addFeed.inputs[1].Focused() {
		t.Fatalf("invalid URL did not show an error and focus its input: error %q", m.addFeed.err)
	}
	m = updateModel(m, cursor.BlinkMsg{})
	if !strings.Contains(m.addFeed.err, "absolute HTTP") {
		t.Fatal("cursor update cleared the validation error without user input")
	}
}

func TestPostFilteringRoutesShortcutCharactersToFocusedFilter(t *testing.T) {
	for _, tc := range []struct {
		text string
		code rune
	}{
		{text: "a", code: 'a'},
		{text: "P", code: 'P'},
		{text: "R", code: 'R'},
		{text: "r", code: 'r'},
		{text: "q", code: 'q'},
	} {
		t.Run(tc.text, func(t *testing.T) {
			m := testModel()
			m = updateModel(m, press("tab", tea.KeyTab))
			m = updateModel(m, press("/", '/'))
			pane := m.activePane()
			if got := pane.list.FilterState(); got != list.Filtering {
				t.Fatalf("posts list did not enter filtering: state %v", got)
			}
			paginationBefore := pane.list.ShowPagination()
			m = updateModel(m, press(tc.text, tc.code))

			if m.modalOpen {
				t.Fatal("shortcut opened modal while filter was active")
			}
			if got := m.activePane().list.FilterInput.Value(); got != tc.text {
				t.Fatalf("filter did not receive typed key: got %q, want %q", got, tc.text)
			}
			if m.activePane().list.ShowPagination() != paginationBefore {
				t.Fatal("pagination visibility unexpectedly changed while filtering")
			}
		})
	}
}

func TestPaginationToggleAffectsOnlyFocusedPane(t *testing.T) {
	m := testModel()
	feedsBefore := m.feeds.list.ShowPagination()
	postsBefore := m.posts.list.ShowPagination()
	m = updateModel(m, press("P", 'P'))

	if m.showFeedPagination {
		t.Fatal("focused feeds pane pagination was not toggled")
	}
	if m.posts.list.ShowPagination() != postsBefore {
		t.Fatal("inactive posts pane pagination was toggled")
	}

	m = updateModel(m, press("tab", tea.KeyTab))
	feedsBefore = m.showFeedPagination
	postsBefore = m.posts.list.ShowPagination()
	m = updateModel(m, press("P", 'P'))
	if m.showFeedPagination != feedsBefore {
		t.Fatal("inactive feeds pane pagination was toggled")
	}
	if m.posts.list.ShowPagination() == postsBefore {
		t.Fatal("focused posts pane pagination was not toggled")
	}
}

func TestListNavigationAffectsOnlyFocusedPane(t *testing.T) {
	m := testModel()
	m = updateModel(m, press("down", tea.KeyDown))
	if got := m.feeds.list.Index(); got != 1 {
		t.Fatalf("focused feeds pane index = %d, want 1", got)
	}
	if got := m.posts.list.Index(); got != 0 {
		t.Fatalf("inactive posts pane index = %d, want 0", got)
	}

	m.openFeedID = 2 // Keep the test's seeded posts when opening the selected feed.
	m = updateModel(m, press("tab", tea.KeyTab))
	m = updateModel(m, press("down", tea.KeyDown))
	if got := m.feeds.list.Index(); got != 1 {
		t.Fatalf("inactive feeds pane index = %d, want 1", got)
	}
	if got := m.posts.list.Index(); got != 1 {
		t.Fatalf("focused posts pane index = %d, want 1", got)
	}
}

func TestPaneSwitchKeyIsConsumedByRootModel(t *testing.T) {
	m := testModel()
	postsList, _ := m.posts.list.Update(press("/", '/'))
	m.posts.list = postsList
	if got := m.posts.list.FilterState(); got != list.Filtering {
		t.Fatalf("posts test setup did not enter filtering: state %v", got)
	}

	m = updateModel(m, press("tab", tea.KeyTab))
	if m.focus != focusPosts {
		t.Fatal("tab did not focus posts pane")
	}
	if got := m.posts.list.FilterState(); got != list.Filtering {
		t.Fatalf("pane-switch key reached newly focused posts pane: state %v", got)
	}
}

func TestPaneCommandsTagAsyncResults(t *testing.T) {
	pane := initialPostsModel(newStyles(false))
	_ = pane.list.SetItems([]list.Item{item{name: "Test", url: "https://example.test/post"}})
	cmd := pane.Update(press("/", '/'))
	if cmd == nil {
		t.Fatal("starting the filter produced no command")
	}

	result := cmd()
	msg, ok := result.(paneMessage)
	if !ok {
		t.Fatalf("pane command returned %T, want paneMessage", result)
	}
	if msg.pane != focusPosts {
		t.Fatalf("async result pane = %v, want posts", msg.pane)
	}
}

func TestPostPaneAndHelpFitWithinTerminalHeight(t *testing.T) {
	posts := make([]database.Post, 100)
	for i := range posts {
		posts[i] = database.Post{ID: int64(i + 1), Title: fmt.Sprintf("Post %03d", i), Link: fmt.Sprintf("https://example.test/posts/%03d", i)}
	}
	for _, terminalHeight := range []int{12, 16, 20, 24, 32} {
		t.Run(fmt.Sprintf("height_%d", terminalHeight), func(t *testing.T) {
			m := testModel()
			_ = m.posts.list.SetItems(nil)
			m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: terminalHeight})
			m = updateModel(m, press("tab", tea.KeyTab))
			m = updateModel(m, postPageResult{request: m.postRequest, feedID: m.openFeedID, posts: posts})
			if m.posts.list.Paginator.TotalPages < 2 {
				t.Fatal("test setup did not create a paginated post list")
			}
			view := m.View().Content

			if got := lipgloss.Height(view); got > terminalHeight {
				t.Fatalf("rendered view height = %d rows, exceeds terminal height %d", got, terminalHeight)
			}
			if !strings.Contains(view, "refresh feed") {
				t.Fatalf("help commands missing from rendered view: %q", view)
			}
		})
	}
}

func TestFeedAndPostListsStartAtSameVerticalPosition(t *testing.T) {
	for _, tc := range []struct {
		name          string
		refreshing    bool
		refreshFailed bool
	}{
		{name: "idle"},
		{name: "refreshing", refreshing: true},
		{name: "refresh failed", refreshFailed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := testModel()
			m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 24})
			if tc.refreshing {
				m.refreshing[1] = true
			}
			if tc.refreshFailed {
				m.refreshErrors[1] = "could not resolve the remote feed host"
			}

			feedTop := listTopRow(m.feedView())
			postsView := m.postsView()
			postsTop := listTopRow(postsView)
			if feedTop != postsTop {
				t.Fatalf("feed list begins at row %d, posts list at row %d", feedTop, postsTop)
			}
			if feedHeight, postHeight := lipgloss.Height(m.feedView()), lipgloss.Height(postsView); feedHeight != postHeight {
				t.Fatalf("feed pane height = %d rows, posts pane height = %d rows", feedHeight, postHeight)
			}
		})
	}
}

func listTopRow(view string) int {
	for i, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "╭") {
			return i
		}
	}
	return -1
}

func TestPaneCommandsPreserveBubbleTeaControlMessages(t *testing.T) {
	pane := initialFeedsModel(newStyles(false))
	cmd := pane.Update(press("q", 'q'))
	if cmd == nil {
		t.Fatal("quit key produced no command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("list pane did not pass Bubble Tea quit message through")
	}
}

func TestAsyncListResultOnlyUpdatesItsOriginatingPane(t *testing.T) {
	m := testModel()
	feeds, _ := m.feeds.list.Update(press("/", '/'))
	m.feeds.list = feeds
	posts, _ := m.posts.list.Update(press("/", '/'))
	m.posts.list = posts
	postsVisible := len(m.posts.list.VisibleItems())
	if postsVisible == 0 {
		t.Fatal("test setup did not populate posts filter results")
	}

	updated, _ := m.Update(paneMessage{
		pane: focusFeeds,
		msg:  list.FilterMatchesMsg(nil),
	})
	m = updated.(model)

	if got := len(m.feeds.list.VisibleItems()); got != 0 {
		t.Fatalf("originating feeds pane did not apply async result: got %d visible items", got)
	}
	if got := len(m.posts.list.VisibleItems()); got != postsVisible {
		t.Fatalf("async result crossed into posts pane: got %d items, want %d", got, postsVisible)
	}
}
