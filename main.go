package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	path, err := DBPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "todo-board: %v\n", err)
		os.Exit(1)
	}

	db, err := OpenDB(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "todo-board: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	m, err := NewModel(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "todo-board: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "todo-board: %v\n", err)
		os.Exit(1)
	}
}
