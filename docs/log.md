# Documentation update log

This log tracks the evolution of the knowledge base: page additions,
deprecations, and structural refactors. It does not track software releases
or implementation milestones; repository changes belong in the commit history.

## 2026-09-01

* **Creation**: Added the "The copr.yaml manifest" guide
  (`docs/usage/manifest.md`) documenting the full settings schema, the
  declared-only apply rule, the declared-vs-zero family, and the three
  treatment classes (readable and reconciled, create-only, write-only).
* **Update**: `monitor` now requests and renders the per-chroot log URLs,
  backend log URLs, and integer `status` that `/monitor` exposes via
  `additional_fields[]`, and accepts `:dir` side-repo monitoring. The human
  table gained `BUILD` and an elided `LOG` column; full URLs stay JSON-only.
  The `ui` command and its TUI forward the reference's dir.
* **Update**: The build response schema is now decoded to match the api_3
  wire shape: `source_package` (name/version/url) carries package identity and
  the server `state` is the build rollup, with per-chroot detail fetched via
  `build-chroot/list`. The output-shape changes land in one bullet: the
  never-present `packagename`/`source_type` keys are gone, the enrichment-only
  `builds` key no longer appears in build output, and `BuildChroot` no longer
  carries a `build_id`.
* **Update**: Verified ground-truth wire shapes for two build/package
  operations. `POST /build/create/url` returns every created build in an
  `items` envelope, so a single-URL submit is a one-item list, not a flat build
  object. `DELETE /package/delete` takes a body of
  `ownername`/`projectname`/`package_name`; the path-segment route does not
  exist upstream.
* **Update**: `--output jsonl` on a collection command streams one object per
  line instead of a single array line, so every slice-rendering command
  (submit, list) emits line-delimited JSON.
* **Creation**: Added "Submitting builds" (`docs/usage/submitting-builds.md`)
  covering the generic build options shared by every source type, chroot globs
  with `exclude_chroots`, the mutually exclusive batch options, the upload and
  `--from` caveats, the atomic batch delete, and the reproduce fallback to the
  stored source build config. Linked it from the usage index and noted the
  reproduce fallback in the debugging guide.

## 2026-08-30

* **Creation**: Added the "Contributor guide" (`docs/contribution/guide.md`)
  distilling the contribution workflow and conventions from `AGENTS.md`, and
  pointed the contribution index, `docs/index.md`, and `AGENTS.md` at it.
  Removed the self-referential "Usage index" link from the usage index.
* **Creation**: Added the "Group projects" guide (`docs/usage/group-projects.md`)
  covering `@alias` namespaces: the one-time FAS and web-UI activation, group
  references in project-family commands, permissions for non-members, and the
  not-activated error hint.
* **Creation**: Added the "Instances, staging, and profiles" guide
  (`docs/usage/instances.md`) covering profile detection, authenticating to any
  instance, and using Fedora staging for testing.

## 2026-08-28

* **Update**: Corrected the NVR release steps in the release process to match
  the tag-only webhook: a branch push does not rebuild, so an NVR release
  pushes an `N-V-R` tag (`coprctl-<version>-<release>`). Verified live against
  Copr staging and against the upstream webhook source
  (`get_for_webhook_rebuild`).
* **Update**: Added a "Ground truth" section to the maintainer guide and a
  matching note in AGENTS.md, establishing the upstream Copr source
  (`github.com/fedora-copr/copr`) as the authoritative reference for the
  server, the API, and existing clients, ahead of the incomplete swagger.
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
