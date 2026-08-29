# One global board outside any repo replaces per-repo trackers

Issue tracking so far was per-repo: GitHub Issues, or `.scratch/` markdown with per-repo config in `docs/agents/issue-tracker.md`. We decided to replace that with a single global Board (`~/.kanban/`) tracking issues across all projects, with each issue pointing at a registered Repo. The motivation is cross-project visibility — one place to see what's alive everywhere — and long-running task management that outlives any single checkout.

## Considered Options

- **Per-repo boards** (e.g. `.kanban/` inside each repo, git-tracked): keeps board next to code, gets history and trust via git for free, matches the per-repo assumptions of the skill ecosystem — but gives N places to look for N projects and no cross-project view.
- **Global board** (chosen): single DB file outside all repos; repos are registered explicitly (`kanban repo add`) and identified by git remote URL so renames/moves don't orphan issues.

## Consequences

- The Board loses git history and code proximity; audit is reconstructed from the events table, and trust in agent mutations depends on the CLI logging rather than repo diffs.
- All state must stay inside the one DB file — no per-machine caches — so that multi-machine support, if ever needed, remains a file-copy or replication problem rather than a sync-protocol retrofit. Single-machine for now.
- Repos that choose the global Board should be configured with the dedicated `setup-kanban` skill instead of local-markdown setup; no importer is built for existing `.scratch/` content.
