package ui

import (
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/su1uv/atom1c/internal"
)

func testModel() model {
	return newModel(&internal.State{}).(model)
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
	if m.modalOpen {
		t.Fatal("enter on Submit did not close the modal")
	}
	if got := m.addFeed.inputs[0].Value(); got != "name" {
		t.Fatalf("submitting unexpectedly cleared the draft: got %q", got)
	}
}

func TestFilteringRoutesShortcutCharactersToFocusedFilter(t *testing.T) {
	for _, tc := range []struct {
		pane string
		text string
		code rune
	}{
		{pane: "feeds", text: "a", code: 'a'},
		{pane: "feeds", text: "P", code: 'P'},
		{pane: "feeds", text: "q", code: 'q'},
		{pane: "posts", text: "a", code: 'a'},
		{pane: "posts", text: "P", code: 'P'},
		{pane: "posts", text: "q", code: 'q'},
	} {
		t.Run(tc.pane+"/"+tc.text, func(t *testing.T) {
			m := testModel()
			if tc.pane == "posts" {
				m = updateModel(m, press("tab", tea.KeyTab))
			}
			m = updateModel(m, press("/", '/'))
			pane := m.activePane()
			if got := pane.list.FilterState(); got != list.Filtering {
				t.Fatalf("%s list did not enter filtering: state %v", tc.pane, got)
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

	if m.feeds.list.ShowPagination() == feedsBefore {
		t.Fatal("focused feeds pane pagination was not toggled")
	}
	if m.posts.list.ShowPagination() != postsBefore {
		t.Fatal("inactive posts pane pagination was toggled")
	}

	m = updateModel(m, press("tab", tea.KeyTab))
	feedsBefore = m.feeds.list.ShowPagination()
	postsBefore = m.posts.list.ShowPagination()
	m = updateModel(m, press("P", 'P'))
	if m.feeds.list.ShowPagination() != feedsBefore {
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
	pane := initialFeedsModel(newStyles(false))
	cmd := pane.Update(press("/", '/'))
	if cmd == nil {
		t.Fatal("starting the filter produced no command")
	}

	result := cmd()
	msg, ok := result.(paneMessage)
	if !ok {
		t.Fatalf("pane command returned %T, want paneMessage", result)
	}
	if msg.pane != focusFeeds {
		t.Fatalf("async result pane = %v, want feeds", msg.pane)
	}
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
	m = updateModel(m, press("/", '/'))
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
