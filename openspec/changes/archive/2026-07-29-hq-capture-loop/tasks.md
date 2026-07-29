# Tasks — hq-capture-loop

Staged per the commander's 2026-07-19 call: **do NOT build all three layers at once.**
Ship ① + ② + the dispatch-KB echo, run distillation manually/periodically for a while and
**observe**; add ③'s auto-triggers only if capture still slips. Each group is a pure
increment landing behind green CI before the next.

- **PR1 · group A** — playbook v8 (protocol ①, consult hard-precondition, board/KB weld). Zero engineering. **Ships today.**
- **PR2 · group B** — `gtmux capture` PUBLIC command + spool (dedup key + topic tag) + manual/periodic distill drain.
- **PR3 · group C** — dispatch-time KB echo (first-class, right after ②).
- **PR4 · group D** — ③ distill auto-triggers (density/correction/spool). **DEFERRED behind the observation gate.**

## PR1 · group A — playbook v8 (protocol ①, consult, board/KB weld) — ships today

Pure playbook / documentation. No behavior code.

- [x] A1. Bump `hqPlaybookVersion` 7 → 8 in `internal/hq/hq.go`, and add a `v8 —
      hq-capture-loop: …` entry to the version-history comment block.
- [x] A2. In `hqInstructions`, add the **capture-verify** to the loop: teach the turn
      shape `SENSE → JUDGE → CAPTURE? → REPORT`, and mandate a capture verdict ONLY on
      `correction` / `crash` / `recurrence` (any footgun/fact hit a second time) — either
      `⟣ 📓 captured: <topic-file>` or an explicit "nothing durable" clause (with the
      reusable ∧ cross-cutting ∧ not-conversation-unique criterion). Teach `done` /
      `resolved` as OPPORTUNISTIC capture with a SILENT default (capture + mark only if a
      real reusable fact surfaced; never a forced verdict — forcing there breeds ritual
      filler). (en+zh.)
- [x] A3. Add the `⟣ 📓` (captured) glyph to the **signal-register** section of
      `hqInstructions`, alongside `✅ ▪ ◈ ⚠`; state it is emitted ONLY on a real capture.
- [x] A4. Harden **consult** into a HARD precondition in `hqInstructions`: consult the
      relevant KB topic BEFORE advising or dispatching; name the KB entry advice rests
      on; a no-KB-coverage gap is itself a capture trigger. (en+zh.)
- [x] A5. **Weld the board/KB definitions** into `hqInstructions`: board = ephemeral
      private posture (gtmux never reads back); KB = durable cross-session machine
      memory; the capture-verify routes ONLY to the KB; "I noted the board" is never
      capture; both may be written but neither substitutes. (en+zh.)
- [x] A6. Update the seeded `knowledge/README.md` scaffold text if the capture/consult
      wording there needs to match the new charter (seed-if-absent; no forced rewrite of
      curated content).
- [x] A7. Spec: land the `supervisor-agent` deltas for the capture-verify requirement
      (scoped to correction/crash/recurrence), the signal-register glyph MODIFY, the
      consult hard-precondition, and the board/KB weld (see `specs/supervisor-agent/spec.md`).
- [x] A8. Tests: extend `internal/hq/hq_test.go` — assert the fresh AGENTS.md contains
      the `⟣ 📓` glyph, the correction/crash/recurrence forced-capture wording, the
      opportunistic-silent done/resolved wording, the consult-precondition wording, and
      the board/KB weld; assert `parsePlaybookVersion` == 8 and the v7→v8 upgrade backs
      up `AGENTS.md.bak-v7`.
- [x] A9. `make check` + `scripts/check-design.sh` green (playbook/version consistency,
      `openspec validate --specs --strict`).

## PR2 · group B — `gtmux capture` PUBLIC command + spool (dedup key + topic tag)

- [x] B1. Add the `gtmux capture "<lesson> @<topic>"` command + `--list`: parse the
      `@topic` (accounts|workflows|best-practices|pitfalls|corrections), compute a dedup
      key (topic + lesson-slug, or an explicit key), auto-collect event context
      (current/related `pane_id`, event `seq`, `task_id`, timestamp), append one JSON line
      to the spool.
