package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	styleTitle    = lipgloss.NewStyle().Bold(true)
	styleLaneHead = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	styleSelected = lipgloss.NewStyle().Background(lipgloss.Color("237")).Bold(true)
	styleHint     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	styleErr      = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	styleOk       = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleMenu     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	styleMenuSel  = lipgloss.NewStyle().Background(lipgloss.Color("237"))
	styleStatus   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	styleClaimed  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
)

func (m *Model) View() string {
	if m.width == 0 {
		return "starting…"
	}
	switch m.mode {
	case modeHelp:
		return m.centered(m.helpView())
	case modeDetail:
		return m.detailView()
	case modeMoveMenu:
		return m.overlay(m.moveMenuView())
	case modeRepoPicker:
		return m.overlay(m.repoPickerView())
	case modeStopChoice:
		return m.overlay(m.stopChoiceView())
	case modeInput:
		return m.overlay(m.inputView())
	default:
		return m.boardView()
	}
}

func (m *Model) boardView() string {
	var b strings.Builder
	header := styleTitle.Render("kanban board") + "  " + styleHint.Render("worker: "+m.worker+" (human)")
	if m.repoFilter != "" || m.search != "" {
		var parts []string
		if m.repoFilter != "" {
			parts = append(parts, "repo:"+m.repoFilter)
		}
		if m.search != "" {
			parts = append(parts, "search:"+m.search)
		}
		header += "  " + styleHint.Render("filters: "+strings.Join(parts, " · "))
	}
	b.WriteString(header + "\n\n")

	lanes := m.lanes()
	showDetail := m.detailWidth() > 0
	laneWidth := m.width / len(lanes)
	if laneWidth < 16 {
		laneWidth = 16
	}

	var cols []string
	for i, lane := range lanes {
		cols = append(cols, m.laneView(lane, laneWidth-1, i == m.laneIdx))
	}
	board := lipgloss.JoinHorizontal(lipgloss.Top, cols...)
	if showDetail {
		detail := m.detailPaneView(m.detailWidth())
		board = lipgloss.JoinHorizontal(lipgloss.Top, board, "  ", detail)
	}
	b.WriteString(board + "\n\n")
	b.WriteString(m.statusBar())
	return b.String()
}

func (m *Model) detailWidth() int {
	lanes := len(m.lanes())
	boardMin := lanes * 16
	if m.width-boardMin >= 40 {
		return m.width - boardMin - 4
	}
	return 0
}

