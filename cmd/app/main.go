package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/db"
	"github.com/su1uv/atom1c/internal/ui"
)

func main() {

	godotenv.Load()
	cfg := internal.Config{
		CurrentUsername: "su1uv",
		DbURL:           os.Getenv("GOOSE_DBSTRING"),
	}
	state := internal.State{
		Cfg: &cfg,
	}

	dbConn, err := sql.Open("postgres", cfg.DbURL)
	if err != nil {
		log.Fatalf("connection to database failed: %v", err)
	}

	dbQueries := db.New(dbConn)
	state.Db = dbQueries

	p := ui.NewProgram(&state)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Something went wrong: %v\n", err)
		os.Exit(1)
	}
}
