package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"kanban/internal/domain"
	_ "modernc.org/sqlite"
)

type Store struct{ DB *sql.DB }

type Repo struct {
	ID                   int64
	Name, Identity, Path string
	CreatedAt            string
}
type Issue struct {
	ID                   int64
	Title, Body, Column  string
	RepoName             sql.NullString
	RepoID               sql.NullInt64
	WaitingReason        sql.NullString
	BlockedBy            sql.NullInt64
	CreatedAt, UpdatedAt string
}
type Comment struct{ Author, Body, CreatedAt string }
type ResumeNote struct{ Worker, Body, CreatedAt string }

func Open() (*Store, error) {
	path := os.Getenv("KANBAN_DB")
	if path == "" {
		home := os.Getenv("KANBAN_HOME")
		if home == "" {
			var err error
			home, err = os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			home = filepath.Join(home, ".kanban")
		}
		path = filepath.Join(home, "kanban.db")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	s := &Store{DB: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.seedHuman(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.DB.Close() }

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS repos (id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE, identity TEXT NOT NULL UNIQUE, path TEXT NOT NULL, created_at TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS issues (id INTEGER PRIMARY KEY, title TEXT NOT NULL, body TEXT NOT NULL, repo_id INTEGER REFERENCES repos(id), column TEXT NOT NULL, waiting_reason TEXT, blocked_by INTEGER REFERENCES issues(id), created_at TEXT NOT NULL, updated_at TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS workers (id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE, kind TEXT NOT NULL, first_seen_at TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS claims (issue_id INTEGER PRIMARY KEY REFERENCES issues(id), worker_id INTEGER NOT NULL REFERENCES workers(id), started_at TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS comments (id INTEGER PRIMARY KEY, issue_id INTEGER NOT NULL REFERENCES issues(id), author TEXT NOT NULL, body TEXT NOT NULL, created_at TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS resume_notes (id INTEGER PRIMARY KEY, issue_id INTEGER NOT NULL REFERENCES issues(id), worker_id INTEGER NOT NULL REFERENCES workers(id), body TEXT NOT NULL, created_at TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS events (id INTEGER PRIMARY KEY, actor TEXT NOT NULL, action TEXT NOT NULL, issue_id INTEGER, created_at TEXT NOT NULL);`,
	}
	for _, stmt := range stmts {
		if _, err := s.DB.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// CurrentUser is the OS-username human Worker that unauthenticated
// frontends (the TUI) act as.
func CurrentUser() string { return currentUser() }

type Claim struct {
	IssueID   int64
	Worker    string
	StartedAt string
}

// AllClaims returns every active Claim on the Board.
func (s *Store) AllClaims() ([]Claim, error) {
	rows, err := s.DB.Query(`SELECT c.issue_id, w.name, c.started_at FROM claims c JOIN workers w ON w.id=c.worker_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Claim
	for rows.Next() {
		var c Claim
		if err := rows.Scan(&c.IssueID, &c.Worker, &c.StartedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func currentUser() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		parts := strings.Split(u.Username, "\\")
		return parts[len(parts)-1]
	}
	return "unknown"
}

func (s *Store) seedHuman() error { _, err := s.ensureWorker(currentUser(), "human"); return err }
func (s *Store) ensureWorker(name, kind string) (int64, error) {
	var id int64
	if err := s.DB.QueryRow(`SELECT id FROM workers WHERE name=?`, name).Scan(&id); err == nil {
		return id, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	res, err := s.DB.Exec(`INSERT INTO workers(name, kind, first_seen_at) VALUES(?,?,?)`, name, kind, now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) AddRepo(identity, path, requestedName string) (Repo, error) {
	name := requestedName
	if name == "" {
		name = filepath.Base(path)
	}
	name = s.uniqueRepoName(name)
	res, err := s.DB.Exec(`INSERT INTO repos(name, identity, path, created_at) VALUES(?,?,?,?)`, name, identity, path, now())
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Repo{}, fmt.Errorf("repo already registered: %s", identity)
		}
		return Repo{}, err
	}
	id, _ := res.LastInsertId()
	return Repo{ID: id, Name: name, Identity: identity, Path: path}, nil
}
func (s *Store) uniqueRepoName(base string) string {
	n := base
	for i := 2; ; i++ {
		var id int
		err := s.DB.QueryRow(`SELECT id FROM repos WHERE name=?`, n).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return n
		}
		n = fmt.Sprintf("%s-%d", base, i)
	}
}
func (s *Store) Repos() ([]Repo, error) {
	rows, err := s.DB.Query(`SELECT id,name,identity,path,created_at FROM repos ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Repo
	for rows.Next() {
		var r Repo
		if err := rows.Scan(&r.ID, &r.Name, &r.Identity, &r.Path, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) CreateIssue(title, body, repoName string) (int64, error) {
	var repoID any = nil
	if repoName != "" {
		var id int64
		if err := s.DB.QueryRow(`SELECT id FROM repos WHERE name=?`, repoName).Scan(&id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return 0, fmt.Errorf("unknown repo: %s", repoName)
			}
			return 0, err
		}
		repoID = id
	}
	t := now()
	res, err := s.DB.Exec(`INSERT INTO issues(title,body,repo_id,column,created_at,updated_at) VALUES(?,?,?,?,?,?)`, title, body, repoID, domain.Inbox, t, t)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	_ = s.event(currentUser(), "issue.created", id)
	return id, nil
}
func (s *Store) Issues(column, repoName string, includeWontfix bool) ([]Issue, error) {
	q := `SELECT i.id,i.title,i.body,i.column,i.repo_id,r.name,i.waiting_reason,i.blocked_by,i.created_at,i.updated_at FROM issues i LEFT JOIN repos r ON r.id=i.repo_id WHERE 1=1`
	var args []any
	if column != "" {
		q += ` AND i.column=?`
		args = append(args, column)
	} else if !includeWontfix {
		q += ` AND i.column<>?`
		args = append(args, domain.Wontfix)
	}
	if repoName != "" {
		q += ` AND r.name=?`
		args = append(args, repoName)
	}
	q += ` ORDER BY i.id`
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Issue
	for rows.Next() {
		var is Issue
		if err := rows.Scan(&is.ID, &is.Title, &is.Body, &is.Column, &is.RepoID, &is.RepoName, &is.WaitingReason, &is.BlockedBy, &is.CreatedAt, &is.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, is)
	}
	return out, rows.Err()
}
// EditBody replaces an Issue body. Terminal Issues are editable on purpose:
// ticking acceptance criteria after closing is a primary use case.
func (s *Store) EditBody(id int64, body string) error {
	if _, err := s.Issue(id); err != nil {
		return err
	}
	_, err := s.DB.Exec(`UPDATE issues SET body=?, updated_at=? WHERE id=?`, body, now(), id)
	if err == nil {
		err = s.event(currentUser(), "issue.edited", id)
	}
	return err
}
// ClaimedBy returns all Issues with an active Claim by the named Worker,
// regardless of column, oldest claim first.
func (s *Store) ClaimedBy(worker, repoName string) ([]Issue, error) {
	q := `SELECT i.id,i.title,i.body,i.column,i.repo_id,r.name,i.waiting_reason,i.blocked_by,i.created_at,i.updated_at FROM issues i JOIN claims c ON c.issue_id=i.id JOIN workers w ON w.id=c.worker_id LEFT JOIN repos r ON r.id=i.repo_id WHERE w.name=?`
	args := []any{worker}
	if repoName != "" {
		q += ` AND r.name=?`
		args = append(args, repoName)
	}
	q += ` ORDER BY c.started_at`
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Issue
	for rows.Next() {
		var is Issue
		if err := rows.Scan(&is.ID, &is.Title, &is.Body, &is.Column, &is.RepoID, &is.RepoName, &is.WaitingReason, &is.BlockedBy, &is.CreatedAt, &is.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, is)
	}
	return out, rows.Err()
}

func (s *Store) Issue(id int64) (Issue, error) {
	var is Issue
	err := s.DB.QueryRow(`SELECT i.id,i.title,i.body,i.column,i.repo_id,r.name,i.waiting_reason,i.blocked_by,i.created_at,i.updated_at FROM issues i LEFT JOIN repos r ON r.id=i.repo_id WHERE i.id=?`, id).Scan(&is.ID, &is.Title, &is.Body, &is.Column, &is.RepoID, &is.RepoName, &is.WaitingReason, &is.BlockedBy, &is.CreatedAt, &is.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return is, fmt.Errorf("unknown issue: %d", id)
	}
	return is, err
}
func (s *Store) Comments(issueID int64) ([]Comment, error) {
	rows, err := s.DB.Query(`SELECT author,body,created_at FROM comments WHERE issue_id=? ORDER BY id`, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.Author, &c.Body, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *Store) ResumeNotes(issueID int64) ([]ResumeNote, error) {
	rows, err := s.DB.Query(`SELECT w.name,n.body,n.created_at FROM resume_notes n JOIN workers w ON w.id=n.worker_id WHERE issue_id=? ORDER BY n.id`, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ResumeNote
	for rows.Next() {
		var n ResumeNote
		if err := rows.Scan(&n.Worker, &n.Body, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) Move(id int64, to, reason string, blockedBy *int64) error {
	is, err := s.Issue(id)
	if err != nil {
		return err
	}
	claimed, _ := s.claimer(id)
	if err := domain.CanMove(is.Column, to, reason != "" || blockedBy != nil, claimed != ""); err != nil {
		return err
	}
	if blockedBy != nil {
		if _, err := s.Issue(*blockedBy); err != nil {
			return fmt.Errorf("unknown blocker: %d", *blockedBy)
		}
	}
	_, err = s.DB.Exec(`UPDATE issues SET column=?, waiting_reason=?, blocked_by=?, updated_at=? WHERE id=?`, to, nullString(reason), nullInt(blockedBy), now(), id)
	if err == nil {
		err = s.event(currentUser(), "issue.moved", id)
	}
	return err
}
func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func nullInt(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}
func (s *Store) AddComment(id int64, body string) error {
	if _, err := s.Issue(id); err != nil {
		return err
	}
	_, err := s.DB.Exec(`INSERT INTO comments(issue_id,author,body,created_at) VALUES(?,?,?,?)`, id, currentUser(), body, now())
	if err == nil {
		err = s.event(currentUser(), "comment.created", id)
	}
	return err
}
func (s *Store) Handoff(id int64, worker, body string) error {
	claimer, err := s.claimer(id)
	if err != nil {
		return err
	}
	if err := domain.CanRelease(claimer, worker); err != nil {
		return err
	}
	wid, err := s.ensureWorker(worker, "agent")
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`INSERT INTO resume_notes(issue_id,worker_id,body,created_at) VALUES(?,?,?,?)`, id, wid, body, now())
	if err == nil {
		err = s.event(worker, "resume_note.created", id)
	}
	return err
}
func (s *Store) Start(id int64, worker string) error {
	if _, err := s.Issue(id); err != nil {
		return err
	}
	c, err := s.claimer(id)
	if err != nil {
		return err
	}
	if err := domain.CanClaim(c); err != nil {
		return err
	}
	wid, err := s.ensureWorker(worker, "agent")
	if err != nil {
		return err
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO claims(issue_id,worker_id,started_at) VALUES(?,?,?)`, id, wid, now()); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE issues SET column=?, updated_at=? WHERE id=?`, domain.InProgress, now(), id); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO events(actor,action,issue_id,created_at) VALUES(?,?,?,?)`, worker, "claim.started", id, now()); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) Stop(id int64, worker, note string) error {
	c, err := s.claimer(id)
	if err != nil {
		return err
	}
	if err := domain.CanRelease(c, worker); err != nil {
		return err
	}
	wid, err := s.ensureWorker(worker, "agent")
	if err != nil {
		return err
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if note != "" {
		if _, err = tx.Exec(`INSERT INTO resume_notes(issue_id,worker_id,body,created_at) VALUES(?,?,?,?)`, id, wid, note, now()); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`DELETE FROM claims WHERE issue_id=?`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE issues SET column=?, updated_at=? WHERE id=?`, domain.Ready, now(), id); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO events(actor,action,issue_id,created_at) VALUES(?,?,?,?)`, worker, "claim.stopped", id, now()); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) claimer(id int64) (string, error) {
	var w string
	err := s.DB.QueryRow(`SELECT w.name FROM claims c JOIN workers w ON w.id=c.worker_id WHERE c.issue_id=?`, id).Scan(&w)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return w, err
}
func (s *Store) event(actor, action string, issueID int64) error {
	_, err := s.DB.Exec(`INSERT INTO events(actor,action,issue_id,created_at) VALUES(?,?,?,?)`, actor, action, issueID, now())
	return err
}
func (s *Store) EventCount() (int, error) {
	var n int
	err := s.DB.QueryRow(`SELECT count(*) FROM events`).Scan(&n)
	return n, err
}
func (s *Store) WorkerKind(name string) (string, error) {
	var k string
	err := s.DB.QueryRow(`SELECT kind FROM workers WHERE name=?`, name).Scan(&k)
	return k, err
}
