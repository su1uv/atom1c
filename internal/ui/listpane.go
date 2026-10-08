package ui

import (
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/su1uv/atom1c/internal/database"
)

const listChromeRows = 3 // Title, status, and pagination.

type listPane struct {
	list     list.Model
	delegate list.DefaultDelegate
	style    lipgloss.Style
	id       focusState
}

func initialFeedsModel(styles Styles) listPane {
	pane := newListPane("Feeds", nil, styles, focusFeeds)
	pane.list.SetShowPagination(false)
	return pane
}

func initialPostsModel(styles Styles) listPane {
	return newListPane("Posts", nil, styles, focusPosts)
}

func feedItems(feeds []database.Feed) []list.Item {
	items := make([]list.Item, len(feeds))
	for i, feed := range feeds {
		items[i] = item{id: feed.ID, name: feed.Name, url: feed.Url}
	}
	return items
}

func postItems(posts []database.Post) []list.Item {
	items := make([]list.Item, len(posts))
	for i, post := range posts {
		items[i] = item{id: post.ID, name: post.Title, url: post.Link}
	}
	return items
}

func newListPane(title string, items []item, styles Styles, id focusState) listPane {
	listItems := make([]list.Item, len(items))
	for i, it := range items {
		listItems[i] = it
	}

	delegate := list.NewDefaultDelegate()
	l := list.New(listItems, delegate, 0, 0)
	l.Title = title
	l.Styles.Title = styles.title
	l.SetShowHelp(false)

	return listPane{list: l, delegate: delegate, style: styles.list, id: id}
}

func (m *listPane) setSize(w, h int) {
	frameWidth, frameHeight := m.style.GetFrameSize()
	contentWidth := max(w/2-frameWidth, 0)
	contentHeight := max(h-frameHeight-helpHeight, 0)
	delegate := m.delegate
	fullItemHeight := list.NewDefaultDelegate().Height()
	delegate.ShowDescription = contentHeight >= listChromeRows+fullItemHeight+delegate.Spacing()
	m.delegate = delegate
	m.list.SetDelegate(delegate)
	m.list.SetSize(contentWidth, contentHeight)
	// Lipgloss v2 Width includes the border. The list's content width does not.
	m.style = m.style.Width(contentWidth + frameWidth)
}

func (m *listPane) Update(msg tea.Msg) tea.Cmd {
	updated, cmd := m.list.Update(msg)
	m.list = updated
	return tagPaneCommand(cmd, m.id)
}

type paneMessage struct {
	pane focusState
	msg  tea.Msg
}

func tagPaneCommand(cmd tea.Cmd, pane focusState) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		if msg == nil {
			return nil
		}
		// Bubble Tea control messages must reach the program directly rather than
		// being consumed by a list pane.
		if _, ok := msg.(tea.QuitMsg); ok {
			return msg
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			wrapped := make(tea.BatchMsg, len(batch))
			for i, child := range batch {
				wrapped[i] = tagPaneCommand(child, pane)
			}
			return wrapped
		}
		return paneMessage{pane: pane, msg: msg}
	}
}

func (m listPane) View() tea.View {
	return tea.NewView(m.style.Render(m.list.View()))
}
