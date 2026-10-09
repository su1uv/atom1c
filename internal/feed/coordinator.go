package feed

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/database"
)

const refreshOperationTimeout = 15 * time.Second
const maxCoalescedRefreshFeedIDs = 64

var errRefreshCoordinatorClosed = errors.New("feed refresh coordinator is closed")

type refreshOperationFunc func(context.Context, database.Feed) error

// RefreshCoordinator shares one bounded refresh operation among callers for the
// same feed and publishes feed IDs after a successful transaction commits.
type RefreshCoordinator struct {
	mu           sync.Mutex
	ctx          context.Context
	cancel       context.CancelFunc
	refresh      refreshOperationFunc
	operations   map[int64]*refreshOperation
	subscribers  map[uint64]*refreshSubscription
	nextSubID    uint64
	closed       bool
	operationsWG sync.WaitGroup
}

type refreshOperation struct {
	feedID   int64
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	callers  int
	err      error
	complete bool
}

type refreshSubscription struct {
	mu        sync.Mutex
	owner     *RefreshCoordinator
	id        uint64
	feedIDs   map[int64]struct{}
	reconcile bool
	wake      chan struct{}
	done      chan struct{}
	closed    bool
	once      sync.Once
}

// NewRefreshCoordinator creates a process-owned refresh coordinator backed by
// the application's SQLite database.
func NewRefreshCoordinator(parent context.Context, db *sql.DB) (*RefreshCoordinator, error) {
	if parent == nil {
		return nil, errors.New("feed refresh parent context is nil")
	}
	if db == nil {
		return nil, errors.New("feed refresh database is nil")
	}
	return newRefreshCoordinator(parent, func(ctx context.Context, storedFeed database.Feed) error {
		return RefreshFeed(ctx, db, storedFeed)
	}), nil
}

func newRefreshCoordinator(parent context.Context, refresh refreshOperationFunc) *RefreshCoordinator {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	return &RefreshCoordinator{
		ctx:         ctx,
		cancel:      cancel,
		refresh:     refresh,
		operations:  make(map[int64]*refreshOperation),
		subscribers: make(map[uint64]*refreshSubscription),
	}
}

// Refresh joins an existing operation for the feed or starts one. Caller
// cancellation only leaves that request; shared work continues while another
// caller remains and is canceled when the final caller leaves.
func (c *RefreshCoordinator) Refresh(ctx context.Context, storedFeed database.Feed) error {
	if ctx == nil {
		return errors.New("feed refresh caller context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return errRefreshCoordinatorClosed
	}
	op := c.operations[storedFeed.ID]
	if op == nil {
		opCtx, cancel := context.WithTimeout(c.ctx, refreshOperationTimeout)
		op = &refreshOperation{
			feedID: storedFeed.ID,
			ctx:    opCtx,
			cancel: cancel,
			done:   make(chan struct{}),
		}
		c.operations[storedFeed.ID] = op
		c.operationsWG.Add(1)
		go c.runRefresh(op, storedFeed)
	}
	op.callers++
	c.mu.Unlock()

	select {
	case <-op.done:
		c.leaveOperation(op)
		return op.err
	case <-ctx.Done():
		select {
		case <-op.done:
			c.leaveOperation(op)
			return op.err
		default:
		}
		c.leaveOperation(op)
		return ctx.Err()
	}
}

func (c *RefreshCoordinator) runRefresh(op *refreshOperation, storedFeed database.Feed) {
	defer c.operationsWG.Done()
	defer op.cancel()
	err := c.refresh(op.ctx, storedFeed)

	c.mu.Lock()
	if c.operations[op.feedID] == op {
		delete(c.operations, op.feedID)
	}
	op.err = err
	op.complete = true
	close(op.done)
	if err == nil && !c.closed {
		for _, subscriber := range c.subscribers {
			subscriber.notify(op.feedID)
		}
	}
	c.mu.Unlock()
}

func (c *RefreshCoordinator) leaveOperation(op *refreshOperation) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if op.callers == 0 {
		return
	}
	op.callers--
	if op.callers == 0 && !op.complete {
		if c.operations[op.feedID] == op {
			delete(c.operations, op.feedID)
		}
		op.cancel()
	}
}

