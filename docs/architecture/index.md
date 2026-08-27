# Architecture overview

The tool is a single static binary built with the Go toolchain. Its command
layer is a Cobra command registry that is the single source of truth: CLI,
completions, JSON schema, MCP tools, docs, and the agent skill are all
generated from it.

## Layers

| Layer | Role |
|---|---|
| `cmd/coprctl` | entrypoint |
| `internal/cli` | Cobra command registry plus schema generation |
| `internal/copr` | API client against the `/api_3` REST surface |
| `internal/ref` | the one reference parser, shared by every command |
| `internal/events` | the event bus that drives watch, tail, and monitor |
| `internal/logstream` | the log tailer |
| `internal/manifest` | declarative `copr.yaml` schema, diff, apply, export |
| `internal/render` | table, plain, json, jsonl, yaml formatters |
| `internal/tui` | Bubble Tea programs consuming the event bus |
| `internal/state` | local state for webhook secrets and the chroot cache |
| `internal/config` | profiles, precedence, legacy config import |
| `internal/forge` | GitHub (and later other) hook management |

## Repository layout

See the working spec for the full layout. The key invariant: the command
registry is the only place a new capability is declared.

## API client

The Copr v3 REST API is consumed directly. A machine-readable Swagger document
is served at the instance's `/api_3/swagger.json`, which informs client
generation. The client is kept behind an interface for testability.

See [agent contract](agent-contract.md) for the machine-facing contract.