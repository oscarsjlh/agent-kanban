---
name: setup-kanban
description: Sets up a repo to use the global Kanban CLI Board with engineering skills by writing AGENTS.md/CLAUDE.md guidance and docs/agents tracker/domain files. Use before to-prd, to-issues, triage, qa, tdd, diagnose, or handoff workflows when the repo should track work with `kanban` instead of GitHub, GitLab, or .scratch markdown.
disable-model-invocation: true
---

# Setup Kanban

Configure the current repo so engineering skills use the global Kanban Board through the `kanban` CLI.

## What this writes

- An `## Agent skills` block in `CLAUDE.md` or `AGENTS.md`
- `docs/agents/issue-tracker.md` with the Kanban CLI convention
- `docs/agents/triage-labels.md` mapping canonical triage roles onto Kanban columns/commands
- `docs/agents/domain.md` with domain-doc consumer rules

## Process

### 1. Explore

Read the repo state first:

- `kanban repo list` if `kanban` is installed
- `AGENTS.md` and `CLAUDE.md` at the repo root
- `CONTEXT.md` and `CONTEXT-MAP.md`
- `docs/adr/` and any `src/*/docs/adr/`
- `docs/agents/`

### 2. Confirm repo registration

Check whether the current repo appears in `kanban repo list`.

If not registered, ask for confirmation, then run:

```sh
kanban repo add <repo-root> [--name <repo-name>]
```

Use a short repo name derived from the directory unless the user asks for another name.

### 3. Choose agent file

- If `CLAUDE.md` exists, edit it.
- Else if `AGENTS.md` exists, edit it.
- If neither exists, ask the user which one to create.

If an `## Agent skills` block already exists, update it in place. Do not append a duplicate.

### 4. Write the Agent skills block

```markdown
## Agent skills

### Issue tracker

Issues live on the global Kanban Board and must be accessed via the `kanban` CLI only. See `docs/agents/issue-tracker.md`.

### Triage labels

Triage roles map to Kanban columns and lifecycle commands. See `docs/agents/triage-labels.md`.

### Domain docs

[Single-context or multi-context] domain docs. See `docs/agents/domain.md`.
```

Use `Single-context` when the repo has root `CONTEXT.md`/`docs/adr/`. Use `Multi-context` when `CONTEXT-MAP.md` exists.

### 5. Write `docs/agents/issue-tracker.md`

Use this content:

```markdown
# Issue tracker: Kanban CLI

Issues and PRDs for this repo live on the user's global local Kanban Board. Use the `kanban` CLI for all operations. Do not read or write the SQLite database directly.

## Repo

This repo should be registered with the Board:

\```sh
kanban repo add <repo-root> [--name <repo-name>]
kanban repo list
\```

Use the registered repo name with `--repo <repo-name>` when creating or filtering Issues.

## Publish to the issue tracker

Create one Kanban Issue per PRD or vertical slice:

\```sh
kanban new --title "Short imperative title" --repo <repo-name> --body-file /path/to/body.md
\```

Put long-form markdown in the body file. Use markdown checkboxes (`- [ ] ...`) for acceptance criteria so `kanban resume` can summarize open work. Omit `--repo` only for repo-less work.

## Fetch the relevant ticket

Use markdown for human/agent context:

\```sh
kanban show <id>
\```

For a fresh Worker picking up an Issue, prefer:

\```sh
kanban resume <id>
\```

Use JSON for machine selection:

\```sh
kanban list --json
kanban list --column ready --json
kanban list --repo <repo-name> --json
\```

## Triage and lifecycle

\```sh
kanban move <id> ready
kanban move <id> waiting --reason "blocked on review"
kanban move <id> waiting --blocked-by <other-id>
kanban move <id> done
kanban move <id> wontfix
\```

Waiting always needs `--reason` or `--blocked-by`. Terminal Issues (`done`, `wontfix`) are not moved again.

Blockers persist across moves. When a blocker reaches `done`, its blocked Issues in Waiting auto-promote to Ready. Clear a blocker explicitly with `--unblocked`; moving to Ready while the blocker is open is rejected.

## Claim, handoff, and release

Agents must pass a stable worker name on every claim, handoff, and release command:

\```sh
--worker=<harness>-<session>
\```

Examples: `pi-2026-08-29T12-00Z`, `claude-code-abc123`, `codex-run-42`.

Before implementation:

\```sh
kanban start <id> --worker=<harness>-<session>
\```

For resumable work state:

\```sh
kanban handoff <id> --worker=<harness>-<session> --body-file /path/to/resume-note.md
kanban stop <id> --worker=<harness>-<session> --note-file /path/to/resume-note.md
\```

Stopping without `--note-file` is allowed but warns that the next Worker starts cold.

## Comments

Use Comments for discussion, questions, triage notes, and decisions:

\```sh
kanban comment <id> --body-file /path/to/comment.md
\```

Use Resume Notes, not Comments, for in-progress state that the next Worker needs.

## Error handling

Successful commands print human-readable output to stdout. Failures exit non-zero and print `error: ...` to stderr. Skills should branch on exit code. Prefer `--json` for reads that need parsing.
```

### 6. Write `docs/agents/triage-labels.md`

Use this content:

```markdown
# Triage Labels

The skills speak in terms of five canonical triage roles. On the Kanban Board these roles map to columns or lifecycle commands instead of tracker labels.

| Canonical role | Kanban action | Meaning |
| --- | --- | --- |
| `needs-triage` | Issue is in `Inbox` | Maintainer needs to evaluate this Issue |
| `needs-info` | `kanban move <id> waiting --reason "needs info: ..."` | Waiting on reporter or external input |
| `ready-for-agent` | `kanban move <id> ready` | Fully specified, ready for an AFK agent |
| `ready-for-human` | `kanban move <id> waiting --reason "needs human: ..."` | Needs human implementation or decision |
| `wontfix` | `kanban move <id> wontfix` | Will not be actioned |

When a skill says to apply a label, perform the corresponding Kanban action instead.
```

### 7. Write `docs/agents/domain.md`

Copy the standard domain-doc rules, adjusted for the detected layout:

- Single-context: root `CONTEXT.md` + root `docs/adr/`
- Multi-context: root `CONTEXT-MAP.md` points at context-specific docs

### 8. Done

Tell the user setup is complete. Mention that `to-prd`, `to-issues`, `triage`, `qa`, `tdd`, `diagnose`, and handoff workflows will now read `docs/agents/*.md` and use Kanban CLI conventions.
