# supervisor-agent Specification

## Purpose
TBD - created by archiving change supervisor-mvp. Update Purpose after archive.
## Requirements
### Requirement: Launchable supervisor session

The system SHALL provide `gtmux hq` (中控): it creates — or focuses, when one
already runs — a dedicated tmux session running the user's coding agent (Claude
by default, per existing agent profiles) with its working directory set to the
persistent supervisor home (`~/.config/gtmux/hq/`). The home SHALL be seeded
SINGLE-SOURCE and IDEMPOTENT: AGENTS.md is the canonical FULL playbook (the
cross-agent convention Codex/Cursor/Amp read natively) and CLAUDE.md is a one-line
`@AGENTS.md` import so Claude reads the SAME content with no two-doc drift
(`--agent`/`GTMUX_HQ_AGENT` pick which agent runs). A home that already holds a
policy file SHALL be treated as already-seeded: the system SHALL NOT add a SECOND
full policy doc and SHALL NOT overwrite any policy file. In particular a legacy full
CLAUDE.md SHALL remain authoritative and SHALL NOT get a zombie AGENTS.md dropped
beside it; when only AGENTS.md exists, the cheap CLAUDE.md `@AGENTS.md` import MAY be
added. `gtmux hq` SHALL WARN — rather than silently proceed — when it detects a
redundant layout (a full CLAUDE.md alongside AGENTS.md) or a broken one (a CLAUDE.md
`@AGENTS.md` import while AGENTS.md is missing). The seeded playbook teaches the
supervisor to loop — read `gtmux digest --json`, judge, drill into a pane
(`tmux capture-pane`) only when warranted, drive via `gtmux send`, report to the
user with a token-usage section ALWAYS included in status reports (the per-type
rollup + any `usage_warn` sessions, via `gtmux usage --json`) — and the
supervisor's accumulated knowledge persists across sessions.

#### Scenario: Fresh home seeds single-source

- **WHEN** `gtmux hq` runs and `~/.config/gtmux/hq/` has NO policy file
- **THEN** AGENTS.md (the full playbook) and CLAUDE.md (the `@AGENTS.md` import) are
  generated, and a tmux session starts the agent there

#### Scenario: A legacy full CLAUDE.md gets no zombie AGENTS.md

- **WHEN** `gtmux hq` runs and the home already holds a full CLAUDE.md but no AGENTS.md
- **THEN** the CLAUDE.md is left untouched and NO AGENTS.md is created beside it

#### Scenario: An AGENTS.md-only home gains only the import

- **WHEN** `gtmux hq` runs and the home holds AGENTS.md but no CLAUDE.md
- **THEN** a one-line CLAUDE.md `@AGENTS.md` import is added — never a second full copy

#### Scenario: Relaunch reuses, never clobbers

- **WHEN** `gtmux hq` runs while a supervisor session is already live
- **THEN** it focuses the existing session instead of spawning a second, and every
  existing (possibly user-edited) policy file is left untouched

#### Scenario: A redundant or broken layout warns

- **WHEN** the home has a full CLAUDE.md alongside AGENTS.md, or a CLAUDE.md
  `@AGENTS.md` import while AGENTS.md is missing
- **THEN** `gtmux hq` prints a warning naming the redundant/broken doc and how to
  resolve it, rather than silently proceeding

### Requirement: Supervisor visibility in the radar

A supervisor session SHALL appear in the radar like any agent, additionally
marked with an optional `role:"supervisor"` field in `agents --json` — detected
by its pane cwd being the supervisor home (robust to session renames) — so
surfaces can pin or badge it. The field is additive and absent for normal
agents.

#### Scenario: Supervisor row is marked

- **WHEN** the supervisor session is live and `gtmux agents --json` runs
- **THEN** its row carries `role:"supervisor"`; all other rows are unchanged

### Requirement: Explicit proxy for agent launch

