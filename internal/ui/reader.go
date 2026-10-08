package ui

import (
	"math"
	"strings"
	"sync"
	"sync/atomic"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type articleReader struct {
	article    articleSnapshot
	viewport   viewport.Model
	loading    bool
	hasContent bool
	progress   float64
	session    uint64
}

// One renderer is shared across sessions. Queued obsolete jobs exit before
// parsing or wrapping, and width-independent content is cached for reflow.
type articleRenderer struct {
	mu            sync.Mutex
	latest        atomic.Uint64
	latestSession atomic.Uint64
	session       uint64
	document      string
	build         func(articleSnapshot) string
}

func (r *articleRenderer) render(request, session uint64, article articleSnapshot, width int) tea.Msg {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.latest.Load() != request {
		return nil
	}
	if r.session != session {
		r.document = r.build(article)
		r.session = session
	}
	if r.latestSession.Load() != session {
		r.document, r.session = "", 0
		return nil
	}
	if r.latest.Load() != request {
		return nil
	}
	content := wrapArticleDocument(r.document, width)
	if r.latest.Load() != request {
		return nil
	}
	return articleRenderResult{request: request, content: content}
}

type articleRenderResult struct {
	request uint64
	content string
}

func (m *model) openArticle() tea.Cmd {
	article, ok := m.selectedArticle()
	if !ok {
		return nil
	}
	m.readerOpen = true
	m.reader = articleReader{article: article, viewport: viewport.New(viewport.WithWidth(max(m.width, 1)), viewport.WithHeight(max(m.height-1, 0)))}
	m.reader.session = m.readerRequest + 1
	m.reader.viewport.MouseWheelEnabled = false
	m.reader.viewport.SetContent("Rendering article…")
	return m.beginArticleRender()
}

func (m *model) beginArticleRender() tea.Cmd {
	m.readerRequest++
	request := m.readerRequest
	article, width := m.reader.article, m.reader.viewport.Width()
	renderer, session := m.articleRenderer, m.reader.session
	renderer.latest.Store(request)
	renderer.latestSession.Store(session)
	m.reader.loading = true
	return func() tea.Msg { return renderer.render(request, session, article, width) }
}

func (m model) updateArticleRenderResult(msg articleRenderResult) (tea.Model, tea.Cmd) {
	if !m.readerOpen || msg.request != m.readerRequest {
		return m, nil
	}
	m.reader.viewport.SetContent(msg.content)
	m.reader.viewport.SetYOffset(int(math.Round(m.reader.progress * float64(max(m.reader.viewport.TotalLineCount()-m.reader.viewport.Height(), 0)))))
	m.reader.loading = false
	m.reader.hasContent = true
	return m, nil
}

func (m *model) resizeReader(width, height int) tea.Cmd {
	oldWidth := m.reader.viewport.Width()
	if !m.reader.loading {
		m.reader.progress = m.reader.viewport.ScrollPercent()
	}
	atBottom := m.reader.viewport.AtBottom()
	m.reader.viewport.SetWidth(max(width, 1))
	m.reader.viewport.SetHeight(max(height-1, 0))
	if atBottom {
		m.reader.viewport.GotoBottom()
	} else {
		m.reader.viewport.SetYOffset(m.reader.viewport.YOffset())
	}
	if oldWidth != m.reader.viewport.Width() {
		return m.beginArticleRender()
	}
	return nil
}

func (m model) updateReaderKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.readerOpen = false
		m.readerRequest++
		m.articleRenderer.latest.Store(m.readerRequest)
		m.articleRenderer.latestSession.Store(0)
		m.reader = articleReader{}
		// List dimensions were held stable while reading. Apply any terminal resize
		// on return without reconstructing the lists or discarding their filters.
		return m.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
	case "q", "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		if m.reader.hasContent {
			m.reader.viewport.ScrollDown(1)
		}
	case "k", "up":
		if m.reader.hasContent {
			m.reader.viewport.ScrollUp(1)
		}
	case "pgdown":
		if m.reader.hasContent {
			m.reader.viewport.PageDown()
		}
	case "pgup":
		if m.reader.hasContent {
			m.reader.viewport.PageUp()
		}
	case "home":
		m.reader.viewport.GotoTop()
		m.reader.progress = 0
		return m, nil
	case "end":
		m.reader.viewport.GotoBottom()
		m.reader.progress = 1
		return m, nil
	default:
		return m, nil
	}
	if m.reader.hasContent {
		m.reader.progress = m.reader.viewport.ScrollPercent()
	}
	return m, nil
}

func (m model) readerView() tea.View {
	if m.width <= 0 || m.height <= 0 {
		return tea.NewView("")
	}
	status := "j/k ↑/↓ scroll • PgUp/PgDown page • Home/End • esc back • q quit"
	if m.reader.loading {
		status = "Rendering article… • esc back • q quit"
	}
	parts := make([]string, 0, 2)
	if m.height > 1 {
		parts = append(parts, m.reader.viewport.View())
	}
	parts = append(parts, ansi.Truncate(status, m.width, ""))
	content := lipgloss.NewStyle().MaxWidth(m.width).MaxHeight(m.height).Render(strings.Join(parts, "\n"))
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}
