# Tasks

- [x] Add scoped transcript content/path and usage readers, preserving fresh direct APIs.
- [x] Reuse discovery across complete digest/usage producers and ordered HQ gathers.
- [x] Cover producer discovery count, content switches, freshness and independent scopes; verify mutations.
- [x] Run targeted race tests, make check and strict design/spec checks; sync bilingual docs and archive.

## Acceptance

Targeted race tests across radar/usage/driver/transcript/hq and full `make check` passed. Three mutations (unscoped digest, unscoped usage, cached rows) were rejected.

No live-home timing benchmark, release, install or real-agent operation is part of these fixture tests.
