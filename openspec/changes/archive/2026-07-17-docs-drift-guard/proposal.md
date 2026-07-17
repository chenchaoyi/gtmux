# Proposal: docs-drift-guard

## Why

The repo's defense against docs drift is **periodic manual audits**, and manual audits are
exactly what keeps failing.

- **2026-07-17 (#478):** `docs/cli.md`'s wake section described the channel as it worked
  *before hq-perception-v2 shipped* — a format the builder can no longer produce
  (`[gtmux] waiting·permission …`), a `done` trigger that had been generalized from
  "a dispatched task" to "any session", and 8 of 12 wake classes simply missing.
- **2026-07-15 (#473):** a manual "post-v0.28 audit — sync docs with the shipped
  pair/share + doctor + push state": 7 files, 45 insertions. **That audit ran two days
  before #478 and walked straight past the wake section**, which was a version out of date
  and sits in the same file.
- **The `attach` incident:** a command shipped absent from the usage and `docs/cli.md` —
  which is why `check-design.sh` §6 (the command-registry check) exists at all.
- **CLAUDE.md's `代码位置对照` table** exists because `DESIGN.md`/`HANDOFF.md` still
  reference Go files deleted in the Swift migration. Drift so entrenched it earned a
  *compatibility table* instead of a fix.

`check-design.sh` §6 says the quiet part out loud: *"Spec↔behavior + 'is this command
worth documenting' stay REVIEW-GATE items."* So the one class of drift that IS mechanical
(does the command exist?) is gated, and everything else — what the command DOES, what a
format looks like, what the members of an enumeration are — rests on a reviewer noticing.
Over four incidents, the reviewer didn't.

This is not a call for more discipline. A doc that shows a format the code cannot produce
is a **testable falsehood**, and we should test it.

## What Changes

Two phases. Phase A catches every incident above and is cheap; Phase B removes the class
of bug rather than detecting it, and is worth doing only if A proves the machinery.

### Phase A — test the checkable subset

1. **Rendered examples must be rendered.** A doc example of a code-produced format is
   wrapped in a marker and compared against the real builder's output:

   ```markdown
   <!-- gtmux:rendered wake-done -->
   » gtmux·done  web:2.0 (%11) │ 3m │ goal:"fix the login bug" │ tail:"tests pass" · #a3f1c2
   <!-- /gtmux:rendered -->
   ```

   A Go test renders `wake-done` from a small registry and fails on any difference,
   printing both. `make docs-fix` rewrites the regions from the code. (#478's examples were
   verified exactly this way — by hand, with a probe I then deleted. This makes it CI's
   job.)

2. **Enumerations must be complete.** Every wake class constant (`hqwake.Class*`) MUST
   appear in `docs/cli.md`'s class table and in the seeded playbook — the two places a
   human and HQ respectively learn the vocabulary. A new class that reaches neither is the
   #478 failure exactly. An explicit allowlist covers a class deliberately not surfaced,
   mirroring §6's `HIDDEN`.

3. **Retired vocabulary must stay retired.** A denylist of tokens that must not reappear
   in the docs that describe CURRENT behavior. Each entry names the change that retired
   it, so the list is an audit trail rather than a pile of greps.

   Two things the implementation taught: `openspec/changes/**` must be excluded (a
   proposal has to be able to QUOTE what it retires — this one does), and the CLAUDE.md
   `代码位置对照` table legitimately names deleted paths on purpose, so entries carry file
   exemptions. A third lesson went the other way: where a SPEC quoted a retired format to
   narrate history, the spec was wrong to — a spec says what IS; the archaeology belongs
   in a change's design.

### Phase B — generate the factual core (deferred, proposed for judgment)

Make the class table *generated* from a registry where each class's one-line description
lives next to its constant, so prose can only ever be commentary around facts the code
owns. `make docs` regenerates; CI fails if the tree is dirty. Strictly better than
checking, and strictly more invasive: docs gain generated regions a reviewer must not
hand-edit. **Not in this change** — proposed so the phasing is a decision, not an
oversight.

## Explicitly NOT solved

**Prose truth.** "The `done` wake fires for any session that reached idle" is a sentence
about behavior; no grep will ever catch it being wrong. Phase A would NOT have caught
#478's wrong `done` trigger on its own — only the missing classes and the dead format.
That sentence stays a review-gate item, and this change must not be read as making the
review optional. What it does is shrink the surface a reviewer has to hold in their head,
so the judgment they DO make is about meaning rather than about whether a glyph is stale.

**Test fixtures that pin strings instead of intent.** `hq_test.go` asserts playbook
content with `strings.Contains`; when hq-attention-stream inverted what `--severity
important` MEANS, the fixture passed green with a comment that now said the opposite. A
checker cannot fix that — it is a test-design problem, noted here so it isn't lost.

## Capabilities

### New Capabilities

- `docs-conformance`: what the CI docs gate guarantees — rendered examples match their
  builders, enumerations are complete across the surfaces that teach them, and retired
  vocabulary cannot return. Also the honest boundary: prose-to-behavior truth stays a
  review-gate item.

## Impact

- `scripts/check-design.sh` (the enumeration + denylist checks, in its existing
  `note`/`fail=1` style), a new doc-example fixture test (Go), `Makefile`
  (`docs-fix`), CLAUDE.md (the rule + the escape hatch, alongside the existing
  spec⇄code⇄test rule).
- **Product code, because the guard found some.** `internal/hook/usagewatch.go` bypassed
  the wake channel entirely (`tmux.SendText(target, msg, true)` — no draft guard, so a
  warning firing mid-sentence appended itself to the user's message and submitted it);
  `internal/app/watchdog.go` hand-built the retired format, so its escalation could not
  even be read as a class and queued at default priority. Both now build declared classes
  (`usage·warn`, `stuck·waiting`) through `hqwake.Line` + `hqnudge`. Playbook v5 teaches
  them; the class table and three stale specs are corrected.
- The only failure mode the guard itself introduces is a false RED (a doc region
  intentionally illustrative rather than exact) — handled by not marking that region.
