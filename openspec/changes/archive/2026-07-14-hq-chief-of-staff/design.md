# Design — HQ chief-of-staff

## Context

This builds directly on the shipped charter (`supervisor-agent`, `agent-dispatch`,
`session-events` specs). The relevant existing machinery:

- `events.Record` (`internal/events/events.go`) already carries `event/state/kind/
  summary/class`; `class` is a deterministic `asking|report` on `Stop`. The log is
  append-only + rotated; `events.Append` is the single write path, called fire-and-forget
  from the hook. `gtmux events [--follow] [--json] [--since]` reads it.
- The HQ seed lives entirely in `internal/app/hq.go`: `hqInstructions` (the AGENTS.md
  playbook), `hqKnowledgeSeeds` (the `knowledge/` scaffold), `seedHQHome`/`seedHQKnowledge`
  (write-when-absent, never clobber). `notes/` already exists as HQ's private dir.
- Nudges are draft-guarded and tier-deduped; the `resolved` nudge already retracts a chase
  when a wait clears WHILE HQ is watching.

## Goals / Non-Goals

- **Goal:** HQ holds a durable posture across context resets; escalates by graded severity;
  reconciles before relaying; learns from corrections. All deterministic, cgo-free, additive.
- **Non-Goal:** no new push surface, no machine-parsed board, no auto-edited charter, no LLM
  in the hot path. Nothing changes the existing marker/notify state machines or nudge dedup.

## Key decisions (forks resolved)

### D1 — Severity is a 3-tier deterministic classifier over lifecycle records; `critical` is a runtime policy tier, not an event class

The event severity classifier emits `routine | notable | important` from fields the
record ALREADY holds — no usage/limits lookup, no LLM, no new hook coupling:

| condition | severity |
|---|---|
| `Waiting` with a kind (needs the user) | `important` |
| `Stop` with `class == "asking"` (reply-text question) | `important` |
| `Stop` with `class == "report"` (a completed turn) | `notable` |
| `SessionStart` / `SessionEnd` / `Resumed` / `PreCompact` | `notable` |
| `UserPromptSubmit`, `Notification`, working ticks, anything else | `routine` |

**Why not a `critical` event class?** "Critical" (quota near-exhaustion, production issue,
one agent blocking others) is a *runtime judgment* the event stream cannot make
deterministically at write time — quota lives in `gtmux limits`/`usage`, "production" and
"blocking-others" are semantic. Forcing a fragile `critical` into every lifecycle record
would be wrong-by-construction. So `critical` is defined by the **escalation policy** (§3):
HQ layers the criticality test over `important`-tier events using the tools it already has.
This keeps the code classifier honest and the escalation model expressive.

Severity is stamped in `events.Append` (source), so it is persisted and queryable without
recompute, and every writer gets it uniformly. A record that already carries a severity
(future-proofing) is left as-is.

`gtmux events --severity <level>` filters to that level and above (rank
`routine=0 < notable=1 < important=2`), reusing the existing `Read`/`Follow` plumbing —
the filter is applied in the CLI printer, so both the bare and `--follow` forms honor it.

### D2 — The situation board is HQ-curated markdown seeded once, not machine state

The board is `notes/board.md`, seeded write-when-absent exactly like the KB scaffold, and
maintained by HQ (an LLM) as prose/table — NOT a gtmux-parsed JSON. Rationale: it is HQ's
*synthesis*, not a derivable projection; a code-owned schema would fight HQ's judgment and
duplicate `gtmux tasks`/`digest` (which ARE the machine truth). The board persists HQ's
cross-turn posture; the digest/tasks/events remain the deterministic source of record. The
seed template gives the shape; HQ fills and prunes it.

`notes/` is HQ's existing private area, so the board sits beside its other notes and is
never clobbered on re-seed.

### D3 — Decision tiers and escalation are seed policy, enforced by review, not by a lock

Items 2 and 3's behavior lands in the seed (agent-neutral, single-source) as the DEFAULT
policy the user may edit — consistent with how the whole charter is encoded. gtmux does not
*enforce* the autonomy matrix (it never has, for the role whitelist either); the value is a
portable, explicit default that works out-of-box on any workstation. Tests pin that the seed
CONTAINS the policy (spec⇄code consistency), the same pattern `TestHQPlaybookCharter` uses.

### D4 — critical→push rides the existing pipeline; no new HQ push command

The notification/push pipeline (`internal/server` → relay → APNs) already pushes
attention-worthy events (waiting/…) to the phone automatically. Rather than invent an
HQ-invoked push surface (new auth/dedup/relay path, real risk), the escalation policy defines
`critical` as the tier the existing pipeline already surfaces, and HQ's job is to
**reconcile** (not re-push) so the phone alert isn't stale. A dedicated `gtmux notify`/push
command is a possible future follow-up, out of scope here.

### D5 — Reconcile-before-relay is a policy step, backed by the already-deterministic digest

The reconcile is: `gtmux digest --json` (or `gtmux tasks --json`) is the live truth; before
HQ relays/escalates a needs-you it re-reads that pane's row and drops the item if the state
moved. This needs no new code — the digest is already deterministic and cheap — only a seed
policy line and a spec requirement. It complements, not replaces, the `resolved` nudge
retraction (which only fires when HQ is watching at the transition edge).

## Risks / trade-offs

- **Severity granularity.** Three tiers is intentionally coarse — enough to separate
  "needs attention" from "chatter" without a taxonomy HQ must reason about. If finer tiers
  prove needed, the field is additive and can grow values without breaking readers.
- **Seed only affects fresh homes.** Existing HQ homes are grandfathered (never clobbered).
  The productization target is out-of-box on a NEW workstation; an existing user re-seeds by
  removing their policy file, as today.
- **Board staleness.** A curated board can drift from reality; the seed mitigates by making
  the deterministic digest/tasks the source of truth and the board a synthesis HQ refreshes,
  never a thing other code trusts.

## Migration

Fully additive. `severity` is an optional field (absent on old records; readers default to
treating an absent value as `routine` for filtering). New seed files are write-when-absent.
No state-path, contract, or nudge-dedup change.
