# SQLite store behind a CLI, not markdown files

The Board is the single source of truth for issue tracking, and everything else in this ecosystem (skills, agent briefs, PRDs, ADRs) is agent-readable markdown. We chose SQLite anyway, with the `kanban` CLI as the only sanctioned interface to it, because claims, heartbeats, and column moves need atomicity and concurrency that markdown conventions can't guarantee — two workers flipping a `Status:` line in parallel clobbers one of them.

## Considered Options

- **One markdown file per issue** (the `.scratch/` pattern, extended with frontmatter): directly readable and writable by any agent with file tools, grep is a query language — but no atomicity, no natural home for per-issue append-only history at scale, and ordering within a column requires ugly hacks.
- **SQLite + CLI** (chosen): every interaction, including by agents, goes through `kanban` subcommands; reads expose `--json` for machines and markdown rendering for humans.

## Consequences

- Skills never read the store directly; they shell out to the CLI. CLI ergonomics *are* the product for agents.
- Lost git-diffability is recovered with an append-only events table (who, when, what) on every mutation.
- `kanban resume <id>` must print a markdown briefing good enough to inject straight into a fresh agent session, since agents can no longer just `cat` an issue file.
