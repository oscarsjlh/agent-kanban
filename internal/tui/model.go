package tui

import (
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/oscarsjlh/agent-kanban/internal/domain"
	"github.com/oscarsjlh/agent-kanban/internal/render"
	"github.com/oscarsjlh/agent-kanban/internal/store"
)

const pollInterval = 3 * time.Second

type mode int

const (
	modeBoard mode = iota
	modeDetail
	modeHelp
	modeMoveMenu
	modeRepoPicker
	modeStopChoice
	modeInput
)

// inputCtx distinguishes what a one-line text input is collecting.
type inputCtx int

const (
	inputNone inputCtx = iota
	inputSearch
	inputReason
	inputComment
	inputNote
	inputStopNote
)

type editorDoneMsg struct {
	ctx  inputCtx
	path string
	err  error
}

type tickMsg time.Time

// lanes always visible; the wontfix lane is appended when toggled on.
var boardLanes = []string{domain.Inbox, domain.Ready, domain.InProgress, domain.Waiting, domain.Done}
var wontfixLanes = []string{domain.Wontfix}

type Model struct {
	s      *store.Store
	worker string

	width, height int
	mode          mode

	issues []store.Issue
	claims map[int64]store.Claim
	repos  []store.Repo

	laneIdx int
	cursors map[string]int

	repoFilter   string
	search       string
	showTerminal bool

	input    textinput.Model
	inputFor inputCtx
	menuIdx  int // move menu / repo picker / stop choice selection

	selectedID int64 // issue in focus for detail pane / fullscreen

	vp viewport.Model // fullscreen detail scroll

	statusMsg string
	errMsg    string
	fatal     error
}

func newModel(s *store.Store) *Model {
	ti := textinput.New()
	ti.Prompt = "> "
	return &Model{
		s:       s,
		worker:  store.CurrentUser(),
		cursors: map[string]int{},
		input:   ti,
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.reload(), tea.Tick(pollInterval, func(t time.Time) tea.Msg { return tickMsg(t) }))
}

// reload refreshes Board data from the store and returns nil (plain func,
// called from Update); Init wraps it for the initial load.
func (m *Model) reload() tea.Cmd {
	includeTerminal := true // Done is always fetched; lane visibility is a view choice
	issues, err := m.s.Issues("", m.repoFilter, includeTerminal)
	if err != nil {
		m.errMsg = err.Error()
		return nil
	}
	if !m.showTerminal {
		filtered := issues[:0:0]
		for _, is := range issues {
			if is.Column != domain.Wontfix {
				filtered = append(filtered, is)
			}
		}
		issues = filtered
	}
	sort.SliceStable(issues, func(i, j int) bool { return issues[i].UpdatedAt > issues[j].UpdatedAt })
	m.issues = issues

	claims, err := m.s.AllClaims()
	if err == nil {
		m.claims = map[int64]store.Claim{}
		for _, c := range claims {
			m.claims[c.IssueID] = c
		}
	}
	if repos, err := m.s.Repos(); err == nil {
		m.repos = repos
	}
	m.clampSelection()
	return nil
}

func (m *Model) lanes() []string {
	if m.showTerminal {
		return append(append([]string{}, boardLanes...), wontfixLanes...)
	}
	return boardLanes
}

func (m *Model) laneIssues(col string) []store.Issue {
	var out []store.Issue
	for _, is := range m.issues {
		if is.Column == col && strings.Contains(strings.ToLower(is.Title), strings.ToLower(m.search)) {
			out = append(out, is)
		}
	}
	return out
}

func (m *Model) clampSelection() {
	lanes := m.lanes()
	if m.laneIdx >= len(lanes) {
		m.laneIdx = len(lanes) - 1
	}
	for _, l := range lanes {
		if m.cursors[l] >= len(m.laneIssues(l)) {
			m.cursors[l] = len(m.laneIssues(l)) - 1
		}
		if m.cursors[l] < 0 {
			m.cursors[l] = 0
		}
	}
	m.refreshSelectedID()
}

func (m *Model) refreshSelectedID() {
	is := m.selected()
	if is == nil {
		m.selectedID = 0
		return
	}
	m.selectedID = is.ID
}

func (m *Model) selected() *store.Issue {
	lanes := m.lanes()
	if len(lanes) == 0 {
		return nil
	}
	col := lanes[m.laneIdx]
	issues := m.laneIssues(col)
	i := m.cursors[col]
	if i < 0 || i >= len(issues) {
		return nil
	}
	is := issues[i]
	return &is
}

