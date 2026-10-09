package feed

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/su1uv/atom1c/internal/database"
)

func TestRunRefreshSchedulerSweepsImmediatelySequentiallyAndContinuesOnFeedErrors(t *testing.T) {
	feeds := []database.Feed{{ID: 1, Name: "First"}, {ID: 2, Name: "Broken"}, {ID: 3, Name: "Last"}}
	lister := &scriptedRefreshFeedLister{results: []refreshFeedListResult{{feeds: feeds}}}
	var logOutput bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logOutput, nil))
	var active atomic.Int32
	var maxActive atomic.Int32
	var mu sync.Mutex
	var attempted []int64
	refresher := schedulerRefreshFunc(func(_ context.Context, stored database.Feed) error {
		current := active.Add(1)
		for prior := maxActive.Load(); current > prior && !maxActive.CompareAndSwap(prior, current); prior = maxActive.Load() {
		}
		defer active.Add(-1)
		mu.Lock()
		attempted = append(attempted, stored.ID)
		mu.Unlock()
		if stored.ID == 2 {
			return errors.New("feed offline")
		}
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	waitCalls := 0
	err := runRefreshScheduler(ctx, lister, refresher, 7*time.Minute, logger, func(waitCtx context.Context, interval time.Duration) error {
		waitCalls++
		if interval != 7*time.Minute {
			t.Errorf("wait interval = %v, want 7m", interval)
		}
		mu.Lock()
		got := append([]int64(nil), attempted...)
		mu.Unlock()
		if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
			t.Errorf("attempted feeds before interval wait = %v, want [1 2 3]", got)
		}
		cancel()
		return waitCtx.Err()
	})
	if err != nil {
		t.Fatalf("run scheduler: %v", err)
	}
	if waitCalls != 1 {
		t.Fatalf("interval waits = %d, want one after the immediate sweep", waitCalls)
	}
	if got := maxActive.Load(); got != 1 {
		t.Fatalf("maximum concurrent scheduled feeds = %d, want sequential execution", got)
	}
	if !strings.Contains(logOutput.String(), "feed refresh failed") || !strings.Contains(logOutput.String(), "Broken") {
		t.Fatalf("log output %q does not include failed feed details", logOutput.String())
	}
}

func TestRunRefreshSchedulerRetriesEnumerationAtNextIntervalAndSeesNewFeeds(t *testing.T) {
	lister := &scriptedRefreshFeedLister{results: []refreshFeedListResult{
		{err: errors.New("database busy")},
		{feeds: []database.Feed{{ID: 1, Name: "Existing"}}},
		{feeds: []database.Feed{{ID: 1, Name: "Existing"}, {ID: 2, Name: "Added during runtime"}}},
	}}
	var logOutput bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logOutput, nil))
	var refreshed []int64
	refresher := schedulerRefreshFunc(func(_ context.Context, stored database.Feed) error {
		refreshed = append(refreshed, stored.ID)
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	waits := 0
	err := runRefreshScheduler(ctx, lister, refresher, time.Minute, logger, func(waitCtx context.Context, interval time.Duration) error {
		waits++
		if interval != time.Minute {
			t.Errorf("wait interval = %v, want 1m", interval)
		}
		if waits == 3 {
			cancel()
			return waitCtx.Err()
		}
		return nil
	})
	if err != nil {
		t.Fatalf("run scheduler: %v", err)
	}
	if waits != 3 {
		t.Fatalf("interval waits = %d, want three completed sweeps", waits)
	}
	if len(refreshed) != 3 || refreshed[0] != 1 || refreshed[1] != 1 || refreshed[2] != 2 {
		t.Fatalf("refreshed feed IDs = %v, want [1 1 2] across snapshots", refreshed)
	}
	if got := lister.calls; got != 3 {
		t.Fatalf("feed enumeration calls = %d, want 3", got)
	}
	if !strings.Contains(logOutput.String(), "feed refresh sweep failed") || !strings.Contains(logOutput.String(), "database busy") {
		t.Fatalf("log output %q does not include enumeration failure", logOutput.String())
	}
}

func TestRunRefreshSchedulerHandlesEmptyAndDisabledSchedules(t *testing.T) {
	t.Run("empty sweep waits after enumeration", func(t *testing.T) {
		lister := &scriptedRefreshFeedLister{results: []refreshFeedListResult{{feeds: nil}}}
		refresher := schedulerRefreshFunc(func(context.Context, database.Feed) error {
			t.Fatal("refresher called for empty feed snapshot")
			return nil
		})
		ctx, cancel := context.WithCancel(context.Background())
		waited := false
		err := runRefreshScheduler(ctx, lister, refresher, time.Minute, slog.Default(), func(waitCtx context.Context, _ time.Duration) error {
			waited = true
			cancel()
			return waitCtx.Err()
		})
		if err != nil {
			t.Fatalf("run empty sweep: %v", err)
		}
		if !waited {
			t.Fatal("scheduler did not wait after empty sweep")
		}
	})

	t.Run("zero interval skips startup sweep", func(t *testing.T) {
		lister := &scriptedRefreshFeedLister{results: []refreshFeedListResult{{feeds: []database.Feed{{ID: 1}}}}}
		refresher := schedulerRefreshFunc(func(context.Context, database.Feed) error {
			t.Fatal("refresher called with disabled scheduling")
			return nil
		})
		waited := false
		if err := runRefreshScheduler(context.Background(), lister, refresher, 0, slog.Default(), func(context.Context, time.Duration) error {
			waited = true
			return nil
		}); err != nil {
			t.Fatalf("run disabled scheduler: %v", err)
		}
		if lister.calls != 0 || waited {
			t.Fatalf("disabled scheduler did work: enumerations=%d waited=%v", lister.calls, waited)
		}
	})
}

func TestRunRefreshSchedulerStopsOnCancellationDuringSweep(t *testing.T) {
	lister := &scriptedRefreshFeedLister{results: []refreshFeedListResult{{feeds: []database.Feed{{ID: 1}, {ID: 2}}}}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var attempted []int64
	refresher := schedulerRefreshFunc(func(_ context.Context, stored database.Feed) error {
		attempted = append(attempted, stored.ID)
		cancel()
		return context.Canceled
	})
	err := runRefreshScheduler(ctx, lister, refresher, time.Minute, slog.Default(), func(context.Context, time.Duration) error {
		t.Fatal("scheduler waited after cancellation")
		return nil
	})
	if err != nil {
		t.Fatalf("run canceled scheduler: %v", err)
	}
	if len(attempted) != 1 || attempted[0] != 1 {
		t.Fatalf("attempted feeds after cancellation = %v, want [1]", attempted)
	}
}

type refreshFeedListResult struct {
	feeds []database.Feed
	err   error
}

type scriptedRefreshFeedLister struct {
	mu      sync.Mutex
	results []refreshFeedListResult
	calls   int
}

func (l *scriptedRefreshFeedLister) GetFeedsForRefresh(context.Context) ([]database.Feed, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.calls >= len(l.results) {
		return nil, errors.New("unexpected feed enumeration")
	}
	result := l.results[l.calls]
	l.calls++
	return result.feeds, result.err
}

type schedulerRefreshFunc func(context.Context, database.Feed) error

func (f schedulerRefreshFunc) Refresh(ctx context.Context, stored database.Feed) error {
	return f(ctx, stored)
}
