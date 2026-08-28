---
type: Guide
title: Debugging a failing build
description: Find why a Copr build failed, reproduce it locally, and test a fix.
status: stable
---

# Debugging a failing Copr build

When a build fails, the tool is built to take you from the failure to a
reproduced, fixed build without reading whole logs.

## 1. Find why it failed

Do not ingest a whole build log. Extract the failing region from each failed
chroot:

```bash
coprctl log failures BUILD_ID
```

For a second opinion in plain language, ask the log-detective service:

```bash
coprctl log detective BUILD_ID/CHROOT
```

If the service does not know the build, it falls back to local analysis.

## 2. Reproduce locally

The exact reproduction recipe Copr wrote into the build log:

```bash
coprctl build reproduce BUILD_ID/CHROOT
```

This prints the `copr-rpmbuild --task-url ...` invocation for mock-level
fidelity.

If you have a container runtime (podman or docker), `coprctl try` runs a local
clean-room preflight build:

```bash
coprctl try ./rpm --chroot fedora-rawhide-x86_64
```

`try` resolves the Copr chroot to an rpmbuilder image, runs the source-build
then chroot-build stages, and reports coverage and a fidelity report. Mock is
the fallback when no runtime is available (needs mock and privileges).

## 3. Test a fix before pushing

- For tito projects, commit the spec change first (tito ignores uncommitted
  changes), then run `coprctl try`. Squash WIP commits before tagging.
- Declare all dependencies in `BuildRequires`/`Requires`; do not install them
  manually in the container.
- Iterate until the local build is clean, then rebuild in Copr:

```bash
coprctl build rebuild OWNER/PROJECT/PKG --preflight
```

## Building and submitting a source RPM

To produce a source RPM from a local spec without a local `rpmbuild`, use a
container:

```bash
coprctl build srpm ./rpm --chroot fedora-rawhide-x86_64
```

This runs the same `SRPM_ONLY` stage as `try` inside an rpmbuilder image and
writes the `.src.rpm` into the spec directory.

To build the source RPM and submit it in one step, `build submit` chains the
two:

```bash
coprctl build submit OWNER/PROJECT --from ./rpm
```

`--from` builds the SRPM locally via the container, then uploads it to Copr
and queues the build. It needs a container runtime and write access to the
project.

## Rules

1. Start with `coprctl log failures BUILD_ID`, never a full log dump.
2. Reproduce locally before queueing another Copr build.
3. A local pass is a filter, not a proof: read the `try` fidelity report.
4. Commit before `try` on a tito project; squash WIP before push.

## Agent skill

The `coprctl-debug` skill encodes this workflow and is installed with
`coprctl skill install coprctl-debug`.