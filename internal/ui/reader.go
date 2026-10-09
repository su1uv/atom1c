package ui

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/su1uv/atom1c/internal/database"
	"github.com/su1uv/atom1c/reader"
	readerview "github.com/su1uv/atom1c/reader/view"
)

type articleReader struct {
	article          articleSnapshot
	viewport         readerview.Model
	loading          bool
	hasContent       bool
	progress         float64
	session          uint64
	fetchSession     uint64
	fetching         bool
	fetchError       string
	cacheError       string
	cacheChecked     bool
	autoFetchStarted bool
	fetchRequest     uint64
	fetchCancel      context.CancelFunc
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

func (r *articleRenderer) render(request, session uint64, article articleSnapshot, width int, dark bool) tea.Msg {
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
	content, err := reader.RenderMarkdown(r.document, width, dark)
	if err != nil {
		return articleRenderResult{request: request, err: err}
	}
	if r.latest.Load() != request {
		return nil
	}
	return articleRenderResult{request: request, content: content}
}

type articleRenderResult struct {
	request uint64
	content string
	err     error
}

type articleOpenResult struct {
	session   uint64
	postID    int64
	sourceURL string
	cache     database.ArticleCache
	err       error
}

type articleFetchResult struct {
	session   uint64
	request   uint64
	postID    int64
	sourceURL string
	article   reader.Article
	err       error
	cacheErr  error
}

func (m *model) openArticle() tea.Cmd {
	article, ok := m.selectedArticle()
	if !ok {
		return nil
	}
	m.readerOpen = true
	m.reader = articleReader{article: article, viewport: readerview.New(m.width, m.height)}
	m.reader.session = m.readerRequest + 1
	m.reader.fetchSession = m.reader.session
	m.reader.viewport.SetContent("Rendering article…")
	if m.articleCache != nil {
		m.reader.loading = true
		return m.beginArticleCacheLoad()
	}
	m.reader.cacheChecked = true
	return m.beginArticleRender()
}

func (m *model) beginArticleCacheLoad() tea.Cmd {
	session, article, store := m.reader.session, m.reader.article, m.articleCache
	sessionCtx := m.sessionContext()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(sessionCtx, 5*time.Second)
		defer cancel()
		cache, err := store.Get(ctx, article.post.ID, article.post.Link)
		return articleOpenResult{session: session, postID: article.post.ID, sourceURL: article.post.Link, cache: cache, err: err}
	}
}

func (m model) updateArticleOpenResult(msg articleOpenResult) (tea.Model, tea.Cmd) {
	if !m.readerOpen || m.reader.session != msg.session || m.reader.article.post.ID != msg.postID || m.reader.article.post.Link != msg.sourceURL {
		return m, nil
	}
	m.reader.cacheChecked = true
	if msg.err == nil {
		article := cachedArticle(msg.cache)
		m.reader.article.full = &article
		m.reader.article.showFull = true
	} else if msg.err != sql.ErrNoRows {
		m.reader.cacheError = msg.err.Error()
	}
	m.reader.loading = false
	return m, m.beginArticleRender()
}

func (m *model) beginArticleRender() tea.Cmd {
	m.readerRequest++
	request := m.readerRequest
	article, width := m.reader.article, m.reader.viewport.Width()
	renderer, session := m.articleRenderer, m.reader.session
	dark := m.darkBackground
	renderer.latest.Store(request)
	renderer.latestSession.Store(session)
	m.reader.loading = true
	return func() tea.Msg { return renderer.render(request, session, article, width, dark) }
}

func (m model) updateArticleRenderResult(msg articleRenderResult) (tea.Model, tea.Cmd) {
	if !m.readerOpen || msg.request != m.readerRequest {
		return m, nil
	}
	if msg.err != nil {
		m.reader.loading = false
		m.reader.fetchError = "Could not render article: " + msg.err.Error()
		return m, nil
	}
	m.reader.viewport.SetContent(msg.content)
	m.reader.viewport.SetYOffset(int(math.Round(m.reader.progress * float64(max(m.reader.viewport.TotalLineCount()-m.reader.viewport.Height(), 0)))))
	m.reader.loading = false
	m.reader.hasContent = true
	if m.reader.cacheChecked && !m.reader.autoFetchStarted && m.reader.article.full == nil && m.reader.article.post.Link != "" && m.articleFetcher != nil {
		m.reader.autoFetchStarted = true
		return m, m.beginArticleFetch()
	}
	return m, nil
}

func (m *model) resizeReader(width, height int) tea.Cmd {
	oldWidth := m.reader.viewport.Width()
	if !m.reader.loading {
		m.reader.progress = m.reader.viewport.ScrollPercent()
	}
	m.reader.viewport.SetSize(width, height)
	if oldWidth != m.reader.viewport.Width() {
		return m.beginArticleRender()
	}
	return nil
}

func (m model) updateReaderKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.reader.fetchCancel != nil {
			m.reader.fetchCancel()
		}
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
	case "R":
		return m, m.beginArticleFetch()
	case "r":
		if m.reader.fetchError != "" {
			return m, m.beginArticleFetch()
		}
		return m, nil
	case "f":
		if m.reader.article.full == nil {
			return m, nil
		}
		m.reader.article.showFull = !m.reader.article.showFull
		m.reader.session++
		m.reader.progress = 0
		return m, m.beginArticleRender()
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
	status := "j/k scroll • PgUp/PgDown page • Home/End • R reload • f feed/full • esc back • q quit"
	if m.reader.loading {
		status = "Rendering article… • esc back • q quit"
	}
	if m.reader.fetching {
		status = "Loading full article… • R reload • f feed preview • esc back"
	}
	if m.reader.fetchError != "" {
		status = "Full article unavailable: " + m.reader.fetchError + " • r retry • f feed preview • esc back"
	}
	if m.reader.cacheError != "" {
		status += " • cache unavailable: " + m.reader.cacheError
	}
	if m.reader.hasContent {
		status += fmt.Sprintf(" • %s • %3.0f%%", articleSourceLabel(m.reader.article), m.reader.viewport.ScrollPercent()*100)
	}
	return m.reader.viewport.View(status)
}

func readerColumnWidth(width int) int { return readerview.ColumnWidth(width) }

func articleSourceLabel(article articleSnapshot) string {
	if article.showFull && article.full != nil {
		return "Full article"
	}
	return "Feed preview"
}
