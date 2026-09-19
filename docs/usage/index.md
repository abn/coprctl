# Usage

Guides and worked examples for using the tool.

* [Quick start](quickstart.md) - first steps
* [Webhook integrations](webhook-integrations.md) - wiring Copr to GitHub or
  GitLab repos, disabling, and custom webhooks
* [Submitting builds](submitting-builds.md) - generic build options, chroot
  globs, uploads, and batch delete
* [Debugging a failing build](debugging-builds.md) - reproduce and fix failures
  locally
* [Local builds](local-builds.md) - how SRPM and preflight builds work, and the
  container, mock, and native backends
* [Group projects](group-projects.md) - own a project as a team with group
  namespaces (@alias)
* [Instances, staging, and profiles](instances.md) - work with any Copr
  instance, including Fedora staging, via profiles
* [The copr.yaml manifest](manifest.md) - the declarative project state schema,
  the declared-only apply rule, and what diff and export verify
* [Migrating a project to webhook builds](migrating-to-webhooks.md) -
  moving a hand-submitted project to tag-driven rebuilds, including source
  conversion and EOL chroot cleanup
* [Packaging a Rust workspace for Copr](rust-workspace.md) - from a Cargo
  workspace to tag-driven RPM builds with a template spec, end to end

Usage documentation is generated alongside the command reference where
possible, and curated by the technical writer.
