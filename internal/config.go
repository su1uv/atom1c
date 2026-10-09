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
	Next(context.Context) (FeedRefreshNotification, error)
	Close()
}

// FeedRefreshNotification carries changed feed IDs. Reconcile asks subscribers
// to reload their current feed when the bounded ID set overflowed.
type FeedRefreshNotification struct {
	FeedIDs   []int64
	Reconcile bool
}
