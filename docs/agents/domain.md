# Domain docs

This repo uses a single-context domain documentation layout.

## Required reading

Before changing code or filing/planning Issues, agents must read:

1. `CONTEXT.md` for the repo vocabulary, domain model, and naming rules.
2. `docs/adr/` for accepted architectural decisions and constraints.
3. Any feature-local docs referenced by `CONTEXT.md` or relevant ADRs.

## How to use domain docs

- Use the terms from `CONTEXT.md` in Issues, PRDs, comments, Resume Notes, commit messages, and code names.
- Treat ADRs as constraints. If implementation pressure conflicts with an ADR, stop and ask before changing direction.
- Prefer updating domain docs when new durable vocabulary, invariants, or architectural decisions are discovered.
- Do not create parallel terms for existing concepts. For this repo, use `Board`, `Issue`, `Resume Note`, `Comment`, `Worker`, `Claim`, `Stale Claim`, `Steal`, `Repo`, and `Blocker` as defined in `CONTEXT.md`.

## Updating docs

- Update `CONTEXT.md` for durable language, entity relationships, and workflow invariants.
- Add an ADR under `docs/adr/` for consequential architectural decisions.
- Keep issue-tracker mechanics in `docs/agents/issue-tracker.md`, not in `CONTEXT.md`.
