# supervisor-agent Specification

## Purpose
TBD - created by archiving change supervisor-mvp. Update Purpose after archive.

## Requirements

### Requirement: Launchable supervisor session

The system SHALL provide `gtmux hq` (中控): it creates — or focuses, when one
already runs — a dedicated tmux session running the user's coding agent (Claude
by default, per existing agent profiles) with its working directory set to the
persistent supervisor home (`~/.config/gtmux/hq/`). The playbook SHALL be
gtmux-OWNED and VERSION-TRACKED: AGENTS.md is the canonical FULL playbook (the
cross-agent convention Codex/Cursor/Amp read natively) carrying a machine-parseable
VERSION marker, and CLAUDE.md is a one-line `@AGENTS.md` import so Claude reads the
SAME content with no two-doc drift (`--agent`/`GTMUX_HQ_AGENT` pick which agent runs).
User PERSONALIZATION SHALL live in a separate seed-once `LOCAL.md` that the generated
AGENTS.md `@`-imports (reaching Claude through the CLAUDE.md→AGENTS.md→LOCAL.md chain);
gtmux SHALL create `LOCAL.md` once from a template and SHALL NEVER overwrite it. On
`gtmux hq`, when the SHIPPED playbook version is newer than the installed one, the
system SHALL UPGRADE the managed AGENTS.md: back up the prior file to
`AGENTS.md.bak-v<old>` FIRST, regenerate at the new version, and print a one-line
notice. When the versions match it SHALL be idempotent (no rewrite). An existing
AGENTS.md with NO version marker SHALL be treated as version 0 and MIGRATED once via
the same backup-then-regenerate path, with the notice directing the user to move any
personal edits into `LOCAL.md`. The situation board (`notes/board.md`) and knowledge
base (`knowledge/*`) SHALL remain seed-if-absent and SHALL NOT be touched by an
upgrade. A legacy full CLAUDE.md (pre-AGENTS.md convention) SHALL remain authoritative
and SHALL NOT get a zombie AGENTS.md dropped beside it. `gtmux hq` SHALL WARN — rather
than silently proceed — when it detects a redundant layout (a full CLAUDE.md alongside
AGENTS.md) or a broken one (a CLAUDE.md `@AGENTS.md` import while AGENTS.md is missing).
The seeded playbook teaches the supervisor to loop — read `gtmux digest --json`, judge,
drill into a pane (`tmux capture-pane`) only when warranted, drive via `gtmux send`,
report to the user with a token-usage section ALWAYS included in status reports (the
per-type rollup + any `usage_warn` sessions, via `gtmux usage --json`) — and the
supervisor's accumulated knowledge persists across sessions. "Already runs" SHALL mean
the supervisor AGENT is actually alive in its pane: if the HQ pane is still stamped but
its agent has EXITED (the foreground process is an interactive shell — the user quit it
and left the window), `gtmux hq` SHALL RELAUNCH the agent in that same pane rather than
focus a dead prompt while claiming the supervisor is running.

#### Scenario: Stamped HQ pane whose agent has exited

- **WHEN** `gtmux hq` runs and the HQ pane still exists but its foreground process is a
  shell (the supervisor agent was quit, the tmux window left open)
- **THEN** it relaunches the supervisor agent in that same pane (not a dead-window
  focus), and focuses it — so the user never lands on a live-looking but dead HQ

#### Scenario: Fresh home seeds the managed playbook + LOCAL.md

- **WHEN** `gtmux hq` runs and `~/.config/gtmux/hq/` has NO policy file
- **THEN** a version-stamped AGENTS.md (the full playbook), a CLAUDE.md `@AGENTS.md`
  import, and an empty `LOCAL.md` template are generated, and a tmux session starts the
  agent there

#### Scenario: A newer shipped version upgrades the playbook

- **WHEN** `gtmux hq` runs and the installed AGENTS.md version is older than the shipped
  `hqPlaybookVersion`
- **THEN** the prior AGENTS.md is backed up to `AGENTS.md.bak-v<old>`, AGENTS.md is
  regenerated at the shipped version, and `gtmux hq` prints a one-line upgrade notice

#### Scenario: Matching version is idempotent

- **WHEN** `gtmux hq` runs and the installed AGENTS.md version equals the shipped one
- **THEN** AGENTS.md is left unchanged (no rewrite) and no notice is printed

#### Scenario: LOCAL.md is never overwritten

- **WHEN** `gtmux hq` upgrades the playbook and the user has content in `LOCAL.md`
- **THEN** `LOCAL.md` is left exactly as the user wrote it, and its content still reaches
  the agent via the AGENTS.md import

#### Scenario: A legacy unversioned AGENTS.md is migrated once

- **WHEN** `gtmux hq` runs and the home holds an AGENTS.md with no version marker
- **THEN** it is backed up to `AGENTS.md.bak-v0`, regenerated at the shipped version, and
  the notice directs the user to move any personal edits into `LOCAL.md`

#### Scenario: The board and knowledge base survive an upgrade

- **WHEN** `gtmux hq` upgrades the playbook
- **THEN** `notes/board.md` and every `knowledge/*` file are left untouched

#### Scenario: A legacy full CLAUDE.md gets no zombie AGENTS.md

- **WHEN** `gtmux hq` runs and the home already holds a full CLAUDE.md but no AGENTS.md
- **THEN** the CLAUDE.md is left untouched and NO AGENTS.md is created beside it

#### Scenario: A redundant or broken layout warns

- **WHEN** the home has a full CLAUDE.md alongside AGENTS.md, or a CLAUDE.md
  `@AGENTS.md` import while AGENTS.md is missing
- **THEN** `gtmux hq` prints a warning naming the redundant/broken doc and how to
  resolve it, rather than silently proceeding

### Requirement: HQ agent selection

`gtmux hq` SHALL resolve WHICH agent runs the supervisor rather than always launching
`claude`, so a machine signed into a different agent does not get an HQ stuck on "Not
logged in · Please run /login". Resolution order on a spawn or relaunch SHALL be: the
`--agent` flag (an explicit choice, remembered), then the `GTMUX_HQ_AGENT` env override
(transient, not remembered), then a REMEMBERED prior choice, then — only at an interactive
terminal and only when no choice has been made — an interactive PICKER of the agents that
are actually installed (the hook-equipped agents whose launch binary is on PATH, so the
supervisor gets its wake/event stream), then `claude`. The picker's result SHALL be
remembered so the user is asked at most once; a non-interactive caller SHALL never block
and SHALL fall back to the default. Selection SHALL run only on launch/relaunch — focusing
an already-live supervisor SHALL NOT prompt.

#### Scenario: A fresh HQ asks which installed agent to run

- **WHEN** `gtmux hq` spawns a new supervisor at an interactive terminal with no `--agent`,
  no `GTMUX_HQ_AGENT`, no remembered choice, and more than one hook-equipped agent installed
- **THEN** it lists the installed agents and launches the one the user picks, and remembers
  that choice for the next `gtmux hq`

#### Scenario: Only one installed agent needs no prompt

- **WHEN** exactly one hook-equipped agent is installed (e.g. only Codex)
- **THEN** `gtmux hq` launches that agent without prompting — there is no choice to make

#### Scenario: Non-interactive or explicit selection never prompts

- **WHEN** `gtmux hq` runs with `--agent`/`GTMUX_HQ_AGENT` set, or from a non-terminal
  stdin, or with a remembered choice already on file
- **THEN** the resolved agent is used with no picker shown; focusing a live supervisor is
  likewise never a prompt

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

