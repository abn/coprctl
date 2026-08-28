---
type: Guide
title: Maintainer guide
description: Process guidance for maintainers of the project.
status: stable
---

# Maintainer guide

This page captures the maintainer workflow: reviewing changes, keeping the
wiki honest, and driving milestones to completion.

The operational contract lives in `AGENTS.md`. The reviewer role card in
`.agents/agents/reviewer.md` describes the skeptical-maintainer review gate
used before any push.

Releases follow the [release process](release-process.md): pull request only,
squash merge, and the normal-versus-NVR bump decision.

## Ground truth

When a question touches the server, the API, or how an existing client
behaves, the upstream source is authoritative. The instance's
`/api_3/swagger.json` is the API's formal contract, but it is incomplete in
practice: request bodies are often unmodeled, and the web UI exercises
endpoints and form fields the swagger never documents.

Prefer, in order:

1. The upstream Copr source at
   `https://github.com/fedora-copr/copr`, especially the Flask views under
   `frontend/coprs_frontend/coprs/views/apiv3_ns/` and `coprs/views/`. These
   show exactly what the API accepts and how the web forms translate to
   model changes.
2. The copr-cli client (`python-copr`) for how an established client calls the
   API.
3. Live probing against a staging instance when a behaviour is still unclear.

A field or endpoint that works in the web UI but is absent from the swagger
is not a bug in the docs; it is the upstream reality to match. When in doubt,
read the source before changing client code.