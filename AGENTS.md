# AGENTS.md

Operational contract for humans and agents working in this repository.

## Project

`coprctl` is a reimagined command-line and agent interface for the Fedora Copr
build system. Full design and rationale live in the wiki under `docs/`.

## Invariants

- **Noun-verb grammar, one reference parser.** `coprctl <resource> <verb> [ref]`.
  Every command accepts the shared `owner/project[:dir][/segment]` reference.
  A new source type is a flag value, never a new command.
- **One registry, many artefacts.** The Cobra command registry is the single
  source of truth. CLI, completions, JSON schema, MCP tools, docs, and the agent
  skill are all generated from it. Adding a command must not require touching
  any generated file by hand.
- **Machine-readable everywhere.** `--output json|jsonl|yaml` on every command,
  stable exit codes, structured error objects. Human output is a rendering of
  the same structs.
- **Instance-agnostic.** Never hardcode the Fedora instance; profiles are
  first-class. Offline-tolerant: cache the chroot catalog.
- **Always-public-ready docs.** `docs/` is an OKF v0.2 bundle. No internal
  names, codenames, hostnames, absolute paths, tokens, or task identifiers.
- **No AI slop.** No em-dashes, no marketing fluff, no filler prose, no
  comments that restate the code. Code and docs read like a human wrote them.

## Automation and conventions

- **Prefer automation over manual conformance.** Hooks and `make check` own
  style, conventions, and quality gates. Do not hand-polish what a tool can
  enforce.
- **Tool-specific assets stay out of the repo.** A tool's own working files
  (shims, local state, per-tool prompts) and the project's internal working
  notes are git-ignored. Never commit a tool-specific asset.
- **Makefile** is the single automation entrypoint (`make help` by default).
  Targets: `build`, `check`, `fmt`, `lint`, `test`.
- **Commits** follow Conventional Commits. No trailers (no Co-Authored-By, no
  Signed-off-by). Stage explicit paths, never `git add -A`.
- **Changes** happen in a dedicated worktree with a conventional branch name
  (`feat/`, `fix/`, `docs/`, `chore/`, `refactor/`), rebased on latest `main`.
- **Clean history**: fixup/amend into the owning commit on active branches.
- **Never rotate a webhook secret implicitly**, never pass a token where it
  would be logged, never print a secret without `--reveal`.
- **No sudo ever** in this repo unless a human explicitly requests elevation.

## Verification

- A change is not done until `make check` passes and the relevant tests are
  green, with real captured output.
- `docs/log.md` is updated as the wiki evolves.

## Contributor guide

See `docs/contribution/`, especially the [release process](docs/contribution/release-process.md)
for normal and NVR releases. For maintainers, see `docs/contribution/maintainers.md`.