# Architecture overview

The tool is a single static binary built with the Go toolchain. Its command
layer is a Cobra command registry that is the single source of truth: CLI,
completions, JSON schema, MCP tools, docs, and the agent skill are all
generated from it.

## Layers

The implemented package layout mirrors the working spec:

| Layer | Role |
|---|---|
| `cmd/coprctl` | entrypoint |
| `internal/cli` | Cobra command registry plus schema generation |
| `internal/copr` | API client against the `/api_3` REST surface |
| `internal/ref` | the one reference parser, shared by every command |
| `internal/cerr` | structured error objects and stable exit codes |
| `internal/config` | profiles, precedence, legacy config import |
| `internal/render` | table, plain, json, jsonl, yaml formatters |
| `internal/state` | local cache (chroot catalog) |

Planned layers that later milestones add: `internal/events` (event bus),
`internal/logstream` (log tailer), `internal/manifest` (declarative
`copr.yaml`), `internal/tui` (TUI), and `internal/forge` (hook management).

## Repository layout

See the working spec for the full layout. The key invariant: the command
registry is the only place a new capability is declared.

## API client

The Copr v3 REST API is consumed directly. A machine-readable Swagger document
is served at the instance's `/api_3/swagger.json`, which informs client
generation. The client is kept behind an interface for testability.

See [agent contract](agent-contract.md) for the machine-facing contract.