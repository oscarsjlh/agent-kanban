package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

type app struct{ bin, db, home string }

type result struct {
	code           int
	stdout, stderr string
}

func buildApp(t *testing.T) app {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "kanban")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-buildvcs=false", "-o", bin, ".")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return app{bin: bin, db: filepath.Join(dir, "board", "kanban.db"), home: filepath.Join(dir, "home")}
}

func (a app) run(t *testing.T, stdin string, args ...string) result {
	t.Helper()
	cmd := exec.Command(a.bin, args...)
	cmd.Env = append(os.Environ(), "KANBAN_DB="+a.db, "HOME="+a.home)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("run failed: %v", err)
		}
	}
	return result{code: code, stdout: out.String(), stderr: errb.String()}
}

func (a app) must(t *testing.T, stdin string, args ...string) result {
	r := a.run(t, stdin, args...)
	if r.code != 0 {
		t.Fatalf("kanban %v failed\nstdout:%s\nstderr:%s", args, r.stdout, r.stderr)
	}
	return r
}
func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func normalize(s string) string {
	s = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z`).ReplaceAllString(s, "<ts>")
	return s
}

func TestRepoRegistrationTracerBullet(t *testing.T) {
	a := buildApp(t)
	dir := t.TempDir()
	api1 := filepath.Join(dir, "one", "api")
	api2 := filepath.Join(dir, "two", "api")
	if err := os.MkdirAll(api1, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(api2, 0700); err != nil {
		t.Fatal(err)
	}
	a.must(t, "", "repo", "add", api1)
	dup := a.run(t, "", "repo", "add", api1)
	if dup.code == 0 || !strings.Contains(dup.stderr, "repo already registered") {
		t.Fatalf("expected duplicate error, got %#v", dup)
	}
	a.must(t, "", "repo", "add", api2)
	list := a.must(t, "", "repo", "list").stdout
	if !strings.Contains(list, "api") || !strings.Contains(list, "api-2") {
		t.Fatalf("missing suffixed repos:\n%s", list)
	}
	entries, err := os.ReadDir(filepath.Dir(a.db))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "kanban.db" {
		t.Fatalf("expected only kanban.db, got %v", entries)
	}
}

func TestRepoUsesGitRemoteIdentity(t *testing.T) {
	a := buildApp(t)
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.Mkdir(repo, 0700); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"git", "init"}, {"git", "remote", "add", "origin", "https://example.com/acme/repo.git"}} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	a.must(t, "", "repo", "add", repo)
	list := a.must(t, "", "repo", "list").stdout
	if !strings.Contains(list, "https://example.com/acme/repo.git") {
		t.Fatalf("remote identity not shown:\n%s", list)
	}
}

func TestIssueCreateListShowCommentsAndJSON(t *testing.T) {
	a := buildApp(t)
	dir := t.TempDir()
	body := writeFile(t, dir, "body.md", "Spec body\n\n- [ ] first\n- [x] second\n")
	comment := writeFile(t, dir, "comment.md", "A decision.\n")
	created := a.must(t, "", "new", "--title", "Build it", "--body-file", body).stdout
	if !strings.Contains(created, "created issue 1") {
		t.Fatal(created)
	}
	a.must(t, "", "comment", "1", "--body-file", comment)
	list := a.must(t, "", "list").stdout
	if !strings.Contains(list, "1") || !strings.Contains(list, "Inbox") || !strings.Contains(list, "Build it") {
		t.Fatal(list)
	}
	js := a.must(t, "", "list", "--json").stdout
	var parsed struct {
		Issues []struct {
			ID            int `json:"id"`
			Title, Column string
			Repo          *string `json:"repo"`
		} `json:"issues"`
	}
	if err := json.Unmarshal([]byte(js), &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Issues) != 1 || parsed.Issues[0].ID != 1 || parsed.Issues[0].Repo != nil {
		t.Fatalf("bad json: %s", js)
	}
	show := normalize(a.must(t, "", "show", "1").stdout)
	want := "# Issue 1: Build it\n\nColumn: Inbox\nRepo: none\nCreated: <ts>\nUpdated: <ts>\n\n## Body\n\nSpec body\n\n- [ ] first\n- [x] second\n\n## Comments\n\n### "
	if !strings.HasPrefix(show, want) || !strings.Contains(show, "A decision.") {
		t.Fatalf("bad show:\n%s", show)
	}
	bad := a.run(t, "", "show", "99")
	if bad.code == 0 || !strings.Contains(bad.stderr, "unknown issue: 99") {
		t.Fatalf("expected unknown issue, got %#v", bad)
	}
	bad = a.run(t, "", "list", "--column", "nonesuch")
	if bad.code == 0 || !strings.Contains(bad.stderr, "unknown column") {
		t.Fatalf("expected bad column, got %#v", bad)
	}
}

func TestMoveEventsWontfixAndWaitingRules(t *testing.T) {
	a := buildApp(t)
	body := writeFile(t, t.TempDir(), "b.md", "body")
	a.must(t, "", "new", "--title", "Move me", "--body-file", body)
	bad := a.run(t, "", "move", "1", "waiting")
	if bad.code == 0 || !strings.Contains(bad.stderr, "requires --reason") {
		t.Fatalf("expected waiting error: %#v", bad)
	}
	a.must(t, "", "move", "1", "ready")
	a.must(t, "", "move", "1", "wontfix")
	list := a.must(t, "", "list").stdout
	if strings.Contains(list, "Move me") {
		t.Fatalf("wontfix visible by default:\n%s", list)
	}
	list = a.must(t, "", "list", "--column", "wontfix").stdout
	if !strings.Contains(list, "Move me") {
		t.Fatalf("wontfix not queryable:\n%s", list)
	}
	bad = a.run(t, "", "move", "1", "ready")
	if bad.code == 0 || !strings.Contains(bad.stderr, "terminal") {
		t.Fatalf("expected terminal error: %#v", bad)
	}
	if n := eventCount(t, a.db); n != 3 {
		t.Fatalf("events=%d, want 3", n)
	}
}

func TestClaimsHandoffResumeAndStop(t *testing.T) {
	a := buildApp(t)
	dir := t.TempDir()
	body := writeFile(t, dir, "body.md", "- [ ] open\n- [X] done\n")
	note := writeFile(t, dir, "note.md", "Tried tests; next fix parser.\n")
	stopNote := writeFile(t, dir, "stop.md", "Parser fixed; run final tests.\n")
	a.must(t, "", "new", "--title", "Claim me", "--body-file", body)
	a.must(t, "", "move", "1", "ready")
	a.must(t, "", "start", "1", "--worker", "pi-session-1")
	if kind := workerKind(t, a.db, "pi-session-1"); kind != "agent" {
		t.Fatalf("worker kind=%s", kind)
	}
	bad := a.run(t, "", "start", "1", "--worker", "pi-session-2")
	if bad.code == 0 || !strings.Contains(bad.stderr, "claimed by pi-session-1") {
		t.Fatalf("expected claim error: %#v", bad)
	}
	bad = a.run(t, "", "stop", "1", "--worker", "pi-session-2")
	if bad.code == 0 || !strings.Contains(bad.stderr, "claimed by pi-session-1") {
		t.Fatalf("expected wrong claimer: %#v", bad)
	}
	a.must(t, "", "handoff", "1", "--worker", "pi-session-1", "--body-file", note)
	resume := normalize(a.must(t, "", "resume", "1").stdout)
	for _, want := range []string{"Latest Resume Note", "Tried tests", "Open: 1", "Checked: 1", "No blocker recorded"} {
		if !strings.Contains(resume, want) {
			t.Fatalf("resume missing %q:\n%s", want, resume)
		}
	}
	a.must(t, "", "stop", "1", "--worker", "pi-session-1", "--note-file", stopNote)
	cold := a.run(t, "", "stop", "1", "--worker", "pi-session-1")
	if cold.code == 0 || !strings.Contains(cold.stderr, "not claimed") {
		t.Fatalf("expected unclaimed error: %#v", cold)
	}
	a.must(t, "", "start", "1", "--worker", "pi-session-1")
	stopped := a.run(t, "", "stop", "1", "--worker", "pi-session-1")
	if stopped.code != 0 || !strings.Contains(stopped.stderr, "next worker starts cold") {
		t.Fatalf("expected nag: %#v", stopped)
	}
}

func TestListWorkerFilter(t *testing.T) {
	a := buildApp(t)
	body := writeFile(t, t.TempDir(), "b.md", "body")
	a.must(t, "", "new", "--title", "Claimed one", "--body-file", body)
	a.must(t, "", "new", "--title", "Unclaimed", "--body-file", body)
	a.must(t, "", "move", "1", "ready")
	a.must(t, "", "start", "1", "--worker", "runner-x")

	got := a.must(t, "", "list", "--worker", "runner-x").stdout
	if !strings.Contains(got, "Claimed one") || strings.Contains(got, "Unclaimed") {
		t.Fatalf("worker filter wrong:\n%s", got)
	}
	if !strings.Contains(got, "CLAIMED SINCE") {
		t.Fatalf("missing claimed-since column:\n%s", got)
	}

	js := a.must(t, "", "list", "--worker", "runner-x", "--json").stdout
	var parsed struct {
		Issues []struct {
			ID        int    `json:"id"`
			Title     string `json:"title"`
			ClaimedBy string `json:"claimed_by"`
		} `json:"issues"`
	}
	if err := json.Unmarshal([]byte(js), &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Issues) != 1 || parsed.Issues[0].ClaimedBy != "runner-x" {
		t.Fatalf("bad worker json: %s", js)
	}

	// composes with --repo: a repo filter that matches nothing yields no rows
	empty := a.must(t, "", "list", "--worker", "runner-x", "--repo", "nosuch").stdout
	if strings.Contains(empty, "Claimed one") {
		t.Fatalf("repo filter ignored:\n%s", empty)
	}

	// claims list regardless of column: move is blocked while claimed, so
	// verify via stop + re-list instead
	a.must(t, "", "stop", "1", "--worker", "runner-x")
	got = a.must(t, "", "list", "--worker", "runner-x").stdout
	if strings.Contains(got, "Claimed one") {
		t.Fatalf("released issue still listed:\n%s", got)
	}

	bad := a.run(t, "", "list", "--worker", "runner-x", "--column", "ready")
	if bad.code == 0 || !strings.Contains(bad.stderr, "cannot be combined") {
		t.Fatalf("expected combination error: %#v", bad)
	}
}

func TestEditIssueBodyRoundTrip(t *testing.T) {
	a := buildApp(t)
	dir := t.TempDir()
	body := writeFile(t, dir, "body.md", "- [ ] first\n- [ ] second\n")
	ticked := writeFile(t, dir, "ticked.md", "- [x] first\n- [x] second\n")
	a.must(t, "", "new", "--title", "Edit me", "--body-file", body)

	bad := a.run(t, "", "edit", "1")
	if bad.code == 0 || !strings.Contains(bad.stderr, "usage: kanban edit") {
		t.Fatalf("expected usage error: %#v", bad)
	}
	bad = a.run(t, "", "edit", "99", "--body-file", ticked)
	if bad.code == 0 || !strings.Contains(bad.stderr, "unknown issue: 99") {
		t.Fatalf("expected unknown issue: %#v", bad)
	}

	a.must(t, "", "edit", "1", "--body-file", ticked)
	resume := a.must(t, "", "resume", "1").stdout
	if !strings.Contains(resume, "Open: 0") || !strings.Contains(resume, "Checked: 2") {
		t.Fatalf("resume not reflecting edit:\n%s", resume)
	}
	show := a.must(t, "", "show", "1").stdout
	if !strings.Contains(show, "- [x] first") {
		t.Fatalf("show not reflecting edit:\n%s", show)
	}

	// Terminal Issues stay editable: tick criteria after closing.
	a.must(t, "", "move", "1", "done")
	a.must(t, "", "edit", "1", "--body-file", body)
	resume = a.must(t, "", "resume", "1").stdout
	if !strings.Contains(resume, "Open: 2") {
		t.Fatalf("terminal issue not editable:\n%s", resume)
	}

	var n int
	db, err := sql.Open("sqlite", a.db)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.QueryRow(`SELECT count(*) FROM events WHERE action='issue.edited'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("issue.edited events=%d, want 2", n)
	}
}

