package internal

import (
	"context"
	"database/sql"
	"fmt"
	"time"

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

// FeedRefreshSubscription returns coalesced feed IDs after successful refreshes.
type FeedRefreshSubscription interface {
	Next(context.Context) ([]int64, error)
	Close()
}

// LoadRefreshInterval reads the server's automatic feed-refresh interval. An
// unset value defaults to 15 minutes; zero disables scheduled refresh.
func LoadRefreshInterval(getenv func(string) string) (time.Duration, error) {
	if getenv == nil {
		return 0, fmt.Errorf("refresh interval environment lookup is nil")
	}
	value := getenv("ATOM1C_REFRESH_INTERVAL")
	if value == "" {
		return 15 * time.Minute, nil
	}
	interval, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid ATOM1C_REFRESH_INTERVAL %q: %w", value, err)
	}
	if interval < 0 {
		return 0, fmt.Errorf("invalid ATOM1C_REFRESH_INTERVAL %q: duration must not be negative", value)
	}
	return interval, nil
}