func (m *Model) laneView(col string, width int, focused bool) string {
	issues := m.laneIssues(col)
	head := styleLaneHead.Render(col) + styleHint.Render(" ("+itoa(len(issues))+")")
	var b strings.Builder
	b.WriteString(padRight(head, width) + "\n")
	b.WriteString(strings.Repeat("─", min(width, 24)) + "\n")
	if len(issues) == 0 {
		b.WriteString(styleHint.Render(trunc("  empty", width)) + "\n")
	}
	cur := m.cursors[col]
	for i, is := range issues {
		line := "  #" + itoa(int(is.ID)) + " " + is.Title
		if c, claimed := m.claims[is.ID]; claimed {
			line = "▣ #" + itoa(int(is.ID)) + " " + is.Title
			if i == cur && focused {
				line += "  [" + c.Worker + "]"
			}
		}
		if is.Column == "Waiting" && is.WaitingReason.Valid {
			line += "  — " + is.WaitingReason.String
		}
		line = trunc(line, width)
		if i == cur && focused {
			line = styleSelected.Render(padRight(line, width))
		} else if i == cur {
			line = styleHint.Render(line)
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func (m *Model) detailPaneView(width int) string {
	is := m.selected()
	if is == nil {
		return styleHint.Render("no issue selected")
	}
	var b strings.Builder
	b.WriteString(styleTitle.Render("#"+itoa(int(is.ID))) + "\n\n")
	b.WriteString(m.detailMarkdown(is, width-2))
	return trunc(b.String(), width*m.height)
}

func (m *Model) detailView() string {
	var b strings.Builder
	is := m.selected()
	title := "issue detail"
	if is != nil {
		title = "#" + itoa(int(is.ID)) + " " + is.Title
	}
	b.WriteString(styleTitle.Render(title) + "  " + styleHint.Render("(q/esc to close, ↑↓ to scroll)") + "\n\n")
	b.WriteString(m.vp.View() + "\n")
	b.WriteString(m.statusBar())
	return b.String()
}

func (m *Model) moveMenuView() string {
	is := m.selected()
	if is == nil {
		return "no issue selected"
	}
	var b strings.Builder
	b.WriteString(styleTitle.Render("move #"+itoa(int(is.ID))+" from "+is.Column) + "\n\n")
	for i, d := range m.moveDests(is) {
		line := "  " + d
		if i == m.menuIdx {
			line = styleMenuSel.Render("> " + d)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + styleHint.Render("enter: move · esc: cancel"))
	return styleMenu.Render(b.String())
}

func (m *Model) repoPickerView() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("filter by repo") + "\n\n")
	entries := []string{"all repos"}
	for _, r := range m.repos {
		entries = append(entries, r.Name)
	}
	for i, e := range entries {
		line := "  " + e
		if m.repoFilter == e || (m.repoFilter == "" && i == 0) {
			e2 := e + " (active)"
			if i == m.menuIdx {
				line = styleMenuSel.Render("> " + e2)
			} else {
				line = "  " + styleOk.Render(e2)
			}
		} else if i == m.menuIdx {
			line = styleMenuSel.Render("> " + e)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + styleHint.Render("enter: apply · esc: cancel"))
	return styleMenu.Render(b.String())
}

func (m *Model) stopChoiceView() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("release claim") + "\n\n")
	opts := []string{"inline resume note, then stop", "resume note in $EDITOR, then stop", "stop without a note (next worker starts cold)"}
	for i, o := range opts {
		line := "  " + o
		if i == m.menuIdx {
			line = styleMenuSel.Render("> " + o)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + styleHint.Render("i: inline · e: editor · s/enter: skip · esc: cancel"))
	return styleMenu.Render(b.String())
}

func (m *Model) inputView() string {
	var b strings.Builder
	label := ""
	switch m.inputFor {
	case inputSearch:
		label = "search titles"
	case inputReason:
		label = "waiting reason"
	case inputComment:
		label = "comment on #" + itoa(int(m.selectedID))
	case inputNote:
		label = "resume note on #" + itoa(int(m.selectedID))
	case inputStopNote:
		label = "resume note + release claim on #" + itoa(int(m.selectedID))
	}
	b.WriteString(styleTitle.Render(label) + "\n\n")
	b.WriteString(m.input.View() + "\n\n")
	b.WriteString(styleHint.Render("enter: submit · esc: cancel"))
	return styleMenu.Render(b.String())
}

func (m *Model) statusBar() string {
	var msg string
	switch {
	case m.errMsg != "":
		msg = styleErr.Render(m.errMsg)
	case m.statusMsg != "":
		msg = styleOk.Render(m.statusMsg)
	default:
		msg = styleStatus.Render("board refreshed every 3s")
	}
	hints := styleStatus.Render("?: help  m: move  s/S: claim/release  c: comment  n: resume note  f: repo  /: search  t: wontfix  r: refresh  q: quit")
	return msg + "\n" + hints
}

func (m *Model) helpView() string {
	help := `kanban board — keys

navigation
  h/l, ←/→, tab     switch column lane
  j/k, ↑/↓          move within lane
  enter             open full issue detail
  esc               close / clear filters

board actions
  m                 move issue (legal destinations only)
  s                 claim as ` + m.worker + `
  S                 release claim (note inline / $EDITOR / skip)
  c / C             add comment (inline / $EDITOR)
  n / N             add resume note (inline / $EDITOR) — own claim only

filters
  f                 filter by repo
  /                 search issue titles
  t                 toggle the wontfix lane
  r                 refresh now

other
  ?                 this help
  q                 quit`
	return styleMenu.Render(help)
}

func (m *Model) overlay(content string) string {
	board := m.boardView()
	return board + "\n" + content
}

func (m *Model) centered(content string) string {
	lines := strings.Split(content, "\n")
	w := 0
	for _, l := range lines {
		if len([]rune(l)) > w {
			w = len([]rune(l))
		}
	}
	top := (m.height - len(lines)) / 2
	if top < 0 {
		top = 0
	}
	var b strings.Builder
	for i := 0; i < top; i++ {
		b.WriteString("\n")
	}
	b.WriteString(content)
	return b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