func (m *Model) flash(msg string) { m.errMsg = msg }
func (m *Model) ok(msg string)    { m.statusMsg = msg }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.vp = viewport.New(msg.Width-4, msg.Height-6)
		return m, nil
	case tickMsg:
		m.reload()
		return m, tea.Tick(pollInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
	case editorDoneMsg:
		return m.onEditorDone(msg)
	case tea.KeyMsg:
		m.errMsg = ""
		return m.onKey(msg)
	}
	return m, nil
}

func (m *Model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeInput:
		return m.onKeyInput(msg)
	case modeHelp:
		if msg.String() == "q" || msg.String() == "?" || msg.String() == "esc" {
			m.mode = modeBoard
		}
		return m, nil
	case modeMoveMenu:
		return m.onKeyMoveMenu(msg)
	case modeRepoPicker:
		return m.onKeyRepoPicker(msg)
	case modeStopChoice:
		return m.onKeyStopChoice(msg)
	case modeDetail:
		return m.onKeyDetail(msg)
	default:
		return m.onKeyBoard(msg)
	}
}

func (m *Model) onKeyBoard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lanes := m.lanes()
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "h", "left":
		m.laneIdx = (m.laneIdx - 1 + len(lanes)) % len(lanes)
	case "l", "right":
		m.laneIdx = (m.laneIdx + 1) % len(lanes)
	case "tab":
		m.laneIdx = (m.laneIdx + 1) % len(lanes)
	case "shift+tab":
		m.laneIdx = (m.laneIdx - 1 + len(lanes)) % len(lanes)
	case "j", "down":
		m.cursors[lanes[m.laneIdx]]++
	case "k", "up":
		m.cursors[lanes[m.laneIdx]]--
	case "enter":
		if is := m.selected(); is != nil {
			m.openDetail(is)
			m.mode = modeDetail
		}
	case "m":
		return m.openMoveMenu()
	case "s":
		return m.startClaim()
	case "S":
		return m.openStopChoice()
	case "c":
		return m.beginInput(inputComment, "comment")
	case "C":
		return m.beginEditor(inputComment)
	case "n":
		return m.beginInput(inputNote, "resume note")
	case "N":
		return m.beginEditor(inputNote)
	case "f":
		m.menuIdx = 0
		m.mode = modeRepoPicker
	case "/":
		return m.beginInput(inputSearch, "")
	case "t":
		m.showTerminal = !m.showTerminal
	case "r":
		m.reload()
		m.ok("refreshed")
	case "?":
		m.mode = modeHelp
	case "esc":
		m.search, m.repoFilter = "", ""
	}
	m.clampSelection()
	return m, nil
}

func (m *Model) onKeyDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.mode = modeBoard
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m *Model) onKeyInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeBoard
		m.input.Blur()
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "enter":
		val := strings.TrimSpace(m.input.Value())
		m.input.SetValue("")
		m.input.Blur()
		m.mode = modeBoard
		return m, m.submitInput(val)
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *Model) beginInput(ctx inputCtx, placeholder string) (tea.Model, tea.Cmd) {
	if ctx == inputComment || ctx == inputNote {
		if is := m.selected(); is == nil {
			m.flash("no issue selected")
			return m, nil
		}
		if ctx == inputNote {
			if c, claimed := m.claims[m.selectedID]; !claimed || c.Worker != m.worker {
				m.flash("resume notes require your own active claim (s to claim, S to stop)")
				return m, nil
			}
		}
	}
	m.inputFor = ctx
	m.input.Placeholder = placeholder
	if ctx == inputSearch {
		m.input.SetValue(m.search)
	} else {
		m.input.SetValue("")
	}
	m.input.Focus()
	m.mode = modeInput
	return m, textinput.Blink
}

func (m *Model) submitInput(val string) tea.Cmd {
	id := m.selectedID
	switch m.inputFor {
	case inputSearch:
		m.search = val
	case inputReason:
		if val == "" {
			m.flash("waiting needs a reason")
			break
		}
		if _, err := m.s.Move(id, domain.Waiting, val, nil, false); err != nil {
			m.flash(err.Error())
		} else {
			m.ok("moved to Waiting")
		}
	case inputComment:
		if val == "" {
			break
		}
		if err := m.s.AddComment(id, val); err != nil {
			m.flash(err.Error())
		} else {
			m.ok("comment added")
		}
	case inputNote:
		if val == "" {
			break
		}
		if err := m.s.Handoff(id, m.worker, val); err != nil {
			m.flash(err.Error())
		} else {
			m.ok("resume note recorded")
		}
	case inputStopNote:
		if err := m.s.Stop(id, m.worker, val); err != nil {
			m.flash(err.Error())
		} else {
			m.ok("claim released with resume note")
		}
	}
	m.inputFor = inputNone
	m.reload()
	return nil
}

