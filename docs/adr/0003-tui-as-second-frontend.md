# TUI is a second frontend, not a second process

ADR-0001 says the `kanban` CLI is the only sanctioned interface to the SQLite store. When the Bubble Tea TUI was built, that rule was interpreted as *no external process touches the database* — not "every interaction must shell out to a subcommand". The TUI lives in the same binary and calls `internal/store` and `internal/domain` in-process, reusing the exact same validation and event-logging code paths as the CLI commands.

## Considered Options

- **Shell out to `kanban` subcommands**: obeys "CLI is the only interface" literally, but re-opens the DB per action, re-parses output, duplicates error handling, and breaks when the binary isn't on PATH (e.g. `go run`). Gains nothing — the events and domain rules already live in the store/domain packages.
- **In-process calls into `store`/`domain`** (chosen): single code path for Board rules; the TUI and CLI can never disagree about what a legal move or claim is.

## Consequences

- The CLI and TUI are two frontends of one Board; any new Board capability must land in `store`/`domain` first so both frontends get it.
- Agent workflows stay on CLI subcommands (`--json`, exit codes); the TUI is a human-facing surface and is not a contract for skills.
