# Onboarding a coding agent

How to teach gtmux about a new coding agent (Claude Code, Codex, Gemini, Cursor,
opencode, …) — the process, the one place identity lives, and the pitfalls that have
cost real time. Read this before wiring a new agent; follow the checklist at the end
before calling it done.

Spec: `openspec/specs/agent-integration/spec.md`. Registry: `internal/agents`.

---

## 1. The mental model: three support tiers

gtmux supports agents in tiers. Pick the tier you're targeting — you do not have to do
all of it at once, and every tier degrades gracefully to the one below.

| Tier | What lights up | What it needs |
|---|---|---|
| **0 · Sensed** | The agent shows in the radar (row, status via title glyph / subtree), can be focused, typed into, resumed. | A **manifest** with detection commands (+ optionally an idle glyph, icon, resume argv). |
| **1 · Event parity** | `waiting`/`done` detection, receipt-verified `send`/dispatch, notifications, HQ wakes. | Tier 0 **plus** a **hook installer** so the agent emits `UserPromptSubmit`/`Stop`/`PermissionRequest`/… into gtmux's event stream. |
| **2 · Digest parity** | The deterministic digest (`goal`/`last`/`ask`) for the agent, HQ chief-of-staff view. | Tier 1 **plus** a **transcript parser** for the agent's session log. |

**Graceful degradation is a hard rule (invariant I2):** a missing capability must never
regress an agent below the tier beneath it. No transcript parser → the digest falls back
to its screen-derived form. A hook event that never arrives → verification falls to the
two-frame screen read, exactly as for a hook-less agent. *Absence of evidence is not
failure.* Never write code that hard-fails on a capability an agent doesn't provide.

---

## 2. Identity lives in ONE place: the registry

Historically each subsystem kept its own agent-keyed list, with inconsistent keys and
drifting membership. Now there is **one `Manifest` per agent** in `internal/agents`, and
each subsystem *derives* its list from it. Adding an agent's identity is authoring one
manifest.

```go
// internal/agents/registry.go
type Manifest struct {
    Key, Label      string   // "claude", "Claude Code"
    Aliases         []string // alternate keys that resolve here (cursor-agent → cursor)
    Detect          []string // radar process-subtree commands; "" ⇒ no radar profile
    IdleGlyph, Icon string   // the idle marker its TUI paints; a vendor app path (no bundled trademark)
    Resume          []string // resume argv; nil ⇒ not resumable by session id
    Resource        string   // resource-attribution name; "" ⇒ not attributed
    HookDisplay     bool     // registered in the hook-time known-agent gate
    Hooked          bool     // events feed the receipt/ready stream (Tier 1)
    Content         string   // transcript-parser key (Tier 2); "" ⇒ none
    Headless        string   // headless one-shot key; "" ⇒ none
    Semantics       bool     // has a DEDICATED classifier event table (else the generic one)
}
```

These subsystems already read the registry — **you do not edit them**:

| Concern | Accessor | Consumer |
|---|---|---|
| Hook-equipped set | `agents.HookEquippedKeys()` | `internal/driver` |
| Radar detection profiles | `agents.Profiles()` | `internal/radar` |
| Resume command | `agents.ResumeArgv()` | `internal/resume` |
| Resource attribution | `agents.ResourceNames()` | `internal/resource` |
| Hook display name | `agents.DisplayNames()` | `internal/hook` |
| Transcript / headless keys | `agents.Content/HeadlessKeys()` | `internal/driver` |

The registry's data is pinned by golden tests in `internal/agents/registry_test.go` (copied
verbatim from the legacy maps) and by per-subsystem migration-guard tests.

### What stays domain-local (and why)

Three things are **behavior**, not identity, and stay in their own package — keyed by the
registry's agent key, not moved into the pure-data registry (moving a domain enum into a
data package is over-abstraction):

- **Event semantics** — the native-event → gtmux-semantic table — `internal/hook/classify.go`
  (`agentEventSemantics`, else the generic table).
- **Prompt / ready signatures** — boot banners, prompt/selector glyphs — `internal/prompt`.
- **Hook install spec** — the file/plugin gtmux writes — `internal/app/agent_hooks.go`.

The **conformance check** (`scripts/check-design.sh`) ties these back to the registry so
none is forgotten: every `Hooked` agent must have an install spec, etc.

---

## 3. Step by step

### Step 0 — Manifest (always)

