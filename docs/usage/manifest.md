---
type: Guide
title: The copr.yaml manifest
description: Declarative project and package state: the schema, the declared-only apply rule, and what diff and export can verify.
status: stable
---

# The copr.yaml manifest

A manifest is the declarative form of a Copr project. It is a `coprctl/v1`
`Project` document that names the target project, describes the desired
state, and is reconciled with `coprctl apply -f copr.yaml`. `coprctl diff`
reports field-level drift and `coprctl export` writes a manifest from a live
project.

```yaml
apiVersion: coprctl/v1
kind: Project
metadata:
  owner: you
  name: mypkg
  profile: production
spec:
  description: My package
  instructions: |
    dnf copr enable you/mypkg
    dnf install mypkg
  homepage: https://example.com/mypkg
  contact: https://github.com/you/mypkg/issues
  settings:
    enableNet: true
    autoPrune: true
    bootstrap: on
    unlistedOnHomepage: true
  chroots:
    enabled:
      - fedora-42-x86_64
      - epel-9-x86_64
  packages:
    - name: mypkg
      source:
        type: scm
        cloneUrl: https://github.com/you/mypkg
        committish: main
        spec: mypkg.spec
      maxBuilds: 10
      timeout: 36000
      chrootDenylist:
        - 'fedora-rawhide-*'
  permissions:
    builders: [builder1]
    admins: [admin1]
```

## Project settings

The `spec.settings` map carries project-level flags. Every field maps to a
Copr project setting with the same meaning; see the Copr project settings in
the web UI or the upstream schema for the details of each one.

Fields that are applied both on create and on edit:

| Field | Setting | Wire constraint |
| --- | --- | --- |
| `autoPrune` | auto-delete old builds | bool |
| `bootstrap` | bootstrap mode | `default`, `off`, `on`, or `image` |
| `isolation` | build isolation | `default`, `nspawn`, or `simple` |
| `moduleHotfixes` | repository holds module hotfixes | bool |
| `appstream` | generate AppStream metadata | bool |
| `packitForgeProjectsAllowed` | forge projects allowed to build via Packit | list of forge strings |
| `followFedoraBranching` | auto-follow Fedora branching | bool |
| `repoPriority` | repository priority | integer >= 1 |
| `unlistedOnHomepage` | hide the project from the home page | bool |
| `multilib` | enable multilib support | bool |
| `fedoraReview` | run fedora-review on packages | bool |
| `runtimeDependencies` | external repositories enabled with this project | list of base URLs |
| `deleteAfterDays` | delete the project after N days | integer -1..720 |

Create-only fields, sent only at project creation (the edit API has no field
for them, so re-apply cannot change them on an existing project). `validate`
warns when they are set:

| Field | Setting | Wire constraint |
| --- | --- | --- |
| `persistent` | immune to deletion | bool |
| `storage` | backing storage, admin only | `backend` or `pulp` |

`additionalRepos` is not yet modeled by the client and warns on validate.

`persistent` and `deleteAfterDays` are mutually exclusive, matching the
upstream form rule; `validate` rejects a manifest that sets both.

## Declared-only apply

`apply` sends a project setting only when the manifest declares it. For the
fields in the tables above, declaring means setting a non-zero value: a bool
that is `true`, a non-empty string, a non-empty list, or a non-nil integer.
Everything else is left to the live value or the server default. This is the
behavior the upstream edit API backs: it applies only the fields present in
the request body. `develMode` and `enableNet` are the exception: apply always
sends them, so omitting them writes the false value.

The consequence is the declared-vs-zero family, a single limitation that
shows up in several fields:

- A manifest cannot express an explicit `false` for a bool. Declare `autoPrune: true`
  to force it on; leaving it out keeps whatever the project has.
- An empty `chrootDenylist` cannot clear a live denylist. Omit the field
  instead of writing an empty list.
- `repoPriority` cannot declare `0`, which is below the upstream minimum of 1.
  Omit it to keep the live priority.
- An empty `packitForgeProjectsAllowed` cannot clear live entries. Omit the
  field instead of writing an empty list.

`export` writes only non-zero values, so an exported manifest round-trips:
apply it and nothing it omitted is touched.

## What diff and export verify

Project settings fall into three treatment classes:

- **Readable and reconciled, diffed only when declared**: `autoPrune`,
  `bootstrap`, `isolation`, `moduleHotfixes`, `appstream`,
  `packitForgeProjectsAllowed`, `followFedoraBranching`, `repoPriority`,
  `unlistedOnHomepage`. `diff` compares these only when the manifest declares
  them, and `export` emits them from live state.
- **Create-only, echoed by GET but unreconcilable**: `persistent`, `storage`.
  `diff` never compares them and `export` never emits them: the edit API has
  no field, so apply could never converge drift.
- **Write-only, set on apply and never verifiable by diff or export**:
  `multilib`, `fedoraReview`, `deleteAfterDays`, `runtimeDependencies`, and
  the package settings below. The API does not echo them back, so there is no
  live value to compare.

Because `diff` only compares declared fields, a manifest that declares nothing
stays clean against the live defaults (`auto_prune` true, `bootstrap`
"default", `isolation` "default", `follow_fedora_branching` true). The
workflow for a clean diff is declare-or-export: either declare a field
non-zero, or leave it out so `export` omits it too. Running `apply` after
`export` is always clean.

## Package settings

Each entry in `spec.packages` may carry the per-package settings:

| Field | Setting | Wire constraint |
| --- | --- | --- |
| `maxBuilds` | keep only the newest N builds | integer 0..100 |
| `timeout` | max build time in seconds | integer >= 0 |
| `chrootDenylist` | chroot patterns that never build this package | list of patterns |

These are write-only through the API: GET does not echo them, so `diff` and
`export` cannot verify them. They are sent on apply when declared. `apply`
falls back to editing an existing package when the add conflicts, so the
settings reach packages that already exist; the upstream edit merges the
stored source, which stays safe.

`chrootDenylist` patterns are joined with commas on the wire, matching the
upstream cleanup filter. The local `validate` check accepts the upstream
pattern set, but the server re-validates each pattern against the active
chroots and rejects patterns that match none or all of them.

## Reconcile workflow

```bash
coprctl validate -f copr.yaml     # schema and constraint checks, no network
coprctl diff -f copr.yaml         # field-level drift against the live project
coprctl apply -f copr.yaml        # create or update to match the manifest
coprctl apply -f copr.yaml --dry-run
coprctl export you/mypkg -o copr.yaml   # adopt an existing project
```

`apply` is additive and safe to re-run: it creates a missing project or
package, edits existing state to match the declared fields, and never removes
anything unless `--prune` is given (which restricts chroots to the manifest's
list). `--dry-run` reports the drift without changing anything.