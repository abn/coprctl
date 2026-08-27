---
type: Reference
title: Agent contract
description: Exit codes, event schemas, error objects, and the generated skill.
status: stable
---

# Agent contract

The machine-facing contract is versioned independently of the CLI's human
surface. Breaking it is a major version bump.

## Principles

1. **Discoverable without `--help` parsing.** `coprctl schema` emits the whole
   command tree as JSON.
2. **Deterministic.** Stable field names, stable ordering, no spinners in
   non-TTY mode.
3. **Failure is legible.** Structured error objects with a retryability hint.
4. **Safe by construction.** Destructive operations require explicit consent an
   agent cannot supply by accident.
5. **Cheap in context.** `--fields` limits list output; `log failures`
   summarises build failures instead of ingesting whole logs.

## Exit codes

Codes `1`-`7` keep `copr-cli` meanings for script and muscle-memory
compatibility; `8`+ are new.

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Generic failure / bad request |
| 2 | Usage error |
| 3 | No configuration found |
| 4 | Build failed or was canceled; `status`/`monitor` unhealthy |
| 5 | Transport or protocol error |
| 6 | Configuration error |
| 7 | Authentication failed |
| 8 | Not found |
| 9 | Permission denied |
| 10 | Conflict / already exists |
| 11 | Timeout / deadline exceeded |
| 12 | Drift detected (`diff`, `apply --check`, `sync --check`) |
| 13 | Precondition failed |
| 130 | Interrupted |

## Error object

Errors are objects emitted to stderr, with a closed enumeration of codes:

```json
{
  "schema": "coprctl.error/v1",
  "code": "chroot_not_active",
  "message": "chroot 'fedora-38-x86_64' is EOL",
  "hint": "run 'coprctl chroot list --state active'",
  "retryable": false,
  "resource": "owner/project/chroot",
  "exit_code": 13
}
```

## Event stream (`--output jsonl`)

Long-running operations emit one JSON object per line:

```json
{"schema":"coprctl.event/v1","ts":"...","event":"chroot.state","build_id":10653539,"chroot":"srpm-builds","state":"running","previous":"pending"}
{"schema":"coprctl.event/v1","ts":"...","event":"log.line","build_id":10653539,"chroot":"fedora-rawhide-x86_64","stream":"builder-live","seq":1042,"line":"error: ..."}
```

Event kinds include `build.state`, `chroot.state`, `log.line`,
`log.truncated`, `build.finished`, and `error`.

## Discovery and execution

- **`coprctl schema`** emits the command tree as JSON, MCP tool definitions, or
  markdown. Generated from the command registry, so it is always accurate.
- **`coprctl mcp serve`** exposes the same surface as MCP tools over stdio,
  tiered into read (default), write (`--allow-write`), and destructive
  (`--allow-destructive`).
- **`coprctl skill print` / `install`** ship agent skills. The `coprctl` skill
  is generated from the registry; `coprctl-debug` adds the debugging workflow.

All of these are generated from one command registry, so the CLI, completions,
JSON schema, MCP tools, docs, and skills cannot drift. A CI drift check fails
the build if the generated files diverge.