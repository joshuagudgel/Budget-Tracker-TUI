package main

import (
	"budget-tracker-tui/internal/storage"
	"log"

	"budget-tracker-tui/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	store := storage.NewStore()
	if err := store.Init(); err != nil {
		log.Fatalf("unable to init store: %v", err)
	}

	// Ensure proper cleanup of database connection
	defer func() {
		if err := store.Close(); err != nil {
			log.Printf("Error closing store: %v", err)
		}
	}()

	m := ui.NewModel(store)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatalf("unable to run tui: %v", err)
	}
}