The system SHALL, when a tmux agent enters waiting and a supervisor session is live,
wake the supervisor with ONE compact line carrying the location, waiting kind, and title,
riding the notification pipeline's existing dedup so an unchanged waiting state is not
re-nudged. The SAME channel SHALL carry usage warnings (`usage·warn`, deduped per
session+layer — see `usage-watch`) and the lifecycle watchdog's escalation
(`stuck·waiting`). It SHALL never wake the supervisor about its own waiting states, SHALL
be a no-op when no supervisor session is live, and SHALL be disableable via configuration
(`hqNudge: false`, default on).

Delivery SHALL go through the single wake channel (see `hq-wake-protocol`): the declared
class, the `» gtmux·<class>` signal format, and the channel's draft guard, ack, and queue.
No caller SHALL type into the supervisor's pane directly, and no requirement SHALL
describe a delivery mechanism of its own — a superseded requirement that is merely
contradicted by a newer one, rather than retired, keeps blessing the code that still obeys
it.

The watchdog's escalation SHALL further require that the wait is an ASK — a waiting state
whose kind was recorded from the agent's own hook event. `stuck·waiting` asserts that a
PERSON IS BLOCKED, and that can only be true if someone was asked; a waiting state gtmux
inferred from the SCREEN (a dispatch it believes is stuck before running) has no asker and
SHALL NOT escalate.

The discriminator SHALL be the wait's PROVENANCE, never the presence of parsed option
text. A parseable menu is not what makes someone blocked: an agent stopped on a free-form
question offers no options and is blocked all the same, and refusing to escalate that
would trade one silent-alarm failure for another. An escalation refused on this ground
SHALL NOT consume the episode's once-per-episode allowance, so a later genuine ask in the
same episode still escalates.

#### Scenario: Agent blocks, supervisor learns

- **WHEN** another agent enters waiting (permission/plan/question) while an hq
  session is live
- **THEN** one `waiting·<kind>` wake line reaches the hq pane, at most once per waiting
  transition

#### Scenario: A wait nobody asked for does not escalate

- **WHEN** a pane has been marked waiting by gtmux's own screen inference (kind `startup`
  / `draft`) and stays that way past the watchdog timeout
- **THEN** no `stuck·waiting` escalation is sent, and the episode's allowance is unspent

#### Scenario: A free-form question still escalates

- **WHEN** a pane is waiting on a question the agent asked in prose, with no numbered menu
  to parse, and nobody answers past the timeout
- **THEN** `stuck·waiting` escalates exactly once — the absence of option text is not
  evidence that nobody is blocked

#### Scenario: Usage warning reaches the supervisor

- **WHEN** a session breaches (or projects into) a usage layer while HQ is live
- **THEN** one `usage·warn` wake line reaches the hq pane, at most once per session+layer

#### Scenario: Every injection is draft-guarded

- **WHEN** any of these wakes fires while the user is composing in the hq pane
- **THEN** nothing is typed: the wake queues and lands once the box is empty

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

The playbook SHALL further fix the STANDARD ACTION for carrying a goal into that
dispatch: any goal that is not a short single line SHALL be written to a file and passed
as `gtmux spawn --goal-file <path>` (equivalently `gtmux send --message-file`), and the
playbook SHALL state the REASON — a goal passed as a command-line argument is necessarily
parsed by a shell, so a backtick, `$`, quote or newline inside it is executed or mangled
rather than delivered. The instruction SHALL be phrased as a mechanism, not as a caution:
this exact footgun was recorded in HQ's knowledge base twice and generalized into a rule
before it recurred, which is evidence that "remember to quote carefully" is not a
guarantee a supervisor can hold.

#### Scenario: Playbook points dispatch at `gtmux spawn`

- **WHEN** the HQ playbook and knowledge seeds are generated
- **THEN** they instruct dispatch via `gtmux spawn` and note that a bare
  `send-keys` launch is un-proxied and will 403

#### Scenario: Playbook fixes the file channel as the standard action

- **WHEN** the HQ playbook is generated
- **THEN** its dispatch section tells the supervisor to write any multi-line or
  special-character goal to a file and dispatch it with `--goal-file`, and states that a
  goal passed as an argument must survive shell parsing

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
SHALL NOT send Enter — the nudge is queued instead. The draft read SHALL be COLOR-aware
and SHALL EXCLUDE the agent's suggested-next-command GHOST text — the dim autosuggestion
rendered faint (SGR 2), which is NOT user input — so a faint ghost suggestion in the HQ
composer does NOT hold a nudge behind a phantom draft; only genuinely half-typed USER
input (normal brightness) SHALL defer delivery. Delivery SHALL occur only when the box
is confirmed empty over TWO reads a short interval apart, and a queued nudge SHALL be
delivered on a later empty box: on the next injection attempt, on HQ's own turn-end
(`Stop`, box reliably empty — coalesced), or on the serve tick. It is an INVARIANT that
no code path sends Enter into a non-empty HQ input box.

#### Scenario: A half-typed draft is never clobbered

- **WHEN** a nudge fires while the HQ input box holds a non-empty draft
- **THEN** nothing is typed and no Enter is sent; the nudge is queued

#### Scenario: A queued nudge is delivered once the box is empty

- **WHEN** the HQ box is confirmed empty over two reads (or HQ finishes a turn)
- **THEN** the queued nudge(s) are delivered, coalesced, exactly once

#### Scenario: A faint ghost suggestion does not hold a nudge

- **WHEN** a nudge fires while the HQ composer shows only the agent's faint
  suggested-next-command ghost text (SGR 2), with no real half-typed input
- **THEN** the ghost text is not read as a draft, so the nudge is delivered rather than
  queued behind a phantom draft

### Requirement: Dual-channel dispatch — HQ senses user-direct tasks

The system SHALL let HQ track work the user dispatches through EITHER channel: via HQ
(`gtmux spawn`, tracked) or by typing directly into an agent's own window. When a
`UserPromptSubmit` occurs in a pane that is NOT the HQ pane, the system SHALL push a
`goal-changed` nudge to a live HQ carrying the pane and the prompt head (as DATA), gated
on a live HQ pane and `hqNudge`, and never about HQ's own prompts.

The nudge SHALL be deduplicated per pane on a FINGERPRINT of the full cleaned prompt
carrying a timestamp, suppressing only an identical prompt within a 5-minute window — a
resubmit of the same prompt inside the window does not spam, and the same instruction
repeated after it wakes HQ again. The pane's goal that the `done` wake reads back SHALL
be recorded separately from that dedup fingerprint, so the expiry cannot churn it.

A submission with no prose that the user nonetheless made — a slash command — SHALL
still wake, with its goal labelled as DATA (`goal:"(slash-command) /compact"`); only
content the user did not author (harness-injected blocks, gtmux's own wake lines echoed
back) SHALL be silent.

The HQ playbook SHALL instruct that observing an agent working on a task NOT in the
ledger, the FIRST assumption is the user dispatched it directly — HQ verifies (records it
as `user-direct`) rather than "correcting", interrupting, or overwriting it.

#### Scenario: A user-direct prompt reaches HQ

- **WHEN** the user submits a prompt directly in a non-HQ agent pane and an HQ pane
  is live
- **THEN** HQ receives one `goal-changed` nudge for that pane (deduped per prompt
  fingerprint within the window)

#### Scenario: The same instruction after the window wakes HQ again

- **WHEN** the user submits the same prompt into the same pane after the dedup window has
  expired
- **THEN** a second `goal-changed` nudge is delivered

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

