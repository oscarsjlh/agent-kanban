package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/oscarsjlh/agent-kanban/internal/domain"
	"github.com/oscarsjlh/agent-kanban/internal/render"
	"github.com/oscarsjlh/agent-kanban/internal/store"
	"github.com/oscarsjlh/agent-kanban/internal/tui"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, out, errw io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("no command given\n\n%s", usageText)
	}
	switch args[0] {
	case "help", "--help", "-h":
		fmt.Fprint(out, usageText)
		return nil
	}
	s, err := store.Open()
	if err != nil {
		return err
	}
	defer s.Close()
	switch args[0] {
	case "tui":
		return tui.Run(s)
	case "repo":
		return repoCmd(s, args[1:], out)
	case "new":
		return newCmd(s, args[1:], out)
	case "list":
		return listCmd(s, args[1:], out)
	case "show":
		return showCmd(s, args[1:], out)
	case "move":
		return moveCmd(s, args[1:], out)
	case "edit":
		return editCmd(s, args[1:], out)
	case "comment":
		return commentCmd(s, args[1:], out)
	case "start":
		return startCmd(s, args[1:], out)
	case "stop":
		return stopCmd(s, args[1:], out, errw)
	case "handoff":
		return handoffCmd(s, args[1:], out)
	case "resume":
		return resumeCmd(s, args[1:], out)
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

const usageText = `kanban — issue tracker for agent workflows

usage: kanban <command> [args]

commands:
  tui      interactive board UI
  repo     register and list git repos (repo add|list)
  new      create an issue (--title, --body-file|--body-stdin, --repo)
  list     list issues (--column, --repo, --worker, --json, --all)
  show     show an issue with its comments
  resume   show resume context for an issue
  edit     replace an issue body (--body-file)
  move     move an issue to a column (--reason, --blocked-by)
  comment  add a comment to an issue (--body-file)
  start    claim an issue (--worker)
  stop     release a claim (--worker, --note-file)
  handoff  record a handoff note (--body-file, --worker)

run a command with no or wrong arguments to see its usage.

the board lives at ~/.kanban/kanban.db
(override with KANBAN_HOME or KANBAN_DB)
`

func repoCmd(s *store.Store, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: kanban repo <add|list>")
	}
	switch args[0] {
	case "add":
		fs := flag.NewFlagSet("repo add", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		name := fs.String("name", "", "")
		if err := fs.Parse(flagsFirst(args[1:], map[string]bool{"--name": true})); err != nil {
			return err
		}
		if fs.NArg() != 1 {
			return fmt.Errorf("usage: kanban repo add <path> [--name NAME]")
		}
		p, err := filepath.Abs(fs.Arg(0))
		if err != nil {
			return err
		}
		rp, err := filepath.EvalSymlinks(p)
		if err == nil {
			p = rp
		}
		ident := repoIdentity(p)
		r, err := s.AddRepo(ident, p, *name)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "registered repo %s\n", r.Name)
		return nil
	case "list":
		repos, err := s.Repos()
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "NAME\tPATH\tIDENTITY")
		for _, r := range repos {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Name, r.Path, r.Identity)
		}
		return tw.Flush()
	default:
		return fmt.Errorf("unknown repo command: %s", args[0])
	}
}
func repoIdentity(path string) string {
	cmd := exec.Command("git", "-C", path, "config", "--get", "remote.origin.url")
	b, err := cmd.Output()
	if err == nil && strings.TrimSpace(string(b)) != "" {
		return strings.TrimSpace(string(b))
	}
	return path
}

func newCmd(s *store.Store, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	title := fs.String("title", "", "")
	repo := fs.String("repo", "", "")
	bodyFile := fs.String("body-file", "", "")
	bodyStdin := fs.Bool("body-stdin", false, "")
	if err := fs.Parse(flagsFirst(args, map[string]bool{"--title": true, "--repo": true, "--body-file": true})); err != nil {
		return err
	}
	if *title == "" {
		return fmt.Errorf("--title is required")
	}
	if (*bodyFile == "") == (*bodyStdin == false) {
		return fmt.Errorf("provide exactly one of --body-file or --body-stdin")
	}
	var b []byte
	var err error
	if *bodyFile != "" {
		b, err = os.ReadFile(*bodyFile)
	} else {
		b, err = io.ReadAll(os.Stdin)
	}
	if err != nil {
		return err
	}
	id, err := s.CreateIssue(*title, string(b), *repo)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "created issue %d\n", id)
	return nil
}

