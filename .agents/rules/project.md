# Coprctl project rules

Project-specific rules that extend the global standards. These are invariants
for any agent or human working in this repository.

## Scope discipline

- Before any work, state the scope in one sentence: unit of work (feature |
  bug fix | maintenance) + target + outcome. Anything not needed to produce
  that outcome is out of scope by default.
- Never widen a diff to look more thorough. The smallest change that fully and
  correctly satisfies the scope, with passing tests and honest docs, is the
  target.

## Worktrees and history

- Every scoped change gets its own worktree with a conventional branch name
  (`feat/`, `fix/`, `docs/`, `chore/`, `refactor/`). Rebase on latest `main`
  before starting. This repository is upstream, so `origin` is authoritative.
- Commit messages follow Conventional Commits, summary-first, no trailers.
  Fixup/amend into the owning commit on active branches.

## Generated artefacts

- The Cobra command registry is the single source of truth. Never hand-edit
  generated files (completions, JSON schema, MCP tools, docs, skill). If a
  change needs a generated file, the change belongs in the registry or the
  generator, then regenerate.

## Secrets and safety

- Never print a secret without `--reveal`. Never rotate a webhook secret
  implicitly. Never pass a token where it would be logged.
- No sudo in this repo unless a human explicitly requests elevation.

## Verification gate

- A change is done only when `make check` passes and the relevant tests are
  green, with real captured output. `docs/log.md` and `.scratch/outbox/log.md`
  reflect the change.