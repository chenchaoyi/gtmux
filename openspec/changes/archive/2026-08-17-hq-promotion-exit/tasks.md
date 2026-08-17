# Tasks — hq-promotion-exit

Status: IMPLEMENTED (2026-08-17, one pass; every slice's tests landed with it). The risky direction is a queue that silently never drains
(charter-flags rot in a new costume); every slice pairs a "pending surfaces" test
with a "landed clears" test.

## 1. The lifecycle in the ledger

- [x] 1.1 Ops `promote` (carries `why`, optional `target`) and `land` (carries
      `ref`) on a LIVE entry; the fold attaches the state to the live entry
      (promotedAt/why/target, landedAt/ref); a later `promote` re-opens a landed
      entry (the lesson evolved). Write-path refusals: promote a dead or
      already-pending entry; land a dead or never-promoted entry. Bounds: `why`
      reuses the 300 B budget; `target`/`ref` ≤ 200 B, loud.
- [x] 1.2 Tests: the full lifecycle fold (promote → pending; land → closed;
      re-promote → pending again); each refusal; supersede does NOT inherit
      promotion state (content changed — re-judge it); bounds.

## 2. The brief (the artifact)

- [x] 2.1 Deterministic render of one pending promotion under
      `knowledge/promotions/<topic>-<slug>.md`: marker header, the lesson
      (title/body), the why and target, the entry's full provenance footer, and
      the closing instruction (`gtmux knowledge land <id> --ref …`). Written on
      promote, REMOVED on land, regenerated with every mutation's render pass.
- [x] 2.2 Topic renders badge the state in the provenance footer: `⚑ promoted
      (pending)` / `→ landed <ref>`.
- [x] 2.3 Tests: brief appears on promote with the provenance intact; disappears
      on land; regenerates after an unrelated mutation; the badge tracks the
      state through the lifecycle.

## 3. The verbs + the queue view

- [x] 3.1 `gtmux knowledge promote <id> --why … [--target …]` and
      `gtmux knowledge land <id> --ref …` — mutations through the shared
      HQ-home gate + commit path (ledger, renders, `gtmux:audit:knowledge`).
- [x] 3.2 `gtmux knowledge promotions` — read-only, open to anyone: the pending
      queue with each entry's age and brief path, HEADED by the count and the
      oldest age (the anti-rot instrument charter-flags.md never had); `--json`.
- [x] 3.3 Tests: verb chain through `CmdKnowledge` incl. the role gate both ways;
      audit records for both ops; the list's header and pending set.

## 4. Rot surfaces in doctor

- [x] 4.1 `hq.PromotionsStatus(now)`: pending count + oldest age; OK when empty
      or young, flagged past the staleness floor (14 d). A `knowledge promotions`
      row joins doctor's HQ maintenance section (quiet-empty, aged-flags shape of
      the existing rows).
- [x] 4.2 Tests: empty → OK; young pending → OK with figures; stale pending →
      flagged; landed → back to OK.

## 5. Teaching + docs + gates (same PR)

- [x] 5.1 Playbook v25: the correction loop's "FLAG it" becomes `promote` with the
      full lifecycle incl. `land`; the distill ritual's checklist gains the
      promotions queue; teach migrating the charter-flags.md backlog through the
      verbs (judged entry-by-entry). `hqPlaybookVersion` 24 → 25.
- [x] 5.2 `docs/cli.md`'s knowledge section gains promote/land/promotions and the
      brief; CLAUDE.md's correction-loop clause names the mechanism.
- [x] 5.3 Spec deltas: hq-knowledge ADDED (the lifecycle + the doctor row);
      supervisor-agent MODIFIED (correction loop; the distill ritual's
      charter-level sentence).
- [x] 5.4 Gates green: `make check`, `CGO_ENABLED=0 go build ./cmd/gtmux`,
      `scripts/check-design.sh`, `openspec validate --strict`.