// Subscribe registers a session-scoped coalescing notification subscription.
// The subscription removes itself when ctx is canceled.
func (c *RefreshCoordinator) Subscribe(ctx context.Context) internal.FeedRefreshSubscription {
	if ctx == nil {
		ctx = context.Background()
	}
	subscription := &refreshSubscription{
		owner:   c,
		feedIDs: make(map[int64]struct{}),
		wake:    make(chan struct{}, 1),
		done:    make(chan struct{}),
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		subscription.markClosed()
		return subscription
	}
	c.nextSubID++
	subscription.id = c.nextSubID
	c.subscribers[subscription.id] = subscription
	c.mu.Unlock()
	context.AfterFunc(ctx, subscription.Close)
	return subscription
}

// Close rejects new requests, cancels active work, closes subscriptions, and
// waits until every refresh operation has returned before callers close SQLite.
func (c *RefreshCoordinator) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		c.operationsWG.Wait()
		return
	}
	c.closed = true
	c.cancel()
	subscribers := make([]*refreshSubscription, 0, len(c.subscribers))
	for _, subscription := range c.subscribers {
		subscribers = append(subscribers, subscription)
	}
	c.subscribers = make(map[uint64]*refreshSubscription)
	c.mu.Unlock()

	for _, subscription := range subscribers {
		subscription.markClosed()
	}
	c.operationsWG.Wait()
}

func (s *refreshSubscription) Next(ctx context.Context) (internal.FeedRefreshNotification, error) {
	if ctx == nil {
		return internal.FeedRefreshNotification{}, errors.New("feed refresh subscription context is nil")
	}
	select {
	case <-ctx.Done():
		s.Close()
		return internal.FeedRefreshNotification{}, ctx.Err()
	case <-s.done:
		return internal.FeedRefreshNotification{}, io.EOF
	case <-s.wake:
	}
	if err := ctx.Err(); err != nil {
		s.Close()
		return internal.FeedRefreshNotification{}, err
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return internal.FeedRefreshNotification{}, io.EOF
	}
	select {
	case <-s.wake:
	default:
	}
	notification := internal.FeedRefreshNotification{FeedIDs: make([]int64, 0, len(s.feedIDs)), Reconcile: s.reconcile}
	for feedID := range s.feedIDs {
		notification.FeedIDs = append(notification.FeedIDs, feedID)
	}
	s.feedIDs = make(map[int64]struct{})
	s.reconcile = false
	s.mu.Unlock()
	sort.Slice(notification.FeedIDs, func(i, j int) bool { return notification.FeedIDs[i] < notification.FeedIDs[j] })
	return notification, nil
}

func (s *refreshSubscription) notify(feedID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.reconcile {
		return
	}
	if _, exists := s.feedIDs[feedID]; !exists && len(s.feedIDs) >= maxCoalescedRefreshFeedIDs {
		s.feedIDs = make(map[int64]struct{})
		s.reconcile = true
	} else {
		s.feedIDs[feedID] = struct{}{}
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *refreshSubscription) Close() {
	s.once.Do(func() {
		s.markClosed()
		if s.owner != nil {
			s.owner.removeSubscription(s)
		}
	})
}

func (s *refreshSubscription) markClosed() {
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		s.feedIDs = nil
		s.reconcile = false
		close(s.done)
	}
	s.mu.Unlock()
}

func (c *RefreshCoordinator) removeSubscription(subscription *refreshSubscription) {
	c.mu.Lock()
	if c.subscribers[subscription.id] == subscription {
		delete(c.subscribers, subscription.id)
	}
	c.mu.Unlock()
}
