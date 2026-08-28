# Documentation update log

This log tracks the evolution of the knowledge base: page additions,
deprecations, and structural refactors. It does not track software releases
or implementation milestones; repository changes belong in the commit history.

## 2026-08-28

* **Update**: Removed the redundant H1 title from the ADR bodies (the
  frontmatter `title` is the display name), and added ADRs 0003-0006 covering
  the anonymous-read fallback, secret-handler support, the tag-only webhook
  default, and instance detection. The ADR register now records the implemented
  decisions.

## 2026-08-27

* **Update**: Filled in the wiki pages that were placeholders, so the docs now
  reflect status quo. Added the canonical terminology and reference syntax,
  the CLI grammar, the log-streaming architecture, the agent contract (exit
  codes, error objects, event schema, MCP, skills), and a real quick start.
  Marked the decided ADR 0001 and the overview/maintainer pages stable, listed
  the full implemented package layers, and added OKF frontmatter to the
  generated command reference. The wiki is now content-complete and
  OKF-conformant with all internal links resolving.
* **Creation**: Documented the release process (normal and NVR releases, the
  Release Please PR-only and squash-merge conventions, and bump procedures for
  packaging-only changes) at `docs/contribution/release-process.md`. Linked it
  from the contribution index, maintainer guide, README, and AGENTS.md.
* **Creation**: Added ADR 0002 standardizing the project and binary name on
  `coprctl` and retiring the *Coppersmith* working name.
* **Update**: Architecture page reflects the implemented package layout.
* **Initialization**: Created the bundle root and section scaffolds
  (overview, design, architecture, adr, usage, reference, contribution).
* **Curation**: OKF conformance pass. Removed frontmatter from the design and
  architecture section index files so only the bundle root carries frontmatter.
  Added stub pages for the referenced-but-missing concepts (terminology, CLI
  grammar, log streaming, agent contract) to fix broken cross-links. Audit
  found no privacy leaks or em-dashes.