The system SHALL, on the supervisor's FIRST turn (a fresh spawn, OR a relaunch of a
stamped-but-dead HQ pane), deliver a MINIMAL, agent-agnostic TRIGGER — a single
`» gtmux·startup` signal line, NOT a large multi-line prompt — after the agent comes up
(via the verified dispatch path: wait-for-ready, then a land-verified deliver, the same
`gtmux spawn` uses). The briefing's CONTENT and format SHALL live in
the seeded playbook (AGENTS.md's "## First turn" section, which every agent reads
through its own convention file — Claude via CLAUDE.md→AGENTS.md, Codex/Cursor/Amp
natively), defining the two-part first output: (a) a one/two-sentence self-introduction
of the supervisor role (sense · decide · dispatch · supervise · report + curate the
knowledge base), and (b) an immediate status report EXACTLY per the playbook's status
policy — a COLUMN-ALIGNED TABLE from `gtmux digest --json` / `gtmux usage --json` /
`gtmux limits --json` (needs-you first, token-usage rollup + subscription-window room),
never a prose paragraph. Keeping the content in the playbook and the injected text a
one-line trigger makes it ROBUST (a one-liner submits reliably where a big multi-line
paste was flaky — typed-but-not-submitted) and AGENT-AGNOSTIC (not Claude-Code-specific).

The playbook SHALL additionally teach a FIRST-LAUNCH personalization step: when
`LOCAL.md` is still the seeded template (no user content — a mechanical trigger),
the briefing ends by asking the commander three questions — what they mainly work
on, how they want status reported, and any quiet hours — and HQ writes the
answers into `LOCAL.md`, acting as the user's scribe into the user's own slot.
An edited `LOCAL.md` skips the step entirely; the questions are asked once, not
per rotation.

The trigger SHALL NOT be re-delivered when `gtmux hq` merely focuses an ALREADY-LIVE
supervisor (agent still running). It SHALL be best-effort and non-fatal (a delivery
that does not land SHALL NOT fail `gtmux hq`), bilingual (follows `GTMUX_LANG`), and
opt-out-able via `GTMUX_HQ_BRIEF` (`off`/`0`/`false`/`no`), defaulting on.

#### Scenario: A fresh spawn briefs on the first turn

- **WHEN** `gtmux hq` spawns a new supervisor session and the agent comes up
- **THEN** a single `» gtmux·startup` trigger is delivered into its pane, and the
  supervisor's first output — per its playbook's "First turn" section — introduces
  itself and reports the fleet status (needs-you first, token usage + subscription room)

#### Scenario: A first launch asks who the user is

- **WHEN** the briefing runs while `LOCAL.md` is still the untouched seeded template
- **THEN** the playbook directs HQ to close the briefing with the three
  personalization questions and to write the commander's answers into `LOCAL.md`

#### Scenario: A personalized home is not re-interviewed

- **WHEN** the briefing runs and `LOCAL.md` already carries user content
- **THEN** no personalization questions are asked

#### Scenario: A relaunched dead pane briefs too

- **WHEN** `gtmux hq` relaunches the agent in a stamped-but-dead HQ pane
- **THEN** the same `» gtmux·startup` trigger is delivered, so the revived supervisor
  briefs just like a fresh spawn

#### Scenario: A focused live supervisor is not re-briefed

- **WHEN** `gtmux hq` runs while a supervisor session is already live (agent running)
- **THEN** it focuses the existing session and NO startup trigger is delivered

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

The seeded playbook SHALL direct the supervisor to triage from the event stream and the
digest — the per-record `summary` for what was said — and NOT to read raw transcripts
line-by-line, which doubles token cost.

It SHALL name the THREE reads for what they are, and SHALL NOT present any one of them as
"the attention stream":

- `gtmux events --since-seq <n>` (unfiltered) — the DELTA since HQ's cursor: what a wake
  tells HQ to run, and the reconcile path whenever HQ doubts its picture;
- `gtmux events --severity notable` — the FLEET-CHANGE stream: instructions reaching
  sessions, turn-ends, lifecycle;
- `gtmux events --severity important` — the ESCALATION stream: blocked, asking, crashed.
  A subset to triage FIRST, never the whole picture.

The playbook SHALL state the rule that generalizes: a filtered read is a triage shortcut,
NOT HQ's model of the world — every filter is a claim about what does not matter, so HQ
reconciles with the unfiltered delta (or the digest) rather than trusting one tier. The
Toolbox section SHALL document `gtmux events --severity` with that framing.

#### Scenario: The playbook names the three reads

- **WHEN** the HQ home is seeded
- **THEN** the playbook describes the unfiltered `--since-seq` delta as the reconcile
  path, `--severity notable` as the fleet-change stream, and `--severity important` as the
  escalation subset — and instructs HQ to record summaries rather than raw transcripts

#### Scenario: A filtered read is never the whole picture

- **WHEN** the playbook covers triage from a severity filter
- **THEN** it states that a filter is a shortcut rather than HQ's model of the world, and
  points at the unfiltered delta (or the digest) for reconciling

#### Scenario: The user's direct instruction is visible by pull

- **WHEN** the user submits an instruction directly into a non-HQ agent pane and HQ later
  catches up by pull rather than by the wake line
- **THEN** the instruction is in the stream HQ is told to read (`notable` and above), not
  filtered out as routine chatter

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
SHALL distill the durable lesson and land it — a PORTABLE behavior lesson into the
`best-practices` or `pitfalls` knowledge topics (through the knowledge verbs), and, when the
lesson is CHARTER-LEVEL (it holds on another machine AND belongs in a DURABLE RULE CARRIER
beyond this machine's knowledge base — a project's `AGENTS.md`/`CLAUDE.md`, a team runbook,
`LOCAL.md` when it governs this supervisor itself, or gtmux's own playbook/specs/code),
PROMOTE it: `gtmux knowledge promote <id> --why "…" [--target "…"]` writes the promotion
brief that IS the exit, and `gtmux knowledge land <id> --ref "…"` closes the loop when it
lands in its carrier — the ref naming a PR, an issue, a runbook, or a file alike. A local
flags list is NOT the mechanism — an un-carried flag rots and drifts (measured: 34
accumulated items, and an estimate off by ~50× by the time it was audited). A
MACHINE-SPECIFIC instance goes into local notes. The playbook SHALL state the trigger
points (a commander correction; a repeated footgun) and the landing path explicitly, so HQ
actually self-upgrades from the interaction, and SHALL direct HQ to migrate any
pre-existing local flags backlog through the promotion verbs — judged entry-by-entry, never
bulk-imported. The knowledge scaffold SHALL include a `corrections` topic as the landing
place for distilled corrections.

#### Scenario: A correction is distilled and landed

- **WHEN** the playbook covers the commander correcting HQ, or a footgun recurring
- **THEN** it directs HQ to distill the lesson into the knowledge base (portable) or local
  notes (machine-specific) and to PROMOTE a charter-level lesson through
  `gtmux knowledge promote`, closing with `land` when it reaches its carrier

#### Scenario: The scaffold has a corrections topic

- **WHEN** `gtmux hq` seeds the knowledge scaffold
- **THEN** a `corrections` topic exists and the KB README lists it

#### Scenario: A flags file is not an exit

- **WHEN** HQ holds charter-level lessons only in a local notes file
- **THEN** the playbook directs it to promote the ones that still hold (and dismiss or
  retire the rest), so the queue — not the file — carries them

#### Scenario: A non-developer's lesson has a legitimate landing

- **WHEN** a charter-level lesson governs the user's own fleet or projects rather than
  gtmux itself
- **THEN** the taught landing is the user's carrier (a project AGENTS.md, a team runbook,
  LOCAL.md) — never a mandate to edit gtmux's source tree

### Requirement: HQ subscribes to the silent feed and gates its own output

