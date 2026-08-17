# hq-knowledge (delta)

## MODIFIED Requirements

### Requirement: A charter-level entry is promoted into an export brief, and the loop closes on landing

A LIVE entry SHALL be promotable as charter-level through
`gtmux knowledge promote <id> --why "…" [--target "…"]` — a ledger operation (the
required `why` states the promotion case; the optional `target` names where the
lesson should land) that writes a PROMOTION BRIEF: a deterministic, gtmux-owned
render under `knowledge/promotions/` carrying the lesson, the why, the target,
the entry's full provenance, and the closing instruction. Charter-level means
the lesson belongs in a DURABLE RULE CARRIER beyond this machine's knowledge
base — a project's `AGENTS.md`/`CLAUDE.md`, a team runbook or wiki, `LOCAL.md`
(when the rule governs this supervisor itself), or gtmux's own repo (an openspec
change or seed edit for a developer, or a GitHub issue carrying the brief for
anyone else). The brief is the EVIDENCE PACKAGE a human — or a worker they
dispatch — carries to that carrier; gtmux SHALL NOT write into any repo or
external system itself, and nothing here dispatches work on its own. The brief's
closing instruction SHALL name the promotion's `target` when one was given, and
otherwise present the carrier options neutrally — never a single hardcoded
destination.

`gtmux knowledge land <id> --ref "…"` SHALL close the loop when the lesson lands
— the reference is whatever names the landing (a PR, an issue URL, a runbook
name, a file path) — removing the brief while the ledger keeps the whole
lifecycle; a later `promote` MAY re-open a landed entry (the lesson evolved). The
write path SHALL refuse promoting a dead or already-pending entry and landing a
dead or never-promoted entry. A SUPERSEDE does not inherit promotion state — the
content changed, so the successor is re-judged. Both operations are mutations
under the knowledge role gate and journal through `gtmux:audit:knowledge`.

`gtmux knowledge promotions` SHALL list the pending queue — read-only, open to
anyone — headed by the count and the OLDEST pending age, and topic renders SHALL
badge a promoted-pending entry and a landed one in the provenance footer, so the
state is visible where the lesson lives.

#### Scenario: A promotion produces a carryable brief

- **WHEN** HQ promotes a live entry with a why and a target
- **THEN** one brief appears under `knowledge/promotions/` carrying the lesson, the
  why, the target, the entry's provenance (seqs/capture/task/pane/date), and a
  closing instruction that names THAT target — and one `gtmux:audit:knowledge`
  record is journaled

#### Scenario: A target-less brief offers the carriers, not a repo mandate

- **WHEN** a promotion carries no `--target`
- **THEN** the brief's closing instruction lists the carrier options (a project
  AGENTS.md/CLAUDE.md, a team runbook, LOCAL.md, gtmux's repo or an issue) rather
  than directing every user into gtmux's source tree

#### Scenario: Landing closes the loop

- **WHEN** the lesson lands in its carrier and HQ runs `land <id> --ref "…"` — the
  ref being a PR, an issue URL, or a runbook name alike
- **THEN** the brief is removed, the entry's render badge flips to landed with the
  reference, the pending queue no longer names it, and the ledger still holds both
  operations

#### Scenario: The lifecycle refuses nonsense states

- **WHEN** a promote targets an already-pending entry, or a land targets a
  never-promoted one
- **THEN** the operation is refused loudly and nothing is appended

#### Scenario: A successor is re-judged

- **WHEN** a promoted entry is superseded
- **THEN** the successor carries no promotion state — promoting it again is a fresh
  judgment
