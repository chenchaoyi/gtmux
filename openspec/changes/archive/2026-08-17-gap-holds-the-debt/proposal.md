# gap-holds-the-debt — a retention gap survives being warned about once

Origin: the commander's second whole-of-hq review (2026-08-17), finding #1.
retire-perception-spool moved gap detection to the read, but left the
consumption writeback unconditional — so the one mechanism built to make loss
impossible to miss could itself forgive it.

## Why — the measured findings

1. **The warned read still advances the watermark.** `gtmux events --since-seq`
   prints the CRITICAL gap warning and then calls the implicit consumption
   writeback anyway. The warning gets exactly one chance to be seen (stderr,
   mid-rotation, a truncated screen) — and once the watermark has jumped the
   hole, the unread net reports nothing and the loss is final. The spec says
   "rebuild before acking"; the implementation acks implicitly.
2. **The unread net is blind to the hole.** `unreadScan` discards ReadSince's
   gap flag (`recs, _ :=`), and its N==0 path mechanically advances the
   watermark over anything that is only echo/blink — including a hole followed
   by nothing but echoes.
3. **docs/TROUBLESHOOTING.md's "Seed is generated ONCE" section is obsolete
   and harmful**: it claims AGENTS.md is "never overwritten" and teaches a
   manual delete-and-reseed — versioned-hq-playbook (and now the language
   switch) regenerate it automatically with a backup; the manual procedure is
   pure risk. (Doc finding from the same review, riding along.)

## What changes

- **A gap read leaves the debt standing.** When `--since-seq` detects a gap,
  the implicit writeback is skipped; the warning says so and names the exit:
  rebuild from `gtmux digest --json`, then `gtmux events --ack <seq>`. Every
  subsequent pull re-warns until the explicit ack — the debt keeps asking,
  exactly like every other debt.
- **The unread net sees the hole**: `unreadScan` carries `Gap`; a tally with a
  gap knocks even at N==0 and is never auto-consumed; the knock names the
  gap and the rebuild-then-ack exit. `gtmux doctor`'s consumption row reports
  Slipped on a standing gap.
- **TROUBLESHOOTING**: the stale section becomes "The playbook upgrades
  itself" — what actually happens on version/language changes and where the
  backups live.
- Spec: the catch-up requirement gains the debt-holding contract; the
  self-check requirement drops the leftover "feed control record" wording
  (review finding #5, same spec file).

## Non-goals

No new wake class (the gap rides the existing `unread` knock and the read-time
warning); the explicit `--ack` stays the only way over a gap — no auto-heal.

## Impact / risk

Behavior narrows in one direction: a gap now keeps asking instead of being
forgiven. The N==0 auto-consume convenience is preserved for the gapless case,
so a quiet fleet still costs nothing.