func listCmd(s *store.Store, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	col := fs.String("column", "", "")
	repo := fs.String("repo", "", "")
	worker := fs.String("worker", "", "")
	js := fs.Bool("json", false, "")
	all := fs.Bool("all", false, "")
	if err := fs.Parse(flagsFirst(args, map[string]bool{"--column": true, "--repo": true, "--worker": true})); err != nil {
		return err
	}
	if *worker != "" && *col != "" {
		return fmt.Errorf("--worker cannot be combined with --column: claims are listed regardless of column")
	}
	var issues []store.Issue
	if *worker != "" {
		var err error
		issues, err = s.ClaimedBy(*worker, *repo)
		if err != nil {
			return err
		}
	} else {
		column := ""
		if *col != "" {
			var ok bool
			column, ok = domain.CanonicalColumn(*col)
			if !ok {
				return fmt.Errorf("unknown column: %s", *col)
			}
		}
		var err error
		issues, err = s.Issues(column, *repo, *all || column == domain.Wontfix)
		if err != nil {
			return err
		}
	}
	if *js {
		claims, _ := s.AllClaims()
		claimedBy := map[int64]string{}
		for _, c := range claims {
			claimedBy[c.IssueID] = c.Worker
		}
		type row struct {
			ID        int64   `json:"id"`
			Title     string  `json:"title"`
			Column    string  `json:"column"`
			Repo      *string `json:"repo"`
			ClaimedBy *string `json:"claimed_by"`
			CreatedAt string  `json:"created_at"`
			UpdatedAt string  `json:"updated_at"`
		}
		var rows []row
		for _, is := range issues {
			var r *string
			if is.RepoName.Valid {
				v := is.RepoName.String
				r = &v
			}
			var cb *string
			if w, ok := claimedBy[is.ID]; ok {
				cb = &w
			}
			rows = append(rows, row{is.ID, is.Title, is.Column, r, cb, is.CreatedAt, is.UpdatedAt})
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{"issues": rows})
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if *worker != "" {
		fmt.Fprintln(tw, "ID\tCOLUMN\tREPO\tCLAIMED SINCE\tTITLE")
		claims, _ := s.AllClaims()
		since := map[int64]string{}
		for _, c := range claims {
			since[c.IssueID] = c.StartedAt
		}
		for _, is := range issues {
			repo := "-"
			if is.RepoName.Valid {
				repo = is.RepoName.String
			}
			fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n", is.ID, is.Column, repo, since[is.ID], is.Title)
		}
		return tw.Flush()
	}
	fmt.Fprintln(tw, "ID\tCOLUMN\tREPO\tTITLE")
	for _, is := range issues {
		repo := "-"
		if is.RepoName.Valid {
			repo = is.RepoName.String
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", is.ID, is.Column, repo, is.Title)
	}
	return tw.Flush()
}

func showCmd(s *store.Store, args []string, out io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: kanban show <id>")
	}
	id, err := parseID(args[0])
	if err != nil {
		return err
	}
	is, err := s.Issue(id)
	if err != nil {
		return err
	}
	cs, _ := s.Comments(id)
	fmt.Fprint(out, render.Issue(is, cs))
	return nil
}

func editCmd(s *store.Store, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("edit", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	file := fs.String("body-file", "", "")
	if err := fs.Parse(flagsFirst(args, map[string]bool{"--body-file": true})); err != nil {
		return err
	}
	if fs.NArg() != 1 || *file == "" {
		return fmt.Errorf("usage: kanban edit <id> --body-file F")
	}
	id, err := parseID(fs.Arg(0))
	if err != nil {
		return err
	}
	b, err := os.ReadFile(*file)
	if err != nil {
		return err
	}
	if err := s.EditBody(id, string(b)); err != nil {
		return err
	}
	fmt.Fprintf(out, "edited issue %d\n", id)
	return nil
}
func moveCmd(s *store.Store, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("move", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	reason := fs.String("reason", "", "")
	blocked := fs.Int64("blocked-by", 0, "")
	if err := fs.Parse(flagsFirst(args, map[string]bool{"--reason": true, "--blocked-by": true})); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return fmt.Errorf("usage: kanban move <id> <column>")
	}
	id, err := parseID(fs.Arg(0))
	if err != nil {
		return err
	}
	col, ok := domain.CanonicalColumn(fs.Arg(1))
	if !ok {
		return fmt.Errorf("unknown column: %s", fs.Arg(1))
	}
	var bp *int64
	if *blocked != 0 {
		bp = blocked
	}
	if err := s.Move(id, col, *reason, bp); err != nil {
		return err
	}
	fmt.Fprintf(out, "moved issue %d to %s\n", id, col)
	return nil
}
func commentCmd(s *store.Store, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("comment", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	file := fs.String("body-file", "", "")
	if err := fs.Parse(flagsFirst(args, map[string]bool{"--body-file": true})); err != nil {
		return err
	}
	if fs.NArg() != 1 || *file == "" {
		return fmt.Errorf("usage: kanban comment <id> --body-file F")
	}
	id, err := parseID(fs.Arg(0))
	if err != nil {
		return err
	}
	b, err := os.ReadFile(*file)
	if err != nil {
		return err
	}
	if err := s.AddComment(id, string(b)); err != nil {
		return err
	}
	fmt.Fprintf(out, "commented on issue %d\n", id)
	return nil
}
func startCmd(s *store.Store, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	worker := fs.String("worker", "", "")
	if err := fs.Parse(flagsFirst(args, map[string]bool{"--worker": true})); err != nil {
		return err
	}
	if fs.NArg() != 1 || *worker == "" {
		return fmt.Errorf("usage: kanban start <id> --worker W")
	}
	id, err := parseID(fs.Arg(0))
	if err != nil {
		return err
	}
	if err := s.Start(id, *worker); err != nil {
		return err
	}
	fmt.Fprintf(out, "started issue %d as %s\n", id, *worker)
	return nil
}
func stopCmd(s *store.Store, args []string, out, errw io.Writer) error {
	fs := flag.NewFlagSet("stop", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	worker := fs.String("worker", "", "")
	noteFile := fs.String("note-file", "", "")
	if err := fs.Parse(flagsFirst(args, map[string]bool{"--worker": true, "--note-file": true})); err != nil {
		return err
	}
	if fs.NArg() != 1 || *worker == "" {
		return fmt.Errorf("usage: kanban stop <id> --worker W [--note-file F]")
	}
	id, err := parseID(fs.Arg(0))
	if err != nil {
		return err
	}
	note := ""
	if *noteFile != "" {
		b, err := os.ReadFile(*noteFile)
		if err != nil {
			return err
		}
		note = string(b)
	}
	if err := s.Stop(id, *worker, note); err != nil {
		return err
	}
	if note == "" {
		fmt.Fprintln(errw, "warning: stopped without a resume note; the next worker starts cold")
	}
	fmt.Fprintf(out, "stopped issue %d as %s\n", id, *worker)
	return nil
}
func handoffCmd(s *store.Store, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("handoff", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	file := fs.String("body-file", "", "")
	worker := fs.String("worker", "", "")
	if err := fs.Parse(flagsFirst(args, map[string]bool{"--body-file": true, "--worker": true})); err != nil {
		return err
	}
	if fs.NArg() != 1 || *file == "" {
		return fmt.Errorf("usage: kanban handoff <id> --body-file F [--worker W]")
	}
	id, err := parseID(fs.Arg(0))
	if err != nil {
		return err
	}
	w := *worker
	if w == "" {
		w = os.Getenv("KANBAN_WORKER")
	}
	if w == "" {
		return fmt.Errorf("--worker or KANBAN_WORKER is required")
	}
	b, err := os.ReadFile(*file)
	if err != nil {
		return err
	}
	if err := s.Handoff(id, w, string(b)); err != nil {
		return err
	}
	fmt.Fprintf(out, "recorded handoff for issue %d\n", id)
	return nil
}
func resumeCmd(s *store.Store, args []string, out io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: kanban resume <id>")
	}
	id, err := parseID(args[0])
	if err != nil {
		return err
	}
	is, err := s.Issue(id)
	if err != nil {
		return err
	}
	notes, _ := s.ResumeNotes(id)
	fmt.Fprint(out, render.Resume(is, notes))
	return nil
}
func flagsFirst(args []string, valueFlags map[string]bool) []string {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "--") {
			if strings.Contains(a, "=") {
				flags = append(flags, a)
				continue
			}
			flags = append(flags, a)
			if valueFlags[a] && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		pos = append(pos, a)
	}
	return append(flags, pos...)
}

func parseID(s string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid issue id: %s", s)
	}
	return id, nil
}