func (m *Model) openMoveMenu() (tea.Model, tea.Cmd) {
	is := m.selected()
	if is == nil {
		m.flash("no issue selected")
		return m, nil
	}
	if _, claimed := m.claims[is.ID]; claimed {
		m.flash("claimed issue cannot be moved manually; stop the claim first")
		return m, nil
	}
	if len(m.moveDests(is)) == 0 {
		m.flash("no legal moves from " + is.Column)
		return m, nil
	}
	m.menuIdx = 0
	m.mode = modeMoveMenu
	return m, nil
}

func (m *Model) moveDests(is *store.Issue) []string {
	var dests []string
	for _, to := range []string{domain.Ready, domain.Waiting, domain.Done, domain.Wontfix} {
		if to == is.Column {
			continue
		}
		if domain.CanMove(is.Column, to, true, false) == nil {
			dests = append(dests, to)
		}
	}
	return dests
}

func (m *Model) onKeyMoveMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	is := m.selected()
	dests := m.moveDests(is)
	switch msg.String() {
	case "esc", "q", "m":
		m.mode = modeBoard
	case "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		m.menuIdx = (m.menuIdx + 1) % len(dests)
	case "k", "up":
		m.menuIdx = (m.menuIdx - 1 + len(dests)) % len(dests)
	case "enter":
		dest := dests[m.menuIdx]
		m.mode = modeBoard
		if dest == domain.Waiting {
			return m.beginInput(inputReason, "waiting reason")
		}
		if _, err := m.s.Move(m.selectedID, dest, "", nil, false); err != nil {
			m.flash(err.Error())
		} else {
			m.ok("moved to " + dest)
		}
		m.reload()
	}
	return m, nil
}

func (m *Model) onKeyRepoPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// entry 0 = "all repos", then registered repos
	n := len(m.repos) + 1
	switch msg.String() {
	case "esc", "q", "f":
		m.mode = modeBoard
	case "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		m.menuIdx = (m.menuIdx + 1) % n
	case "k", "up":
		m.menuIdx = (m.menuIdx - 1 + n) % n
	case "enter":
		if m.menuIdx == 0 {
			m.repoFilter = ""
		} else {
			m.repoFilter = m.repos[m.menuIdx-1].Name
		}
		m.mode = modeBoard
		m.reload()
	}
	return m, nil
}

func (m *Model) startClaim() (tea.Model, tea.Cmd) {
	is := m.selected()
	if is == nil {
		m.flash("no issue selected")
		return m, nil
	}
	if err := m.s.Start(is.ID, m.worker); err != nil {
		m.flash(err.Error())
		return m, nil
	}
	m.ok("claimed issue " + itoa(int(is.ID)))
	m.reload()
	return m, nil
}

func (m *Model) openStopChoice() (tea.Model, tea.Cmd) {
	is := m.selected()
	if is == nil {
		m.flash("no issue selected")
		return m, nil
	}
	c, claimed := m.claims[is.ID]
	if !claimed {
		m.flash("issue is not claimed")
		return m, nil
	}
	if c.Worker != m.worker {
		m.flash("issue is claimed by " + c.Worker + ", not " + m.worker)
		return m, nil
	}
	m.menuIdx = 0
	m.mode = modeStopChoice
	return m, nil
}

func (m *Model) onKeyStopChoice(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "S":
		m.mode = modeBoard
	case "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		m.menuIdx = (m.menuIdx + 1) % 3
	case "k", "up":
		m.menuIdx = (m.menuIdx + 2) % 3
	case "i":
		m.mode = modeBoard
		return m.beginInput(inputStopNote, "resume note")
	case "e":
		m.mode = modeBoard
		return m.beginEditor(inputStopNote)
	case "s", "enter":
		m.mode = modeBoard
		if err := m.s.Stop(m.selectedID, m.worker, ""); err != nil {
			m.flash(err.Error())
		} else {
			m.ok("stopped — warning: no resume note; the next worker starts cold")
		}
		m.reload()
	}
	return m, nil
}

