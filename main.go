package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"
	"os"
	"os/signal"
	"os/user"
	"strings"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/database"
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
	return atomssh.Run(ctx, sshConfig, &state)
}