The seeded playbook SHALL teach HQ to perceive by PULL-ON-WAKE: on any wake line
it reads the delta (`gtmux events --since <seq>` and/or `gtmux digest --json`)
before acting, rather than requiring a persistently backgrounded
`gtmux hq-feed --tail` subscription (which is agent-specific and is DROPPED as a
playbook requirement — the spool remains available as pull-side data). HQ SHALL
GATE its own user-visible output by surfacing tier: it SHALL print for CRITICAL
and NORMAL items (per the resolved threshold), and for QUIET items it SHALL only
record to the attention ledger and stay silent that turn. HQ SHALL answer
confirm-type asks itself only within the reversible ∧ low-risk ∧ no-fork bound
(recording the auto-answer), and escalate everything else. HQ SHALL always
surface a feed-degradation CRITICAL regardless of the configured threshold.

#### Scenario: Wake then pull, on any agent

- **WHEN** HQ (running on any CLI agent, Claude or not) receives a wake line
  covering seq 341-352
- **THEN** it pulls the delta via CLI commands before acting — no background
  subscription is assumed

#### Scenario: A QUIET event produces no user output

- **WHEN** HQ ingests a QUIET-tier event from a pulled delta
- **THEN** it records the item in the ledger and prints nothing to the user that turn

#### Scenario: A CRITICAL event is surfaced

- **WHEN** HQ ingests a CRITICAL-tier event (a decision-type ask, a crash, or a
  feed degradation)
- **THEN** HQ prints it, and a feed-degradation CRITICAL is surfaced even when quiet
  mode is on

### Requirement: HQ self-check and self-maintenance

The seeded playbook SHALL teach HQ, on a gtmux-raised self-check trigger — delivered as a
STANDING-priority wake line (`» gtmux·self-check …`) and recorded as a
`[CONTROL gtmux:self-check]` entry in the session-event stream — to review and maintain
its OWN artifacts: event-log/feed health, attention-ledger archival and de-duplication,
memory/knowledge-base quality, and accumulated low-value items, using only its existing
write-own-notes authority. HQ SHALL default to SILENT self-maintenance, printing a
one-line brief ONLY when it took a real action, and SHALL escalate a severe finding
(rotation broken, cursor gap, mass-invalid memory) as CRITICAL. The self-check sensor's
own control records SHALL NOT be counted as recent user-facing attention, so a raised
trigger cannot suppress the idle condition that raised it.

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

#### Scenario: A raised self-check does not suppress the next one

- **WHEN** a self-check trigger has been raised into an otherwise quiet fleet and the idle
  window elapses again
- **THEN** the idle trigger still fires — gtmux's own control record is not read as the
  user having been recently pinged

### Requirement: Enrollment — goal-aware dossiers for every sensed session

