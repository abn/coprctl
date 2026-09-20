---
type: Reference
title: Upstream sources
description: Permitted public sources used as ground truth for the Copr API.
status: stable
---

# Upstream sources

When a question touches the server, the API, or how an existing client
behaves, these public sources are authoritative, in order. This page is the
provenance ledger for that ground truth; the maintainer guide explains when
to reach for each layer.

1. The upstream Copr source at `https://github.com/fedora-copr/copr`,
   especially the Flask views under
   `frontend/coprs_frontend/coprs/views/apiv3_ns/` and `coprs/views/`.
   These show exactly what the API accepts and how the web forms translate
   to model changes.
2. The copr-cli client (`python-copr`) for how an established client calls
   the API.
3. Live probing against a staging instance when a behaviour is still
   unclear.

A field or endpoint that works in the web UI but is absent from the
swagger is not a bug in the docs; it is the upstream reality to match.
