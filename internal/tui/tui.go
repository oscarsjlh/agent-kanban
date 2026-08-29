// Package tui is the human-facing Bubble Tea frontend to the Board.
// Per ADR-0003 it is a second frontend, not a second process: it calls
// internal/store and internal/domain in-process and never touches the
// SQLite file directly. Agents stay on the CLI.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"kanban/internal/store"
)

// Run starts the TUI program on the given store and blocks until exit.
func Run(s *store.Store) error {
	m := newModel(s)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
