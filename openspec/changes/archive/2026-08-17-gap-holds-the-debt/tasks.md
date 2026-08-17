# Tasks — gap-holds-the-debt

Status: IMPLEMENTED (2026-08-17). Test-first: each of the three loss paths went
red before its fix.

- [x] 1. `gtmux events --since-seq` with a gap: skip the implicit writeback,
     warn with the rebuild-then-`--ack` exit, re-warn on every pull until the
     explicit ack. Tests: watermark holds across a gap pull (HQ-home env);
     the warning names `--ack`; a contiguous pull still consumes.
- [x] 2. unread net: `unreadScan` carries Gap; N==0 with a gap knocks instead
     of auto-consuming; the knock names the gap; doctor's consumption row
     reports Slipped on a standing gap. Tests: scan flags the hole; sensor
     holds the watermark and knocks on an echo-only gap.
- [x] 3. TROUBLESHOOTING: replace "Seed is generated ONCE" with the real
     upgrade behavior (version/language regeneration + backups).
- [x] 4. Spec delta: catch-up requirement MODIFIED (a gap read is not
     consumption); self-check requirement MODIFIED (journal control record,
     not "feed").
- [x] 5. Gates green: `make check`, `CGO_ENABLED=0 go build ./cmd/gtmux`,
     `scripts/check-design.sh`, `openspec validate --strict`.
