---
name: coprctl-debug
description: Debug a failing Copr build - find why it failed, reproduce
  it locally with rpmbuilder or mock, and test a fix before pushing.
---

# Debugging a failing Copr build

Use this skill when a Copr build fails and you need to find the cause,
reproduce it locally, and verify a fix before pushing.

## 1. Find why the build failed

Do not ingest a whole build log. Run the failures summariser first:

```bash
coprctl log failures BUILD_ID
```

It extracts the failing region from each failed chroot. For a quick
plain-language analysis of the root cause, ask the log-detective helper:

```bash
coprctl log detective BUILD_ID/CHROOT
```

## 2. Reproduce locally

Get the exact reproduction recipe Copr wrote into the build log:

```bash
coprctl build reproduce BUILD_ID/CHROOT
```

This prints the `copr-rpmbuild --task-url ...` invocation. If you have a
container runtime, `coprctl try` runs a local clean-room preflight build:

```bash
coprctl try ./rpm --chroot fedora-rawhide-x86_64
```

`try` resolves the Copr chroot to an rpmbuilder image and runs the
source-build then chroot-build stages, reporting coverage and fidelity.
When no container runtime is available, `build reproduce` with mock is the
fallback (needs mock and privileges).

## 3. Test a fix before pushing

- For tito projects: commit the spec change (tito ignores uncommitted
  changes), then run `coprctl try` in the project. Squash WIP commits
  before tagging; never push intermediate debug commits.
- Declare all dependencies in `BuildRequires`/`Requires`; do not install
  them manually in the container.
- Iterate until the local build is clean, then rebuild in Copr:

```bash
coprctl build rebuild OWNER/PROJECT/PKG --preflight
```

## Rules
1. Start with `coprctl log failures BUILD_ID`, never a full log dump.
2. Reproduce locally before queueing another Copr build.
3. A local pass is a filter, not a proof: read the fidelity report from
   `coprctl try` and name what was not reproduced.
4. Commit before `try` on a tito project; squash WIP before push.
5. Use `coprctl log detective` for a second opinion on the root cause.

## Reference

All commands are part of the `coprctl` skill; print it with
`coprctl skill print coprctl`.
