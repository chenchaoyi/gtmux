## Why

Adding a coding agent today means editing ~12 independent, agent-keyed sites (driver caps,
radar profile, hook installer + format, hook-event semantics, transcript parser, resume
command, resource attribution, prompt/ready signatures, hook display name, menu-bar mark +
icon, mobile mark + icon) — with **inconsistent keys** across them (driver key `claude` vs
display `Claude Code` vs command `claude` vs profile `Name`). There is no single place that
says "this is agent X"; the knowledge is scattered and each new agent re-discovers the same
traps (identity from the process subtree not `pane_current_command`; a hook that must be
installed or the whole event layer stays dark; locale/glyph loss over launchd PTYs). The
concrete trigger is **opencode**: it is the only whitelisted agent with no installer entry
and no transcript parser, and onboarding it exposed that we have no repeatable process.

## What Changes

- **New per-agent MANIFEST as the single source of truth.** One `AgentManifest` per agent
  (key + aliases, display name, launch/detect commands, idle glyph, icon, resume command,
  resource-attribution names, hook-install spec, event-semantics table, transcript-parser
  ref, driver capabilities, prompt/ready signatures) registered in one place.
- **Each subsystem DERIVES its table from the registry** instead of keeping its own
  agent-keyed map. The scattered lists in `driver`, `radar`, `hook`, `app/agent_hooks`,
  `resume`, `resource`, `prompt` read the registry; behavior is preserved (pure move +
  re-wire), enforced by the existing tests.
- **Two explicit support TIERS** a manifest declares: **Tier 1 — event parity** (an install
  spec so `waiting/done/receipt/dispatch-verify/notifications` light up) and **Tier 2 —
  digest parity** (a transcript parser for `goal/last/ask`). A manifest may ship Tier 1 only.
- **A hook-install spec that admits a PLUGIN model, not only JSON command-hooks.** The five
  wired agents write a JSON "run command on event" file; opencode's extensibility is
  JS/TS plugins only. The install spec generalizes to "materialize this integration
  artifact" (a JSON hooks file OR a JS plugin that shells out to `gtmux hook`).
- **opencode onboarded through the new registry (Tier 1)** as the acid test that one manifest
  + one registration is enough. Its idle glyph + icon land, and `install-hooks --agent
  opencode` writes a plugin subscribing to `session.idle → Stop`, `permission.asked →
  PermissionRequest`, `session.created → SessionStart`, `message.updated(user) →
  UserPromptSubmit`, etc.
- **A documented onboarding PLAYBOOK** (`docs/design/agent-onboarding.md`): the step-by-step
  process to add an agent + a **pitfalls checklist** capturing every trap we have paid for.
- **A conformance check** (in `check-design.sh`): every hook-equipped agent has a manifest,
  and no subsystem reintroduces a private agent-keyed list the registry should own.

## Capabilities

### New Capabilities

- `agent-integration`: how a coding agent is defined to gtmux — a single per-agent manifest
  that is the source of truth for every subsystem, the Tier 1 / Tier 2 support contract, the
  install-spec generalization (JSON hooks or JS plugin), and the documented onboarding
  playbook + pitfalls checklist that keep new integrations from re-hitting known traps.

### Modified Capabilities

<!-- None: existing capabilities' observable REQUIREMENTS do not change — the radar still
     detects, dispatch still verifies, notifications still fire. This change moves the
     per-agent knowledge those specs assume into one registry and documents the process.
     opencode gains support, but "support a specific agent" was never a spec-level
     requirement (the specs are agent-agnostic by design). -->

## Impact

- **Consolidated:** `internal/driver/driver.go`, `internal/radar/agents.go`,
  `internal/app/agent_hooks.go` (+ `codex_hooks.go`, `hooks.go`), `internal/hook/hook.go` +
  `classify.go`, `internal/transcript/*.go`, `internal/resume/command.go`,
  `internal/resource/attribute.go`, `internal/prompt/prompt.go` → an
  `internal/agents` registry (or equivalent) they all read.
- **New:** `docs/design/agent-onboarding.md` (playbook + pitfalls checklist); an opencode
  manifest + plugin installer.
- **Docs/conformance:** `CLAUDE.md` gains a pointer to the playbook; `scripts/check-design.sh`
  gains a manifest-completeness check.
- **Surfaces (Swift/RN):** out of scope for the Go registry, but the playbook documents the
  two mark/icon sites so they aren't forgotten; a follow-up may feed them from
  `agents --json`.
- **Contracts preserved:** `agents --json`, `digest --json`, hook event names, state paths —
  all unchanged (registry is internal).
