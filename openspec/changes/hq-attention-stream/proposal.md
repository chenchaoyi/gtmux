# Proposal: hq-attention-stream

## Why

The pull side of HQ perception tells the supervisor to read one stream and calls it
complete — while that stream is defined to exclude the single most important thing a
human does.

- `events.Severity` maps `UserPromptSubmit` to `routine`, and the session-events spec
  pins it: *"prompt submissions … SHALL be `routine`"*, with a scenario literally titled
  **"Routine chatter is routine"**.
- The seeded playbook (`hq.go`) says **"`--severity important` filters to the attention
  stream"**, CLAUDE.md says **"`gtmux events --severity important` = the attention
  stream"**, the `gtmux events` help says **"`--severity` filters to that tier and above
  (the attention stream)"**, and the supervisor-agent spec REQUIRES the playbook to
  *"direct the supervisor to triage from the SEVERITY-filtered event stream — `gtmux
  events --severity important` for what needs attention"*.

Put together: **an HQ that follows its own playbook cannot see the user's instructions.**
The user types "先别动那个分支" directly into an agent window; it lands `routine`; HQ pulls
`--severity important`; HQ never learns. Its only trace is the `goal-changed` wake line —
which is exactly the channel hq-wake-reliability (#474) had to make reliable, and a
knock is a knock: it can be coalesced, held behind a draft, or ignored by an HQ that is
mid-turn. Pull is the recovery path, and the recovery path is blind.

The deeper error is a category one. Severity ranks **urgency** — "someone is blocked,
something died, act now". Relevance — "this changes my model of the fleet" — is a
different axis, and we collapsed them into one word ("attention") and then filtered on
the urgency axis while calling the result complete. A user instruction is not urgent (no
one is blocked on HQ reading it), but it is maximally relevant.

## What Changes

Two halves. Neither works alone: raising the tier without fixing the wording just moves
the blind spot, and fixing the wording without the tier leaves `notable` still missing
the instructions.

### 1 · A submitted instruction is `notable`, not `routine` (code)

A `UserPromptSubmit` that carries a real instruction — typed prose OR a slash command —
SHALL be `notable`. Harness-injected content and gtmux's own wake lines echoed back stay
`routine`: they are not acts, they are noise, and the existing prompt classifier
(`transcript.ClassifyUserPrompt`, hardened in #474) already tells the two apart.

The record grows one additive field, `origin`, carrying `instruction` on such a
submission. **`Severity` is a pure function of the record**, so the verdict has to be ON
the record; and stamping it at the source (the hook, which already computes exactly this
for the `goal-changed` wake) keeps ONE classifier deciding "is this a user act", used by
both the wake and the tier.

**Deliberately NOT distinguishing who typed it.** A task `gtmux spawn` delivered also
fires a `UserPromptSubmit` carrying real prose, and it will land `notable` too. We
investigated telling them apart and rejected it:

- the send side records only a **payload hash** (`dispatch.RecentSend`), and the harness
  appends `<system-reminder>` blocks to the prompt it reports — so the hash misses and
  the verdict would be wrong exactly when it matters;
- a **timing** heuristic ("gtmux typed into this pane 10s ago") fails in the dangerous
  direction: a user who types right after a dispatch would be classified as machine
  traffic and vanish from the stream again — reopening this very bug;
- and a dispatch landing IS a fleet event: it is what dispatch verification wants to
  see, and HQ dedups it against its own ledger (its playbook already presumes off-ledger
  work is user-direct and says to verify, not correct).

So `notable` = "an instruction reached a session", author-agnostic. Honest, cheap, and
it cannot hide a human.

### 2 · Name the three streams for what they are (prompt + docs + spec)

There is no single "attention stream". There are three reads, and the playbook will say
so:

| read | what it is | when |
|---|---|---|
| `--since-seq <n>` (no filter) | the **delta** — everything since your cursor | after a wake; the reconcile-after-doubt path |
| `--severity notable` | the **fleet-change stream** — instructions, turn-ends, lifecycle | catching up after being away; a periodic sweep |
| `--severity important` | the **escalation stream** — blocked, asking, crashed | triage first; it is a subset, never the whole picture |

`important` stops being described as "the attention stream" anywhere: playbook, CLAUDE.md,
`gtmux events --help`, the `events.go` contract comment, and the two specs that pin the
claim. The playbook gains one rule: **a filtered read is a triage shortcut, never your
model of the world — reconcile with `--since-seq` (or `digest`).**

This is a change to HQ's behavioral charter → `hqPlaybookVersion` 3 → 4.

## Capabilities

### Modified Capabilities

- `session-events`: the `origin` field on prompt submissions; an instruction submission
  is `notable`; `--severity` documented as tier-and-above over three named streams
  rather than "the attention stream".
- `supervisor-agent`: the playbook teaches the three reads and the
  filtered-read-is-not-your-model rule (playbook v4), replacing the requirement that it
  triage from `--severity important` as though that were complete.

## Impact

- `internal/events` (the `origin` field + the severity rule), `internal/hook`
  (stamp `origin` from the classifier it already runs for the wake), `internal/app`
  (`hq.go` playbook v4 + version bump; `eventscmd.go` help/comment wording).
- Docs: CLAUDE.md (the HQ section's severity sentence), `docs/cli.md` if it repeats the
  claim.
- Tests: `events` severity table, `hook` origin stamping, `app` playbook fixture
  (`hq_test.go` asserts `--severity important` verbatim today).
- No HTTP surface change; `agents --json` / `digest --json` untouched. The `origin` field
  is additive — a legacy record without it reads as no-instruction, i.e. exactly today's
  behavior.
- Existing homes get playbook v4 on the next `gtmux hq` after `gtmux update`.

## Non-goals

- **Re-tiering anything else.** `Stop/report` stays notable, `Waiting`/`asking`/crash stay
  important. Only the prompt-submission rule changes.
- **Suppressing the `goal-changed` wake for gtmux-dispatched prompts.** Adjacent (same
  root: "we can't tell who typed it"), but it would REMOVE a wake HQ gets today, and the
  detection is exactly the unreliable thing rejected above. Left alone deliberately.
- **A `quiet`-style user-facing knob for the tiers.** `gtmux quiet` already governs what
  HQ PRINTS; this change is about what HQ can SEE.
