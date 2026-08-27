# Documentation update log

This log tracks the evolution of the knowledge base: page additions,
deprecations, and structural refactors. It does not track software releases;
repository changes belong in the commit history.

## 2026-08-27

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