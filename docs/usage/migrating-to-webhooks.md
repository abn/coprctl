---
type: Guide
title: Migrating a project to webhook builds
description: Move a hand-submitted Copr project to tag-driven webhook rebuilds, including source conversion and EOL chroot cleanup.
status: stable
---

# Migrating a project to webhook builds

Projects often start with manual submits: someone builds an SRPM locally
and runs an upload on every release. That works until the uploads lag the
repository, a release goes out without packages, or the one person holding
the token is away. The migration below replaces the manual step with a
tag-driven webhook. Copr clones the tag and builds from it, so the release
tag becomes the single trigger.

## Diagnose

Confirm the project is in the manual state:

```bash
coprctl package list OWNER/PROJECT
coprctl build list OWNER/PROJECT --output json | head -5
```

Look for a package with source type `upload`, auto-rebuild off, and builds
that were submitted by hand rather than by tag. Compare the newest built
version against the repository: a gap between the two is the staleness this
migration removes.

## Capture the current state

Export the live project into a manifest so the migration is reviewable and
repeatable:

```bash
coprctl export OWNER/PROJECT -o copr.yaml
coprctl validate -f copr.yaml
```

## Convert the package source

Change the package from `upload` to the source Copr should build from.
For repositories with a plain spec that is the `scm` source with the
default `rpkg` method; for template specs (`.spec.in`) it is `make_srpm`
with a `.copr/Makefile` srpm target (see the template-spec section of the
webhook integrations guide). Either edit the manifest:

```yaml
packages:
  - name: PKG
    source:
      type: scm
      cloneUrl: https://github.com/OWNER/REPO.git
      committish: main
      spec: mypkg.spec
      method: rpkg
    autoRebuild: true
```

or convert in place:

```bash
coprctl package edit OWNER/PROJECT/PKG --source scm \
  --clone-url https://github.com/OWNER/REPO.git --commit main \
  --spec mypkg.spec --method rpkg --auto-rebuild
```

Then reconcile and inspect the drift first:

```bash
coprctl diff -f copr.yaml
coprctl apply -f copr.yaml
```

## Retire EOL chroots

Migrations are the right moment to clean the chroot set. List the catalog
with lifecycle state and compare against the project:

```bash
coprctl chroot list --state active
coprctl project chroot list OWNER/PROJECT
```

Disable what the project no longer needs, or reconcile from the manifest:

```bash
coprctl project chroot disable OWNER/PROJECT --chroot fedora-42-x86_64 --yes
coprctl apply -f copr.yaml --prune --yes
```

With `follow_fedora_branching` on (the default), branching Fedoras are
picked up automatically, so this list only shrinks by hand.

## Enable the webhook

```bash
coprctl integration rotate-secret OWNER/PROJECT --yes
coprctl integration github enable OWNER/PROJECT --repo OWNER/REPO
```

The default is tag-only: pushing a release tag rebuilds, branch pushes do
not. The package-scoped URL maps bare `vX.Y.Z` tags onto the right package
when the tag and the package name differ. When a forge hook firing on every
tag is too broad, `coprctl integration trigger` drives the receiver from a
release pipeline instead, with the pre-release filter expressed there (see
the webhook integrations guide).

## Verify without pushing a tag

Rebuild from the stored source definition to prove the converted package
builds before any release depends on it:

```bash
coprctl build rebuild OWNER/PROJECT/PKG --watch
```

If that is green, the next release tag drives the build on its own. Keep
the upload form documented in the submitting-builds guide for one-off
submits; it is no longer the release path.

## Steady state

```bash
coprctl sync --check -f copr.yaml   # CI gate against manifest drift
coprctl status OWNER/PROJECT        # health summary, exits 4 on failures
coprctl integration trigger OWNER/PROJECT/PKG --tag vX.Y.Z   # manual or pipeline-driven rebuild
```

## Related

- Webhook integrations: setup, tag-only default, custom webhooks, secrets.
- The copr.yaml manifest: schema, declared-only apply, export.
- Submitting builds: the upload form for one-off submits.
