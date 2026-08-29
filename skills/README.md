# Skills

This directory vendors the pi-agent skills that operate the Kanban Board. Each skill is a directory containing a `SKILL.md` file — markdown with YAML frontmatter (`name`, `description`) that coding agents discover and load automatically when placed in their skills directory.

## Skills

### kanban-runner

An autonomous AFK **Runner** invoked by a scheduler (typically cron) to work the global Kanban Board without human intervention. It resumes its own open Claims first, otherwise picks the oldest eligible Ready Issue, works it in an isolated git worktree under `~/.kanban/worktrees/`, and always exits in a protocol-defined state (done, handed off back to Ready, or moved to Waiting). One invocation works at most one Issue — repetition is the scheduler's job. Two environment variables tune it: `KANBAN_RUNNER_WORKER` sets the stable Worker identity used for claims (defaults to `afk-agent-$(hostname)`), and `KANBAN_RUNNER_REPOS` (comma-separated repo names) restricts which Repos' Issues are eligible.

### setup-kanban

Wires a repo to the global Kanban Board so engineering skills use the `kanban` CLI instead of GitHub/GitLab issues: it writes an `## Agent skills` block into `AGENTS.md` or `CLAUDE.md` and creates the `docs/agents/` convention files (`issue-tracker.md`, `triage-labels.md`, `domain.md`) that other skills read. Its frontmatter sets `disable-model-invocation: true`, so agents will not run it on their own — it is invoked explicitly by the user only.

## Install

Symlink (or copy) each skill directory into your agent's skills directory. For pi:

```sh
ln -s "$PWD/skills/kanban-runner" ~/.pi/agent/skills/kanban-runner
ln -s "$PWD/skills/setup-kanban" ~/.pi/agent/skills/setup-kanban
```

Other harnesses have equivalent skill directories (e.g. `~/.claude/skills` for Claude Code); symlink into whichever your agent reads.

## Prerequisites

- `kanban-runner` assumes the `kanban` binary is on `PATH`.
- It expects the working repo to follow the `docs/agents/issue-tracker.md` convention for board access — which is exactly what `setup-kanban` writes. Run `setup-kanban` in a repo before the Runner picks up its Issues.

## Scheduler example

Run pi non-interactively on an interval so the Runner keeps working the board:

```cron
*/15 * * * * cd <repo> && pi -p "use the kanban-runner skill" >>~/.kanban/runner.log 2>&1
```

This is an example to adapt: substitute your repo path, your harness's non-interactive invocation, and your preferred log location. The Runner's design (claim → work one Issue → exit in a protocol-defined state) makes it safe to invoke on any cadence.