- [x] B2. Spool file: `~/.config/gtmux/hq/knowledge/.pending-distill.jsonl` (or the
      `state.HQHome()`/`state.Dir()` equivalent) — append-only; JSON line carries
      `topic` + `key` (dedup) + lesson + context; `--list` renders the queue.
- [x] B3. Register `capture` in the `internal/app/app.go` command dispatch (registry stays in app).
- [x] B4. CLI-surface docs (drift rule) — `capture` is a **PUBLIC** command: add it to the
      CLAUDE.md command list, add a `## gtmux capture` section to `docs/cli.md`, and add it
      to `gtmux --help` (`internal/app/help.go`, en+zh). Do NOT hide it in the
      `check-design.sh` HIDDEN allowlist.
- [x] B5. Distill pass drains the spool MERGING by (topic, dedup key): fold each candidate
      into the matching KB entry / earlier same-key candidate instead of appending a
      near-duplicate, then truncate the spool. (Runs on the EXISTING manual/periodic
      distill trigger — no auto-trigger yet; that is PR4.) Update the playbook `Iterate`
      wording to name the spool + merge-by-key as a data source — but land any
      `hqInstructions` text edit in PR1's v8 if possible to avoid a second version bump.
- [x] B6. Spec: land the `supervisor-agent` delta for the `gtmux capture` command + spool
      (public command, dedup key + topic tag, drained by the distill pass).
- [x] B7. Tests: capture parses topic + writes a well-formed spool line with a dedup key;
      a bad/missing `@topic` errors; `--list` reads the queue; a drain merges two same-key
      candidates into one KB entry (no near-duplicate) and truncates the spool.
- [x] B8. `make check` + `check-design.sh` green.

## PR3 · group C — dispatch-time KB echo (first-class; right after ②)

- [x] C1. At `gtmux spawn` / dispatch, auto-echo the matching pitfalls/workflows KB
      summary (by cwd repo name + goal keywords), handed to the worker at launch; no
      match → silent no-op.
- [x] C2. Spec: land the `supervisor-agent` delta for the dispatch-time KB echo.
- [x] C3. Tests: a dispatch into a repo with `pitfalls`/`workflows` entries surfaces the
      matching summary; a no-match dispatch echoes nothing and does not error.
- [x] C4. `make check` + `check-design.sh` green.

## PR4 · group D — ③ distill auto-triggers — DEFERRED behind the observation gate

Build ONLY after ① + ② + C have run and observation shows capture still slips (e.g. spool
depth persistently above N between periodic passes, or another flagged capture miss).

- [ ] D1. Decide + record the concrete "still slipping" trip condition before writing code.
- [ ] D2. In `internal/hq/distill.go`, extend `shouldDistill` / `distillSensor` with the
      new triggers: density (≥ K notable **closures** since the watermark), correction
      (any `correction`-class event in the delta, still obeying `distillMinInterval`),
      spool depth (≥ N spool entries) — keeping the existing rate limit + zero-change gate
      first and the weekly/volume periodic floor as the lower bound.
- [ ] D3. Add `K` (density, default 10, range 10–12) and `N` (spool depth, default 5) as
      **config** (tunable without a release) next to the existing `distill*` values.
- [ ] D4. Spec: land the `supervisor-agent` delta MODIFYING the distill-trigger
      requirement (density/correction/spool-driven + periodic floor, config K/N).
- [ ] D5. Tests: `shouldDistill` fires on each new trigger and is still gated by the rate
      limit + zero-change gate; the correction trigger respects the min interval.
- [ ] D6. `make check` + `check-design.sh` green.

## Cross-cutting

- [ ] Keep `hqInstructions` and the generated AGENTS.md in lockstep; any `hqInstructions`
      edit bumps `hqPlaybookVersion`. Land all playbook text in PR1's v8 where possible so
      PR2–PR4 add no further version bumps (if PR2's `Iterate` note must edit
      `hqInstructions`, bump to v9).
- [ ] Update these checkboxes as work lands; archive the change once the shipped PRs merge
      (`/opsx:sync` + `/opsx:archive`). Note ③/PR4 may intentionally never ship if the
      observation gate is not tripped.
