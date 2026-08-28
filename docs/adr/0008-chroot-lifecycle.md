---
type: Decision
title: ADR 0008 - Chroot lifecycle and EOL surfacing
description: coprctl derives chroot EOL state from a maintained table and exposes enable, disable, and prune operations.
status: stable
---

## Status

Stable.

## Context

The Copr API returns the mock-chroot catalog as a `{name: comment}` map with no
machine-readable lifecycle state. Yet maintainers need to know which chroots
are EOL, and to retire them: a distro release reaches EOL, stops accepting new
builds, and the project's repos should stop being built there. The web UI shows
this, but the CLI exposed none of it, so retiring a chroot meant editing a
project by hand or in the web UI.

## Decision

Derive chroot lifecycle state from the name against a maintained EOL table in
a dedicated `internal/chroot` package, classifying each chroot as `active`,
`preserved` (EOL: repos remain, no new builds), or `deleted` (absent from the
catalog). Rolling targets (rawhide, branched) never go EOL. An unknown release
is treated as active rather than guessed EOL, and a chroot absent from the
catalog is flagged deleted.

Surface and act on the state:

- `chroot list` shows a STATE column and filters by `--state active|preserved`.
- Targeting a non-active chroot for a build or a chroot operation warns that it
  will not accept new builds.
- `project chroot enable` adds chroots additively; `project chroot disable`
  removes them (requires `--yes`).
- `apply --prune` disables chroots the manifest no longer lists (requires
  `--yes`), so retiring a chroot is a one-line manifest change.

The project chroot set is set via the project edit endpoint
(`PUT /api_3/project/edit/{owner}/{project}`) with a `chroots` array. The
Copr source (`apiv3_projects.py`, `CoprChrootsLogic.update_from_names`)
reconciles the set: chroots in the array are enabled, those omitted are
disabled. This was verified live against Copr staging: enable, disable,
additive apply, and apply --prune all mutated the chroot set as intended.

## Consequences

- Maintainers see EOL state without opening the web UI and can retire a chroot
  from the CLI or a manifest.
- The EOL table is a maintenance burden and can lag distro releases; an unknown
  release defaults to active so the tool never guesses an EOL and blocks a real
  build.
- `disable` and `--prune` are destructive and require `--yes`, consistent with
  the explicit-destructive-operations invariant.
- Additive `apply` also reconciles chroots: chroots in the manifest that are
  not live are enabled, and live chroots the manifest does not mention are kept
  (unless `--prune`).