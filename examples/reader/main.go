// Command reader demonstrates the standalone article reader without Atom1c,
// SQLite, or feed application state.
package main

import (
	"context"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/su1uv/atom1c/reader"
	readerview "github.com/su1uv/atom1c/reader/view"
)

type model struct {
	url           string
	document      reader.Article
	viewport      readerview.Model
	width, height int
	loading, dark bool
	err           error
}

type result struct {
	article reader.Article
	err     error
}

func (m model) Init() tea.Cmd {
	fetch := func() tea.Msg {
		article, err := reader.NewFetcher(nil).Fetch(context.Background(), m.url)
		return result{article: article, err: err}
	}
	background := func() tea.Msg { return tea.RequestBackgroundColor() }
	return tea.Batch(fetch, background)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case result:
		m.loading, m.err = false, msg.err
		if msg.err == nil {
			m.document = msg.article
			m.err = m.renderDocument()
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.viewport.SetSize(msg.Width, msg.Height)
		if m.document.Markdown != "" {
			m.err = m.renderDocument()
		}
		return m, nil
	case tea.BackgroundColorMsg:
		m.dark = msg.IsDark()
		if m.document.Markdown != "" {
			m.err = m.renderDocument()
		}
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "j", "down":
			m.viewport.ScrollDown(1)
		case "k", "up":
			m.viewport.ScrollUp(1)
		case "pgdown":
			m.viewport.PageDown()
		case "pgup":
			m.viewport.PageUp()
		case "home":
			m.viewport.GotoTop()
		case "end":
			m.viewport.GotoBottom()
		}
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *model) renderDocument() error {
	title, _ := reader.FeedContentMarkdown(m.document.Title, "text", "")
	markdown := "# " + title + "\n\n"
	if m.document.SiteName != "" {
		site, _ := reader.FeedContentMarkdown(m.document.SiteName, "text", "")
		markdown += "**Site:** " + site + "\n\n"
	}
	if m.document.Author != "" {
		author, _ := reader.FeedContentMarkdown(m.document.Author, "text", "")
		markdown += "**Author:** " + author + "\n\n"
	}
	markdown += m.document.Markdown
	text, err := reader.RenderMarkdown(markdown, m.viewport.Width(), m.dark)
	if err == nil {
		m.viewport.SetContent(text)
	}
	return err
}

func (m model) View() tea.View {
	status := "j/k scroll • PgUp/PgDown • Home/End • esc quit"
	if m.loading {
		status = "Fetching article… • esc quit"
	}
	if m.err != nil {
		status = "Could not read article: " + m.err.Error() + " • esc quit"
	}
	if !m.loading && m.err == nil {
		status = fmt.Sprintf("%s • %3.0f%% • esc quit", m.document.URL, m.viewport.ScrollPercent()*100)
	}
	return m.viewport.View(status)
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./examples/reader <public-article-url>")
		os.Exit(2)
	}
	m := model{url: os.Args[1], viewport: readerview.New(0, 0), loading: true, dark: true}
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
