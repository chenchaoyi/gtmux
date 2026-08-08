# hq-unread-noise — the unread knock stops counting what HQ cannot or need not read

Origin: HQ's charter-flags ledger (`~/.config/gtmux/hq/knowledge/charter-flags.md`),
entries **C14 · C7 · B9**, batched on the commander's 2026-08-07 order to move the
backlog out of HQ's notes and into the repo. Companion changes from the same batch:
`standing-wake-backoff` (the re-knock discipline family) and `hq-signal-ergonomics`
(presentation). All three are proposals only — the perception layer is what keeps HQ
seeing; it changes via review, not directly.

**Task 1's audit is done — read `audit-echo-2026-08-08.md` first.** It settles the two open
questions below and CORRECTS two things this proposal originally got wrong: C7 is ~2 % of
the debt (not a major noise source) and its literal rule would have caused a regression;
and the ~70 % echo is real but belongs to the PULL, not the knock. Sections below carry
`AUDIT:` notes where the measurement overrode the original text.

## Why

The `unread` watermark knock (change `hq-watermark-wakes`, #650) is HQ perception's
completeness net: anything past HQ's consumption watermark eventually knocks. The net
works — but three measured noise/failure modes make it knock about things HQ cannot
read, need not read, or has already read, and today's fleet-scale evidence
(2026-08-07, HQ 40 h on duty, 400+ turns, HQ estimates **~70% of its wakes were echoes
of its own activity**) says the cost is no longer marginal.

### C14 — self-excitation: the knock fed by HQ's own echo (ledger evidence, re-verified)

Ledger evidence (2026-08-04): 4 consecutive knock rounds, cursor 8439→8442→8447→8450,
each round's "1 unconsumed" judged by HQ to be **its own previous reply's `Stop`**; the
120 s debounce capped the frequency but每轮 still cost HQ a full turn.

**Verification against HEAD corrects the attribution.** The pane-id exclusion the
ledger asks for ALREADY exists — `unreadCount` skips `r.Pane == hqPane`
(`internal/hq/unread.go:121`, shipped in #650 / v0.44.10 on 2026-08-01, spec'd in
`hq-wake-protocol` "Records authored by the HQ pane itself SHALL be excluded"), and a
pure-echo backlog is stepped over (`unread.go:171-176`). Reading the actual stream for
those cursors shows genuine `%21` events interleaved in every window (seq 8442, 8444,
8449 — a user-direct conversation running concurrently), so the four knocks were
*countable* — but each of those `%21` events had ALREADY reached HQ as a class wake
(`origin:"instruction"` submissions), and the pull each knock demanded returned mostly
HQ's own echo around one already-judged event. HQ's own reconcile note at seq 8451 says
exactly this: 「增量已消费(8443–8447)—— 全是本轮回响 + `%21` 那条追问的原始投递,无新事」.

So the residual C14 problem at HEAD is **echo-dominated, double-knocked pulls**, not a
missing pane filter:

1. an event already delivered (and driver-ack'd) as a class wake still stands as unread
   debt, so HQ is knocked twice for one fact unless it re-pulls after every wake;
2. the pull's *content* is dominated by HQ's own excluded-from-count records, burying
   the one new line;
3. the knock line gives no composition ("1 unconsumed" — of what? from which pane?),
   so a feedback loop cannot be diagnosed from the line itself; HQ needed four rounds
   and a manual stream read to see the shape.

It is also **uncertain** whether the 2026-08-04 machine's resident serve was already
running the #650 binary (a `gtmux update` swaps the binary but the resident serve keeps
running the old code until restarted) — the audit task below settles this before any
further mechanism is blamed.

> **AUDIT (settled):** it was NOT a pre-#650 serve. The first `unread` knock in the stream
> is seq 7039, 2026-08-02 — two days before the loop — so the sensor and its pane filter
> were live. The residual is real. Instrumented, it is **68.7 % of the records each knock's
> pull returns are HQ's own** (75.1 % for the `≤2 unconsumed` case that is 79 % of knocks),
> against a knock volume of only ~8/day at a median of 1 event. Point 2 above is therefore
> the dominant cost, not points 1 and 3 — see "Audit-driven addition" below.

### C7 — empty-pane session events are counted as consumable (confirmed at HEAD)

Ledger evidence (measured ×2): `SessionStart`/`SessionEnd` arriving as same-second
pairs with EMPTY `pane` and `session` fields — short-lived child processes/subagents
whose hook fires without a pane. Ground truth in the stream: seq 8472–8475
(ts 1785826945/46), two Start/End pairs inside one second, no pane, no session, and
each stamped `severity:"notable"`.

At HEAD these records **count toward the unread debt**: `unreadCount` excludes only
`r.Pane == hqPane`, and an empty pane never equals the HQ pane. HQ gets knocked to pull
a delta whose content is a process blink it can neither act on nor attribute.

> **AUDIT (criterion corrected — the ledger's cheap rule is unsafe).** "A session event
> with an empty `pane` is not countable debt" was the original criterion. Measured, an
> empty pane is not a blink signature at all: of 98 pane-less records in the window, **39
> are real agent turns** (Claude Code 27, Codex 12 — pane-less native sessions, growing)
> and **9 are gtmux's own maintenance triggers** (`gtmux:self-check` ×8, `gtmux:distill`)
> which #647 shipped specifically so they would reach HQ. Excluding by empty pane would
> silently un-ship #647 and blind HQ to every native agent — and per §1.3 of the audit
> those records have NO other channel, because the whole class-wake path is gated on
> `pane != ""`. The `session` field cannot rescue the rule either: `session` is the tmux
> session_name, empty by construction on every pane-less record.
>
> **Adopted instead — the liveness pairing** (the ledger's own fallback, promoted to the
> rule): a pane-less `SessionStart` is excluded only when a pane-less `SessionEnd` from the
> same agent lands within ≤ 10 s; both are then excluded. 5 s and 60 s windows catch the
> identical 15/28 starts, so the constant sits on the flat part of the curve. Size: **30
> records = 1.9 % of the debt, ≤ 8 solo knocks over 6.6 days.** Worth fixing because it is
> noise HQ can never act on — not because it is large. Excluded records stay in the stream
> and in every other consumer's view.

### B9 — a `cd`-drifted read silently fails to consume, while `--ack` fails loudly

The consumption writeback is cwd-keyed: an unfiltered `gtmux events --since-seq` counts
as consumption ONLY when run exactly from the HQ home (`fromHQHome()`,
`internal/hq/eventscmd.go:241-247`, cwd `==` `state.HQHome()`). Ledger evidence:
**reproduced ×5** — HQ's Bash cwd persists across calls, so after writing the board
(`notes/`) or the KB (`knowledge/`) its next `gtmux events` ran from a SUBDIRECTORY of
the HQ home, consumed nothing, and the watermark sat still while `unread` re-knocked
the same cursor. The 4th recurrence was a fresh post-`/clear` instance (proof that a
knowledge-base-only discipline does not transfer); the 5th happened **in the very turn
after writing the B9 ledger entry itself** — the sharpest possible evidence that a
"remember to do it right" measure cannot hold.

The tool-side inconsistency is exact and confirmed at HEAD: `--ack` from the wrong cwd
**refuses loudly** (`eventscmd.go:264`), while the read path **silently skips** the
writeback (`eventscmd.go:250-255`). Two paths, one meaning, opposite failure modes.

(The charter-side half of B9 — "HQ prefixes every gtmux call with `cd <hq-home> &&`" —
is a playbook edit, tracked in the ledger; this change covers the tool-side hardening
the ledger asks for.)

### Verified-fixed, for the record

Ledger **B3** ("`--severity important` is not the attention stream") was checked as part
of this batch and is ALREADY FIXED — playbook v4 (`hq-attention-stream`) renamed it "the
ESCALATION stream: … a SUBSET — never your whole picture" (`internal/hq/hq.go:784`).
No work item; the ledger entry should move to 已关闭 (HQ's own edit, not this PR's).

## What changes

1. **Pane-less lifecycle BLINKS do not count** (C7, criterion corrected by the audit).
   `unreadCount` skips a pane-less `SessionStart` whose pane-less `SessionEnd` from the
   same agent arrives within `unreadBlinkPairSec` (10 s), and that end with it. Everything
   else pane-less — native agents' turns, gtmux's control records, a `SessionStart` that
   is NOT quickly matched (a native session coming online) — keeps counting. Excluded
   records remain in the stream and in every other consumer's view. Pin with a test built
   from the seq 8472–8475 shape AND with the three populations the blunt rule would have
   eaten.

2. **The knock line names its composition** (C14 residual, diagnosability). The
   `unread` line carries a compact by-pane/kind breakdown (e.g.
   `3 unconsumed (%21 ×2 · control ×1)`) so a self-feeding or echo-dominated loop is
   visible from the line itself, instead of costing four turns and a manual stream read.

3. **Echo audit before further mechanism** (C14 residual) — **DONE**, see
   `audit-echo-2026-08-08.md`. The open design question it was to decide — does an event
   delivered inside an ack'd wake batch count as consumption-equivalent? — is **decided
   NO**: the measured cost is the pull's composition, not the knock count (8/day, median 1
   event); up to 78 % of the debt is class-eligible while the 21.8 % no class claims is
   exactly what the net exists for; a wake line is a summary, not the record; and pane-less
   records can never be delivered as a class wake at all (`hook.go:679`), so "delivered ⇒
   read" would eventually blind the one population that has no other channel.

4. **A non-counting read that looks like HQ warns loudly** (B9). An unfiltered
   `--since-seq` read that does NOT advance the watermark, invoked from a cwd
   **strictly inside the HQ home** (a subdirectory — the exact ×5 failure shape), emits
   a one-line stderr warning: this read was not counted as consumption, run from
   `<hq-home>` (or `--ack`) to consume. Symmetric with `--ack`'s existing loud refusal.
   The warning deliberately keys on "inside the HQ home but not at it" so a regular
   user running `gtmux events` elsewhere is never nagged about a watermark they don't
   own. Whether to ALSO key on `$TMUX_PANE == HQ pane` (catching a drift to an
   unrelated cwd entirely) is an open question for implementation — it widens coverage
   but couples the read path to pane resolution.

## Audit-driven addition — NOT implemented, the commander's call

The audit's headline number has no work item in the four above, because the proposal was
written before the measurement: **68.7 % of what a knock's pull returns is HQ's own
records**. The cause is an asymmetry inside the mechanism — `unreadCount` excludes HQ's own
pane when defining the debt (`unread.go:121`), while `gtmux events --since-seq` returns
everything when clearing it (`eventscmd.go:153-165`). One set says what HQ owes; a larger,
different set is what HQ must read.

The fix that follows from the audit is to **make the read's exclusion set equal the
count's**: an unfiltered `--since-seq` pull run from the HQ home hides exactly what the
counter already excludes (HQ's own pane records, and the paired blinks), with an opt-in
flag to see everything. That is defensible against the "a filtered read is not consumption"
invariant precisely because it is not a filter on what HQ owes — it IS what HQ owes; the
debt definition and the read that clears it would finally name the same set.

It is held back from this change on purpose: it changes what a documented command returns
for the supervisor, needs a `--all` escape hatch, and touches the playbook (hence
`hqPlaybookVersion`). Scoped as a companion change if the commander wants it.

## Impact

- Specs: `hq-wake-protocol` (two MODIFIED requirements — see delta).
- Code (when implemented): `internal/hq/unread.go`, `internal/hq/eventscmd.go`,
  `internal/hqwake` (line composition); tests alongside.
- No contract breaks: the events schema, watermark file, and wake-line grammar prefix
  (`» gtmux·unread`) are unchanged; the line's payload gains detail.
- Risk: this is HQ's perception layer — a wrong exclusion silently blinds the
  completeness net. Every exclusion added here must be pinned by a test that asserts
  the excluded record still appears in the unfiltered stream read.
