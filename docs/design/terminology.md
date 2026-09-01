---
type: Reference
title: Terminology
description: The canonical Copr vocabulary the CLI uses, and the reference syntax.
status: stable
---

# Terminology

`coprctl` aligns to official Copr vocabulary. The canonical terms below are
the words the CLI, its help text, its JSON keys, its errors, and its skill all
use. A few historical synonyms are accepted as aliases, but each concept has
one canonical name.

## Glossary

| Canonical term | Also called (accepted alias) | Definition |
|---|---|---|
| **owner** | user, group, namespace | A FAS user, or a group written `@groupname`. |
| **project** | copr, repo | A named container owned by an owner. Produces one RPM repository per chroot. Canonical reference: `owner/project`. |
| **project directory** | side repo, side tag, PR dir | A named sub-repository inside a project: `project:tag`, `project:pr:123`. |
| **mock chroot** | chroot, target | An available build target in the instance catalog: `<distro>-<version>-<arch>`. |
| **project chroot** | enabled chroot | A mock chroot enabled on a project, plus its per-project buildroot config. |
| **build chroot** | build target result | One execution of a build in one chroot, with its own state and result. |
| **package** | — | A named source definition inside a project, not an RPM. |
| **source type** | build method | One of `scm`, `distgit`, `pypi`, `rubygems`, `custom`, `url`, `upload`. `rpm-upload` is a build-submit-only source (a direct RPM publish), not a package definition. |
| **build** | job | One submission: a source-build phase followed by N build chroots. |
| **devel mode** | "create repositories manually", `--disable_createrepo` | Suppresses automatic repo metadata regeneration. |
| **module** | — | Module *building* was removed upstream; the `additional_modules` project-chroot field still round-trips as `module_toggle` but has no build effect. |

## Reference syntax

One parser is used by every command. The shared form is
`owner/project[:dir][/segment]`:

| Form | Resolves to | Example |
|---|---|---|
| `name` | project owned by the authenticated user | `aetherpak` |
| `owner/name` | project | `quadzero/aetherpak` |
| `@group/name` | group project | `@copr/copr-dev` |
| `owner/name:tag` | project directory | `quadzero/aetherpak:testing` |
| `owner/name/package` | package | `quadzero/aetherpak/aetherpak-cli` |
| `owner/name/chroot` | project chroot | `quadzero/aetherpak/fedora-42-x86_64` |
| `<int>` | build | `10653539` |
| `<int>/<chroot>` | build chroot | `10653539/fedora-42-x86_64` |

A three-segment form is disambiguated against the cached chroot catalog: if the
third segment matches a known chroot it is a project chroot, otherwise a
package. `--as-package` and `--as-chroot` force the interpretation.

## Disambiguation by nesting

The word `chroot` means three different things, and `coprctl` disambiguates by
nesting under the parent resource, mirroring the API:

- `coprctl chroot list` is the instance catalog (mock chroots).
- `coprctl project chroot ...` is project chroots.
- `coprctl build chroot ...` is build chroots.

See [CLI grammar](cli-grammar.md) for the full command surface.