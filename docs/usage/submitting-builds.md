---
type: Guide
title: Submitting builds
description: Submit builds with the generic build options, chroot globs, batch options, uploads, and batch delete.
status: stable
---

# Submitting builds

`coprctl build submit` queues a build from a source definition. The source
type is chosen with `--source` and the reference names the project:

```bash
coprctl build submit OWNER/PROJECT --source scm --clone-url https://github.com/me/rpm.git
coprctl build submit OWNER/PROJECT --source url --url https://example.com/hello.spec
```

A `:dir` suffix in the reference builds into a project directory (side repo);
the `--dir` flag is the fallback when the reference has none.

## Generic build options

Every source type accepts the same option set, sent only when you set it:

| Flag | Wire key | Meaning |
| ---- | -------- | ------- |
| `--background` | `background` | Queue and return; the server reports `is_background`. |
| `--enable-net` | `enable_net` | Enable network access in the buildroot. Absent keeps the project default; `--enable-net=false` disables it explicitly. |
| `--timeout` | `timeout` | Per-build timeout in seconds (a positive integer). |
| `--bootstrap` | `bootstrap` | `on`, `off`, `default`, `image`, or `unchanged`. |
| `--isolation` | `isolation` | `simple`, `nspawn`, `default`, or `unchanged`. |
| `--exclude-chroot` | `exclude_chroots` | Chroot globs to exclude; repeatable. |
| `--after-build-id` | `after_build_id` | Build after the batch containing this build id. |
| `--with-build-id` | `with_build_id` | Build in the same batch as this build id. |

`--background` and `--enable-net` are tri-state: not passing them leaves the
server or project default in place, while an explicit `--enable-net=false`
actively disables the network. `--after-build-id` and `--with-build-id` are
mutually exclusive. `--timeout` is seconds, not minutes.

Chroot globs work on both sides: `--chroot 'fedora-*'` selects a set and
`--exclude-chroot 'fedora-rawhide-*'` subtracts from it.

## URL submits create one build per URL

`--source url` accepts multiple `--url` values and creates one build per URL.
The command prints every created build: a row per build in the human table, a
JSON array under `--output json`, and one object per build under
`--output jsonl`. `--watch` waits for each submitted build.

Every submit, not just URL, returns a JSON array under `--output json`; a
single build is a one-element array, so a script reads `.[0]`.

## Upload and local source RPMs

`--source upload --upload PATH` uploads a local SRPM. The chroot set comes from
the SRPM itself, so `--chroot` is ignored there, but the generic options still
apply.

`--from ./rpm` builds a source RPM locally from a spec directory (via the
container, native, or mock backend) and uploads it in one step:

```bash
coprctl build submit OWNER/PROJECT --from ./rpm --watch
```

## Publishing an already-built RPM

`--source rpm-upload --rpm PATH` publishes a local, already-built RPM directly
into the chosen chroots, skipping the SRPM build and dist-git import. It is a
build-submit-only source and is not offered to `package create` or
`package edit`.

```bash
coprctl build submit OWNER/PROJECT --source rpm-upload --rpm ./hello-1.0.rpm --chroot fedora-rawhide-x86_64
```

`--chroot` is required: an omitted chroot list would publish to every active
project chroot. `--sha256 HEX` verifies the uploaded file against an expected
digest, and the server rejects the build on mismatch.

The route is gated by the instance's `DIRECT_RPM_UPLOAD` setting. On instances
where it is off (including Fedora infrastructure) the command fails with a
`feature_disabled` error rather than silently succeeding.

## Migrating a copr-cli SRPM upload

`copr-cli build OWNER/PROJECT file.src.rpm` passes the SRPM as a positional
argument. coprctl has no positional SRPM, so the file moves into `--upload`:

```bash
coprctl build submit OWNER/PROJECT --source upload --upload ./foo-1.0-1.src.rpm --watch
```

`coprctl compat copr-cli -- build OWNER/PROJECT file.src.rpm` prints the
equivalent submit (add `--watch` if wanted). The translation expects the
project reference first; a second `.src.rpm` is rejected (submit each
separately), and a prebuilt `.rpm` is not an upload at all: publish it with
`--source rpm-upload --rpm` and `--chroot` instead.

Prefer a tag-only webhook over uploads for release automation: the webhook
needs no standing Copr API token in CI (pushing and tagging still use the
forge's own credentials) and Copr builds from the tag itself (see the
webhook integrations guide). Keep the upload form for one-off submits and
for the transition period.

## GitHub Actions

For the submits that still run in CI, download a pinned release asset
(archives are named `coprctl_<version>_<Os>_<Arch>.tar.gz`), pipe the
credentials into `auth login`, and submit with machine output:

```yaml
- name: Submit SRPM to Copr
  env:
    COPR_CONFIG: ${{ secrets.COPR_CONFIG }}
    COPR_REPO: OWNER/PROJECT
    COPRCTL_VERSION: 1.0.1
    SRPM_PATH: ./foo-1.0-1.src.rpm
  run: |
    curl -sSL -o coprctl.tar.gz \
      "https://github.com/abn/coprctl/releases/download/v${COPRCTL_VERSION}/coprctl_${COPRCTL_VERSION}_Linux_x86_64.tar.gz"
    tar xzf coprctl.tar.gz coprctl && chmod +x coprctl
    printf '%s' "$COPR_CONFIG" | ./coprctl auth login --no-open
    ./coprctl build submit "$COPR_REPO" \
      --source upload --upload "$SRPM_PATH" \
      --output json --watch
```

`COPR_CONFIG` holds the `[copr-cli]` block (login, username, token,
`copr_url`), the same block the Copr website offers and `coprctl auth`
accepts. `COPR_REPO` is the `OWNER/PROJECT` reference, `COPRCTL_VERSION`
pins the tool release, and `SRPM_PATH` points at the built source RPM.
Piping the config through `auth login` validates the parse and writes
the profile without touching config file paths by hand; `--profile`
selects a non-default instance. Confirm the asset name on the release
page and bump `COPRCTL_VERSION` instead of tracking latest. After the
webhook migration this job goes away; what stays is the existing test
suite plus `coprctl sync --check` against the manifest.

## Batch delete

`coprctl build delete BUILD_ID... --yes` deletes all given builds in one
request to the list endpoint. The delete is atomic: a single invalid or still
running id aborts the whole batch instead of deleting the valid ones.

## Reproducing a build

`coprctl build reproduce BUILD_ID/CHROOT` prints the exact `copr-rpmbuild`
recipe Copr ran, and the stored source definition behind it. When the log
carries no recipe, the command reconstructs the submit from the stored source
build config; it only fails when neither is available.
