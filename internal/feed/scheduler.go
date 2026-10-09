package feed

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/database"
)

type refreshFeedLister interface {
	GetFeedsForRefresh(context.Context) ([]database.Feed, error)
}

type scheduledFeedRefresher interface {
	Refresh(context.Context, database.Feed) error
}

type refreshIntervalWaiter func(context.Context, time.Duration) error

var refreshErrorURLPattern = regexp.MustCompile(`(?i)https?://[^\s"'<>]+`)

// RunRefreshScheduler begins with an immediate complete feed sweep and then
// waits one interval after each sweep has finished. Interval zero disables work.
func RunRefreshScheduler(ctx context.Context, queries *database.Queries, refresher internal.FeedRefreshManager, interval time.Duration, logger *slog.Logger) error {
	if ctx == nil {
		return errors.New("feed refresh scheduler context is nil")
	}
	if interval < 0 {
		return errors.New("feed refresh interval must not be negative")
	}
	if interval == 0 {
		return nil
	}
	if queries == nil {
		return errors.New("feed refresh scheduler database is nil")
	}
	if refresher == nil {
		return errors.New("feed refresh scheduler refresher is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return runRefreshScheduler(ctx, queries, refresher, interval, logger, waitForRefreshInterval)
}

func runRefreshScheduler(ctx context.Context, lister refreshFeedLister, refresher scheduledFeedRefresher, interval time.Duration, logger *slog.Logger, wait refreshIntervalWaiter) error {
	if interval == 0 {
		return nil
	}
	for {
		if ctx.Err() != nil {
			return nil
		}
		refreshSweep(ctx, lister, refresher, logger)
		if ctx.Err() != nil {
			return nil
		}
		if err := wait(ctx, interval); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
	}
}

func refreshSweep(ctx context.Context, lister refreshFeedLister, refresher scheduledFeedRefresher, logger *slog.Logger) {
	feeds, err := lister.GetFeedsForRefresh(ctx)
	if err != nil {
		if ctx.Err() == nil {
			logger.Error("feed refresh sweep failed", "error", err)
		}
		return
	}
	for _, storedFeed := range feeds {
		if ctx.Err() != nil {
			return
		}
		if err := refresher.Refresh(ctx, storedFeed); err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Error("feed refresh failed",
				"feed_id", storedFeed.ID,
				"feed_name", storedFeed.Name,
				"error", redactRefreshError(err, storedFeed.Url),
			)
		}
	}
}

func redactRefreshError(err error, feedURL string) string {
	message := err.Error()
	if feedURL != "" {
		message = strings.ReplaceAll(message, feedURL, safeRefreshURL(feedURL))
	}
	return refreshErrorURLPattern.ReplaceAllStringFunc(message, safeRefreshURL)
}

func safeRefreshURL(rawURL string) string {
	if queryStart := strings.IndexAny(rawURL, "?#"); queryStart >= 0 {
		rawURL = rawURL[:queryStart]
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return "[redacted URL]"
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	parsed.RawFragment = ""
	return parsed.String()
}

func waitForRefreshInterval(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
