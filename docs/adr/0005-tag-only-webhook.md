---
type: Decision
title: ADR 0005 - Tag-only webhook default
description: GitHub webhooks trigger on tag creation by default, with branch pushes opt-in.
status: stable
---

## Status

Stable.

## Context

Copr's tag-triggered rebuilds fire on the GitHub `create` event, which is
raised when a tag is created, and Copr uses the tag name to decide which
package to rebuild. Branch pushes fire the `push` event. Reb building on every
branch push is noisy and often unintended; the common, deliberate workflow is
to rebuild when a release tag is pushed.

## Decision

Default `integration github enable` to tag-only: the hook listens for the
`create` event. Branch pushes are opt-in via `--tag-only=false` (which adds
`push` alongside `create`) or an explicit `--events` list. The default event
set is therefore `["create"]`.

## Consequences

- A newly enabled webhook does not rebuild on branch pushes until the user
  opts in, so the default behavior matches the deliberate tag-release flow.
- Users who want branch-push rebuilds pass `--tag-only=false` or `--events`.
- The tag name must still match Copr's `PKGNAME-VERSION[-RELEASE]` format (or
  the webhook URL carries the package-name suffix) for the tag event to resolve
  a package.