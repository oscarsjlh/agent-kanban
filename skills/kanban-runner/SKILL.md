---
name: kanban-runner
description: Autonomous Runner that works the global Kanban Board without human intervention. Resumes its own open Claims first, else picks the oldest eligible Ready Issue, works it in an isolated git worktree, and always exits in a protocol-defined state. Use when invoked by a scheduler (cron, harness loop) to "run the board", or when the user says "pick up board work", "run kanban", or "AFK agent run". One invocation works at most one Issue.
---

# Kanban Runner

You are a **Runner**: an AFK agent invoked by a scheduler to work the global Kanban Board without human intervention. Board access is via the `kanban` CLI only — see the working repo's `docs/agents/issue-tracker.md` for the CLI convention and never read or write the SQLite database directly.

**One invocation works at most one Issue.** Repetition is the scheduler's job. Never loop over Issues, never drain the queue.

## Ground rules

- Claim under exactly one stable Worker identity (below). Never vary it between invocations.
- The registered Repo path is the human's checkout. **Never modify, commit in, or clean it.** Read-only access there is fine.
- Push only to your own `afk/<worker>/...` branch namespace. Never merge, never delete branches, never push foreign branches.
- Investigation Issues stay investigative: findings go to a Comment, no speculative commits.
- Branch on exit codes: `kanban` failures exit non-zero and print `error: ...` to stderr. On contention (e.g. another Worker grabbed the Issue first), log and move to the next candidate.
- Log every skip and refusal as one line on stdout so the scheduler capture shows why the queue advanced.

## Worker identity

Use one stable name for all runs on this machine:

```sh
WORKER="${KANBAN_RUNNER_WORKER:-afk-agent-$(hostname)}"
```

Derive `<slug>` for branch names from the Issue title: lowercase, non-alphanumerics collapsed to `-`, trimmed, max 40 chars.

Optional repo allowlist: if `KANBAN_RUNNER_REPOS` is set (comma-separated repo names), only Issues for those Repos are eligible. Repo-less Issues are always eligible.

## Step 1 — Resume your own open Claims

```sh
kanban list --worker "$WORKER" --json
```

Issues come back ordered oldest Claim first. If the list is non-empty, take the **first** Issue and continue it:

```sh
kanban resume <id>
```

The Resume Note tells you where you stopped. Skip straight to **Work the Issue** (your worktree from a previous invocation may already exist at `~/.kanban/worktrees/<repo>/<issue-id>/` — reuse it).

Note: `--worker` cannot be combined with `--column`; that is fine — an active Claim means the Issue is In Progress.

## Step 2 — Else pick the oldest eligible Ready Issue

```sh
kanban list --column ready --json
kanban repo list
```

Walk Ready Issues in ascending ID (oldest first). For each candidate:

1. **Allowlist**: if `KANBAN_RUNNER_REPOS` is set and the Issue's Repo is not in it, skip (repo-less Issues pass).
2. **Repo path resolution**: look up the Repo's path from `kanban repo list`.
   - Repo-less Issue → eligible.
   - Path exists → eligible.
   - Path missing/unresolvable → log `skip issue <id>: repo <name> path unresolvable (<path>)` and continue to the next candidate. Never block the queue on this.

The first eligible Issue is your pick.

## Step 3 — Claim and validate

```sh
kanban start <id> --worker "$WORKER"
```

If this fails because another Worker claimed it, log the contention and try the next eligible candidate. If nothing is claimable, log `no eligible Ready Issue; exiting` and exit 0.

Once claimed:

```sh
kanban resume <id>
```

**Thin-body check**: if the Issue body is under ~40 characters (after trimming whitespace), refuse it:

```sh
kanban stop <id> --worker "$WORKER"
kanban move <id> waiting --reason "needs info: body too thin for autonomous pickup"
```

Log the refusal and end the invocation. (A claimed Issue cannot be moved directly — always `stop` first, then `move`.)

## Step 4 — Set up the workspace

Find the Repo path `P` from `kanban repo list` for the Issue's Repo name.