Add one entry to `manifests` in `internal/agents/registry.go`. Fill `Key`, `Label`,
`Detect` (Tier 0). Add `Resume` if it can relaunch a session by id. Run
`go test ./internal/agents/` — the golden tests will tell you if you disturbed an existing
agent. That's Tier 0: the radar detects it, focus/send/resume work.

### Step 1 — Hook installer (Tier 1)

Set `Hooked: true` and `HookDisplay: true`. Then wire an **install spec** so the agent
emits events. Two extension models exist — check which the agent supports:

- **Command-hook model** (Claude, Codex, Cursor, Gemini, Copilot, Kiro): the agent reads a
  JSON/TOML config that runs a shell command on lifecycle events. Add an
  `agentInstaller` entry (`internal/app/agent_hooks.go`) mapping each native event to a
  gtmux token: `beforeSubmitPrompt → UserPromptSubmit`, `afterAgentResponse → Stop`,
  the approval event → `PermissionRequest`, session start/end. Pick or add a `format`.
- **Plugin model** (opencode): the agent has NO command-hook file — only JS/TS plugins.
  The installer writes a small plugin that subscribes to the agent's events and shells
  out to `gtmux hook --agent <key> <event>`. Same `install-hooks --agent <key>` entry
  point; the plugin is a `dedicated` artifact removed cleanly on uninstall.

Map the agent's native events onto gtmux's: `UserPromptSubmit`, `Stop`, `PermissionRequest`
(a real user-facing approval → `waiting`), `PostToolUse`/resolve (clears `waiting`),
`SessionStart`, `SessionEnd`, `PreCompact`/`PostCompact`. If the agent's approval signal is
a *separate* event from its pre-tool event, give it a **dedicated** semantics table
(`agentEventSemantics`, `Semantics: true`) so the pre-tool event stays telemetry; if its
only signal is the pre-tool event, the generic table's `semToolStartMaybeApproval` escalates
side-effecting tools for you.

Verify identity resolves from the **process subtree**, not the foreground command (see
pitfalls). Install, drive a real session, confirm `waiting`/`done` and a receipt-verified
`gtmux send` (`judged_by: driver`).

### Step 2 — Transcript parser (Tier 2)

Add `internal/transcript/<agent>.go` reading the agent's session log into `[]Turn`, set the
manifest's `Content` key (that alone auto-wires `driver.Content` — see `agents.ContentKeys()`),
and add the `resolveLog` + `normalizeAgent` cases. Now the digest renders `goal`/`last`/`ask`.
The pane→session mapping is free: the hook writes a `resume` record from the session id, and
`sessionRef` reads it — so a resumable agent whose hook receives the session id needs no extra
wiring.

**If the agent keeps NO readable transcript on disk** (opencode 1.18.x persists only a
`session_diff`, not messages), gtmux keeps its OWN: the plugin streams the user prompt and the
final assistant text through `gtmux hook` (piping `{session_id, prompt}` / `{session_id,
assistant}` on stdin), the hook appends them via `transcript.AppendOpencode` as
`{timestamp, role, text}` JSONL under `~/.local/share/gtmux/octrans/<session>.jsonl`, and the
parser reads that. Two subtleties paid for: (a) assistant text arrives as a STREAM of
`message.part.updated` events (`part.text` is the full text so far) — accumulate per
message-id and flush the newest on `session.idle`; (b) key the file by the agent's own session
id (piped alongside the prompt) so it lines up with the `resume` record `sessionRef` resolves.

---

## 4. Pitfalls checklist (every trap we've paid for)

- [ ] **Identity from the process SUBTREE, never `pane_current_command`.** Claude Code
  renames its process to its version (`2.1.220`); several agents run as bare `node`. Keying
  identity off the foreground command mis-detects the agent and silently disables the
  receipt path. Use `radar.AgentDriverKey` (walks the subtree). *(Cost a multi-day
  "send stuck forever" hunt.)*
- [ ] **The hook must be INSTALLED or the whole event layer stays dark.** Being in the
  driver's hook-equipped set is necessary but not sufficient — without an installer that
  actually wires the agent's config/plugin, it emits no events and Tier 1 is a no-op
  (opencode was in the whitelist for months with no installer).
- [ ] **Plugin vs command-hook model.** Don't assume a JSON "run a command on event" file
  exists. opencode is plugin-only; forcing it into a command-hook format fails.
