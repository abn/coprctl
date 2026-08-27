---
okf_version: "0.2"
---

# coprctl documentation

A reimagined command-line and agent interface for the Fedora Copr build
system. This is an Open Knowledge Format v0.2 bundle: a human- and
agent-readable wiki that tracks the project's public design, architecture,
usage, and contribution guidance.

The working design specification lives outside this bundle. This wiki is
maintained to reflect status quo as the project evolves.

## Getting started

* [Overview](overview.md) - what the project is and why it exists
* [Contribution guide](contribution/index.md) - how to contribute
* [Maintainer guide](contribution/maintainers.md) - for maintainers

## Design

* [Design overview](design/index.md) - the high-level approach
* [Terminology](design/terminology.md) - the canonical vocabulary the CLI uses
* [CLI grammar](design/cli-grammar.md) - the command surface and reference syntax

## Architecture

* [Architecture overview](architecture/index.md) - components and layout
* [Log streaming](architecture/log-streaming.md) - how live logs are tailed
* [Agent contract](architecture/agent-contract.md) - exit codes, schemas, and
  the generated skill

## Decisions

* [ADR index](adr/index.md) - architecture decision records

## Usage

* [Usage index](usage/index.md) - guides and examples

## Reference

* [Reference index](reference/index.md) - generated command reference

## Repository contract

See `AGENTS.md` for the operational contract for humans and agents. This wiki
never contains internal names, codenames, hostnames, absolute paths, tokens,
or task identifiers; the project progress log is not part of the wiki.