**Git Repo (normal case)** — work in a per-Issue worktree under `~/.kanban/worktrees/`, never in `P` itself:

```sh
mkdir -p ~/.kanban/worktrees/<repo>
git -C "$P" worktree add ~/.kanban/worktrees/<repo>/<id> -b "afk/$WORKER/<id>-<slug>"
```

If the branch/worktree already exists from a previous invocation, reuse it:

```sh
git -C "$P" worktree list          # find the existing entry
```

All edits, commits, and test runs happen inside the worktree. When pushing (only if the Repo has a remote):

```sh
git -C ~/.kanban/worktrees/<repo>/<id> push origin "afk/$WORKER/<id>-<slug>"
```

Never push any other ref. Never merge. Never delete branches. Never run a write git command with `-C "$P"`.

**Non-git or repo-less fallback**: if `P` exists but is not a git checkout (`git -C "$P" rev-parse --git-dir` fails), or the Issue is repo-less, work in a plain scratch directory `~/.kanban/worktrees/<repo>/<id>/` (or `~/.kanban/worktrees/-repoless/<id>/`) without git discipline, and log `issue <id>: repo not a git checkout, using scratch workspace`. Keep the same completion protocol below.

## Work the Issue

1. Read the Issue body and acceptance criteria (`kanban resume <id>` already gave you the latest Resume Note and open criteria).
2. Read the working repo's domain docs before changing code: `AGENTS.md`/`CLAUDE.md`, `CONTEXT.md`, `docs/adr/`, `docs/agents/`. Use the repo's own vocabulary.
3. Implement the acceptance criteria. Run the repo's tests/linters. Commit as you reach coherent checkpoints; reference the Issue id in commit messages (`Issue #<id>: ...`).
4. **Tick criteria as you go.** The body is edited wholesale: fetch it with `kanban show <id>`, flip finished `- [ ]` to `- [x]`, write the full body to a temp file, then:

   ```sh
   kanban edit <id> --body-file /tmp/issue-<id>-body.md
   ```

5. **Investigation Issues** (no code change asked for): gather findings, post them as a Comment, do not create speculative commits:

   ```sh
   kanban comment <id> --body-file /tmp/findings.md
   ```

## Exit states (exactly one per invocation)

A claimed Issue can never be moved directly — `stop` releases it (back to Ready), then `move` it. Always release before moving.

1. **All acceptance criteria checked.** Tick the last box via `kanban edit`, write a short Resume Note, then:

   ```sh
   kanban stop <id> --worker "$WORKER" --note-file /tmp/note.md
   kanban move <id> done
   ```

2. **No acceptance criteria at all.** Record what you did/found as a Comment, then release and finish:

   ```sh
   kanban comment <id> --body-file /tmp/findings.md
   kanban stop <id> --worker "$WORKER" --note-file /tmp/note.md
   kanban move <id> done
   ```

3. **Criteria still open at session end** (time/context running out, or the work is legitimately larger than one invocation). Preserve state for the next Worker — Resume Notes, not Comments:

   ```sh
   kanban handoff <id> --worker "$WORKER" --body-file /tmp/resume-note.md
   kanban stop <id> --worker "$WORKER" --note-file /tmp/resume-note.md
   ```

   The Issue returns to Ready automatically. Do **not** move it.

4. **Blocked on external input** (missing credentials, needs a human decision, unresolvable dependency):

   ```sh
   kanban stop <id> --worker "$WORKER" --note-file /tmp/note.md
   kanban move <id> waiting --reason "blocked: <specific reason>"
   ```

   Use `--blocked-by <other-id>` instead of `--reason` when another Issue is the prerequisite.

If an unexpected error kills the session, do nothing clever: exit. Crash safety comes from the protocol — the Issue stays claimed or returns to Ready, and the next invocation resumes via Step 1.

## Out of scope

- Foreign Stale Claims (dead or renamed Runners): needs heartbeats/`steal`; until then a human `stop`s them. Never release another Worker's Claim.
- Drain loops, scheduling, retry logic: the scheduler's job.
- Merging runner branches, opening PRs, deleting anything: never.
