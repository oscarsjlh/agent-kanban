package tui

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"kanban/internal/store"
)

// upd drives a key/msg through the model and returns the updated model.
func upd(m *Model, msg tea.Msg) *Model {
	mm, _ := m.Update(msg)
	return mm.(*Model)
}

// tempStore opens a throwaway Board for the test.
func tempStore(t *testing.T) *store.Store {
	t.Helper()
	t.Setenv("KANBAN_DB", filepath.Join(t.TempDir(), "kanban.db"))
	s, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// TestModelSmoke drives the TUI model through window sizing, a poll tick,
// and basic navigation against a real Board. It asserts no panics and sane
// selection state — the deep Board rules are already covered by the CLI
// e2e seam.
func TestModelSmoke(t *testing.T) {
	s := tempStore(t)
	id, err := s.CreateIssue("smoke issue", "body with\n- [ ] criterion", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Move(id, "Ready", "", nil); err != nil {
		t.Fatal(err)
	}

	m := newModel(s)
	m = upd(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m = upd(m, tickMsg{})

	if len(m.laneIssues("Ready")) != 1 {
		t.Fatalf("want 1 issue in Ready lane, got %d", len(m.laneIssues("Ready")))
	}
	// navigate to the Ready lane
	for m.lanes()[m.laneIdx] != "Ready" {
		m = upd(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	}
	if sel := m.selected(); sel == nil || sel.ID != id {
		t.Fatalf("expected issue %d selected, got %+v", id, sel)
	}
	// search filter
	m.search = "nomatch"
	if len(m.laneIssues("Ready")) != 0 {
		t.Fatal("search filter did not apply")
	}
	m.search = ""
	// help opens and closes
	m = upd(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if m.mode != modeHelp {
		t.Fatal("? did not open help")
	}
	m = upd(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if m.mode != modeBoard {
		t.Fatal("q did not close help")
	}
}

// TestMoveMenuLegalDests checks the move menu only offers domain-legal
// destinations: a Ready issue may go to Waiting/Done/wontfix but not back
// to Inbox.
func TestMoveMenuLegalDests(t *testing.T) {
	s := tempStore(t)
	id, _ := s.CreateIssue("dests", "", "")
	_ = s.Move(id, "Ready", "", nil)
	m := newModel(s)
	m = upd(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m = upd(m, tickMsg{})

	is := m.issues[0]
	dests := m.moveDests(&is)
	joined := ""
	for _, d := range dests {
		joined += d + ","
	}
	if joined != "Waiting,Done,wontfix," {
		t.Fatalf("got destinations %q, want Waiting,Done,wontfix", joined)
	}
}
