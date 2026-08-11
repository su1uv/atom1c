package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	_ "github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/su1uv/atom1c/internal"
)

type model struct {
	state *internal.State
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m model) View() tea.View {
	v := tea.NewView("this is a new view")
	return v
}

func initialModel() model {
	godotenv.Load()
	cfg := internal.Config{
		CurrentUsername: "su1uv",
		DbURL:           os.Getenv("GOOSE_DBSTRING"),
	}
	state := internal.State{
		Cfg: &cfg,
	}

	return model{
		state: &state,
	}
}

func main() {

	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Something went wrong: %v\n", err)
		os.Exit(1)
	}
}