func TestOneWorkerCanHoldSeveralIssuesAndHumanPreseeded(t *testing.T) {
	a := buildApp(t)
	body := writeFile(t, t.TempDir(), "b.md", "body")
	a.must(t, "", "new", "--title", "One", "--body-file", body)
	a.must(t, "", "new", "--title", "Two", "--body-file", body)
	a.must(t, "", "start", "1", "--worker", "agent-many")
	a.must(t, "", "start", "2", "--worker", "agent-many")
	if kind := workerKind(t, a.db, "agent-many"); kind != "agent" {
		t.Fatal(kind)
	}
	if countWorkers(t, a.db, "human") < 1 {
		t.Fatal("expected pre-seeded human worker")
	}
}

func eventCount(t *testing.T, dbpath string) int {
	t.Helper()
	db, err := sql.Open("sqlite", dbpath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM events`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
func workerKind(t *testing.T, dbpath, name string) string {
	t.Helper()
	db, err := sql.Open("sqlite", dbpath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var k string
	if err := db.QueryRow(`SELECT kind FROM workers WHERE name=?`, name).Scan(&k); err != nil {
		t.Fatal(err)
	}
	return k
}
func countWorkers(t *testing.T, dbpath, kind string) int {
	t.Helper()
	db, err := sql.Open("sqlite", dbpath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM workers WHERE kind=?`, kind).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
