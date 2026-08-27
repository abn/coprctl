---
type: Reference
title: Overview
description: What coprctl is and why it exists.
tags: [project, introduction]
status: draft
---

# Overview

`coprctl` is a reimagined command-line and agent interface for the Fedora
Copr build system. Copr is Fedora's lightweight build service: you give it
sources and it gives you signed RPM repositories.

The project has two audiences:

- **Humans** who use Copr as a daily driver and want a coherent command
  grammar, live build logs, and declarative project state.
- **Agents** who need a discoverable, deterministic, machine-readable
  interface that can be driven from `schema` output and structured events.

## Goals

1. One grammar: `coprctl <resource> <verb> [ref]`. A new source type is a
   flag value, never a new command.
2. Official Copr terminology as canonical, with aliases accepted.
3. Everything machine-readable: `--output json|jsonl|yaml` on every command,
   stable exit codes, structured error objects.
4. Live logs as a first-class verb, including concurrent multi-chroot tail.
5. Declarative project state with apply, diff, and export.
6. Webhooks configured end to end in one command.
7. A TUI only where it beats text and never blocks automation.
8. A generated agent skill that cannot drift from the CLI.
9. A single static binary with no runtime.
10. Instance-agnostic: Fedora, staging, openEuler, and self-hosted are all
    first-class profiles.

See the [design](design/index.md) section for how these are realized.