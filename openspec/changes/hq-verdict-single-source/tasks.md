# Tasks — hq-verdict-single-source

Status: IMPLEMENTED (2026-08-11). The risky direction is a surface that renders a verdict it did not
compute, so every slice keeps a working fallback for an absent `hq` object.

## 1. Core: decide once

- [x] 1.1 Add the six-state resolver + headline FACTS to `internal/radar` (pure,
      priority-ordered: hq_call > needs_you > resource > working > normal; absent when no
      supervisor row). Reuse the resource tier the radar already samples.
- [x] 1.2 Emit it as an OPTIONAL `hq` object on `digest --json` / `GET /api/digest`.
- [x] 1.3 Tests: each state's priority, including the two the phone was missing
      (hq_call outranks a quiet fleet; a red resource tier is never `normal`).

## 2. Contract + surfaces

- [x] 2.1 `api/contract.md`: document the additive `hq` object and its optionality.
- [x] 2.2 macapp: **deliberately still resolves locally, and the reason is a cost.** The
      menu bar polls `agents --json` on a FAST tick; the verdict rides the DIGEST, and
      sampling the machine on every fast tick is exactly what its separate slow resource
      timer exists to avoid. Its resolver was already complete (all five states, same
      order) — the phone's was the incomplete one. Instead of a fetch it cannot afford,
      it gains `testHQStateMatchesCoreVerdictOrdering`: the same eight cases as the Go
      test, in the same order, so the two cannot drift in silence again. That silent
      drift is the whole defect this change is about.
- [x] 2.3 mobile: `assessment()` consumes `hq` when present, keeps today's logic as the
      fallback, and GAINS the two missing branches either way.
- [x] 2.4 Tests both sides: same fleet ⇒ same verdict; absent `hq` ⇒ the old local answer.

## 3. Consistency

- [x] 3.1 DESIGN §12 / MOBILE §17: record that the verdict is core-decided and only the
      wording is local; keep the deliberate FORM difference (card vs disc) documented as
      deliberate, so a future reader does not "fix" it.
- [ ] 3.2 Release note: the phone will now speak on HQ-call and resource states where it
      used to say "all normal". (Rides the next mobile build.)
- [ ] 3.3 Archive — after 3.2.