// beginEditor opens $EDITOR (or $VISUAL) on a scratch file; falls back to
// inline input when neither is set.
func (m *Model) beginEditor(ctx inputCtx) (tea.Model, tea.Cmd) {
	if ctx == inputNote || ctx == inputStopNote {
		if _, claimed := m.claims[m.selectedID]; !claimed {
			m.flash("resume notes require your own active claim")
			return m, nil
		}
	}
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		m.flash("no $EDITOR set; using inline input")
		return m.beginInput(ctx, "text")
	}
	f, err := os.CreateTemp("", "kanban-*.md")
	if err != nil {
		m.flash(err.Error())
		return m, nil
	}
	path := f.Name()
	if _, err := f.WriteString("#KANBAN: write below, save and exit to attach; lines starting with #KANBAN: are stripped.\n"); err != nil {
		m.flash(err.Error())
		return m, nil
	}
	f.Close()
	c := exec.Command(editor, path)
	return m, tea.ExecProcess(c, func(err error) tea.Msg {
		return editorDoneMsg{ctx: ctx, path: path, err: err}
	})
}

func (m *Model) onEditorDone(msg editorDoneMsg) (tea.Model, tea.Cmd) {
	defer os.Remove(msg.path)
	m.mode = modeBoard
	if msg.err != nil {
		m.flash("editor failed: " + msg.err.Error())
		return m, nil
	}
	b, err := os.ReadFile(msg.path)
	if err != nil {
		m.flash(err.Error())
		return m, nil
	}
	body := stripKanbanLines(strings.TrimSpace(string(b)))
	if body == "" {
		m.flash("editor content empty; nothing attached")
		return m, nil
	}
	id := m.selectedID
	switch msg.ctx {
	case inputComment:
		if err := m.s.AddComment(id, body); err != nil {
			m.flash(err.Error())
		} else {
			m.ok("comment added")
		}
	case inputNote:
		if err := m.s.Handoff(id, m.worker, body); err != nil {
			m.flash(err.Error())
		} else {
			m.ok("resume note recorded")
		}
	case inputStopNote:
		if err := m.s.Stop(id, m.worker, body); err != nil {
			m.flash(err.Error())
		} else {
			m.ok("claim released with resume note")
		}
	}
	m.reload()
	return m, nil
}

func stripKanbanLines(s string) string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "#KANBAN:") {
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// openDetail loads full Issue content into the viewport.
func (m *Model) openDetail(is *store.Issue) {
	cs, _ := m.s.Comments(is.ID)
	notes, _ := m.s.ResumeNotes(is.ID)
	var b strings.Builder
	b.WriteString(render.Issue(*is, cs))
	if len(notes) > 0 {
		b.WriteString("\n## Resume Notes\n\n")
		for _, n := range notes {
			b.WriteString("### " + n.Worker + " — " + n.CreatedAt + "\n\n" + strings.TrimRight(n.Body, "\n") + "\n\n")
		}
	}
	m.vp.SetContent(renderMarkdown(m.width-4, b.String()))
	m.vp.GotoTop()
}

// detailMarkdown builds the compact briefing for the side pane.
func (m *Model) detailMarkdown(is *store.Issue, width int) string {
	var b strings.Builder
	b.WriteString("### " + is.Title + "\n\n")
	b.WriteString("- Column: " + is.Column + "\n")
	repo := "none"
	if is.RepoName.Valid {
		repo = is.RepoName.String
	}
	b.WriteString("- Repo: " + repo + "\n")
	if c, claimed := m.claims[is.ID]; claimed {
		b.WriteString("- Claim: " + c.Worker + " since " + c.StartedAt + "\n")
	}
	if is.WaitingReason.Valid {
		b.WriteString("- Waiting: " + is.WaitingReason.String + "\n")
	}
	if is.BlockedBy.Valid {
		b.WriteString("- Blocked by: #" + itoa(int(is.BlockedBy.Int64)) + "\n")
	}
	if crit := render.CriteriaSummary(is.Body); crit != "" {
		b.WriteString("- Criteria: " + crit + "\n")
	}
	notes, _ := m.s.ResumeNotes(is.ID)
	if len(notes) > 0 {
		n := notes[len(notes)-1]
		b.WriteString("\n**Latest Resume Note** — " + n.Worker + "\n\n" + strings.TrimRight(n.Body, "\n") + "\n")
	}
	return renderMarkdown(width, b.String())
}
