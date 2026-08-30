---
type: Guide
title: Contributor guide
description: How to contribute to coprctl: conventions, workflow, and verification.
status: stable
---

# Contributor guide

This page covers how to contribute to `coprctl`. The operational contract
lives in `AGENTS.md`; this is the human-readable how-to for a first change.

## The workflow

All changes land on `main` through a pull request, never by pushing directly.
Each merged PR contributes one conventional-commit message, which Release
Please parses to compute the next version and build the changelog.

1. **Start from latest `main`.** Rebase your work on `main` before opening a
   change.
2. **Work in a dedicated worktree or branch.** Use a conventional branch name
   (`feat/`, `fix/`, `docs/`, `chore/`, `refactor/`) so the intent is obvious.
3. **Keep the change scoped.** One logical unit per PR. Fixup or amend into the
   owning commit on active branches rather than piling up intermediate commits.
4. **Write tests and update docs.** A change is not done until the relevant
   tests are green and the wiki reflects the new behaviour.
5. **Open a pull request.** Squash-merge into `main` so each PR contributes
   exactly one commit.

## Conventions

- **Commits** follow Conventional Commits (`feat:`, `fix:`, `docs:`, ...). No
  trailers (no Co-Authored-By, no Signed-off-by). Stage explicit paths, never
  `git add -A`.
- **Verification** is automated. `make check` runs format, vet, tests, and the
  generated-artefact drift check; hooks enforce style and scope at commit time.
  Do not hand-polish what a tool enforces.
- **Machine-readable everywhere.** New commands and flags must keep the
  `--output json|jsonl|yaml` contract and stable exit codes.
- **Docs stay public-ready.** `docs/` is an OKF v0.2 bundle with no internal
  names, paths, or credentials. `docs/log.md` records wiki evolution.

## Ground rules

- Never rotate a webhook secret implicitly, never pass a token where it would
  be logged, never print a secret without `--reveal`.
- No `sudo` unless a human explicitly requests elevation.

## Related

- [Maintainer guide](maintainers.md) - the review gate and wiki-keeping.
- [Release process](release-process.md) - normal and NVR releases.
- `AGENTS.md` - the authoritative operational contract.