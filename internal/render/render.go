// Package render produces the shared markdown and styled output for both
// CLI frontends (show, resume) and the TUI. The CLI and the TUI are two
// frontends of one Board (ADR-0003); they must never disagree about how an
// Issue is presented, so all rendering lives here.
package render

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/oscarsjlh/agent-kanban/internal/store"
)

// Issue renders the full markdown view used by `kanban show` and the TUI
// full-screen detail.
func Issue(is store.Issue, cs []store.Comment) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Issue %d: %s\n\n", is.ID, is.Title)
	fmt.Fprintf(&b, "Column: %s\n", is.Column)
	if is.RepoName.Valid {
		fmt.Fprintf(&b, "Repo: %s\n", is.RepoName.String)
	} else {
		fmt.Fprintf(&b, "Repo: none\n")
	}
	if is.WaitingReason.Valid {
		fmt.Fprintf(&b, "Waiting reason: %s\n", is.WaitingReason.String)
	}
	if is.BlockedBy.Valid {
		fmt.Fprintf(&b, "Blocked by: #%d\n", is.BlockedBy.Int64)
	}
	fmt.Fprintf(&b, "Created: %s\nUpdated: %s\n\n", is.CreatedAt, is.UpdatedAt)
	b.WriteString("## Body\n\n")
	b.WriteString(strings.TrimRight(is.Body, "\n"))
	b.WriteString("\n\n## Comments\n\n")
	if len(cs) == 0 {
		b.WriteString("No comments yet.\n")
	} else {
		for _, c := range cs {
			fmt.Fprintf(&b, "### %s — %s\n\n%s\n\n", c.Author, c.CreatedAt, strings.TrimRight(c.Body, "\n"))
		}
	}
	return b.String()
}

// Resume renders the injectable briefing used by `kanban resume` and the
// TUI detail pane: latest Resume Note, acceptance-criteria counts, blocker
// state.
func Resume(is store.Issue, notes []store.ResumeNote) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Resume Issue %d: %s\n\n", is.ID, is.Title)
	b.WriteString("## Latest Resume Note\n\n")
	if len(notes) == 0 {
		b.WriteString("No resume notes yet.\n\n")
	} else {
		n := notes[len(notes)-1]
		fmt.Fprintf(&b, "From %s at %s:\n\n%s\n\n", n.Worker, n.CreatedAt, strings.TrimRight(n.Body, "\n"))
	}
	open, checked := CheckboxCounts(is.Body)
	fmt.Fprintf(&b, "## Acceptance Criteria\n\nOpen: %d\nChecked: %d\n\n", open, checked)
	b.WriteString("## Blocker State\n\n")
	if is.BlockedBy.Valid {
		fmt.Fprintf(&b, "Blocked by issue #%d.\n", is.BlockedBy.Int64)
	} else if is.WaitingReason.Valid {
		fmt.Fprintf(&b, "Waiting: %s\n", is.WaitingReason.String)
	} else {
		b.WriteString("No blocker recorded.\n")
	}
	return b.String()
}

// CriteriaSummary renders the compact open/total criteria count used in the
// TUI detail pane, e.g. "3/7 done". Returns "" when the body has no
// checkbox criteria.
func CriteriaSummary(body string) string {
	open, checked := CheckboxCounts(body)
	if open+checked == 0 {
		return ""
	}
	return fmt.Sprintf("%d/%d done", checked, open+checked)
}

var checkboxRe = regexp.MustCompile(`(?m)^\s*- \[([ xX])\]`)

// CheckboxCounts counts markdown checkbox acceptance criteria in a body.
func CheckboxCounts(body string) (open, checked int) {
	for _, m := range checkboxRe.FindAllStringSubmatch(body, -1) {
		if strings.TrimSpace(m[1]) == "" {
			open++
		} else {
			checked++
		}
	}
	return
}
