package main

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"os/user"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/database"
	"github.com/su1uv/atom1c/internal/feed"
	atomssh "github.com/su1uv/atom1c/internal/ssh"
	_ "modernc.org/sqlite"
)

//go:embed sql/schema/*.sql
var embedMigrations embed.FS

func main() {
	if err := run(); err != nil {
		log.Printf("atom1c: %v", err)
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load()
	owner, err := user.Current()
	if err != nil {
		return fmt.Errorf("resolve operating-system user: %w", err)
	}
	sshConfig, err := atomssh.LoadConfig(os.Getenv, owner.HomeDir, owner.Username)
	if err != nil {
		return err
	}
	refreshInterval, err := internal.LoadRefreshInterval(os.LookupEnv)
	if err != nil {
		return err
	}
	dbURL := os.Getenv("GOOSE_DBSTRING")
	if dbURL == "" {
		return fmt.Errorf("GOOSE_DBSTRING is required")
	}
	separator := "?"
	if strings.Contains(dbURL, "?") {
		separator = "&"
	}
	db, err := sql.Open("sqlite", dbURL+separator+"_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()
	// Serializing SQLite work across SSH sessions avoids writer-lock races while
	// keeping network fetches and all TUI work outside database connections.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("sqlite"); err != nil {
		return err
	}

	if err := goose.Up(db, "sql/schema"); err != nil {
		return fmt.Errorf("apply database migrations: %w", err)
	}

	dbQueries := database.New(db)
	state := internal.State{Db: dbQueries, SQLDB: db}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runSSHServices(ctx, &state, refreshInterval, func(ctx context.Context, state *internal.State) error {
		return atomssh.Run(ctx, sshConfig, state)
	})
}

func runSSHServices(ctx context.Context, state *internal.State, refreshInterval time.Duration, serve func(context.Context, *internal.State) error) error {
	if ctx == nil {
		return errors.New("server context is nil")
	}
	if state == nil || state.Db == nil || state.SQLDB == nil {
		return errors.New("server database state is incomplete")
	}
	if serve == nil {
		return errors.New("SSH server runner is nil")
	}
	serviceCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	refreshManager, err := feed.NewRefreshCoordinator(serviceCtx, state.SQLDB)
	if err != nil {
		return err
	}
	previousRefreshManager := state.FeedRefresh
	state.FeedRefresh = refreshManager
	defer func() { state.FeedRefresh = previousRefreshManager }()

	schedulerDone := make(chan error, 1)
	go func() {
		schedulerDone <- feed.RunRefreshScheduler(serviceCtx, state.Db, refreshManager, refreshInterval, slog.Default())
	}()
	serveErr := serve(serviceCtx, state)
	cancel()
	schedulerErr := <-schedulerDone
	refreshManager.Close()
	return errors.Join(serveErr, schedulerErr)
}
