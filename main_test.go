package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/database"
	_ "modernc.org/sqlite"
)

func TestRunSSHServicesRefreshesWithoutConnectedSSHClients(t *testing.T) {
	requestStarted := make(chan struct{})
	continueResponse := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(requestStarted)
		<-continueResponse
		_, _ = fmt.Fprint(w, `<feed xmlns="http://www.w3.org/2005/Atom"><entry><id>entry-1</id><title>Background post</title></entry></feed>`)
	}))
	defer server.Close()

	state := testMainState(t)
	storedFeed, err := state.Db.CreateFeed(context.Background(), database.CreateFeedParams{Name: "Background", Url: server.URL})
	if err != nil {
		t.Fatalf("create test feed: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	serve := func(ctx context.Context, state *internal.State) error {
		<-requestStarted
		subscription := state.FeedRefresh.Subscribe(ctx)
		defer subscription.Close()
		close(continueResponse)
		feedIDs, err := subscription.Next(ctx)
		if err != nil {
			return fmt.Errorf("wait for background refresh: %w", err)
		}
		if len(feedIDs) != 1 || feedIDs[0] != storedFeed.ID {
			return fmt.Errorf("refresh notifications = %v, want [%d]", feedIDs, storedFeed.ID)
		}
		posts, err := state.Db.GetPostsByFeed(ctx, storedFeed.ID)
		if err != nil {
			return fmt.Errorf("read background posts: %w", err)
		}
		if len(posts) != 1 || posts[0].Title != "Background post" {
			return fmt.Errorf("background posts = %#v, want persisted Background post", posts)
		}
		return nil
	}
	if err := runSSHServices(ctx, state, time.Hour, serve); err != nil {
		t.Fatalf("run server services without an SSH client: %v", err)
	}
	if state.FeedRefresh != nil {
		t.Fatal("closed refresh manager remained on application state")
	}
}

func TestRunSSHServicesCancelsRefreshBeforeDatabaseCanClose(t *testing.T) {
	requestStarted := make(chan struct{})
	requestCanceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-r.Context().Done()
		close(requestCanceled)
	}))
	defer server.Close()

	state := testMainState(t)
	storedFeed, err := state.Db.CreateFeed(context.Background(), database.CreateFeedParams{Name: "Pending", Url: server.URL})
	if err != nil {
		t.Fatalf("create test feed: %v", err)
	}
	serveErr := errors.New("SSH serve stopped")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	serve := func(ctx context.Context, _ *internal.State) error {
		select {
		case <-requestStarted:
		case <-ctx.Done():
			return ctx.Err()
		}
		cancel()
		return serveErr
	}
	if err := runSSHServices(ctx, state, time.Hour, serve); !errors.Is(err, serveErr) {
		t.Fatalf("run services error = %v, want %v", err, serveErr)
	}
	select {
	case <-requestCanceled:
	default:
		t.Fatal("server returned before in-flight feed HTTP request was canceled")
	}
	posts, err := state.Db.GetPostsByFeed(context.Background(), storedFeed.ID)
	if err != nil {
		t.Fatalf("query database after refresh shutdown: %v", err)
	}
	if len(posts) != 0 {
		t.Fatalf("posts after canceled refresh = %d, want no partial writes", len(posts))
	}
	storedAfter, err := state.Db.GetFeedsForRefresh(context.Background())
	if err != nil {
		t.Fatalf("query feeds after refresh shutdown: %v", err)
	}
	if len(storedAfter) != 1 || storedAfter[0].LastFetchedAt.Valid {
		t.Fatalf("feed after canceled refresh = %#v, want unchanged fetch timestamp", storedAfter)
	}
}

func TestRunSSHServicesKeepsManualRefreshWhenSchedulingIsDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `<feed xmlns="http://www.w3.org/2005/Atom"><entry><id>manual-entry</id><title>Manual post</title></entry></feed>`)
	}))
	defer server.Close()
	state := testMainState(t)
	storedFeed, err := state.Db.CreateFeed(context.Background(), database.CreateFeedParams{Name: "Manual", Url: server.URL})
	if err != nil {
		t.Fatalf("create test feed: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	serve := func(ctx context.Context, state *internal.State) error {
		if state.FeedRefresh == nil {
			return errors.New("manual refresh manager unavailable")
		}
		if err := state.FeedRefresh.Refresh(ctx, storedFeed); err != nil {
			return fmt.Errorf("manual refresh: %w", err)
		}
		posts, err := state.Db.GetPostsByFeed(ctx, storedFeed.ID)
		if err != nil {
			return fmt.Errorf("read manual post: %w", err)
		}
		if len(posts) != 1 || posts[0].Title != "Manual post" {
			return fmt.Errorf("manual posts = %#v, want persisted Manual post", posts)
		}
		return nil
	}
	if err := runSSHServices(ctx, state, 0, serve); err != nil {
		t.Fatalf("run services with disabled scheduler and manual refresh: %v", err)
	}
}

func testMainState(t *testing.T) *internal.State {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "server.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	t.Cleanup(func() { _ = db.Close() })
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("sqlite"); err != nil {
		t.Fatalf("set sqlite migration dialect: %v", err)
	}
	if err := goose.Up(db, "sql/schema"); err != nil {
		t.Fatalf("apply test migrations: %v", err)
	}
	return &internal.State{Db: database.New(db), SQLDB: db}
}
