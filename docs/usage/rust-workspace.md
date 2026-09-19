---
type: Guide
title: Packaging a Rust workspace for Copr
description: From a Cargo workspace to tag-driven RPM builds, end to end, using a template spec and a generated srpm target.
status: stable
---

# Packaging a Rust workspace for Copr

This guide takes a Cargo workspace with no packaging at all to tag-driven
RPM builds. It uses a template spec because Rust releases need version
stamping and vendored dependencies, and both happen at SRPM time rather
than in the repository.

## Lay out the repository

Keep the template next to the Rust sources, for example
`packaging/rpm/myapp.spec.in`:

```spec
Name:           myapp
Version:        @RPM_VERSION@
Release:        @RPM_RELEASE@
Summary:        Short description
License:        MIT
URL:            https://github.com/OWNER/REPO
Source0:        myapp-%{version}.tar.gz

BuildRequires:  cargo >= 1.90
BuildRequires:  rust >= 1.90

%description
Long description.

%prep
%autosetup -n myapp-%{version}

%build
cargo build --release --locked --offline -p myapp

%install
install -Dm0755 target/release/myapp %{buildroot}%{_bindir}/myapp

%files
%{_bindir}/myapp
```

Match the `cargo` BuildRequires against the workspace `rust-version`.
The `%build` step stays `--locked --offline`: the network is available
when the SRPM is generated, never when the package builds.

## Detect before creating anything

```bash
coprctl detect ./myapp
```

Expect the template with `"template": true` and method `make_srpm`, the
`has_cargo` and `rust_version` signals, and two decisions: the chroot set
(which is never guessed) and the missing `.copr/Makefile` (resolved by
init). Pick active chroots carrying a recent enough cargo:

```bash
coprctl chroot list --state active --distro fedora
```

## Scaffold and create the project

```bash
coprctl init ./myapp --owner OWNER --chroot fedora-44-x86_64 --yes
```

This writes `copr.yaml` and `.copr/Makefile` into the repository and
creates the Copr project with the package. The Makefile target derives
the version from the pushed tag, renders every `@TOKEN@` placeholder,
packs the sources, runs `cargo vendor` with the offline override, and
builds the SRPM into `$(outdir)`. The tarball stem and substitutions are
parsed from the template; verify both, plus the `VERSION` default, before
the first real build. Vendoring needs a committed `Cargo.lock`.

Commit the Makefile: Copr clones the repository at build time, so the
target must be in git. Then prove the package builds without pushing a
tag:

```bash
coprctl build rebuild OWNER/PROJECT/myapp --watch
```

## Enable the webhook

```bash
coprctl integration rotate-secret OWNER/PROJECT --yes
coprctl integration github enable OWNER/PROJECT --repo OWNER/REPO
```

Tag-only is the default. Every pushed tag fires the webhook; Copr offers
no tag-pattern filter, so a tagged pre-release such as `v1.2.3-rc.1`
builds as version `1.2.3` with a `0.rc.1` release, following standard RPM
practice. If the project does not want pre-release builds, do not push
pre-release tags to the repository.

## Release

Pushing `vX.Y.Z` builds the release on its own across the enabled
chroots. Pre-merge validation of the SRPM (a mock rebuild, as in the
release PR checks) stays useful as a fast signal independent of Copr.

## Steady state

```bash
coprctl sync --check -f copr.yaml   # CI gate against manifest drift
coprctl try ./myapp/packaging/rpm   # local preflight against a rendered spec
```

Note that `try` needs a rendered `.spec`: render the template first (the
`.copr/Makefile` target does this with `VERSION=...`) and point it at
the output directory.

## Related

- Webhook integrations: template specs and Rust workspaces in depth.
- Migrating a project to webhook builds: converting an existing
  hand-submitted project instead of starting here.
- Local builds: backends and fidelity for preflight runs.
