---
type: Decision
title: ADR 0003 - Anonymous read fallback
description: Read-only commands work without any configuration by falling back to an anonymous production client.
status: stable
---

## Status

Stable.

## Context

Browsing, monitoring, and log reading in Copr are all anonymous operations;
the API does not require credentials to read public projects, builds, and
logs. Initially every command built its API client from a configured profile,
so `chroot list`, `project get`, `log failures`, and `build reproduce` failed
with a `no_config` error when a user had no profile or legacy config. That made
read-only inspection of public data require authentication, which is wrong.

## Decision

Add a separate `ReadClient()` that uses the configured profile when one exists
and otherwise falls back to an anonymous production client. Route all
read-only commands through it. Write commands continue to use the authenticated
client and still require configuration, returning a clear `no_config` error.

## Consequences

- Read-only commands work with zero configuration against public data.
- Writes still require credentials, so there is no silent fallback to an
  anonymous write (which would fail with a confusing 401).
- The anonymous fallback targets production by default; a user who wants to
  read a different instance configures a profile for it.