On HQ start the seeded playbook SHALL direct a fleet enrollment: read the full
digest and record a dossier per agent session on the situation board — purpose
(the session's goal), current status, and channel (hq-dispatched / user-direct) —
drilling into a transcript head at most once per session only when the purpose is
not evident from the digest. Thereafter each `new-session` wake SHALL enroll the
newcomer incrementally. Perception SHALL remain goal-aware: board entries name
what a session is FOR, not merely its mechanical state.

#### Scenario: HQ start builds the fleet dossier

- **WHEN** HQ starts with nine live agent sessions
- **THEN** its first turns produce a board with nine dossiers (purpose / status /
  channel), with at most one transcript-head drill per unclear session

#### Scenario: A newcomer is enrolled incrementally

- **WHEN** a new agent session appears while HQ is live
- **THEN** HQ receives one `new-session` wake and appends that session's dossier to
  the board without re-scanning the fleet

### Requirement: Signal register separates wake traffic from conversation

The seeded playbook SHALL mandate two output registers: replies to wake lines use
the SIGNAL register — one line opening with `⟣` and a fixed glyph vocabulary
(✅ done-judgment, ▪ noted-to-board, 📓 captured-to-KB, ◈ tick brief, ⚠ escalation),
with at most two indented detail lines (tick briefs ≤6 lines total) — while replies to the
human use ordinary conversational prose with no sigils. The `⟣ 📓 captured: <topic-file>`
line SHALL be the capture verdict named in the capture-verify requirement and SHALL be
emitted ONLY on a REAL capture (never as an empty "considered it" marker). Wake turns
SHALL be short: pull the delta, judge, capture (when a forced class or a real
opportunistic fact warrants it), update the board, emit the signal line; no narration.

#### Scenario: A done wake gets a one-line judgment

- **WHEN** HQ processes a `done` wake for a session that completed its goal
- **THEN** its reply is a single `⟣ ✅ …` line (judgment + suggested next step),
  visually distinct from conversation

#### Scenario: A capture is marked in the register

- **WHEN** HQ folds a durable lesson from a forced-class closure into a KB topic file
- **THEN** it emits a `⟣ 📓 captured: <topic-file>` line, in the signal register, never
  mixed with human prose

#### Scenario: A trivial done is noted silently

- **WHEN** HQ judges a done wake to be an unremarkable intermediate completion
- **THEN** it updates the board and replies with at most one `⟣ ▪` note line, and emits no
  `⟣ 📓` (capture is opportunistic on done, not forced)

### Requirement: Periodic tick brief

On a `tick` wake the playbook SHALL direct HQ to pull the covered delta, update
the situation board, and emit ONE brief in the signal register — at most six
lines: a `⟣ ◈` headline with fleet counts and the top item needing attention,
then up to five indented outcome lines (completions with a one-clause summary,
new sessions, stalls). The brief SHALL respect the resolved quiet threshold; in
quiet mode board-only unless something CRITICAL rode the tick.

#### Scenario: The brief is bounded and concrete

- **WHEN** a tick wake covers three completions and one new session
- **THEN** HQ emits one ≤6-line `⟣ ◈` brief naming each outcome in one clause, and
  nothing else

### Requirement: Managed playbook migrates legacy homes

`gtmux hq` SHALL migrate a legacy HQ home whose only policy file is a full
CLAUDE.md (no managed AGENTS.md): back up the legacy file alongside
(timestamped, never deleted), generate the managed AGENTS.md at the current
playbook version plus the `@AGENTS.md` CLAUDE.md pointer, seed LOCAL.md once,
and print a one-line notice naming the backup. A home with a managed AGENTS.md
SHALL continue on the existing upgrade path. The seeder SHALL NOT silently skip
any home shape.

#### Scenario: A legacy home is upgraded on next start

- **WHEN** `gtmux hq` runs against a home containing only a legacy full CLAUDE.md
- **THEN** the legacy file is backed up in place, the managed AGENTS.md + pointer +
  LOCAL.md are written at the shipped version, and the notice names the backup

### Requirement: The playbook teaches the wake re-send identifier

The seeded playbook SHALL teach that every wake line ends with a short `#<id>` batch
identifier, and that a line whose identifier HQ has already acted on is a RE-SEND of an
unconfirmed delivery — to be ignored, not treated as a second event. It SHALL likewise
teach the `(slash-command)` goal payload (a user act with no prose, not an agent
message) and the `wake-degraded` class (the wake channel itself is failing to confirm —
reconcile by pull rather than trusting the knock). The shipped playbook version SHALL be
bumped so existing homes receive these conventions on the next `gtmux hq` after an
update.

#### Scenario: A duplicated wake is recognized

- **WHEN** HQ receives two wake lines carrying the same trailing `#<id>`
- **THEN** the playbook has it treat the second as a re-send and take no second action

#### Scenario: Existing homes get the conventions

- **WHEN** `gtmux hq` runs against a home carrying the previous managed playbook version
- **THEN** the playbook is regenerated at the new version (the prior file backed up) and
  states the `#<id>`, `(slash-command)`, and `wake-degraded` conventions

### Requirement: gtmux raises a periodic knowledge-distillation trigger

The system SHALL raise a `distill` trigger to a live HQ when a knowledge-distillation
pass is due, decided LLM-free in the serve slow-tick — no LLM runs in the timing loop; HQ
performs the actual curation on the raised trigger. The baseline trigger SHALL remain the
existing coarse cadence: a rate limit (at most one distill per configured minimum
interval), then an EVENT-VOLUME floor OR a WEEKLY time floor, with a ZERO-CHANGE gate
(nothing accrued → no trigger, no cost). The system SHALL additionally fire on the
pending-distill SPOOL reaching N entries (N CONFIGURABLE, default 5), still behind the
rate limit, so a captured lesson reaches the knowledge base without waiting out the week;
a non-empty spool SHALL also satisfy the zero-change gate. The system MAY additionally
fire on further EVENT-DRIVEN triggers, DEFERRED: (a) a DENSITY threshold of K notable
CLOSURES accrued since the watermark (K CONFIGURABLE, default 10, range 10–12); and (b)
any COMMANDER CORRECTION (a `correction`-class event) in the delta.

The trigger's own control record SHALL NOT count as fleet activity in any sensor's input:
gtmux-authored control records SHALL be excluded when counting the accrued delta, so a
raised trigger can never satisfy its own zero-change gate. Each raised trigger SHALL
advance the `last-distill` watermark (event sequence / timestamp marker) so the next pass
distills only the DELTA. The distillation pass SHALL additionally drain the
pending-distill spool — MERGING each candidate by (topic, dedup key) into the right KB
entry rather than appending a near-duplicate, and truncating the spool. When no HQ pane
exists, no trigger SHALL be raised.

#### Scenario: A weekly pass is due

- **WHEN** the time floor has elapsed since the last distill and at least one notable
  event has accrued
- **THEN** gtmux raises exactly one `distill` trigger and advances the watermark

#### Scenario: A busy fleet distills before the log rotates

- **WHEN** the event-volume floor is reached well before the weekly floor (a high-churn
  period)
- **THEN** the `distill` trigger fires on the volume floor, so the delta is distilled
  before the size-bounded event log rotates it away

#### Scenario: A filled capture spool distills without waiting for the week

- **WHEN** the pending-distill spool holds at least N candidates and the rate limit has
  elapsed, but neither the weekly nor the volume floor has been reached
- **THEN** gtmux raises one `distill` trigger, so captured lessons are filed in days
  rather than at the next weekly floor

#### Scenario: A density of closures triggers a distill (when the event-driven layer is built)

- **WHEN** the event-driven layer is enabled and at least K notable closures have accrued
  since the watermark and the rate limit has elapsed
- **THEN** gtmux raises one `distill` trigger before the periodic floor would have fired

#### Scenario: A commander correction distills promptly (when the event-driven layer is built)

- **WHEN** the event-driven layer is enabled, a `correction`-class event enters the delta,
  and the minimum interval has elapsed
- **THEN** a `distill` trigger is raised promptly rather than waiting for the periodic floor

#### Scenario: Nothing to distill costs nothing

- **WHEN** a cadence boundary is reached but no event accrued since the last distill and
  the capture spool is empty
- **THEN** no `distill` trigger is raised (the zero-change gate)

#### Scenario: A raised trigger does not feed its own gate

- **WHEN** a `distill` trigger has been raised and recorded, and the next cadence boundary
  arrives with no other activity
- **THEN** the zero-change gate still holds — gtmux's own control record is not counted as
  accrued fleet activity

#### Scenario: No trigger without a supervisor

- **WHEN** no HQ pane is present
- **THEN** no `distill` trigger is raised

### Requirement: The seeded playbook teaches the knowledge-distillation ritual

The seeded playbook SHALL teach HQ, on a gtmux-raised `distill` trigger — delivered as a
STANDING-priority wake line (`» gtmux·distill …`) and recorded as a
`[CONTROL gtmux:distill]` entry in the session-event stream — to run a retrospective
knowledge-distillation pass: read the fleet's event/outcome delta since the last distill,
drain the pending-distill spool CANDIDATE BY CANDIDATE (`gtmux knowledge add --capture
<key>` to accept one with its provenance, `gtmux knowledge dismiss --capture <key> --why`
to reject one with a trace), fold durable cross-cutting facts into the right topic
through the knowledge verbs (`add` for a new lesson, `supersede` for one that replaces an
existing entry — the mechanical form of update-over-append), PRUNE stale or dead entries
with `retire --why`, and migrate any legacy-file lesson the pass touches into the ledger.
The pass SHALL also check `gtmux knowledge promotions`: a pending brief past its
staleness floor is escalation material for the commander, because a promotion nobody
carries is silent rot. The ritual SHALL be distinct from `self-check` (own-artifact
health housekeeping) and `tick` (the user-facing summary brief). HQ SHALL default to
SILENT distillation, printing a one-line brief ONLY when it made a real curation; a
charter-level lesson SHALL be PROMOTED (`gtmux knowledge promote`) rather than only
noted locally; and the never-store-secrets rule SHALL continue to apply. Because the
trigger is also a stream record, the playbook SHALL teach that a distill missed on the
wake channel is recoverable by PULL (`gtmux events --since-seq`) rather than lost. The
shipped playbook version SHALL be bumped so existing HQ homes adopt the ritual on their
next managed-playbook upgrade.

#### Scenario: Silent when nothing durable accrued

- **WHEN** a `distill` trigger fires and the period's delta yields no durable new fact,
  no stale entry, and no duplicate to merge
- **THEN** HQ performs the pass and prints nothing

#### Scenario: A real distillation is briefed in one line

- **WHEN** a `distill` trigger fires and HQ folds a recurring cross-session fact into a
  topic through the knowledge verbs and retires a dead entry
- **THEN** HQ prints a single one-line brief of what it curated

#### Scenario: The distill delta is not a duplicate of moment-capture

- **WHEN** a durable fact was already captured in the knowledge base the moment it was
  learned
- **THEN** the distill pass SUPERSEDES that entry rather than adding a second copy,
  because it works the delta since the watermark and consolidates rather than
  re-summarizes

#### Scenario: Secrets are never distilled into the base

- **WHEN** the period's activity includes a password, token, or private key
- **THEN** the distillation records only IDs / methods / pointers, never the secret

#### Scenario: A missed knock is recoverable by pull

- **WHEN** a `distill` wake line does not reach the HQ pane (queue eviction, a wake
  outage) but the trigger was raised
- **THEN** the `[CONTROL gtmux:distill]` record is still in the event stream, so HQ's next
  delta pull surfaces the pending pass instead of it being silently lost

#### Scenario: A stale promotion is escalated, not forgotten

- **WHEN** a distill pass finds a promotion pending past its staleness floor
- **THEN** HQ raises it to the commander instead of letting the queue rot silently

### Requirement: HQ verifies perception self-heal before nagging or restarting

The seeded playbook SHALL teach HQ that a `feed-degraded` or `wake-degraded` wake
reports that gtmux's OWN mechanical self-heal has ALREADY run — it is a report, not a
request for HQ to restart anything. HQ SHALL first VERIFY by pulling the live
digest/events: when perception is actually fresh, HQ SHALL stay silent (record only)
and SHALL NOT repeatedly nag the user to restart. Only when the data is genuinely
stale/broken SHALL HQ act, and per the role boundary it SHALL restart nothing itself —
it SHALL dispatch a worker to restart the feed daemon and escalate to the user. (This
charter discipline is folded into the knowledge-distillation change so the seeded
playbook version bumps once; the code-side disk/feed hardening ships separately and
touches no playbook.)

#### Scenario: Fresh perception after a degraded wake stays silent

- **WHEN** a `feed-degraded` wake arrives but a pull of `gtmux digest`/`events` shows
  perception is current (the mechanical self-heal recovered)
- **THEN** HQ records it and stays silent — it does not nag the user to restart

#### Scenario: A genuinely broken feed is restarted via a worker, not by HQ

- **WHEN** after a degraded wake the pulled digest/events are genuinely stale/broken
- **THEN** HQ dispatches a worker to restart the feed daemon and escalates to the user,
  and never runs the restart command itself

### Requirement: The supervisor identity is precise — the stamp outranks the path

Supervisor-pane resolution SHALL prefer a pane carrying the `@gtmux_hq_home` stamp
(set by `gtmux hq` at spawn) over any pane that merely matches the home by current
or start path; the path criteria SHALL remain solely a fallback for a home with no
stamped pane. The radar SHALL grant `role:"supervisor"` only to the stamped pane
whenever one exists, so a worker session mistakenly parked in the HQ home remains
a normal, visible radar row — never hidden, never woken as the supervisor.

#### Scenario: A worker parked in the home does not steal the identity

- **WHEN** a stamped HQ pane exists and a worker session's cwd is also the HQ home
- **THEN** wakes resolve to the stamped pane and only it carries
  `role:"supervisor"`; the worker lists as a normal agent row

#### Scenario: A legacy home still resolves

- **WHEN** no pane carries the stamp (a home predating it)
- **THEN** the path-based fallback resolves the supervisor as before

### Requirement: The playbook teaches the identity boundary

The seeded playbook SHALL open with an identity self-check — only the session
launched by `gtmux hq` is the supervisor; an agent that received a dispatched task
and finds itself reading the charter was mis-spawned, must not adopt the charter or
spawn anything, and reports for re-dispatch — and its dispatch rules SHALL require
an explicit `--cwd <project dir>` on every spawn. The shipped playbook version
SHALL be bumped so existing homes receive the guard on the next `gtmux hq` after an
update.

#### Scenario: A mis-spawned worker reads the charter

- **WHEN** an agent with a concrete dispatched goal starts inside the HQ home
- **THEN** the charter itself instructs it to decline the supervisor role and
  report for re-dispatching instead of spawning

### Requirement: Capture is a verified step of the closed-loop turn

The seeded playbook SHALL upgrade HQ's closed-loop turn from `SENSE → JUDGE → REPORT`
to `SENSE → JUDGE → CAPTURE? → REPORT`, making knowledge capture a first-class step
rather than an out-of-loop good intention. A capture VERDICT SHALL be MANDATORY on
exactly three closure classes — `correction` (the commander corrects HQ), `crash` /
`StopFailure`, and `recurrence` (any footgun or fact hit a SECOND time) — because each is
a durable, cross-cutting lesson almost by definition. On a forced class the turn SHALL
emit exactly one verdict: either `⟣ 📓 captured: <topic>` (naming the knowledge topic
whose ledger it wrote through `gtmux knowledge add`/`supersede`: accounts | workflows |
best-practices | pitfalls | corrections), OR an explicit "nothing durable" clause stating
why the closure is not a reusable cross-cutting fact. For `done` and `resolved` closures
capture SHALL be OPPORTUNISTIC with a SILENT default — HQ captures and marks a genuinely
reusable fact if one surfaced, but SHALL NOT be forced to emit a verdict, because forcing
on those high-frequency closures would degrade into ritual noise and pressure filler
entries. The capturable criterion SHALL be `reusable ∧ cross-cutting (across sessions /
repos / tasks) ∧ not unique to this conversation`; pure this-task state (who is doing
what, a specific PR number) is board material, NOT a KB entry. Routine intermediate steps
and non-closure wakes (`tick`, `new-session`, `waiting`) SHALL NOT force a verdict. The
shipped playbook version SHALL be bumped so existing homes adopt the capture-verify on
their next managed-playbook upgrade.

#### Scenario: A commander correction forces a capture verdict

- **WHEN** HQ processes a `correction` closure
- **THEN** its turn emits either a `⟣ 📓 captured: <topic>` line naming the KB topic
  whose ledger it wrote (typically `corrections`), or an explicit one-clause "nothing
  durable" judgment — it cannot close the correction without one

#### Scenario: A recurrence forces a capture verdict

- **WHEN** the same footgun or fact is hit a second time
- **THEN** the recurrence forces a capture verdict, because a repeat proves the fact is
  cross-cutting and was not yet captured

#### Scenario: A completed goal is opportunistic, not forced

- **WHEN** HQ processes a `done` or `resolved` closure
- **THEN** it captures and marks a reusable fact only if one genuinely surfaced, and is
  otherwise silent — no capture verdict is forced

#### Scenario: A board note never counts as capture

- **WHEN** HQ records a lesson only to the situation board
- **THEN** the playbook does not accept that as a capture verdict — the verdict must name
  a KB topic written through the knowledge verbs (`⟣ 📓 captured: …`)

### Requirement: HQ consults the knowledge base as a hard precondition before advising or dispatching

The seeded playbook SHALL harden consultation from a soft suggestion into a HARD
precondition: before ADVISING the commander or DISPATCHING a task, HQ SHALL first consult
the relevant knowledge-base topic. When it advises, HQ SHOULD name the KB entry its advice
rests on. When NO KB entry covers the case, that gap SHALL itself be a capture trigger —
HQ captures the missing fact afterward so the next occurrence is covered. This
requirement SHALL NOT loosen the rule that HQ never answers another agent's
permission/plan/design choice.

#### Scenario: Advice is grounded in the base

- **WHEN** HQ gives the commander a recommendation about a repo or workflow the KB covers
- **THEN** it consulted the relevant topic first and names the entry its advice rests on

#### Scenario: A coverage gap becomes a capture

- **WHEN** HQ advises or dispatches on a matter no KB topic covers
- **THEN** it treats the gap as a capture trigger and records the fact afterward

### Requirement: The board and knowledge base have welded, non-interchangeable definitions

The seeded playbook SHALL weld the definitions so an ephemeral board note can never stand
in for durable capture. The SITUATION BOARD (`board.md`) SHALL be defined as HQ's
EPHEMERAL private posture — mode/source, priority, health, pending decisions, standing
context — which gtmux never reads back and HQ re-reads itself after a context reset. The
KNOWLEDGE BASE (`knowledge/`) SHALL be defined as the machine's DURABLE, cross-session,
reusable memory — accounts, workflows, best-practices, pitfalls, corrections. The playbook
SHALL state that the capture-verify routes a lesson ONLY into the KB, that "I noted the
board" can NEVER count as capture, and that the two may be written together but neither
substitutes for the other.

#### Scenario: The charter distinguishes ephemeral posture from durable memory

- **WHEN** the managed playbook is generated
- **THEN** it defines the board as ephemeral private posture and the KB as durable
  cross-session memory, and states that a board note is never a capture

### Requirement: gtmux capture records a distill candidate to the pending-distill spool

The system SHALL provide `gtmux capture "<one-line lesson> @<topic>"` — topic ∈ the
knowledge vocabulary: the built-in topics (accounts | workflows | best-practices |
pitfalls | corrections | environment) plus every topic DECLARED through
`gtmux knowledge topic` — as a PUBLIC command so that HQ OR ANY WORKER can record a
durable-fact CANDIDATE cheaply in the moment, widening the capture surface from a
single supervisor to the whole fleet. Capture and the knowledge verbs SHALL judge
topics through one shared validation, so the two entrances cannot drift. It SHALL be
safe to open this input because a candidate is NOT a KB entry: the distillation pass
(HQ's curation) is the quality gate. The command SHALL append one JSON line — the
lesson, the topic TAG, a DEDUP KEY (topic + a lesson slug, or an explicit key), and
AUTO-COLLECTED event context (current/related `pane_id`, the current event `seq`,
`task_id` if any, a timestamp) — to a pending-distill spool under the HQ home
(`~/.config/gtmux/hq/knowledge/.pending-distill.jsonl` or the state-dir equivalent).
`gtmux capture --list` SHALL render the pending queue. A missing or invalid `@topic`
SHALL be an error naming the current vocabulary. The spool SHALL drain candidate by
candidate, never by blind truncation: `gtmux knowledge add --capture <key>` consumes
EVERY pending line sharing that key (the merge of same-key candidates) into one entry
that inherits their provenance, and `gtmux knowledge dismiss --capture <key> --why`
removes them with a journal trace — so an accepted and a rejected candidate stop
vanishing identically. Being a public command it SHALL be documented per the
command-drift rule: the CLAUDE.md command list, a `docs/cli.md` section, and
`gtmux --help` (en+zh) — NOT the `check-design.sh` HIDDEN allowlist.

#### Scenario: A candidate is captured in one line with a dedup key

- **WHEN** any worker runs `gtmux capture "wrangler TLS-resets from the office; retry @pitfalls"`
- **THEN** one JSON line — lesson + topic tag `pitfalls` + a dedup key + auto-collected
  pane/seq/task/time context — is appended to the pending-distill spool

#### Scenario: A declared custom topic is capturable

- **WHEN** HQ has declared a `datasets` topic and a worker runs
  `gtmux capture "… @datasets"`
- **THEN** the candidate is accepted and tagged `datasets`, exactly as a built-in
  topic would be

#### Scenario: An invalid topic errors

- **WHEN** `gtmux capture` is called with no `@topic` or a topic outside the
  current vocabulary
- **THEN** the command errors naming the vocabulary and writes nothing

#### Scenario: Distill merges same-key candidates rather than duplicating

- **WHEN** two candidates share a (topic, dedup key) and a distill pass drains the spool
- **THEN** one `gtmux knowledge add --capture <key>` consumes both into ONE entry (no
  near-duplicate), the entry's provenance carries both sequences, and the remaining
  unrelated candidates stay pending

### Requirement: HQ echoes matching knowledge at dispatch time

At `gtmux spawn` / dispatch time, the system SHALL auto-echo the pitfalls / workflows
knowledge-base summary that matches the target repository (by the cwd's base name) and the
goal keywords, in the dispatch's advisory output (alongside the proxy / resource
preflight, so it is silent in `--json` mode), so captured knowledge is surfaced at the
moment work starts rather than left to HQ to recall each time. This is the tool-layer
mechanism that closes "captured but never used" — the payoff of the capture layers — and
is a first-class deliverable. The echo SHALL be bounded (a small, fixed line cap) and
read-only over HQ's own knowledge files. When no `pitfalls`/`workflows` entry matches, the
echo SHALL be a no-op (nothing surfaced, no error). (Injecting the summary directly INTO
the worker's pane is a natural extension, deferred to keep the verified-delivery payload
untouched.)

#### Scenario: A dispatch surfaces the repo's known footguns

- **WHEN** HQ dispatches work into a repo whose `pitfalls`/`workflows` topics have entries
  matching the cwd base name or a goal keyword
- **THEN** the matching KB summary is echoed in the dispatch's advisory output

#### Scenario: No match is a silent no-op

- **WHEN** a dispatch targets a repo/goal with no matching KB entry
- **THEN** nothing is echoed and no error is raised

### Requirement: Maintenance triggers are auditable from the event stream

Every gtmux-raised HQ maintenance trigger (`distill`, `self-check`) SHALL be appended to
the session-event journal as a control record carrying its event name, a human summary and
a severity, so that `gtmux events` answers "when did this last run?" without reading the
feed spool. The perception feed daemon SHALL spool these records on its normal journal-tail
path rather than receiving a separately written copy, so exactly one record exists per
raised trigger. The shared event formatter SHALL render a control record legibly (its
control tag, event name and summary) rather than as an empty lifecycle row.

#### Scenario: A raised trigger is queryable after the fact

- **WHEN** a user or HQ runs `gtmux events` over a window in which a distill pass was
  raised
- **THEN** the output contains a legible `[CONTROL gtmux:distill]` line with its reason,
  so "did the periodic pass run?" is answerable directly from the stream

#### Scenario: One record, not two

- **WHEN** a maintenance trigger is raised while the feed daemon is healthy
- **THEN** the record appears exactly once in the feed spool — appended to the journal and
  spooled by the daemon's tail, not written to both by hand

### Requirement: Maintenance staleness is visible in doctor

`gtmux doctor` SHALL report, in an HQ section shown only on a machine that actually has an
HQ home, the freshness of each periodic maintenance pass: `knowledge distill` against its
weekly floor and `HQ self-check` against its daily floor. A pass inside its floor SHALL read
OK; one past the floor but inside a grace window SHALL read as a neutral note (the
zero-change gate legitimately skips quiet periods); one past floor plus grace SHALL be
flagged, because the cadence itself has stopped. A pass that has never run SHALL read as an
informational "never run", not a failure.

The same section SHALL additionally report **HQ session health** — the sensed context
occupancy, session age and turn count of the supervisor's own session — flagged when a
rotation breach is standing. With no live HQ pane, or with no session resolvable for it, the
row SHALL be informational rather than a failure: an absent supervisor is not a degraded
one.

Without these rows a stalled ritual is invisible: both passes are silent by design, so "it
has not distilled in three weeks" is indistinguishable from "nothing needed distilling", and
a session too heavy to judge looks exactly like a quiet one.

#### Scenario: A slipped distill is visible at a glance

- **WHEN** no distill pass has run for longer than its weekly floor plus the grace window
- **THEN** `gtmux doctor` reports the distill row as needing attention, with the age of the
  last pass

#### Scenario: A healthy cadence reads as healthy

- **WHEN** the last distill is within its floor
- **THEN** `gtmux doctor` reports the row as OK with the age of the last pass

#### Scenario: A never-run pass is not an error

- **WHEN** an HQ home exists but no distill has ever been raised
- **THEN** the row reports "never run" as a neutral note rather than a failure

#### Scenario: A stalled cadence is flagged

- **WHEN** no `self-check` pass has been raised for over a day plus its grace window
- **THEN** `gtmux doctor` flags the `HQ self-check` row with a hint to check that
  `gtmux serve` is running with a live HQ

#### Scenario: A heavy HQ session is flagged

- **WHEN** the HQ session is past a rotation threshold and has not rotated
- **THEN** the `HQ session health` row is flagged and names the breached figures

#### Scenario: No HQ running is not a health failure

- **WHEN** `gtmux doctor` runs on a machine with an HQ home but no live HQ pane
- **THEN** the `HQ session health` row is informational, not a warning

### Requirement: The playbook teaches the consumption watermark

The seeded playbook SHALL teach the perception MODEL, not merely the new class, because the
change is to what HQ may assume about its own awareness. It SHALL state that:

- wake classes are PRIORITY labels — what to read first — and never the set of things HQ
  can know about;
- gtmux tracks HQ's consumption watermark and knocks `unread` for anything past it, so HQ
  never has to remember to poll: an unconsumed event knocks again, and keeps knocking,
  until it is read;
- an `unread` line carries a count and no importance claim, so HQ pulls and judges as
  usual, and a repeat of the same count means HQ did not actually consume;
- its own unfiltered `--since-seq` delta read IS the writeback, while a filtered read and a
  skip-ahead read are not, and `gtmux events --ack <seq>` is the explicit form for a stream
  reconciled another way.

Per-class instructions to remember to look for a particular missed trigger SHALL be
retired in favour of that general guarantee. The version SHALL be bumped so existing homes
adopt it.

#### Scenario: The playbook separates priority from coverage

- **WHEN** the HQ home is seeded or upgraded
- **THEN** the playbook states that a class says what to look at first rather than what
  exists, and that anything unconsumed re-knocks until read

#### Scenario: The playbook names what counts as consumption

- **WHEN** the playbook covers the wake→pull loop
- **THEN** it states that the unfiltered `--since-seq` read is the writeback, that a
  filtered or skip-ahead read is not, and that `gtmux events --ack` is the explicit form

### Requirement: HQ consumption lag is visible in doctor

`gtmux doctor` SHALL report, when an HQ home exists, how far HQ's consumption watermark is
behind the event stream and how long that backlog has been standing, flagging it as needing
attention past either threshold (20 events or 30 minutes). A watermark that has never been
recorded SHALL read as a neutral note rather than as a failure.

This exists because the wake channel is silent in both directions: a knock that lands
leaves no trace on any screen the user reads, and neither does one that never lands — so a
supervisor that has stopped consuming is otherwise indistinguishable from a fleet where
nothing has happened, and the only remaining detector is the user noticing that a finished
job went unremarked.

#### Scenario: A supervisor that stopped consuming is visible

- **WHEN** HQ has not advanced its watermark for longer than the standing threshold while
  events accrue
- **THEN** `gtmux doctor` reports the event-consumption row as needing attention, naming
  how far behind and for how long

#### Scenario: A normal in-flight delta is not a fault

- **WHEN** a handful of events have accrued in the last minute
- **THEN** the row reads OK — the backlog is what the knock is for

#### Scenario: A fresh HQ is not overdue

- **WHEN** an HQ home exists but HQ has never pulled the stream
- **THEN** the row is a neutral note rather than a warning

### Requirement: The playbook teaches unattended self-rotation

The seeded playbook SHALL teach HQ that its OWN session degrades, that the degradation is
not something more care can prevent, and that recovering from it is HQ's job and not the
user's.

It SHALL name the concrete failure this rule exists for: in a long, near-full session HQ can
read its OWN prior output as input that arrived from outside, and the event stream is the
arbiter — `UserPromptSubmit` on the HQ pane is the user; `Stop` on the HQ pane is HQ itself.
Two records that read identically as text are different acts, and only the event type
distinguishes them.

On a `self-rotate` wake the playbook SHALL prescribe three steps, in order, with the user
NOT in the loop for any of them:

1. **Make the handoff durable first** — bring the situation board and the knowledge base
   current, because they are the successor session's entire briefing; anything not written
   there does not survive.
2. **Hand off** — record in the board what is in flight, what is owed, and what the
   successor must not re-derive.
3. **Rotate** — `gtmux hq --rotate`, then re-read the board before acting again.

The playbook SHALL state that a repeated `self-rotate` knock after a rotation means the
rotation did not take (the session id did not change), not that a second rotation is owed.

#### Scenario: HQ rotates without being told to

- **WHEN** a `self-rotate` wake arrives
- **THEN** HQ updates board + knowledge base, records the handoff, and runs
  `gtmux hq --rotate` — without escalating the decision to the user

#### Scenario: The playbook names the self-as-input failure

- **WHEN** a reader opens the seeded playbook
- **THEN** it explains that a `Stop` record on the HQ pane is HQ's own output and must never
  be read as the user's instruction

### Requirement: HQ reads and answers in the attention grade, and gates its prints by it

The supervisor SHALL read a wake line's ATTENTION GRADE first — it says how loudly the line
should land before a word of it is read — and SHALL answer in the SAME grade, so the pane
reads as one scale rather than two vocabularies.

The print gate SHALL be grade-explicit. LEDGER-grade content SHALL go to the situation
board and the attention ledger and NOT to the screen; ATTENTION-grade content SHALL print
when the surfacing threshold allows it; DECISION-grade content SHALL always print.
Recording is not reporting: a thing written to the board has been dealt with, and repeating
it on screen is noise rather than service — this is the bulk of what previously printed as
prose and made the screen unreadable.

A periodic brief SHALL name only what CHANGED since the last one. The unchanged part of the
fleet SHALL be a one-clause pointer, never a re-listing: a brief that repeats itself teaches
its reader to skip briefs.

#### Scenario: Bookkeeping is recorded, not announced

- **WHEN** HQ judges a routine outcome that needs nothing from the commander
- **THEN** it is written to the board or ledger and NOT printed to the pane

#### Scenario: A decision is always heard

- **WHEN** a decision-grade signal arrives while the surfacing threshold is quiet
- **THEN** HQ still prints it

#### Scenario: A brief carries the delta

- **WHEN** a periodic brief is due and most of the fleet is unchanged since the last one
- **THEN** the brief names the changes and reduces the unchanged remainder to one clause

### Requirement: The HQ verdict is decided once, in the core

The system SHALL resolve the supervisor's overall verdict in the CORE and expose it on the
digest, so every surface reads one judgment rather than re-deriving its own.

The verdict SHALL be a priority-ordered state: the supervisor is ABSENT; else the
supervisor itself is waiting on the user; else one or more workers are waiting; else the
machine is at its critical resource tier; else the supervisor is working; else normal. The
ordering is part of the contract — a surface SHALL NOT re-order it.

The core SHALL NOT serve a rendered headline sentence. The headline is user-facing prose
and each surface owns its own language state, so a pre-rendered string would override a
reader's language choice. The core SHALL instead serve the FACTS a headline is built
from — how many workers are waiting, who has waited longest, how many others are normal,
how many worker sessions exist — and each surface SHALL render its own sentence from the
state and those facts.

The object SHALL be additive and OPTIONAL, so a surface built against an older core keeps
working; a surface SHALL keep a local fallback for when it is absent.

#### Scenario: A red resource tier is never reported as normal

- **WHEN** the machine is at its critical resource tier and no agent is waiting
- **THEN** the served verdict is the resource state, and no surface can render "all
  normal" beside a red token — the contradiction that existed while each surface decided
  for itself

#### Scenario: The supervisor's own call outranks a quiet fleet

- **WHEN** the supervisor itself is waiting on the user while every worker is idle
- **THEN** the served verdict says the supervisor needs a decision, on every surface

#### Scenario: An older core is tolerated

- **WHEN** a surface receives a digest with no verdict object
- **THEN** it falls back to its local resolver rather than showing nothing

### Requirement: The seeds and playbook speak to any user

The seeded knowledge scaffold and the managed playbook SHALL be written for an
ARBITRARY user, not for gtmux's author or this machine's history:

- **Seeds are domain-neutral templates.** A topic seed describes WHAT belongs
  there, never WHOSE accounts, stacks, or workflows; no seed ships pre-filled
  lessons, undefined internal markers, or a particular vendor's names as the
  canonical examples.
- **Evidence never impersonates the reader.** The playbook may cite a measured
  incident as mechanism-plus-history ("a real dispatch died exactly this way,
  during development, …"), and SHALL NOT phrase it as the reader's own past
  ("a dispatch of yours", "already in your own knowledge base") — on a fresh
  machine such claims are false and an HQ that believes them will recount
  history that never happened there.
- **The board seeds in the user's language** (`GTMUX_LANG`), and the language
  discipline taught is "keep the board in the language it was seeded in — never
  flip an existing board", preserving the don't-machine-translate lesson
  without mandating any particular language.
- **Enforced, not reviewed back in:** a test SHALL ban the known author-leak
  tokens from the seeds and the playbook, so a neutrality regression is a red
  build.

#### Scenario: A fresh user's first day is their own blank page

- **WHEN** `gtmux hq` seeds a brand-new home
- **THEN** every topic file is a template describing its purpose, no seed
  carries another person's lessons or vendor list, and the board's headings are
  in the user's language

#### Scenario: A leaked author token fails the build

- **WHEN** a banned author-specific token is reintroduced into a seed or the
  playbook
- **THEN** the ban-list test fails
