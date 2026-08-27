# Documentation update log

This log tracks the evolution of the knowledge base: page additions,
deprecations, and structural refactors. It does not track software releases;
repository changes belong in the commit history.

## 2026-08-27

* **Creation**: Added the GitHub webhook integration usage guide. Verified the
  integration live against Copr staging and GitHub: idempotent hook create and
  update, ping verification (200), and secret masking. Fixed the idempotency
  check to match on the configured webhook destination rather than the GitHub
  API resource URL, and made ping delivery polling retry until recorded.
* **Creation**: Implemented Wave 08 (M5 polish): shell completions generated
  from the registry, an environment doctor with structured exit codes, version
  injection, GoReleaser/nfpm packaging, the self-hosting `copr.yaml` and spec,
  and a minimal `ui` view that degrades to plain output off a TTY. The full
  Bubble Tea dashboard is deferred.
* **Creation**: Implemented Wave 07 (M4.7 init/sync): read-only `detect`,
  `init`, and three-way `sync` with the inferred-fields provenance contract.
* **Creation**: Implemented Wave 06 (M4.5 preflight): the container runtime
  abstraction and `try` with chroot-to-image resolution, strict-match gating,
  coverage reporting, and the fidelity report. The rpmbuild invocation is a
  declared stub that reports `unsupported` rather than a false pass.
* **Creation**: Implemented Wave 05 (M4 declarative and integrations): the
  `copr.yaml` manifest with validate/diff/apply/export, local state for webhook
  secrets, and the GitHub webhook integration with idempotent hook management
  and ping verification. Verified the declarative lifecycle end to end against
  Copr staging (apply, diff drift with exit 12, export round-trip) and the
  secret rotation/URL path.
* **Creation**: Implemented Wave 04 (M3 agent surface): the versioned JSONL
  event schema, `schema --format mcp`, the stdio MCP server with read/write/
  destructive tiers, and the generated agent skill plus `skill install`.
  Added the `make gen`/`make drift` gate so generated artefacts cannot diverge
  from the command registry.
* **Creation**: Implemented Wave 03 (M2 logs and events): the event bus, the
  polling source, the log tailer with incremental and full-refetch gzip
  strategies, multi-chroot tail, monitor, and status. The tailer was validated
  against a real production Copr log and against synthetic servers covering
  both gzip framings, plain-log fallback, no-torn-lines, and no-duplicates.
* **Creation**: Implemented Wave 02 (M1 CRUD parity): project/package/build
  write commands, all source types under one `--source` flag, the migration
  shim, and JSON-body API calls. Verified the full lifecycle end to end
  against Copr staging (create project, add package, submit build, delete).
* **Creation**: Implemented the M0 skeleton in code: the command registry
  (`project`, `chroot`, `build`, `schema`, `version`), the reference parser,
  config profiles with legacy import, the API client, output renderers, and
  structured error objects with stable exit codes.
* **Update**: Architecture page reflects the implemented package layout.
* **Initialization**: Created the bundle root and section scaffolds
  (overview, design, architecture, adr, usage, reference, contribution).
* **Curation**: OKF conformance pass. Removed frontmatter from the design and
  architecture section index files so only the bundle root carries frontmatter.
  Added stub pages for the referenced-but-missing concepts (terminology, CLI
  grammar, log streaming, agent contract) to fix broken cross-links. Audit
  found no privacy leaks or em-dashes.