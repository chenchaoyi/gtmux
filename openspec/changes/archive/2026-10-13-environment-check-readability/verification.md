# Verification — 2026-10-13

- User-provided screenshot shows installation/tmux/agent rows under Logs and HQ.
  Read-only `doctor --json --no-prompt` returned correctly separated groups; CLI
  rows were not duplicated. The native nested List reused row-offset identities
  across sections. The new display model gives each row a section-qualified,
  label-based identity; explicit scrollable groups avoid flattened List reuse.
- Read-only local report coverage: all 54 reported rows have a purpose description.
  Source inspection corrected descriptions to the actual probe: knowledge topics
  and pending records (not a link audit), store writability (not historical failures),
  seven-day runaway-log warnings and 24-hour diagnostic errors.
- `make check`: passed (format, vet, staticcheck, all Go race tests, 32 publisher tests).
- `swift test`: passed, 394 tests. New tests cover group/row identity, conditional
  insertion, repeated headings/labels, old report decoding, localized timestamp,
  wrapped purposes and narrow/wide English/Chinese native component rendering.
- Native rendering evidence was generated and visually inspected at 560 and
  760 pt. Check names/purposes/results/details are distinct, HQ does not repeat
  installation rows, and text fits both widths. These are production component
  renders with synthetic reports, not captures of the installed application.
- Strict OpenSpec change validation: passed.
- No environment repair, app installation, release or running-app replacement.
  Installed-window interaction and VoiceOver acceptance are not claimed.
- `scripts/check-design.sh`: passed after spec sync/archive. Task-owned temporary
  Swift scratch directory was removed; available disk space was about 14 GiB.
