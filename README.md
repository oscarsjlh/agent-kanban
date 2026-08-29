# agent-kanban

A local-first, global kanban board for humans **and** autonomous AI agents. One SQLite database tracks Issues across all of a user's repos, and the `kanban` CLI is the only sanctioned interface to it — agents shell out to the same commands a human types. Issues carry durable specs, acceptance criteria, and append-only history, so a fresh Worker (human or agent) can pick up any Issue cold. No server, no sync, no per-repo trackers: one Board at `~/.kanban/`, one binary.

## Features

- **Columns**: `inbox`, `ready`, `in progress`, `waiting`, `done`, plus off-board `wontfix`. Terminal Issues leave the visible board but stay queryable (`kanban list --column wontfix`, `--all`).
- **Claims with Worker identities**: at most one Worker may hold an Issue; a Worker may hold several. `kanban start <id> --worker W` claims, `kanban list --worker W` shows a Worker's open Claims.
- **Resume Notes as cold-start briefings**: every handoff/stop appends a Resume Note — what was tried, what failed, next step. `kanban resume <id>` renders the latest state as a markdown briefing good enough to inject straight into a fresh agent session.
- **Append-only Comments and events**: Comments hold discussion and decisions; an events table records every mutation (who, when, what) in place of git history.
- **Blocker chains**: `kanban move <id> waiting --blocked-by <other-id>` links prerequisites; blockers chain transitively.
- **Machine and human output**: `--json` on reads for programs, markdown rendering for people.
- **TUI frontend**: `kanban tui` opens a Bubble Tea board view. It calls the same store/domain code in-process (ADR 0003), so the TUI and CLI can never disagree about what a legal move or Claim is. Agents stay on CLI subcommands.

## Install

Requires Go 1.24.

```sh
go install github.com/oscarsjlh/agent-kanban/cmd/kanban@latest
```

Or build from source:

```sh
git clone https://github.com/oscarsjlh/agent-kanban.git
cd agent-kanban
go build -o kanban ./cmd/kanban
```

The binary uses a pure-Go SQLite driver (`modernc.org/sqlite`); no cgo required.

## Quickstart

Register a repo (identified by its git remote URL, so renames and moves don't orphan Issues):

```sh
kanban repo add ~/projects/myapp --name myapp
```

File an Issue with acceptance criteria as markdown checkboxes:

```sh
cat > /tmp/body.md <<'EOF'
Add a retry loop to the sync job.

## Acceptance criteria
- [ ] Retries up to 3 times with backoff
- [ ] Logs each retry
EOF
kanban new --title "Add retry to sync job" --repo myapp --body-file /tmp/body.md
# created issue 1
```

Triage it:

```sh
kanban move 1 ready
```

Claim it under a stable Worker identity and read the briefing:

```sh
kanban start 1 --worker pi-session-1
kanban resume 1
```

Record progress without releasing the Claim:

```sh
kanban handoff 1 --worker pi-session-1 --body-file /tmp/resume-note.md
```

Release the Claim back to Ready (always leave a Resume Note — the next Worker starts cold otherwise):

```sh
kanban stop 1 --worker pi-session-1 --note-file /tmp/resume-note.md
```

Finish it (a released Issue is unclaimed, so it can be moved directly; a claimed one cannot):

```sh
kanban move 1 done
```

## CLI reference

| Command | Purpose |
| --- | --- |
| `kanban help` | Command overview (`--help`, `-h` work too) |
| `kanban tui` | Interactive board view (human frontend) |
| `kanban repo add <path> [--name NAME]` | Register a Repo with the Board |
| `kanban repo list` | List registered Repos |
| `kanban repo rename <name> <new-name>` | Rename a Repo; hard error if the name is taken |
| `kanban repo move <name> <new-path>` | Re-path a Repo; remote-URL identities are untouched, path identities follow the move |
| `kanban new --title T (--body-file F \| --body-stdin) [--repo NAME]` | Create an Issue in Inbox |
| `kanban list [--column C] [--repo NAME] [--worker W] [--all] [--json]` | List Issues or a Worker's Claims |
| `kanban show <id>` | Render an Issue and its Comments |
| `kanban resume <id>` | Cold-start briefing: Issue + latest Resume Notes |
| `kanban edit <id> --body-file F` | Replace an Issue body |
| `kanban move <id> <column> [--reason R] [--blocked-by ID]` | Move between columns; `waiting` needs `--reason` or `--blocked-by` |
| `kanban comment <id> --body-file F` | Append a Comment |
| `kanban start <id> --worker W` | Claim an Issue |
| `kanban stop <id> --worker W [--note-file F]` | Release a Claim, appending a Resume Note |
| `kanban handoff <id> --body-file F [--worker W]` | Append a Resume Note while keeping the Claim |

Notes:

- Flags may appear before or after positional arguments; `--flag=value` also works.
- `--worker` on `handoff` falls back to the `KANBAN_WORKER` environment variable.
- `--json` on `list` includes each Issue's claimant; exit codes and `error: ...` on stderr are the contract for scripts and skills.
- The Board lives at `~/.kanban/kanban.db`. Override the directory with `KANBAN_HOME` or the exact database file with `KANBAN_DB`.

## Agent skills

This repo ships [pi-agent](https://github.com/earendil-works/pi) skills in `skills/`:

- **`kanban-runner`** — an autonomous Runner: an AFK agent invoked by a scheduler (cron, harness loop) that resumes its own open Claims, else picks the oldest eligible Ready Issue, works it in an isolated git worktree, and always exits in a protocol-defined state. One invocation works at most one Issue; repetition is the scheduler's job (ADR 0004).
- **`setup-kanban`** — wires any repo's `AGENTS.md`/`CLAUDE.md` and `docs/agents/` files to the Board so other skills (triage, to-issues, qa, tdd, …) use the `kanban` CLI instead of a per-repo tracker.

See `skills/README.md` for installation.

## Design docs

- [CONTEXT.md](CONTEXT.md) — the vocabulary and domain model (Board, Issue, Resume Note, Claim, Worker, Runner, Stale Claim, Steal, Blocker). Read this first.
- [docs/adr/](docs/adr/) — architectural decisions: SQLite behind a CLI (0001), one global board outside repos (0002), the TUI as a second frontend (0003), autonomous pickup as a scheduled skill rather than a daemon (0004).
- [docs/agents/](docs/agents/) — the CLI convention, triage-role mapping, and domain-doc rules this repo itself follows.

## Status and limitations

- **Heartbeats and steal are designed, not implemented.** CONTEXT.md defines Stale Claims (missed heartbeats past a threshold) and Steal (forcible takeover of a Stale Claim with acknowledgment of the previous Worker's state), but neither exists in code yet. Until they land, a dead Worker's Claim must be released by a human running `kanban stop`.
- **No remote sync.** The Board is deliberately a single local file (ADR 0002). All state lives in one database so multi-machine support, if ever needed, stays a file-copy or replication problem rather than a sync-protocol retrofit. Single machine for now.
- **Single active Claim per Issue.** Concurrent workers on one Issue are impossible by construction; contention surfaces as a failed `start`, which skills treat as "move to the next candidate".

## License

GPL-3.0 — see [LICENSE](LICENSE).
