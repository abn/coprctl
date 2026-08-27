---
type: Guide
title: Release process
description: How coprctl is released, including normal and NVR releases, and how packaging-only changes bump the RPM.
status: stable
---

# Release process

This page describes how `coprctl` is released. It covers the versioning model,
the two kinds of release (normal and NVR), and the workflow for packaging-only
changes.

The release pipeline is automated with Release Please (GitHub Actions) and
GoReleaser, and Copr builds are triggered from the repository via the SCM
source with auto-rebuild. The full workflow lives in
`.github/workflows/release.yml`.

## Versioning model

There is one source of truth for the version: the Release Please manifest
(`.release-please-manifest.json`), which drives a semantic version
(`v0.1.0`, `v0.2.0`, ...). Release Please owns:

- the semantic version and the `v<semver>` git tag,
- the `CHANGELOG.md` and the GitHub release,
- and, via the `extra-files` configuration, the `Version:` line in
  `coprctl.spec`, so the RPM version always matches the semantic version.

The RPM **Release** field is separate and independent. It is how Copr
distinguishes multiple builds of the same version. This gives two release
paths, described below.

## Release Please conventions

Because Release Please derives the next version from conventional commits, the
repo follows these rules:

- **Pull request only.** All changes land on `main` through a pull request,
  never by pushing directly to `main`.
- **Squash merge.** Merge with a squash merge so each merged PR contributes
  exactly one commit with a conventional-commit message. This is what Release
  Please parses to compute the next version and build the changelog.
- **Conventional commits.** The merged commit message determines the bump:
  `fix:` bumps the patch, `feat:` bumps the minor, and a breaking change (a
  `!` or `BREAKING CHANGE:` body) bumps the major. Changelog sections are
  grouped from these prefixes.

Release Please opens a release pull request (titled
`chore: release <branch>`) that bumps the version, updates `coprctl.spec`
`Version:`, and rewrites `CHANGELOG.md`. Merging that PR publishes the
`v<semver>` tag and the GitHub release, which triggers the GoReleaser build.

## Normal releases

A normal release publishes a new semantic version:

1. Merge feature and fix pull requests to `main` with squash merges and
   conventional-commit messages.
2. Release Please opens a release PR that bumps the version and the spec
   `Version:` field.
3. Review and squash-merge the release PR.
4. The `v<semver>` tag is created and the GitHub release is published.
5. GoReleaser builds and uploads artifacts (tarballs, RPM, deb, checksums).
6. Copr rebuilds `coprctl` from `main` (auto-rebuild on the SCM source), so
   `dnf copr enable abn/coprctl && dnf install coprctl` installs the new
   version.

The RPM version equals the semantic version: `coprctl-<version>-1.fcNN`.

## NVR releases

An NVR (name-version-release) release republishes the same version with a
different release number, without a new semantic version. Use this to fix a
packaging problem (a spec bug, a missing `BuildRequires`, a wrong build flag)
without bumping the application version.

The RPM **Release** field starts at `1` and is bumped manually:

1. Edit `coprctl.spec` and increment `Release:` (for example `1%{?dist}` to
   `2%{?dist}`).
2. Commit and open a pull request. Use a conventional-commit message such as
   `fix(packaging): ...` so it is captured correctly.
3. Squash-merge the PR.
4. Copr auto-rebuilds from `main` and produces `coprctl-<version>-2.fcNN`.

The semantic version and the `v<semver>` tag are unchanged. NVR releases are
common and inexpensive; there is no need to bump the version for a packaging
fix.

## Packaging-only changes

A change that touches only the packaging (the spec, the manifest, the
GoReleaser or release config) still goes through the same PR-only, squash-merge
flow. Decide which release path applies:

- If the change only affects how the package is built (a build flag, a
  dependency, a `Source0` change), it is an **NVR release**: bump `Release:`,
  merge, and let Copr rebuild.
- If the change is a user-facing feature or fix (new command, changed behavior),
  it is a **normal release**: merge with `feat:`/`fix:` and let Release Please
  open the version-bumping release PR.

There is no separate branch or process for packaging-only work; it reuses the
same pull request and squash-merge convention.

## Working with Release Please

- **Do not edit `CHANGELOG.md` or the version by hand.** Release Please owns
  both. Edit only through its release PR.
- **Keep `coprctl.spec`'s `Version:` in sync.** The `x-release-please-version`
  annotation marks the line Release Please updates; do not remove it.
- **Respect the manifest.** `.release-please-manifest.json` records the last
  released version. Do not edit it by hand except during bootstrap.
- **Preview the next release.** Run `release-please release-pr` locally (or in
  CI) to see the proposed bump before merging.

## Related

- [Contribution guide](index.md) for the contribution workflow.
- [Maintainer guide](maintainers.md) for the review gate.
- The release workflow: `.github/workflows/release.yml`.
