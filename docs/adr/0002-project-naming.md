---
type: Decision
title: ADR 0002 - Project and binary naming
description: Standardize on coprctl as the project and binary name; retire Coppersmith.
status: stable
---

# ADR 0002: Project and binary naming

## Status

Stable.

## Context

The design spec originally used *Coppersmith* as a working name for the project
with `coprctl` as the binary, and marked the name swappable. Maintaining two
names creates real friction: the `go.mod` module path is `github.com/abn/coprctl`
while a repo named `coppersmith` would disagree with it, `go install`, the Copr
repo, the docs, and the agent skill would all need to know which token to use,
and public references would drift.

## Decision

Standardize on **`coprctl`** as the single name for the project, the repository,
the module, the binary, the Copr repo, the docs, and the agent skill. Retire
*Coppersmith*; it is not used in any public-facing artefact.

The name `coprctl` is chosen because it pattern-matches instantly for both
humans and agents (alongside `systemctl`, `kubectl`, `copr-cli`), is
self-describing to Copr maintainers, and keeps every reference in agreement.

## Consequences

- One name everywhere: repo `github.com/abn/coprctl`, module
  `github.com/abn/coprctl`, binary `coprctl`, `dnf copr enable abn/coprctl`.
- No mapping table is needed between a project codename and the real name.
- *Coppersmith* is absent from public docs and the blog post; it survives only
  as a historical note in the working spec.