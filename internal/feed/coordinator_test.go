package feed

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/database"
)

func TestRefreshCoordinatorSharesInFlightResult(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	wantErr := errors.New("refresh failed")
	var calls atomic.Int32
	coordinator := newRefreshCoordinator(context.Background(), func(ctx context.Context, _ database.Feed) error {
		calls.Add(1)
		started <- struct{}{}
		select {
		case <-release:
			return wantErr
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	defer coordinator.Close()

	feed := database.Feed{ID: 42, Url: "https://example.test/feed"}
	results := make(chan error, 2)
	go func() { results <- coordinator.Refresh(context.Background(), feed) }()
	<-started
	go func() { results <- coordinator.Refresh(context.Background(), feed) }()
	waitForRefreshCallers(t, coordinator, feed.ID, 2)
	close(release)

	for range 2 {
		if err := <-results; !errors.Is(err, wantErr) {
			t.Errorf("Refresh() error = %v, want shared error %v", err, wantErr)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("refresh calls = %d, want one shared call", got)
	}
	if err := coordinator.Refresh(context.Background(), feed); !errors.Is(err, wantErr) {
		t.Fatalf("refresh after completed failure = %v, want a fresh failure %v", err, wantErr)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("refresh calls after retry = %d, want a new operation", got)
	}
}

func TestRefreshCoordinatorRunsDifferentFeedsIndependently(t *testing.T) {
	started := make(chan int64, 2)
	release := make(chan struct{})
	coordinator := newRefreshCoordinator(context.Background(), func(ctx context.Context, stored database.Feed) error {
		started <- stored.ID
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	defer coordinator.Close()

	results := make(chan error, 2)
	for _, id := range []int64{1, 2} {
		go func(feedID int64) {
			results <- coordinator.Refresh(context.Background(), database.Feed{ID: feedID})
		}(id)
	}
	startedFeeds := map[int64]bool{}
	for range 2 {
		select {
		case feedID := <-started:
			startedFeeds[feedID] = true
		case <-time.After(time.Second):
			t.Fatal("second feed did not start while first feed was still blocked")
		}
	}
	close(release)
	for range 2 {
		if err := <-results; err != nil {
			t.Errorf("Refresh() error = %v, want nil", err)
		}
	}
	if !startedFeeds[1] || !startedFeeds[2] {
		t.Fatalf("started feeds = %v, want both feed IDs", startedFeeds)
	}
}

func TestRefreshCoordinatorCallerCancellationIsIndependent(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	operationCanceled := make(chan struct{})
	coordinator := newRefreshCoordinator(context.Background(), func(ctx context.Context, _ database.Feed) error {
		started <- struct{}{}
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			close(operationCanceled)
			return ctx.Err()
		}
	})
	defer coordinator.Close()

	feed := database.Feed{ID: 7}
	firstCtx, cancelFirst := context.WithCancel(context.Background())
	firstResult := make(chan error, 1)
	secondResult := make(chan error, 1)
	go func() { firstResult <- coordinator.Refresh(firstCtx, feed) }()
	<-started
	go func() { secondResult <- coordinator.Refresh(context.Background(), feed) }()
	waitForRefreshCallers(t, coordinator, feed.ID, 2)
	cancelFirst()
	if err := <-firstResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled caller error = %v, want context.Canceled", err)
	}
	select {
	case <-operationCanceled:
		t.Fatal("shared refresh was canceled while another caller remained")
	default:
	}
	close(release)
	if err := <-secondResult; err != nil {
		t.Fatalf("remaining caller error = %v, want nil", err)
	}
}

func TestRefreshCoordinatorCancelsAfterLastCallerLeaves(t *testing.T) {
	started := make(chan struct{}, 1)
	operationCanceled := make(chan struct{}, 1)
	var calls atomic.Int32
	coordinator := newRefreshCoordinator(context.Background(), func(ctx context.Context, _ database.Feed) error {
		if calls.Add(1) == 2 {
			return nil
		}
		started <- struct{}{}
		<-ctx.Done()
		operationCanceled <- struct{}{}
		return ctx.Err()
	})
	defer coordinator.Close()

	feed := database.Feed{ID: 8}
	callerCtx, cancel := context.WithCancel(context.Background())
	firstResult := make(chan error, 1)
	go func() { firstResult <- coordinator.Refresh(callerCtx, feed) }()
	<-started
	cancel()
	if err := <-firstResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("last caller error = %v, want context.Canceled", err)
	}
	select {
	case <-operationCanceled:
	case <-time.After(time.Second):
		t.Fatal("refresh operation did not observe last-caller cancellation")
	}
	if err := coordinator.Refresh(context.Background(), feed); err != nil {
		t.Fatalf("new request after canceled operation: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("refresh calls = %d, want canceled operation and a fresh request", got)
	}
}

func TestRefreshCoordinatorCloseCancelsAndWaits(t *testing.T) {
	started := make(chan struct{}, 1)
	operationCanceled := make(chan struct{})
	coordinator := newRefreshCoordinator(context.Background(), func(ctx context.Context, _ database.Feed) error {
		started <- struct{}{}
		<-ctx.Done()
		close(operationCanceled)
		return ctx.Err()
	})

	result := make(chan error, 1)
	go func() {
		result <- coordinator.Refresh(context.Background(), database.Feed{ID: 9})
	}()
	<-started
	coordinator.Close()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("refresh result after Close() = %v, want context.Canceled", err)
	}
	select {
	case <-operationCanceled:
	default:
		t.Fatal("Close() returned before in-flight operation observed cancellation")
	}
	if err := coordinator.Refresh(context.Background(), database.Feed{ID: 9}); !errors.Is(err, errRefreshCoordinatorClosed) {
		t.Fatalf("Refresh() after Close() = %v, want coordinator closed error", err)
	}
}

func TestRefreshCoordinatorCoalescesSuccessNotifications(t *testing.T) {
	wantErr := errors.New("refresh error")
	coordinator := newRefreshCoordinator(context.Background(), func(_ context.Context, stored database.Feed) error {
		if stored.ID == 3 {
			return wantErr
		}
		return nil
	})
	defer coordinator.Close()
	subscription := coordinator.Subscribe(context.Background())
	defer subscription.Close()
	secondSubscription := coordinator.Subscribe(context.Background())
	defer secondSubscription.Close()
	subscription.Watch(1)
	secondSubscription.Watch(2)

	for _, id := range []int64{1, 2, 3} {
		if err := coordinator.Refresh(context.Background(), database.Feed{ID: id}); id == 3 {
			if !errors.Is(err, wantErr) {
				t.Fatalf("failed refresh error = %v, want %v", err, wantErr)
			}
		} else if err != nil {
			t.Fatalf("successful refresh %d: %v", id, err)
		}
	}

	for _, test := range []struct {
		listener internal.FeedRefreshSubscription
		feedID   int64
	}{{listener: subscription, feedID: 1}, {listener: secondSubscription, feedID: 2}} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		notification, err := test.listener.Next(ctx)
		cancel()
		if err != nil {
			t.Fatalf("read refresh notifications: %v", err)
		}
		if notification.FeedID != test.feedID {
			t.Fatalf("refresh notification = %#v, want feed ID %d for connected session", notification, test.feedID)
		}
	}
}

func TestRefreshSubscriptionTracksOnlyWatchedFeedAndCoalesces(t *testing.T) {
	coordinator := newRefreshCoordinator(context.Background(), func(context.Context, database.Feed) error { return nil })
	defer coordinator.Close()
	subscription := coordinator.Subscribe(context.Background())
	defer subscription.Close()
	subscription.Watch(1)

	for _, feedID := range []int64{1, 1, 2} {
		if err := coordinator.Refresh(context.Background(), database.Feed{ID: feedID}); err != nil {
			t.Fatalf("refresh feed %d: %v", feedID, err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	notification, err := subscription.Next(ctx)
	cancel()
	if err != nil {
		t.Fatalf("read watched-feed notification: %v", err)
	}
	if notification.FeedID != 1 {
		t.Fatalf("watched notification = %#v, want feed ID 1", notification)
	}

	if err := coordinator.Refresh(context.Background(), database.Feed{ID: 1}); err != nil {
		t.Fatalf("refresh feed 1 before changing watch: %v", err)
	}
	subscription.Watch(2)
	for _, feedID := range []int64{1, 2} {
		if err := coordinator.Refresh(context.Background(), database.Feed{ID: feedID}); err != nil {
			t.Fatalf("refresh feed %d after watch change: %v", feedID, err)
		}
	}
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	notification, err = subscription.Next(ctx)
	cancel()
	if err != nil {
		t.Fatalf("read notification after watch change: %v", err)
	}
	if notification.FeedID != 2 {
		t.Fatalf("notification after watch change = %#v, want feed ID 2", notification)
	}
}

func TestRefreshCoordinatorPersistsBeforeNotifying(t *testing.T) {
	db, queries := openRefreshTestDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(atomDocument(atomEntry("entry-1", "One", "https://example.test/one", ""))))
	}))
	defer server.Close()
	storedFeed := createRefreshFeed(t, queries, server.URL)
	coordinator, err := NewRefreshCoordinator(context.Background(), db)
	if err != nil {
		t.Fatalf("create refresh coordinator: %v", err)
	}
	defer coordinator.Close()
	subscription := coordinator.Subscribe(context.Background())
	defer subscription.Close()
	subscription.Watch(storedFeed.ID)

	if err := coordinator.Refresh(context.Background(), storedFeed); err != nil {
		t.Fatalf("refresh feed: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	notification, err := subscription.Next(ctx)
	if err != nil {
		t.Fatalf("read success notification: %v", err)
	}
	if notification.FeedID != storedFeed.ID {
		t.Fatalf("notification = %#v, want feed ID %d", notification, storedFeed.ID)
	}
	posts, err := queries.GetPostsByFeed(context.Background(), storedFeed.ID)
	if err != nil {
		t.Fatalf("read posts after notification: %v", err)
	}
	if len(posts) != 1 || posts[0].Title != "One" {
		t.Fatalf("persisted posts = %#v, want committed post One before notification", posts)
	}
}

func waitForRefreshCallers(t *testing.T, coordinator *RefreshCoordinator, feedID int64, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		coordinator.mu.Lock()
		operation := coordinator.operations[feedID]
		got := 0
		if operation != nil {
			got = operation.callers
		}
		coordinator.mu.Unlock()
		if got == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("feed %d callers did not reach %d", feedID, want)
}
