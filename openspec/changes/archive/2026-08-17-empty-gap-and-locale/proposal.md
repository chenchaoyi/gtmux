# empty-gap-and-locale — the quietest gap gets a voice, and the locale gets a say

Origin: the commander's third whole-of-hq review (2026-08-17), all three
findings.

## Why

1. **An empty retained tail hid the severest gap.** The sequence counter
   survives the log files, so "journal files gone, counter alive" left a
   positive cursor behind the counter with nothing retained — and
   `ReadSince`'s empty result was hard-coded `gap=false`. No pull warning, an
   unread sensor idling silently every interval: total loss, zero noise.
2. **The language ignored the system locale.** Only `GTMUX_LANG`/`--lang`
   counted (default `en`); `LANG=zh_CN.UTF-8` meant nothing. A Chinese user
   who never exported `GTMUX_LANG` would have their v28 mixed-language charter
   upgraded to a PURE-ENGLISH v32 — a downgrade delivered as an upgrade.
3. **docs/cli.md's watermark section lagged a change**: "only two things
   count" omitted that a gap read no longer counts, and the knock's
   `· sequence gap` variant was undocumented. (Doc fix, rides along.)

## What changes

- `ReadSince` reports a gap when the tail is empty but a positive cursor sits
  behind the sequence counter (cursor 0 keeps its no-prior-position
  exemption). The read-time warning's suggested ack target becomes the
  COUNTER (`events.CurrentSeq()`) — with an empty tail the old suggestion
  equalled the cursor, a no-op; `--ack` already clamps to the counter.
- `GTMUX_LANG` unset falls back to the system locale: `LC_ALL` over `LANG`
  (POSIX order), a `zh*` prefix reads Chinese, everything else English. An
  explicit GTMUX_LANG always wins; a set-but-unknown value resolves to
  English rather than silently falling through.
- docs/cli.md: the language line and the watermark section catch up.

## Non-goals

No new locale grammar beyond the zh* prefix (gtmux ships two languages); no
auto-repair of an empty journal — the gap warning's rebuild-then-ack exit is
the recovery, as everywhere else.

## Impact / risk

Both behavior changes are pinned by table tests; the gap widening only ever
ADDS a warning where silence was wrong.
