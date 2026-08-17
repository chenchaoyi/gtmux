# Tasks — empty-gap-and-locale

Status: IMPLEMENTED (2026-08-17).

- [x] 1. events.ReadSince: empty tail + positive cursor behind CurrentSeq →
     gap; the pull warning suggests --ack <counter>. Tests: empty-tail gap;
     cursor 0 exemption; caught-up cursor stays clean.
- [x] 2. langFromEnv: GTMUX_LANG wins; unset → LC_ALL/LANG zh* prefix;
     broken explicit value → en. Table test.
- [x] 3. docs/cli.md: language line + watermark section catch up.
- [x] 4. Spec deltas: hq-attention-system catch-up MODIFIED (empty-tail
     clause); supervisor-agent language requirement MODIFIED (locale
     fallback).
- [x] 5. Gates green: `make check`, `CGO_ENABLED=0 go build ./cmd/gtmux`,
     `scripts/check-design.sh`, `openspec validate --strict`.
