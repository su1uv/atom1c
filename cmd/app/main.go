package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"

	tea "charm.land/bubbletea/v2"
	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/db"
	"github.com/su1uv/atom1c/internal/ui"
)

func main() {
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

	tabs := []string{"Add Feed", "Feeds", "Posts"}
	tabContent := []string{"Lip Gloss Tab", "Blush Tab", "Eye Shadow Tab"}
	m := ui.TabModel{Tabs: tabs, TabContent: tabContent, Styles: ui.NewTabStyles(true)}
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Something went wrong: %v\n", err)
		os.Exit(1)
	}
}
