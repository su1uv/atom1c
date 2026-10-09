package ui

import (
	"context"
	"database/sql"
	"errors"

	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/database"
)

type articleCacheStore interface {
	Get(context.Context, int64, string) (database.ArticleCache, error)
	Save(context.Context, database.UpsertArticleCacheParams) (database.ArticleCache, error)
}

type stateArticleCacheStore struct{ state *internal.State }

func (s stateArticleCacheStore) Get(ctx context.Context, postID int64, sourceURL string) (database.ArticleCache, error) {
	if s.state == nil || s.state.Db == nil {
		return database.ArticleCache{}, sql.ErrNoRows
	}
	return s.state.Db.GetArticleCache(ctx, database.GetArticleCacheParams{PostID: postID, SourceUrl: sourceURL})
}

func (s stateArticleCacheStore) Save(ctx context.Context, params database.UpsertArticleCacheParams) (database.ArticleCache, error) {
	if s.state == nil || s.state.Db == nil {
		return database.ArticleCache{}, errors.New("article cache database is not configured")
	}
	return s.state.Db.UpsertArticleCache(ctx, params)
}
