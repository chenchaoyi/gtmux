# gtmux capability review

**English** · [中文](capability-review.zh.md)

## 1. Scope and current foundation

Repository review against `c7c7c301` plus mobile-new-session, 2026-10-01. This covers the whole product, not just mobile screens. Recommendations below are not implemented by this feature. No release/installation is included. Simulator checks do not replace unlocked physical iPhone/iPad acceptance.

| Product area | Already present | Remaining opportunity |
|---|---|---|
| Agent observation | Radar/digest, driver facts plus screen fallback, tmux/native and desktop distinctions | Make an unexpected state explainable and reproducible |
| Work control | Send/focus/attach, spawn/reap, worktrees and model choice; new paired-app shell creation | Project-based startup and an explicit delivery/acceptance lifecycle |
| HQ | Wake/watchdog, durable relay, pending decisions, charter and personal LOCAL.md | Clear authorization summary and visible task outcomes |
| Knowledge | Candidate ledger, evidence, promotion/landing, lexical retrieval, distribution, export/migration | Demonstrate that recalled lessons improve actual work |
| Connectivity | Owner/guest roles, routes, multiple saved Macs, per-Mac push preferences | Unified triage and clear reasons when a function is unavailable |
| Continuity | Resurrect, agent resume, CLI restore pick/plan, records migration | Named work sets and an understandable recovery preview |
| Operations | Structured diagnostics/events, doctor bundle, resource/usage monitoring | User-triggered state reports and end-to-end action results |

## 2. Ranked opportunities

### P1 — Explain and correct agent state

**Evidence:** `internal/driver/driver.go` already separates confirmed evidence from fallback; `internal/radar/digest.go` exposes perception tiers; `internal/radar/agents.go` has replayable capture seams. This is not a missing state engine. The repeated warning/desktop/HQ cases show the value of exposing its evidence.

**First slice:** a small Why this state view with source, observation time and freshness; Report incorrect state captures a bounded, redacted evidence window for a local diagnostic bundle. Turn confirmed reports into cross-agent replay fixtures. Measure transition delay and false waiting/completion events on those fixtures. Do not upload transcripts by default or add a second classifier in each UI. Medium effort; high trust benefit across all five surfaces.

### P1 — Start work by project, consistently across surfaces

**Evidence:** `internal/app/spawn.go` supports cwd, agent, model, goal file and worktree. Mobile creation currently opens a home-directory shell; no remote spawn endpoint exists in the reviewed contract.

**First slice:** Mac-configured project/agent presets, a task field and a target-Mac review, backed by a structured owner request that reuses spawn's readiness/receipt checks. Reuse the same presets in CLI/menu bar/phone/iPad/web instead of five launch flows. This removes manual cd/agent startup and inconsistent project defaults. Medium effort. Do not interpolate task text into shell commands.

### P1 — HQ task delivery and acceptance, beyond a stopped turn

**Evidence:** `internal/dispatch/ledger.go` records intent/delivery/disposition; `internal/hq/taskscmd.go` joins live state; `radar.TaskStatusFor` maps idle to done, explicitly meaning ready for review. Relay and advice already have ledgers. They must be reused, not replaced.

**First slice:** associate a task with its result summary, PR/artifact references, validation evidence and a distinct accepted/rework decision. Show the same task from dispatch through review, even after its pane disappears. A stopped agent turn is not proof that the goal passed. High coordination value; medium effort. Acceptance is user/HQ judgment within existing authorization, never inferred merely from idle.

### P2 — Understandable HQ authorization and execution summary

**Evidence:** HQ's `AGENTS.md` charter and never-overwritten `LOCAL.md` govern behavior (`internal/hq/hq.go`); relay explicitly reserves user decisions. This is existing authorization expressed mostly as instructions, not an absent permission system.

**First slice:** show the current allowed actions, source of authorization and active budget; offer reviewable presets for observe/report, routine replies and dispatch. Record which rule an HQ action used. Do not silently authorize publishing, deletion or credentials. CLI command gates can enforce the channels they own; a profile alone cannot confine an agent with unrestricted shell access. This boundary must remain explicit. Medium-to-large effort; reduces repeated confirmations and mistaken delegation.

### P2 — Measure knowledge usefulness, then improve recall

**Evidence:** `openspec/specs/hq-knowledge/spec.md` and `internal/knowledge` already provide shared lexical retrieval, dispatch-time echoes, provenance and settlement. Search and source recording are already built.

**First slice:** curate real task/query examples with expected entry IDs and exclusions; record recalled entry IDs against tasks and whether they helped, conflicted or were rejected. Evaluate relevance, stale advice and missing context before adding semantic recall or model reflection. Keep inferred judgments distinct from verified evidence and settle corrections through the current ledger. Medium effort; helps HQ become a better personal assistant without growing an unverified archive.

### P2 — One needs-attention queue across Macs

**Evidence:** `mobileapp/src/state/AppContext.tsx` stores many Macs and per-Mac push choices, but one `activeUrl`; AgentsContext follows that Mac.

**First slice:** a foreground opt-in queue labelled by Mac, task and last verification, with offline items visibly stale. Opening an item switches to its Mac before selecting the pane. Do not replace existing notification preferences or permanently stream every server in the background. Medium effort; strongest for users already operating several Macs.

### P2 — Named work sets and recovery previews

**Evidence:** `internal/app/restore.go` already has CLI pick/plan/dry-run and selective recovery; `internal/resume` resolves agent conversations. Records migration already handles portable knowledge separately from machine-local state.

**First slice:** save a named project work set and preview what will resume, where it will run and what is unavailable. Expose selective restoration through the app/menu bar, retaining explicit user choice and local prerequisites. Do not revive the old Mac's board during knowledge migration. Medium-to-large effort; high value after reboot or moving machines.

## 3. Supporting engineering and sequence

Feature unavailability should have a clear reason: offline Mac, guest access, old server or missing tmux. An additive operation-capability document can avoid discovering unsupported functions only after submission. This is supporting usability, not the main product direction.

Before remote batch launch/restore, harden cross-process coordination. `internal/state/runlock.go` reads then writes a PID file and treats write failure as acquired; it is a courtesy guard, not atomic exclusion. Reproduce concurrent acquisition in isolation before choosing a real lock. New-session tags survive response loss and serve restart while the session remains live, but concurrent separate serve processes and destroyed sessions remain outside its guarantee. No claim is made that a user's restore has duplicated today.

Recommended sequence: **state trust → project start → HQ result review**, then authorization/knowledge improvements, with multi-Mac and work-set recovery prioritized by actual usage. Preserve one core, one status vocabulary and one Workspace; add no second iPad flow or replacement knowledge database merely to expand scope.