The system SHALL apply an EXPLICITLY-configured proxy when it LAUNCHES a
coding-agent process (the supervisor via `gtmux hq`, and likewise `gtmux adopt` /
restore's resume / `gtmux spawn`), SHALL NEVER probe the network to guess it, and
SHALL hard-code nothing about any particular proxy tool, host, or port — being a
general tool, what proxy (if any) a network needs is the user's to configure.
(The old port-probing `"auto"` is REMOVED: it wrongly proxied a direct-capable
network whose local proxy port happened to be listening.) The choice is resolved in
order, first non-empty wins: the `GTMUX_AGENT_PROXY` env var, then `agentProxy` in
`~/.config/gtmux/config.json`, else none. A value is an HTTP(S) proxy URL to apply,
or `"off"`/empty for no proxy (any non-URL value means none). `gtmux config
agent-proxy <url>|off` sets it; the env var overrides for a per-network switch. A
command that already sets a proxy SHALL NOT be doubled.

#### Scenario: No proxy (the default) launches bare

- **WHEN** no `GTMUX_AGENT_PROXY` env and no `agentProxy` config (or it is `"off"` or
  any non-URL value)
- **THEN** nothing is prefixed — the agent launches with no proxy

#### Scenario: A configured URL is applied verbatim

- **WHEN** `agentProxy` (or `GTMUX_AGENT_PROXY`) is a proxy URL
- **THEN** the launch is prefixed with exactly that URL, with no probing and no
  tool/host/port assumed by gtmux

#### Scenario: Env overrides config for the network switch

- **WHEN** `GTMUX_AGENT_PROXY` is set
- **THEN** it takes precedence over the `agentProxy` config value

### Requirement: The supervisor curates a persistent knowledge base

The supervisor's primary long-term value SHALL be curating a living, cross-cutting
knowledge base under its home (`~/.config/gtmux/hq/knowledge/`). On first run the
system SHALL seed a scaffold — an index README plus topic files (accounts,
workflows, best-practices, pitfalls, environment, and corrections) — each written only when
ABSENT so the supervisor's curated content is never overwritten. The playbook SHALL direct
the supervisor to capture durable, reusable facts once, keep them current, consult
them before advising or driving, and iterate on them — and SHALL forbid storing
secrets (passwords, tokens, keys), recording only IDs, methods, procedures, and
pointers to where a secret lives.

#### Scenario: Knowledge scaffold seeded, never clobbered

- **WHEN** `gtmux hq` first runs (no `knowledge/` yet)
- **THEN** the scaffold (README + topic files, including `corrections.md`) is created; a
  subsequent run adds only missing files and leaves the supervisor's curated content untouched

#### Scenario: No secrets in the knowledge base

- **WHEN** the supervisor records account or service knowledge
- **THEN** its playbook requires IDs/methods/pointers only — never passwords,
  tokens, or private keys

### Requirement: Waiting-event nudge into the supervisor

The system SHALL, when a tmux agent enters waiting and a supervisor session is
live, inject ONE compact line — the location, waiting kind, and title — into the
supervisor's pane (send-keys + Enter), riding the notification pipeline's
existing dedup so an unchanged waiting state is not re-nudged. The SAME channel
SHALL carry usage warnings (`[gtmux] usage·warn <loc> — <detail>`, deduped per
session+layer — see `usage-watch`). It SHALL never nudge the supervisor about
its own waiting states, SHALL be a no-op when no supervisor session is live, and
SHALL be disableable via configuration (`hqNudge: false`, default on).

#### Scenario: Agent blocks, supervisor learns

- **WHEN** another agent enters waiting (permission/plan/question) while an hq
  session is live
- **THEN** one `[gtmux] waiting·<kind> <loc> — <title>` line is typed into the
  hq pane, at most once per waiting transition

#### Scenario: Usage warning reaches the supervisor

- **WHEN** a session breaches (or projects into) a usage layer while HQ is live
- **THEN** one `[gtmux] usage·warn <loc> — <detail>` line is typed into the hq
  pane, at most once per session+layer

#### Scenario: Never about itself, off when absent or disabled

- **WHEN** the supervisor itself is the waiting pane, or no hq session is live,
  or `hqNudge` is false
- **THEN** nothing is injected

### Requirement: Human-in-the-loop boundary (P1)

Beyond the nudge (inform-only), the supervisor MUST NOT be granted automatic
behaviors by gtmux in P1: gtmux SHALL NOT let it auto-answer other agents'
permission prompts on the user's behalf, and ships no orchestration (worktree
spawning, cross-model dispatch). What the supervisor DOES upon a nudge is
governed by its editable instructions file, whose generated default is assess +
report — driving stays a conversational act.

