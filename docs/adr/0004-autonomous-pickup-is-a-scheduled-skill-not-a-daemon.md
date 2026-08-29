# Autonomous pickup is a scheduled skill, not a Board daemon

With the `kanban-runner` skill, AFK agents pick up Ready Issues without human intervention. The pickup mechanism deliberately lives in a harness-scheduled skill, not in the Board binary: no `kanban daemon`, no watcher, no process-lifecycle ownership. One invocation works at most one Issue; repetition is the scheduler's job.

## Considered Options

- **Daemon in the Board binary**: persistent process watching the DB and spawning agents. Rejected — it would give the tracker opinions about agent binaries, credentials, and scheduling, and kill the ADR-0001/0002 simplicity (passive store, CLI-only access, one file).
- **Scheduled skill polling Ready** (chosen): `kanban list --column ready --json` + exclusive claims already make concurrent polling safe; the Board stays passive and the workflow stays editable as skill text.

## Consequences

- Crash safety comes from the protocol, not the binary: stable Runner Worker identity, own-claims-first resume, and `stop` returning Issues to Ready mean every exit state is resumable.
- Foreign stale Claims (dead or renamed Runners) are not recoverable until heartbeats/`steal` land; until then a human `stop`s them.
- Runners work in per-Issue git worktrees under `~/.kanban/worktrees/` and push only to their `afk/<worker>/...` branch namespace; the registered Repo path is the human's checkout and is never touched.
