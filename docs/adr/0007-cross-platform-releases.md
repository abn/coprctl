---
type: Decision
title: ADR 0007 - Cross-platform releases
description: coprctl ships native binaries for Linux, macOS, and Windows, and CI verifies every platform on every push.
status: stable
---

## Status

Stable.

## Context

Copr builds inside Linux containers, and the local preflight feature runs in a
container runtime, so a maintainer does not need a Linux host to build or test
RPMs. The release pipeline already cross-compiled for Linux, macOS, and Windows
via GoReleaser, but CI only built and tested on Linux. A platform regression
(an unguarded `/dev/stdin` read, a Linux-only signal, or a container mount
argument built for Linux paths) would compile on Linux, pass CI, and only fail
at release time or on a user's machine.

## Decision

Treat Linux, macOS, and native Windows as first-class release targets and
verify them in CI:

- `make build-all` cross-compiles every release platform (linux, darwin,
  windows on amd64 and arm64) with CGO disabled, matching the GoReleaser
  matrix.
- CI runs the full quality gate on Linux, native tests on macOS and Windows
  runners, and the cross-compile check on every push and pull request.
- Path code uses `filepath` throughout, TTY detection uses `golang.org/x/term`
  (portable), and platform divergence is confined to small helpers: stdin reads
  use `os.Stdin` rather than `/dev/stdin`, and container mounts slash-normalize
  the host path and apply the SELinux `:z` relabel only on Linux.

## Consequences

- A platform regression is caught on every push, not at release time.
- macOS and Windows users get native binaries for the same feature set, and
  the local preflight loop works with Docker Desktop or podman.
- Secret handlers (pass, gopass, secret-tool) remain Linux-oriented and fail
  soft elsewhere; native macOS Keychain or Windows Credential Manager backends
  are a future enhancement, not a release blocker.