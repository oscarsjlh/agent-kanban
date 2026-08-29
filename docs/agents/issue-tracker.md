# Issue tracker: Kanban CLI

Issues and PRDs for this repo live on the user's global local Kanban Board. Use the `kanban` CLI for all operations. Do not read or write the SQLite database directly.

## Repo

This repo should be registered with the Board:

```sh
kanban repo add <repo-root> [--name <repo-name>]
kanban repo list
```

Use the registered repo name with `--repo <repo-name>` when creating or filtering Issues.

## Publish to the issue tracker

Create one Kanban Issue per PRD or vertical slice:

```sh
kanban new --title "Short imperative title" --repo <repo-name> --body-file /path/to/body.md
```

Put long-form markdown in the body file. Use markdown checkboxes (`- [ ] ...`) for acceptance criteria so `kanban resume` can summarize open work. Omit `--repo` only for repo-less work.

## Fetch the relevant ticket

Use markdown for human/agent context:

```sh
kanban show <id>
```

For a fresh Worker picking up an Issue, prefer:

```sh
kanban resume <id>
```

Use JSON for machine selection:

```sh
kanban list --json
kanban list --column ready --json
kanban list --repo <repo-name> --json
```

## Triage and lifecycle

```sh
kanban move <id> ready
kanban move <id> waiting --reason "blocked on review"
kanban move <id> waiting --blocked-by <other-id>
kanban move <id> done
kanban move <id> wontfix
```

Waiting always needs `--reason` or `--blocked-by`. Terminal Issues (`done`, `wontfix`) are not moved again.

Blockers persist across moves. When a blocker reaches `done`, its blocked Issues in Waiting auto-promote to Ready. Clear a blocker explicitly with `--unblocked`; moving to Ready while the blocker is open is rejected.

## Claim, handoff, and release

Agents must pass a stable worker name on every claim, handoff, and release command:

```sh
--worker=<harness>-<session>
```

Examples: `pi-2026-08-29T12-00Z`, `claude-code-abc123`, `codex-run-42`.

Before implementation:

```sh
kanban start <id> --worker=<harness>-<session>
```

For resumable work state:

```sh
kanban handoff <id> --worker=<harness>-<session> --body-file /path/to/resume-note.md
kanban stop <id> --worker=<harness>-<session> --note-file /path/to/resume-note.md
```

Stopping without `--note-file` is allowed but warns that the next Worker starts cold.

## Comments

Use Comments for discussion, questions, triage notes, and decisions:

```sh
kanban comment <id> --body-file /path/to/comment.md
```

Use Resume Notes, not Comments, for in-progress state that the next Worker needs.

## Error handling

Successful commands print human-readable output to stdout. Failures exit non-zero and print `error: ...` to stderr. Skills should branch on exit code. Prefer `--json` for reads that need parsing.
