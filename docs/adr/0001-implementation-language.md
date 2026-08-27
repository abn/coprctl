---
type: Decision
title: ADR 0001 - Implementation language
description: Choose Go for the coprctl binary.
status: draft
---

# ADR 0001: Implementation language

## Status

Draft, pending confirmation during the M0 milestone.

## Context

The tool must be a single static binary that runs on any distro, in scratch
containers, and in agent sandboxes, with no runtime dependency. It is
concurrency-heavy: multi-chroot log tailing, state polling, and TUI rendering
are a goroutine-and-channel problem. It needs a small dependency surface and
easy cross-compilation for a solo maintainer.

## Decision

Implement in Go. Cobra provides the command framework, Bubble Tea provides
the TUI, the standard library covers HTTP and gzip, and Koanf handles layered
configuration. GoReleaser handles release packaging.

A fallback to a Python-based stack (Textual plus the official client) is
documented as a trigger if API-client parity turns out to be expensive, but is
not the default.

## Consequences

- Portability and a single static binary are satisfied directly.
- The concurrency model matches the domain.
- The API client must be written (or generated from the served Swagger
  document), kept behind an interface for testability.