# Design overview

The design is grounded in research on what Copr is, what its API exposes, and
where the existing `copr-cli` hurts as a daily driver and agent tool. The
working specification details the full research and the problems it solves.

## Design principles

1. **Noun-verb, always.** `coprctl <resource> <verb>`. A new source type is a
   `--source` value, never a new command.
2. **One reference parser.** Everything accepts the shared
   `owner/project[:dir][/segment]` reference. No command invents its own
   argument shape.
3. **Human output is a rendering of machine output.** `--output table` is a
   formatter over the same struct `--output json` serializes.
4. **Never prompt when stdin is not a TTY.** Fail with an actionable error
   naming the flag that would have answered the prompt.
5. **Destructive operations are explicit.** `--yes` is required
   non-interactively; `--dry-run` is available everywhere it means something.
6. **Idempotence where the API allows it.** create with `--if-not-exists`,
   apply, ensure.
7. **The TUI is a view, never a capability.** Anything you can do in the TUI
   has a printable command, and the TUI shows you that command.
8. **Errors are objects.** Code, message, hint, retryability, and doc link.
9. **One registry, many artefacts.** CLI, help, completions, JSON schema, MCP
   tools, docs, and the agent skill are generated from one command registry.
10. **Instance-agnostic and offline-tolerant.** Cache the chroot catalog;
    degrade gracefully; never hardcode Fedora.

## Terminology

The tool aligns to official Copr vocabulary. See
[terminology](terminology.md) for the canonical glossary and the reference
syntax.

## Command surface

The full grammar is documented in [CLI grammar](cli-grammar.md). Notable
subsystems:

- **Log streaming** - a first-class tail over build logs with two fetch
  strategies. See [architecture/log-streaming](../architecture/log-streaming.md).
- **Declarative manifests** - `copr.yaml` with apply, diff, and export for
  project state.
- **Agent contract** - exit codes, event schemas, error objects, and a
  generated skill. See [architecture/agent-contract](../architecture/agent-contract.md).
- **Local preflight** - a tiered local build before submitting to Copr.
- **Init and sync** - turning a source repository into a working Copr project.

## Technology choice

The implementation language and rationale are captured in
[ADR 0001](../adr/0001-implementation-language.md). The working spec
contains the full trade-off analysis.