- [ ] **A plugin that shells `gtmux hook` MUST redirect its stdin (`< /dev/null`).** A JS
  plugin's subprocess inherits the AGENT's controlling TTY as stdin, and `gtmux hook`
  drains stdin — `io.ReadAll` on a TTY never EOFs, so the hook hangs in the agent's
  foreground process group and **steals its keyboard input** (opencode's composer went
  dead after the first send; every subsequent `gtmux send` silently failed `not confirmed`).
  `gtmux hook` now guards this (`stdinIsTerminal` skips a char-device stdin), but the
  plugin must still redirect so an old binary is safe. Pipe-fed calls (`echo … | gtmux
  hook … UserPromptSubmit`) are already safe — the pipe, not the TTY, is stdin. *(Cost a
  full debugging session; the tell is a `gtmux hook` process stuck in state `S+`.)*
- [ ] **Locale/glyph loss over daemon-spawned PTYs.** A `launchd`-spawned `gtmux serve` has
  no `TERM`/locale, so a PTY it spawns mangles CJK and TUI glyphs to dashes/`_`. Force
  `-u` + `LC_CTYPE` and pass `TERM`. This also breaks the radar's glyph classification.
- [ ] **Sparse events fall back to Layer 1 — that's fine.** A low-event-density agent (Codex)
  just lowers the receipt hit rate; `NoEvidence` falls to the two-frame screen read. Do not
  treat a missing event as a failure.
- [ ] **Idle-glyph classification needs LIVE confirmation.** A leftover title glyph on a
  dead shell must NOT classify as a running agent — the classifier requires the process to
  be live (or the subtree to match), not just the title.
- [ ] **Agent icons: committed built-in, OR the vendor's installed app, else a letter mark.**
  §6 now permits a committed official mark for *identification* (nominative use) — drop
  `<key>.png` in `assets/agent-icons/` with provenance in `SOURCES.md`; the serve hands it
  to every surface via `/api/icon`. For agents with a desktop app you can instead point
  `Icon` at `/Applications/<App>.app`. **Gotcha:** the mobile only fetches `/api/icon` when
  `agents --json` reports a NON-EMPTY `icon`, so `radar.IconFor` returns a `builtin:<key>`
  hint whenever a committed icon exists even though the profile `Icon` path is empty —
  without that the phone shows the monogram despite the icon shipping. *(This is exactly
  how opencode showed "OC" until fixed.)* The local menu-bar surface still resolves icons
  itself (install-time drop into `~/.config/gtmux/icons`), so a committed icon reaches it
  only after that follow-up lands.
- [ ] **Approval event vs pre-tool event.** If the agent raises a distinct approval event,
  keep its pre-tool event as telemetry (dedicated table). Otherwise every tool would flag
  "needs you," or real approvals would be dropped (Kiro's lowercase events must be
  registered explicitly).
- [ ] **Read-only tools never flag "needs you."** `sideEffectingTools` in `classify.go` is
  the allowlist; keep read-only tools (Read/Grep/Glob/…) out of it.
- [ ] **Keys must be consistent.** One canonical `Key`; use `Aliases` for alternate command
  names (cursor-agent → cursor). Don't invent a per-subsystem key.

### The two surfaces the Go registry does NOT feed (yet)

The menu-bar and mobile apps keep their own agent mark/icon maps — **remember to update
both** when adding an agent:

- Menu-bar: `macapp/Sources/GtmuxBar/Components.swift` (`agentMark`, `AgentIcons`).
- Mobile: `mobileapp/src/ui/agentMark.ts`.

(A future change may feed these from `agents --json`; until then they are hand-kept.)

---

## 5. Done checklist

- [ ] Manifest added; `go test ./internal/agents/` green (golden tests undisturbed).
- [ ] Tier targeted is actually wired (installer for Tier 1, parser for Tier 2).
- [ ] Identity verified via the process subtree on a real pane.
- [ ] `install-hooks --agent <key>` writes the integration; uninstall removes only what
  gtmux wrote, leaving user config intact.
- [ ] A live session drives `waiting`/`done` and a receipt-verified `gtmux send`
  (`judged_by: driver`) — for Tier 1.
- [ ] Menu-bar + mobile mark/icon updated.
- [ ] `make check` + `scripts/check-design.sh` green.
- [ ] CLAUDE.md / `docs/cli.md` mention the agent if it's user-facing; spec updated if
  behavior changed.
