---
type: Reference
title: CLI grammar
description: The command surface, global flags, and reference syntax of coprctl.
status: stable
---

# CLI grammar

`coprctl` uses a noun-verb grammar: `coprctl <resource> <verb> [ref]`. Every
command accepts the shared reference syntax, so a new source type is a
`--source` flag value, never a new command.

The full, up-to-date command tree is generated from the command registry and
lives in [the command reference](../reference/commands.md). This page covers
the grammar, the global flags, and the output model.

## Global flags

These flags apply to every command:

| Flag | Meaning |
|---|---|
| `--profile NAME` | instance profile (default from config) |
| `--config PATH` | config file path (default `~/.config/coprctl/config.toml`) |
| `--output FORMAT` | `json`, `jsonl`, `yaml`, `table`, `plain` |
| `--fields a,b,c` | project specific fields |
| `--no-color` / `NO_COLOR` | disable colour |
| `--quiet` / `-q` | suppress progress, keep results |
| `--verbose` / `-v` | repeatable; `-vv` logs HTTP requests |
| `--dry-run` | show what would happen; make no mutating calls |
| `--yes` / `-y` | assume yes for confirmations |
| `--timeout DURATION` | client-side deadline |
| `--version` | version and build information |

## Output model

Every data-returning command supports `--output`, and the default is **json
when stdout is not a TTY**. Piping implies machine consumption. Human formats
(table, plain) are renderings of the same structs JSON serializes, so there is
no separate code path to drift.

Two shapes matter to scripts:

- A **collection** under `--output jsonl` streams one object per line, never a
  single array line. `build list`, `project list`, `package list`, `monitor`,
  and `build submit` all do this.
- `build submit` returns a **JSON array** under `--output json` even for a
  single build, so a script reads `.[0]`. This matches the api_3 create
  endpoints, which return an `items` envelope for multi-build submissions.

Build machine output uses the current api_3 shape: package identity is
`source_package` (name/version/url), the server `state` is the build rollup,
and per-chroot detail is fetched via `build-chroot/list`. The `packagename`,
`source_type`, and embedded `builds` keys are not part of the wire shape and
are not emitted.

Exit codes are stable and meaningful: `4` is a failed build, `8` is not found,
`9` is forbidden, `12` is drift. Errors are structured objects with a code, a
hint, and a retryability flag.

## Command families

The surface is grouped by resource:

- **`project`** - list, get, create, edit, delete, fork, regenerate-repos, chroot
- **`package`** - create, edit, get, list, reset, delete
- **`build`** - submit, rebuild, get, list, watch, cancel, delete, reproduce
- **`chroot`** - list (the instance catalog)
- **`log`** - tail, failures, detective
- **`config`** / **`auth`** - profiles, import, migrate, status, login, rotate
- **`detect`** / **`init`** / **`sync`** - source to project
- **`apply`** / **`diff`** / **`export`** / **`validate`** - declarative manifest
- **`integration`** - forge webhooks
- **`try`** - local preflight
- **`schema`** / **`skill`** / **`mcp`** / **`ui`** / **`doctor`** / **`completion`**

See [terminology](terminology.md) for the reference syntax and vocabulary.