#### Scenario: Nudge informs, does not answer

- **WHEN** a nudge lands for another agent's permission prompt
- **THEN** gtmux itself sends nothing to the WAITING pane; any follow-up action
  is the supervisor's turn under its instructions

### Requirement: HQ role boundary — sense/decide/dispatch/supervise/report only

The generated HQ playbook SHALL encode a HARD role whitelist: the supervisor runs
NO concrete command itself and does NO engineering work. Its ONLY permitted actions
are (a) the `gtmux` toolbox (`digest`/`usage`/`limits`/`resource`/`tasks`/`events`/
`spawn`/`send`/`reap`/`focus`), (b) read-only `tmux capture-pane`, and (c) reading
and writing its OWN notes under `~/.config/gtmux/hq/`. EVERYTHING else — including
READ-ONLY queries (`gh pr view`, running a code CLI to inspect a repo, `git log`,
listing a project) as well as builds, git/worktree/process/install operations — HQ
MUST NOT run; it finds the most suitable live agent, or `gtmux spawn`s one, and
delegates. Its verbs are: sense · decide · dispatch · supervise · report. There is
no "read-only so it's fine" exemption — even a harmless read pulls HQ into the work
and muddies attribution. Being a generated-once seed the user may edit, this is the
DEFAULT policy, not an enforced lock.

#### Scenario: The playbook forbids HQ running any concrete command

- **WHEN** the HQ home is seeded
- **THEN** the playbook states HQ's whitelist is the gtmux toolbox + read-only
  `tmux capture-pane` + its own notes, and that everything else — including
  read-only `gh`/code-CLI/`git` queries — is delegated to a spawned agent

#### Scenario: A read-only investigation is delegated, not run

- **WHEN** HQ needs to inspect a repo or a PR to answer the user
- **THEN** the playbook has HQ dispatch that read to an agent rather than run
  `gh`/`git`/a code CLI itself

### Requirement: HQ dispatches through the verified path, never a raw launch

The HQ playbook SHALL direct the supervisor to dispatch new work through
`gtmux spawn` (which applies the CONFIGURED proxy by construction and verifies
delivery), never a hand-rolled `send-keys` launch that would bypass the configured
proxy and 403 on a proxy-needing network. The `environment.md` knowledge seed SHALL
state that the configured proxy covers ONLY gtmux's own launch path, and that the
choice is explicit (`gtmux config agent-proxy` / `GTMUX_AGENT_PROXY`).

#### Scenario: Playbook points dispatch at `gtmux spawn`

- **WHEN** the HQ playbook and knowledge seeds are generated
- **THEN** they instruct dispatch via `gtmux spawn` and note that a bare
  `send-keys` launch is un-proxied and will 403

### Requirement: HQ never sends navigation keys into an agent TUI

The HQ playbook SHALL prohibit sending navigation keys (arrows, Tab, Page,
mode-switch keys) into an agent's TUI. A form or screen HQ cannot read SHALL be
surfaced to the user via `gtmux focus` rather than blind-driven — HQ does not
guess at multi-screen navigation it cannot see.

#### Scenario: Unreadable form is handed to the user

- **WHEN** an agent presents a multi-screen form or view HQ cannot read
- **THEN** the playbook has HQ `gtmux focus` it for the user, not send nav keys

### Requirement: Dispatch ledger nudges HQ on done/stuck

Every dispatch SHALL be tracked in the ledger (see agent-dispatch). When a tracked
task's pane finishes (idle-after-work) or stalls, the HQ nudge channel SHALL inform
the live supervisor, gated on a live HQ pane and the `hqNudge` setting, deduped so
a state is not re-nudged.

#### Scenario: A finished dispatch nudges HQ

- **WHEN** a tracked task's pane transitions to idle-after-work and an HQ pane is live
- **THEN** HQ receives a `done` nudge for that pane (once)

### Requirement: Waiting-resolved nudge and stale-chase retraction

The HQ nudge channel SHALL fire on BOTH edges of a wait. When a pane LEAVES
`waiting` for `working`/`idle` (e.g. the user answered directly in the pane's own
window, or the agent resumed), the system SHALL type a `resolved` nudge to a live
HQ carrying the pane and a short summary of the original ask, deduped so exactly
one resolve fires per wait (the waiting marker's existence at the transition edge
is the dedup). The corresponding dispatch/needs-you ledger entry SHALL be settled
(auto-cleared) on that transition. The HQ playbook SHALL instruct the supervisor
that, on a `resolved` nudge, it RETRACTS any pending relay or chase about that
pane — the matter was already handled.

