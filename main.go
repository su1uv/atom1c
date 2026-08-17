package main

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/database"
	"github.com/su1uv/atom1c/internal/ui"
	_ "modernc.org/sqlite"
)

//go:embed sql/schema/*.sql
var embedMigrations embed.FS

func main() {
	godotenv.Load()

	cfg := internal.Config{
		CurrentUsername: "su1uv",
		DbURL:           os.Getenv("GOOSE_DBSTRING"),
	}
	state := internal.State{
		Cfg: &cfg,
	}

	db, err := sql.Open("sqlite", cfg.DbURL)
	if err != nil {
		log.Fatalf("connection to database failed: %v", err)
	}
	defer db.Close()

	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("sqlite"); err != nil {
		log.Fatal(err)
	}

	if err := goose.Up(db, "sql/schema"); err != nil {
		log.Fatal(err)
	}

	dbQueries := database.New(db)
	state.Db = dbQueries

	p := ui.NewProgram(&state)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Something went wrong: %v\n", err)
		os.Exit(1)
	}
}
