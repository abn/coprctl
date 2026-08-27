# Documentation update log

This log tracks the evolution of the knowledge base: page additions,
deprecations, and structural refactors. It does not track software releases;
repository changes belong in the commit history.

## 2026-08-27

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