package ui

import (
	"context"
	"database/sql"
	"errors"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/su1uv/atom1c/reader"
)

type articleFetcher interface {
	Fetch(context.Context, string) (reader.Article, error)
}

func (m *model) beginArticleFetch() tea.Cmd {
	if !m.readerOpen || m.reader.fetching {
		return nil
	}
	if m.articleFetcher == nil {
		m.reader.fetchError = "article fetching is unavailable"
		return nil
	}
	postID, sourceURL := m.reader.article.post.ID, m.reader.article.post.Link
	if sourceURL == "" {
		m.reader.fetchError = "post has no website link"
		return nil
	}
	m.reader.fetchRequest++
	request, session := m.reader.fetchRequest, m.reader.fetchSession
	fetcher, cache := m.articleFetcher, m.articleCache
	ctx, cancel := context.WithCancel(m.sessionContext())
	m.reader.fetchCancel = cancel
	m.reader.fetching = true
	m.reader.fetchError = ""
	return func() tea.Msg {
		article, err := fetcher.Fetch(ctx, sourceURL)
		result := articleFetchResult{session: session, request: request, postID: postID, sourceURL: sourceURL, article: article, err: err}
		if err == nil {
			if cache == nil {
				result.cacheErr = errors.New("article cache database is not configured")
			} else {
				cacheCtx, cacheCancel := context.WithTimeout(ctx, 5*time.Second)
				defer cacheCancel()
				_, result.cacheErr = cache.Save(cacheCtx, cacheParams(postID, sourceURL, article))
			}
		}
		return result
	}
}

func (m model) updateArticleFetchResult(msg articleFetchResult) (tea.Model, tea.Cmd) {
	if !m.readerOpen || m.reader.fetchSession != msg.session || m.reader.fetchRequest != msg.request || m.reader.article.post.ID != msg.postID || m.reader.article.post.Link != msg.sourceURL {
		return m, nil
	}
	if m.reader.fetchCancel != nil {
		m.reader.fetchCancel()
		m.reader.fetchCancel = nil
	}
	m.reader.fetching = false
	if msg.err != nil {
		m.reader.fetchError = msg.err.Error()
		return m, nil
	}
	if msg.cacheErr == sql.ErrNoRows {
		m.reader.fetchError = "post link changed during fetch; reopen it to load the current article"
		return m, nil
	}
	m.reader.article.full = &msg.article
	m.reader.article.showFull = true
	m.reader.fetchError = ""
	if msg.cacheErr != nil && msg.cacheErr != sql.ErrNoRows {
		m.reader.cacheError = msg.cacheErr.Error()
	} else {
		m.reader.cacheError = ""
	}
	m.reader.session++
	m.reader.progress = 0
	return m, m.beginArticleRender()
}

var _ articleFetcher = (*reader.Fetcher)(nil)
