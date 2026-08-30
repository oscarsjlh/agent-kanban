# Kanban

A global, local-first kanban board that tracks issues across all of a user's projects and manages handoff context between the humans and agents that work on them.

## Language

**Board**:
The single global kanban that tracks all issues, across all projects. Lives outside any repo (e.g. `~/.kanban/`).
_Avoid_: tracker, project

**Issue**:
A unit of tracked work on the Board: a durable spec (what to build, acceptance criteria) plus its history. Issues point at the repo they concern.
_Avoid_: task, ticket, card (a card is only the Issue's visual representation on a column)

**Resume Note**:
An append-only entry on an Issue recording where the last Worker stopped — what was tried, what failed, next step — so the next Worker can pick up cold. The on-ramp: a fresh Worker reads the latest one first.
_Avoid_: handoff doc, status update, comment

**Comment**:
An append-only discussion entry on an Issue: triage notes, questions, decisions. Read to understand why, not to resume work.
_Avoid_: note

**Worker**:
A human or an agent that can claim and work on an Issue.

**Runner**:
An AFK agent invoked by a scheduler (cron, a harness loop) that autonomously claims Ready Issues and works them without human intervention. Claims under one stable Worker identity and resumes its own open Claims before picking new work.
_Avoid_: daemon, bot, auto-agent

**Claim**:
The association of exactly one active Worker with an Issue in progress. At most one Worker may hold an Issue; a Worker may hold several Issues.

**Heartbeat**:
A Worker's sign of life on a Claim, refreshed whenever they touch the claimed Issue and callable explicitly during long quiet stretches. Missed heartbeats make a Claim Stale.
_Avoid_: ping, keepalive

**Stale Claim**:
A Claim whose Worker has missed heartbeats beyond the staleness threshold. The Issue stays In Progress but is flagged; nothing returns it to Ready automatically.
_Avoid_: abandoned, orphaned

**Steal**:
To forcibly take over a Stale Claim. The new Worker must acknowledge the previous Worker's state before proceeding.
_Avoid_: reassign, unlock

**Forecast**:
A Worker's estimate of how long they will work without touching the Board. Extends the staleness deadline of their Claim; never shortens it.
_Avoid_: ETA, estimate

**Repo**:
A registered repository the Board knows about. Identified by its git remote URL (or repo root if no remote), with a resolvable local path. Issues may reference a Repo; repo-less Issues are legal. Identity is stable; the display name and local path are editable.
_Avoid_: project

**Blocker**:
An Issue whose completion is a prerequisite for another Issue. Blockers chain transitively; a wontfixed Blocker does not unblock — it raises a flag for a human.

## Columns

**Inbox**: Issues not yet triaged.
**Ready**: Triaged Issues that can be picked up immediately.
**In Progress**: Issues with an active Claim.
**Waiting**: Issues blocked on something external — a reporter, another Issue, a decision.
**Done**: Completed Issues. Terminal.

Terminal states (`Done`, and the off-board `wontfix`) leave the visible board but remain queryable.
