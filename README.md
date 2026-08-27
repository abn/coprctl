# coprctl

A reimagined command-line and agent interface for the [Fedora Copr](https://copr.fedorainfracloud.org)
build service.

`coprctl` keeps Copr's own vocabulary but fixes the ergonomics that make
`copr-cli` painful as a daily driver and as an automation surface: one
noun-verb grammar, machine-readable output everywhere, declarative project
state, live build logs, and a generated agent interface that cannot drift from
the CLI.

The project and binary are both named `coprctl`. It is instance-agnostic:
Fedora production, staging, openEuler, and self-hosted Copr are all first-class
profiles.

## Features

- **One grammar.** `coprctl <resource> <verb> [ref]`, with one shared
  `owner/project[:dir][/segment]` reference parser. A new source type is a
  `--source` value, never a new command.
- **Machine-readable everywhere.** Every data command supports
  `--output json|jsonl|yaml|table|plain`, defaulting to JSON when stdout is
  not a TTY. Stable exit codes and structured error objects.
- **Declarative state.** A `copr.yaml` manifest with `validate`, `diff`
  (exit 12 on drift), `apply`, and `export`. `detect`, `init`, and `sync`
  close the loop from a source repo to a running project.
- **Live logs and debugging.** `log tail` streams multi-chroot build logs;
  `log failures` extracts the failing region from each failed chroot;
  `log detective` asks log-detective.com for a plain-language explanation;
  `build reproduce` prints the local reproduction recipe.
- **Local preflight.** `try` builds in a container with rpmbuilder, mapping
  Copr chroots to images, with strict exact-match by default and a fidelity
  report. Works on any OS with Docker or podman, so macOS and Windows users
  can build and test RPMs locally.
- **Webhooks, end to end.** `integration github enable` wires the Copr and
  GitHub sides in one command, tag-triggered by default.
- **Agent ready.** `schema` emits the command tree as JSON; `mcp serve`
  exposes it as tiered MCP tools; `skill print`/`install` ship generated
  skills. CLI, completions, JSON schema, MCP tools, docs, and skills are all
  generated from one command registry, so they cannot drift.

## Install

```bash
go install github.com/abn/coprctl/cmd/coprctl@latest
```

Once the project's own Copr repo is live:

```bash
sudo dnf copr enable abn/coprctl
sudo dnf install coprctl
```

## Getting started

Your existing `~/.config/copr` credentials are picked up automatically. The
quickest paths to a fresh token:

```bash
coprctl auth login            # open the API page, paste the block
coprctl config import <<'EOF'
[copr-cli]
login = "..."                 # from the website
username = "you"
token = "..."                 # from the website
copr_url = "https://copr.fedorainfracloud.org"
EOF
```

Then check your environment and the instance:

```bash
coprctl auth status           # who you are, and how long until the token expires
coprctl doctor                # config, auth, and connectivity checks
coprctl chroot list --distro fedora-rawhide
```

## Usage

The documentation wiki under `docs/` is the public reference. Key entry
points:

- [Overview](docs/overview.md)
- [Command reference](docs/reference/commands.md)
- [Debugging a failing build](docs/usage/debugging-builds.md)
- [GitHub webhook integration](docs/usage/github-integration.md)

Common workflows:

```bash
# from a source repo to a running project
coprctl detect ./rpm --output json
coprctl init --owner you --name mypkg \
  --chroot fedora-44-x86_64 --chroot fedora-rawhide-x86_64 --yes

# reconcile the manifest
coprctl validate -f copr.yaml
coprctl diff -f copr.yaml
coprctl apply -f copr.yaml

# rebuild and debug
coprctl build rebuild you/mypkg/mypkg
coprctl log failures 12345678
coprctl build reproduce 12345678/fedora-rawhide-x86_64
coprctl try ./rpm --chroot fedora-rawhide-x86_64

# health as a probe
coprctl status you/mypkg --quiet || notify-send "Copr: something failed"
```

## Development

```bash
make build     # build bin/coprctl
make check     # format, lint, vet, test, and drift gate
make gen       # regenerate generated artefacts from the command registry
```

See [docs/contribution/](docs/contribution/) for the contributor guide and
`AGENTS.md` for the operational contract.

## License

MIT. See [LICENSE](LICENSE).
