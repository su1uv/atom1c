package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/su1uv/atom1c/internal/tui"
)

func main() {
	p := tea.NewProgram(tui.InitialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Something went wrong: %v\n", err)
		os.Exit(1)
	}
}
