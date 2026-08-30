# Staleness is derived at read time, not swept

Claims must eventually be recognized as Stale so other Workers can steal them, but staleness is a fact about the clock, not a mutation of the Board. We decided staleness is a pure function computed on every read — `now() > last_seen_at + deadline`, where the deadline comes from the Claim's heartbeats and Forecast — with no stored flag, no `sweep` command, and no background process.

## Considered Options

- **Persisted flag with a sweep process**: rejected — ADR-0004 forbids a daemon, and any stored flag is wrong between sweeps (a Claim becomes stale by time passing, not by being marked).
- **Explicit `kanban sweep` command run by humans or the scheduler**: rejected — it would still only recompute what the clock already knows, and its output would decay the moment it finished.
- **Derived at read time** (chosen): Claims carry `last_seen_at` and the Forecast; every read (`list`, `show`, `resume`, TUI) computes the stale state. Nothing to run, nothing to schedule, nothing that can lie.

## Consequences

- The `claims` table carries liveness inputs (`last_seen_at`, forecast) but never a stale flag; reads must be clock-aware.
- Steal is the only write-path consumer of staleness: it refuses unless the derivation says stale.
- No component ever returns a stale Issue to Ready automatically — a stale Issue stays In Progress until stopped or stolen, per CONTEXT.md.
- If a future surface needs countdowns or thresholds-as-config, it extends the same derivation; it does not reintroduce a stored flag.
