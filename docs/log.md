# Documentation update log

This log tracks the evolution of the knowledge base: page additions,
deprecations, and structural refactors. It does not track software releases;
repository changes belong in the commit history.

## 2026-08-27

* **Initialization**: Created the bundle root and section scaffolds
  (overview, design, architecture, adr, usage, reference, contribution).
* **Creation**: Added the ADR for the implementation language and the
  architecture overview seed pages.
* **Curation**: OKF conformance pass. Removed frontmatter from the design and
  architecture section index files so only the bundle root carries frontmatter.
  Added stub pages for the referenced-but-missing concepts (terminology, CLI
  grammar, log streaming, agent contract) to fix broken cross-links. Audit
  found no privacy leaks or em-dashes.