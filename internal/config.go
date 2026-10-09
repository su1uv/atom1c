package internal

import (
	"context"
	"database/sql"

	"github.com/su1uv/atom1c/internal/database"
)

type State struct {
	Db          *database.Queries
	SQLDB       *sql.DB
	FeedRefresh FeedRefreshManager
}

// FeedRefreshManager coordinates refresh requests and reports successful
// persistence to interested sessions.
type FeedRefreshManager interface {
	Refresh(context.Context, database.Feed) error
	Subscribe(context.Context) FeedRefreshSubscription
}

// FeedRefreshSubscription returns coalesced notifications after successful refreshes.
type FeedRefreshSubscription interface {
	// Watch selects the one feed whose successful refreshes should be reported; zero clears it.
	Watch(int64)
	Next(context.Context) (FeedRefreshNotification, error)
	Close()
}

// FeedRefreshNotification carries a successful refresh for the watched feed.
type FeedRefreshNotification struct {
	FeedID int64
}
