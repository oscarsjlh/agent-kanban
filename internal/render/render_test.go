package render

import (
	"database/sql"
	"strings"
	"testing"

	"kanban/internal/store"
)

func TestCheckboxCounts(t *testing.T) {
	body := `Some prose.

- [ ] first
- [x] second
- [X] third
- [] not a checkbox flag
  - [ ] indented counts too
`
	open, checked := CheckboxCounts(body)
	if open != 2 || checked != 2 {
		t.Fatalf("got open=%d checked=%d, want open=2 checked=2", open, checked)
	}
}

func TestCheckboxCountsEmpty(t *testing.T) {
	open, checked := CheckboxCounts("no criteria here")
	if open != 0 || checked != 0 {
		t.Fatalf("got open=%d checked=%d, want 0/0", open, checked)
	}
}

func TestCriteriaSummary(t *testing.T) {
	body := "- [x] done\n- [ ] open\n- [ ] open\n"
	if got := CriteriaSummary(body); got != "1/3 done" {
		t.Fatalf("got %q, want %q", got, "1/3 done")
	}
	if got := CriteriaSummary("no checkboxes"); got != "" {
		t.Fatalf("got %q, want empty for bodies without criteria", got)
	}
}

func issueFixture() store.Issue {
	return store.Issue{
		ID:            7,
		Title:         "Ship the TUI",
		Body:          "- [x] grill\n- [ ] build",
		Column:        "In Progress",
		RepoName:      sql.NullString{String: "kanban", Valid: true},
		WaitingReason: sql.NullString{},
		BlockedBy:     sql.NullInt64{Int64: 3, Valid: true},
		CreatedAt:     "2026-07-01T00:00:00Z",
		UpdatedAt:     "2026-07-02T00:00:00Z",
	}
}

func TestIssueGolden(t *testing.T) {
	is := issueFixture()
	cs := []store.Comment{{Author: "oscar", Body: "looks good", CreatedAt: "2026-07-02T01:00:00Z"}}
	got := Issue(is, cs)
	want := `# Issue 7: Ship the TUI

Column: In Progress
Repo: kanban
Blocked by: #3
Created: 2026-07-01T00:00:00Z
Updated: 2026-07-02T00:00:00Z

## Body

- [x] grill
- [ ] build

## Comments

### oscar — 2026-07-02T01:00:00Z

looks good

`
	if got != want {
		t.Errorf("Issue rendering drifted.\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestResumeGolden(t *testing.T) {
	is := issueFixture()
	notes := []store.ResumeNote{{Worker: "pi-run-1", Body: "tried X; failed on Y; next: Z", CreatedAt: "2026-07-02T02:00:00Z"}}
	got := Resume(is, notes)
	want := `# Resume Issue 7: Ship the TUI

## Latest Resume Note

From pi-run-1 at 2026-07-02T02:00:00Z:

tried X; failed on Y; next: Z

## Acceptance Criteria

Open: 1
Checked: 1

## Blocker State

Blocked by issue #3.
`
	if got != want {
		t.Errorf("Resume rendering drifted.\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestResumeNoNotesNoBlocker(t *testing.T) {
	is := issueFixture()
	is.BlockedBy = sql.NullInt64{}
	got := Resume(is, nil)
	if !strings.Contains(got, "No resume notes yet.") {
		t.Error("missing no-notes text")
	}
	if !strings.Contains(got, "No blocker recorded.") {
		t.Error("missing no-blocker text")
	}
}
