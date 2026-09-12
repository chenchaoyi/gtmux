# Tasks — hq-knowledge-engine

## Phase 1 — pure move (no behaviour change)
- [x] 1.1 Create `internal/knowledge`; move ledger/render/api/pool/promote code out of `internal/hq`; hq keeps sensors, playbook, verb shims
- [x] 1.2 Golden test: topic renders, JSON index, `capture --list`, `knowledge list/show` byte-identical before/after
- [x] 1.3 `check-design.sh`: import rule `knowledge` is a leaf; `mine` does not import it
- [ ] 1.4 Archive-style note in `code-architecture` spec

## Phase 2 — the three axes
- [x] 2.1 Ledger v2 fields: kind, tags, provenance{kind,count,last}, audience, status; read-time migration table; backup before first write
- [x] 2.2 Kind vocabulary + validation; `knowledge kind <id> <kind>`
- [x] 2.3 Provenance counting: miner recurrence and correction lexicon bump a live entry's count
- [x] 2.4 `hypothesis` status: own section in render, excluded from distribution
- [x] 2.5 Render: canonical `machine.md`; topic files keep rendering per kind + tag

## Phase 3 — audience and distribution
- [x] 3.1 `promote --for <hq|machine|repo <path>|everyone>`; brief carries the ready-to-paste block
- [x] 3.2 `withdraw <id> --why`
- [x] 3.3 Managed block installer (sentinels + hash) for Claude Code / Codex / opencode / Kimi global instruction files; paths from the agent registry, verified per agent's docs
- [x] 3.4 `knowledge sync` (refresh all carriers); `land` for `hq`/`machine`/`repo` writes then lands
- [x] 3.5 doctor row "knowledge sync": per agent missing / stale / hand-edited; `--fix` refreshes
- [x] 3.6 `everyone`: issue URL prefill + copy; exempt from overdue floor

## Phase 4 — lint and neighbours
- [x] 4.1 `knowledge lint` (orphan, broken link, near-duplicate, stale, assumed-kind); wired into self-check
- [x] 4.2 `knowledge neighbours`; `capture --list` grouped; `add` shows closest three
- [x] 4.3 ACE constraints as tests: no wholesale rewrite; supersede keeps old text

## Phase 5 — surfaces
- [x] 5.1 API: index/entry carry kind/provenance/audience/status/neighbours; `act` gains `carry`, `withdraw`
- [x] 5.2 Menu-bar window: axes on the entry, "write it in" / "feedback to gtmux", grouped spool
- [x] 5.3 Phone: same, per MOBILE.md
- [x] 5.4 Demo data for both screens

## Phase 6 — docs and playbook
- [ ] 6.1 Rewrite `docs/design/knowledge-layers.md` around the four audiences and three axes
- [ ] 6.2 `docs/cli.md` + `docs/cli.zh.md`: new verbs, sync, lint
- [ ] 6.3 Playbook bump: promote asks "who must know", the four words, withdraw, hypothesis, lint in self-check
- [ ] 6.4 Run on this machine: migrate the live ledger, distribute, doctor green; report counts
