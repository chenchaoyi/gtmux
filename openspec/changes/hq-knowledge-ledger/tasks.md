# Tasks — hq-knowledge-ledger

Status: IMPLEMENTED (2026-08-17, one pass; every slice's tests landed with it). The risky direction is losing knowledge (content or consult
coverage); every slice lands its loss-shaped test with the mechanism.

## 1. The ledger (substrate)

- [x] 1.1 `internal/hq/knowledge.go`: the op record (`v`, `op`, `id`, `topic`,
      `title`, `body?`, `at`, `sources{seq, seqRange?, pane?, task?, capture?}`,
      `supersedes?`, `why?`), append (O_APPEND one-line JSON, mirroring the
      capture spool), read (malformed lines skipped), and the LIVE-set fold
      (add → live; supersede → predecessor dead, successor live; retire → dead).
- [x] 1.2 Bounds refuse LOUD: title single-line ≤ 200 B, body ≤ 8 KiB, why
      ≤ 300 B — an over-budget value is an error naming the limit, never a
      truncation. Ids are `<topic>/<slug(title)>` (the capture slug); a collision
      with a LIVE entry refuses and names the supersede alternative.
- [x] 1.3 Tests: round-trip; fold across add/supersede/retire chains; every
      bound's refusal; collision refusal; malformed-line resilience; `v` stamped.

## 2. Projection + migration (the loss-shaped slice)

- [x] 2.1 Deterministic render of a topic's live entries: gtmux-owned marker
      header, entries in ledger order — a short body flattens into the entry's
      bullet line, a long one indents under it — each with a provenance footer
      (id · date · seq/range · capture/task/legacy source).
- [x] 2.2 Migration on first touch: an existing topic file that differs from its
      seeded placeholder moves VERBATIM to `knowledge/legacy/<topic>.md` and the
      projection links to it; a file equal to its placeholder is replaced
      outright. Tests: byte-identical legacy survival; placeholder replacement;
      idempotence (a second mutation does not re-migrate).
- [x] 2.3 `MatchKnowledge` consults projections AND `legacy/` — tests: a rendered
      entry's bullet matches by repo/keyword; a legacy bullet still matches; the
      echo cap holds across both.
- [x] 2.4 `render --check` drift detection: a hand-edited projection is reported
      (nonzero exit naming the file), a clean one passes silently. Render itself
      is idempotent.

## 3. The verbs

- [x] 3.1 `gtmux knowledge add` — flags per proposal; `--capture <key>` consumes
      the pending candidate (removed from the spool atomically) and inherits its
      `seq`/`pane`/`task`; the entry always stamps the current event seq. Appends
      the op, re-renders the topic, journals `gtmux:audit:knowledge`.
- [x] 3.2 `supersede` / `retire` — same append+render+journal path; `retire`
      requires `--why`.
- [x] 3.3 `dismiss --capture <key> --why` — candidate removed WITH a journal
      trace, no ledger op: the quality gate's rejections finally leave evidence.
- [x] 3.4 Role gate: every mutation (and `render`) refuses outside the HQ home
      with the `--ack`-style loud message; `list`/`show` work anywhere. Tests
      mirror `TestEventsAckRefusedOutsideHQHome`.
- [x] 3.5 `list`/`show` (+ `--json`); `events.AuditKnowledge` constructor added to
      the phase-1 vocabulary with its bounds test.

## 4. Teaching + sensors

- [x] 4.1 Playbook v24: the Knowledge-base § re-taught over the verbs (capture
      verdict format and the consult precondition unchanged; the drain becomes
      per-candidate `add --capture` / `dismiss --capture`; supersede replaces
      "update-over-append" prose); role whitelist clause (a) gains `knowledge`;
      `hqPlaybookVersion` 23 → 24.
- [x] 4.2 The distill wake hint (distill.go) names the verbs instead of the
      hand-merge ritual.
- [x] 4.3 Seeded `knowledge/README.md` describes the ledger, the verbs, and the
      legacy directory.

## 5. Docs + gates (same PR)

- [x] 5.1 CLAUDE.md command list + `docs/cli.md` `## gtmux knowledge` section +
      `gtmux --help` (en+zh) — the command-drift rule for a public command.
- [x] 5.2 Spec deltas: new `hq-knowledge` capability; supervisor-agent's distill
      ritual / capture step / capture spool requirements modified;
      session-events' audit enumeration gains the knowledge kind.
- [x] 5.3 Gates green: `make check`, `CGO_ENABLED=0 go build ./cmd/gtmux`,
      `scripts/check-design.sh`, `openspec validate --strict`.