#### Scenario: Answering in-pane clears the chase

- **WHEN** a waiting pane leaves `waiting` (its waiting marker existed and is
  cleared) and an HQ pane is live
- **THEN** HQ receives one `resolved` nudge for that pane, the ledger entry is
  settled, and the playbook has HQ drop any pending chase about it

#### Scenario: Resolve fires at most once

- **WHEN** the waiting→working/idle transition is processed and then a later event
  also clears waiting (a no-op, the marker is already gone)
- **THEN** no second `resolved` nudge is fired

### Requirement: HQ triages every turn-end response

The HQ playbook SHALL instruct the supervisor to sense EVERY agent turn-end
response — not only menu/permission waits — by subscribing to the session-events
stream (e.g. `gtmux events --follow`) and reacting to `asks` nudges. It SHALL triage
each response: a reply that asks a question → relay it to the user, obtain the
decision, and backfill the answer to the agent; a reply reporting completion →
acceptance-verify and report to the user; anything else → record without disturbing
the user. This closes the gap where a question embedded in reply text (raising no
menu) left HQ blind.

The playbook SHALL instruct that a relayed question is presented as NON-BLOCKING
text (the question plus HQ's recommendation), NEVER through a blocking interactive
prompt (e.g. `AskUserQuestion`) that stalls HQ's own turn awaiting a reply. On a
dual-channel machine the user's fastest path is often to answer directly in the
source agent's own pane; a blocking relay would then wait indefinitely for a reply
that never arrives through HQ, manufacturing an artificial stall. HQ SHALL instead
sense that the source pane was answered directly via the `resolved`/`goal-changed`
nudge and retract the pending relay.

#### Scenario: A reply-text question is triaged to the user

- **WHEN** an agent's turn-end reply asks a question (no menu raised) and HQ is nudged
- **THEN** the playbook has HQ relay the question to the user, get the decision, and
  backfill the answer to the agent — not leave it unhandled

#### Scenario: A completion is acceptance-reported, progress is not noise

- **WHEN** a turn-end reply reports completion versus mere progress
- **THEN** the playbook has HQ acceptance-verify + report the former, and merely
  record the latter without disturbing the user

#### Scenario: A relayed question never blocks HQ's own turn

- **WHEN** the playbook instructs HQ to relay an agent's question to the user
- **THEN** it directs HQ to post the question and its recommendation as plain
  non-blocking text — never a blocking prompt like `AskUserQuestion` — so HQ's own
  turn is never stalled awaiting a reply that may instead arrive as the user
  answering directly in the source pane

#### Scenario: A direct in-pane answer retracts the relay, not a blocked wait

- **WHEN** HQ has relayed a question and the user instead answers directly in the
  source agent's own pane
- **THEN** a `resolved` (or `goal-changed`) nudge tells HQ the pane moved on, and the
  playbook has HQ retract the pending relay instead of waiting on it

### Requirement: Reclaim is suggest → approve → execute

The HQ playbook SHALL instruct the supervisor that reclaiming a finished dispatch
(its session/worktree/branch) is always suggest → user approves → execute: on a
`reap-suggest`, HQ PROPOSES the reclamation to the user, naming the
session/worktree/branch and the exact `gtmux reap` command, and runs it ONLY after
the user approves. HQ SHALL NOT auto-delete sessions, worktrees, or branches. When the
user DECLINES a suggestion, the playbook SHALL have HQ snooze the candidate
(`gtmux reap --snooze`) and stop re-suggesting it until the snooze lapses — a user's
"keep it" is a decision to remember, not to re-litigate each tick.

#### Scenario: HQ proposes reclaim and waits

- **WHEN** HQ receives a `reap-suggest` for a finished dispatch
- **THEN** the playbook has HQ propose the `gtmux reap` command to the user and run
  it only after approval — never delete automatically

#### Scenario: A declined suggestion is snoozed, not re-nagged

- **WHEN** the user declines a `reap-suggest`
- **THEN** the playbook has HQ `gtmux reap --snooze` the candidate and not re-suggest
  it until the snooze lapses

### Requirement: Nudge injection guards a half-typed HQ draft

The system SHALL NOT clobber or auto-submit a half-typed draft in the HQ pane when
injecting a nudge. Before typing, it SHALL read the HQ input box (reusing the
dispatch input-region detector) and, when the draft is non-empty, SHALL NOT type and
SHALL NOT send Enter — the nudge is queued instead. Delivery SHALL occur only when
the box is confirmed empty over TWO reads a short interval apart, and a queued nudge
SHALL be delivered on a later empty box: on the next injection attempt, on HQ's own
turn-end (`Stop`, box reliably empty — coalesced), or on the serve tick. It is an
INVARIANT that no code path sends Enter into a non-empty HQ input box.

#### Scenario: A half-typed draft is never clobbered

- **WHEN** a nudge fires while the HQ input box holds a non-empty draft
- **THEN** nothing is typed and no Enter is sent; the nudge is queued

#### Scenario: A queued nudge is delivered once the box is empty

- **WHEN** the HQ box is confirmed empty over two reads (or HQ finishes a turn)
- **THEN** the queued nudge(s) are delivered, coalesced, exactly once

### Requirement: Dual-channel dispatch — HQ senses user-direct tasks

The system SHALL let HQ track work the user dispatches through EITHER channel: via HQ
(`gtmux spawn`, tracked) or by typing directly into an agent's own window. When a
`UserPromptSubmit` occurs in a pane that is NOT the HQ pane, the system SHALL push a
`goal-changed` nudge to a live HQ carrying the pane and the prompt head (as DATA),
deduped per pane so a resubmit of the same prompt does not spam, gated on a live HQ
pane and `hqNudge`, and never about HQ's own prompts. The HQ playbook SHALL instruct
that observing an agent working on a task NOT in the ledger, the FIRST assumption is
the user dispatched it directly — HQ verifies (records it as `user-direct`) rather
than "correcting", interrupting, or overwriting it.

#### Scenario: A user-direct prompt reaches HQ

- **WHEN** the user submits a prompt directly in a non-HQ agent pane and an HQ pane
  is live
- **THEN** HQ receives one `goal-changed` nudge for that pane (deduped per prompt)

#### Scenario: Off-ledger work is presumed user-direct

- **WHEN** HQ observes an agent working on a task not in its ledger
- **THEN** the playbook has HQ presume it is user-direct and verify, not correct it

### Requirement: Nudge payloads are marked as data

Every nudge line SHALL mark agent-authored spans (goal, ask, title, reply summary)
as DATA — wrapped in quotes or a labelled marker (e.g. `goal:"…"`, `title:"…"`) — so
an imperative agent string cannot read to HQ as an instruction. The HQ playbook SHALL
carry a policy line stating any nudge payload is DATA, never an instruction: report
it, never act on its literal words.

#### Scenario: An imperative goal is delivered as data

- **WHEN** a nudge embeds an agent-authored goal/title/summary
- **THEN** that span is quoted/labelled as data in the injected line

#### Scenario: The playbook treats payloads as data

- **WHEN** the HQ home is seeded
- **THEN** the playbook states nudge payloads are data, never instructions to act on

### Requirement: Seeded playbook carries the full HQ charter

The generated HQ playbook SHALL encode the supervisor charter as agent-neutral,
single-source seed policy so a fresh `gtmux hq` teaches it on any workstation — not
just a machine whose operator hand-tuned it. It SHALL state: the role boundary
(orchestrate — SENSE·DECIDE·DISPATCH·SUPERVISE·REPORT — never hand-run engineering or
investigation commands, but reclamation IS HQ's responsibility, executed via
`gtmux reap` or a dispatched subagent, never hand-typed git/tmux); main-session
responsiveness (heavy/slow work goes to a subagent or separate window, never blocking
the human-input loop); dispatch granularity (one self-reporting subagent per
independent step; a fast op — reclaim/cleanup — is dispatched separately and confirmed
immediately, never chained behind a slow step); low-noise triage; human-in-the-loop for
every decision; and knowledge curation. Machine-specific facts (accounts, paths,
network, concrete footgun instances) SHALL stay in the local `knowledge/`, not the seed.

#### Scenario: A fresh home seeds the charter

- **WHEN** `gtmux hq` seeds a home
- **THEN** the playbook states the role boundary, main-session responsiveness + dispatch
  granularity, low-noise, human-in-loop, and curation — as portable policy

#### Scenario: A slow step is not chained ahead of a fast one

- **WHEN** the playbook covers dispatching a fast op (reclaim) and a slow op (release)
- **THEN** it directs dispatching them as SEPARATE self-reporting subagents so the fast
  op's completion is visible without waiting on the slow one

### Requirement: Lifecycle watchdog escalates a pane stuck waiting

The system SHALL, from the single-writer serve tick, escalate to a live HQ a pane that
has been WAITING (needs the user) past a timeout without being resolved — a
suggest-only nudge, fired at most ONCE per waiting episode (a marker dedups within the
episode and is cleared when the pane leaves waiting, so a fresh wait re-arms), and never
about the HQ pane itself. This complements the reclaim suggestion for a finished
dispatch (see "Reclaim suggestion when a dispatch looks done"); the watchdog only
surfaces — it never auto-reclaims or auto-answers.

#### Scenario: A long-unresolved wait escalates

- **WHEN** a pane has been waiting past the timeout without being resolved and an HQ
  pane is live
- **THEN** HQ receives one escalation nudge for that pane, deduped per waiting episode

#### Scenario: Leaving waiting re-arms the escalation

- **WHEN** the pane leaves waiting and later enters a new waiting episode
- **THEN** a fresh escalation may fire (the prior episode's dedup does not suppress it)

### Requirement: HQ opens with a self-introduction and status briefing

When `gtmux hq` FRESH-spawns a supervisor session, the system SHALL deliver a
one-shot startup prompt into the new pane — after the agent comes up, via the
verified dispatch path (wait-for-ready, then a land-verified deliver, the same path
`gtmux spawn` uses) — so the supervisor's FIRST output does two things: (a) it
introduces itself and its role (overseeing every coding agent on the machine —
sense · decide · dispatch · supervise · report — and curating the knowledge base),
and (b) it produces an immediate status report grounded in `gtmux digest --json`,
`gtmux usage --json`, and `gtmux limits --json`, formatted as a COLUMN-ALIGNED
TABLE — never a prose paragraph (`gtmux digest`'s own text output renders this
shape; the supervisor matches its layout): a one-line summary of counts by
state, then a section per state (needs-you leads, then working, then
completed, then errored if any) with one aligned row per agent, and ALWAYS
including the token-usage rollup and the subscription-window room laid out the
same aligned way (the same report shape the seeded playbook's status policy
requires). The briefing SHALL run ONLY on a fresh
spawn: a `gtmux hq` that focuses an already-live supervisor SHALL NOT re-deliver it.
It SHALL be best-effort and non-fatal — a delivery that does not land SHALL NOT fail
`gtmux hq`, since the session is already up and usable. The prompt SHALL be bilingual
(follows `GTMUX_LANG`) and SHALL be opt-out-able via `GTMUX_HQ_BRIEF`
(`off`/`0`/`false`/`no`), defaulting on.

#### Scenario: A fresh spawn briefs on the first turn

- **WHEN** `gtmux hq` spawns a new supervisor session and the agent comes up
- **THEN** a startup prompt is delivered into its pane so the supervisor's first
  output introduces itself and reports the fleet status (needs-you first, who's
  working, token usage + subscription room)

#### Scenario: A focused live supervisor is not re-briefed

- **WHEN** `gtmux hq` runs while a supervisor session is already live
- **THEN** it focuses the existing session and NO startup briefing is delivered

#### Scenario: Opt-out spawns HQ silently

- **WHEN** `GTMUX_HQ_BRIEF` is `off`/`0`/`false`/`no` and `gtmux hq` fresh-spawns
- **THEN** no startup briefing is delivered — the supervisor waits at its prompt

#### Scenario: A briefing that cannot land does not fail the command

- **WHEN** the agent does not come up in time, or the delivery cannot be verified
- **THEN** `gtmux hq` still succeeds (the session is up and usable) rather than
  reporting failure

### Requirement: HQ maintains a persistent situation board across context resets

The system SHALL seed a situation board at `~/.config/gtmux/hq/notes/board.md` — written
only when ABSENT (never clobbering HQ's curated content), the same write-when-absent
discipline as the knowledge scaffold — and the seeded playbook SHALL direct the supervisor
to keep it current as its durable command posture: one row per ship (agent) carrying its
task, command mode / dispatch source, priority, health, any pending decision, and the most
recent lesson. Because HQ's own context is periodically compacted or reset, the playbook
SHALL instruct HQ to re-read the board at the start of a turn after a reset — before acting —
so posture survives the reset, and to treat the deterministic `gtmux digest`/`tasks`/`events`
as the source of record while the board is HQ's synthesis. The board SHALL be HQ-curated
markdown, NOT a gtmux-parsed schema (gtmux does not read it back).

#### Scenario: A fresh home seeds the board

- **WHEN** `gtmux hq` seeds a home with no `notes/board.md`
- **THEN** a `board.md` template is created (per-ship task · mode/source · priority · health ·
  pending · lesson), and a subsequent run leaves HQ's curated board untouched

#### Scenario: Posture survives a context reset

- **WHEN** the seeded playbook covers HQ resuming after a `/compact` or context reset
- **THEN** it directs HQ to re-read `notes/board.md` before acting, rather than re-deriving
  the whole fleet from scratch

### Requirement: Query the attention stream, not raw transcripts

The seeded playbook SHALL direct the supervisor to triage from the SEVERITY-filtered event
stream and the digest — `gtmux events --severity important` for what needs attention, the
per-record `summary` for what was said — and NOT to read raw transcripts line-by-line, which
doubles token cost. The Toolbox section SHALL document `gtmux events --severity`.

#### Scenario: The playbook points triage at the filtered stream

- **WHEN** the HQ home is seeded
- **THEN** the playbook instructs HQ to read `gtmux events --severity important` and record
  summaries rather than raw transcripts, and the Toolbox lists `--severity`

### Requirement: Decision-authority tiers — when HQ decides versus escalates

The seeded playbook SHALL encode the commander's three interaction modes — ① dispatch a ship
directly, ② adopt HQ's suggestion, ③ discuss then let HQ decide and delegate — and an explicit
autonomy matrix for mode ③: HQ MAY decide-and-dispatch autonomously ONLY when the action is
REVERSIBLE **and** LOW-RISK **and** WITHIN AN ALREADY-DISCUSSED DIRECTION; HQ MUST escalate to
the commander when the action is IRREVERSIBLE, touches PERMISSIONS/CREDENTIALS, FORKS the
plan/approach, or falls OUTSIDE the discussed scope. This SHALL NOT loosen the existing rule
that HQ never answers another agent's permission/plan/design choice on the user's behalf —
it makes mode ③ concrete without granting HQ authority over the commander's decisions.

#### Scenario: A reversible in-scope action may be decided

- **WHEN** the playbook covers a reversible, low-risk action within a direction the commander
  already discussed (e.g. re-dispatching a follow-up the user asked to continue)
- **THEN** it permits HQ to decide and dispatch it, noting what it did and to whom

#### Scenario: An irreversible or forking action is escalated

- **WHEN** the action is irreversible, touches permissions/credentials, forks the
  plan/approach, or is outside the discussed scope
- **THEN** the playbook directs HQ to escalate to the commander rather than decide it

### Requirement: Graded escalation and reconcile-before-relay

The seeded playbook SHALL define GRADED escalation channels keyed on severity — `routine`
items update the situation board only (no interrupt); `important` items reach HQ as a
coalesced summary; `critical` conditions ensure the commander is pushed (via the existing
notification pipeline, which already surfaces attention events to the phone) — so only
genuinely critical conditions "ring". The playbook SHALL define `critical` as the runtime
judgment HQ layers over important events: quota near-exhaustion (from `gtmux limits`/`usage`),
a production/线上 issue, or one agent blocking others. The playbook SHALL further require a
RECONCILE step: before relaying or escalating any needs-you, HQ re-checks the LIVE
`gtmux digest`/`tasks` for that pane and DROPS the item if the state already moved (the pane
was answered directly, resumed, or finished) — eliminating stale needs-you false positives.
This complements the `resolved`-nudge retraction, covering the delayed/queued/post-reset case
where no `resolved` nudge was observed.

#### Scenario: Only critical conditions ring

- **WHEN** the playbook covers a routine turn-end versus a quota-near-exhaustion condition
- **THEN** it directs the routine item to the board silently and the critical one to a push,
  with `important` items coalesced into an HQ summary in between

#### Scenario: A stale needs-you is reconciled away

- **WHEN** HQ is about to relay a needs-you and the live digest shows that pane already left
  waiting (answered directly / resumed / finished)
- **THEN** the playbook directs HQ to reconcile against the live digest and DROP the relay
  rather than forward a stale one

### Requirement: Correction-to-charter learning loop

The seeded playbook SHALL make learning from corrections a FIRST-CLASS ritual, not an ad-hoc
afterthought: when the commander CORRECTS HQ, or the SAME footgun is hit more than once, HQ
SHALL distill the durable lesson and land it — a PORTABLE behavior lesson into
`knowledge/best-practices.md` or `knowledge/pitfalls.md` (and, when the lesson is
charter-level, FLAG it for a seed/spec update rather than only noting it locally); a
MACHINE-SPECIFIC instance into local notes. The playbook SHALL state the trigger points
(a commander correction; a repeated footgun) and the landing path explicitly, so HQ actually
self-upgrades from the interaction. The knowledge scaffold SHALL include a `corrections.md`
topic as the landing place for distilled corrections.

#### Scenario: A correction is distilled and landed

- **WHEN** the playbook covers the commander correcting HQ, or a footgun recurring
- **THEN** it directs HQ to distill the lesson into the knowledge base (portable) or local
  notes (machine-specific) and to flag a charter-level lesson for a seed/spec update

#### Scenario: The scaffold has a corrections topic

- **WHEN** `gtmux hq` seeds the knowledge scaffold
- **THEN** a `corrections.md` topic file exists and the KB README lists it

### Requirement: HQ subscribes to the silent feed and gates its own output

The seeded playbook SHALL teach HQ to consume the full event stream through the silent
perception feed (a background subscription to `gtmux hq-feed`) and to GATE its own
user-visible output by surfacing tier: it SHALL print to the user for CRITICAL and NORMAL
items (per the resolved `surfaceTier`), and for QUIET items it SHALL only record to the
attention ledger and stay silent that turn. HQ SHALL answer confirm-type asks itself only
within the reversible ∧ low-risk ∧ no-fork bound (recording the auto-answer), and escalate
everything else. HQ SHALL always surface a feed-degradation CRITICAL regardless of the
configured threshold.

#### Scenario: A QUIET event produces no user output

- **WHEN** HQ receives a QUIET-tier event through the feed
- **THEN** it records the item in the ledger and prints nothing to the user that turn

#### Scenario: A CRITICAL event is surfaced

- **WHEN** HQ receives a CRITICAL-tier event (e.g. a decision-type ask, an error, or a
  feed degradation)
- **THEN** HQ prints it to the user, and a feed-degradation CRITICAL is surfaced even
  when quiet mode is on

#### Scenario: The threshold moves the bar, not HQ's awareness

- **WHEN** the user raises the surfacing threshold (quiet on)
- **THEN** HQ still ingests every event but prints only CRITICAL items, the rest going to
  the ledger

### Requirement: HQ self-check and self-maintenance

The seeded playbook SHALL teach HQ, on a gtmux-raised self-check trigger, to review and
maintain its OWN artifacts — event-log/feed health, attention-ledger archival and
de-duplication, memory/knowledge-base quality, and accumulated low-value items — using only
its existing write-own-notes authority. HQ SHALL default to SILENT self-maintenance,
printing a one-line brief ONLY when it took a real action, and SHALL escalate a severe
finding (rotation broken, cursor gap, mass-invalid memory) as CRITICAL.

#### Scenario: Silent maintenance when nothing needed

- **WHEN** a self-check trigger fires and HQ finds nothing to fix
- **THEN** HQ performs the pass and prints nothing

#### Scenario: A real cleanup is briefed in one line

- **WHEN** a self-check trigger fires and HQ archives closed ledger items or prunes stale
  memory
- **THEN** HQ prints a single one-line brief of what it did

#### Scenario: A severe finding escalates

- **WHEN** a self-check finds a broken rotation, a cursor gap, or mass-invalid memory
- **THEN** HQ surfaces it as CRITICAL rather than quietly cleaning up

