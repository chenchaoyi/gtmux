# Multi-agent + Multi-terminal — design (2026-06-18)

Design for the two scope items the maintainer opted into on 2026-06-18. Review
this before implementation; each section ends with a sequenced plan.

## Scope (decided)

- ✅ **Multi-agent** — surface and act on agents beyond Claude Code (Codex, …).
- ✅ **Multi-terminal HOST** — tmux running under a **non-Ghostty** terminal
  (iTerm2 / kitty / WezTerm / Apple Terminal).
- ❌ **NATIVE (non-tmux) agents** — still **deferred**. Different problem; do not
  conflate. The `source/project/terminal/tab` fields stay latent groundwork.

Invariant that holds throughout: **detection is already terminal-agnostic and (as
of PR #32) process-driven.** Only the "remote" side and the "needs-input/notify"
side carry the Claude/Ghostty assumptions we're generalizing.

---

## A. Terminal driver (multi-terminal host)

### Problem
`internal/ghostty` hardcodes Ghostty AppleScript for the four "remote" ops:
`FocusTab`, `IsViewing`, `OpenWindow`, `SpawnTabs` (used by `focus` / `restore` /
`new`). `agents` / `overview` are already terminal-agnostic, so nothing there
changes.

### Interface
```go
// internal/terminal
type Terminal interface {
    Name() string
    FocusTab(session string) error    // bring the tab titled "<session> — …" to front
    IsViewing(session string) bool     // is that tab the frontmost/active one
    OpenWindow(command string) error   // new terminal window running command
    SpawnTabs(sessions []string) error // restore: one tab per session
}
```
`internal/ghostty` becomes the first impl (no behavior change). A registry +
`Resolve()` returns the active driver.

### Host detection (the crux)
`focus`/`restore` are invoked by the **menu-bar app or a tmux keybinding**, which
have no `$TERM_PROGRAM`. Resolve the host terminal by the **tmux client's process
ancestry** (same trick that cleanly separated native vs tmux earlier):
`tmux list-clients -F '#{client_pid}'` → walk the parent chain → the terminal
app bundle (`…/Ghostty.app/…`, `…/iTerm.app/…`, `kitty`, `wezterm-gui`, …).
- Cache the result (re-detect on miss).
- Overrides: `GTMUX_TERMINAL=ghostty|iterm2|…`; and `$TERM_PROGRAM` when a command
  *is* run inside a terminal.
- No client attached (fully detached): fall back to the last known / configured.

### "Find the tab by title"
Every driver locates a session's tab by its **title `#S — #W`**, which tmux
`set-titles` writes. This is already required for Ghostty `focus`; it becomes a
hard prerequisite for all terminals (the `doctor` must verify it).

### Drivers & feasibility
| terminal | focus / spawn mechanism | status |
|---|---|---|
| **Ghostty** | AppleScript (existing) | driver #1, no behavior change |
| **iTerm2** | AppleScript (rich tab/session API) | ✅ high |
| **Apple Terminal** | AppleScript | ✅ |
| **kitty** | `kitty @ ls` (JSON tabs+titles) + `kitty @ focus-tab` | ✅ needs `allow_remote_control` |
| **WezTerm** | `wezterm cli list` + `wezterm cli activate-tab` | ✅ |
| **Warp** | weak automation, poor tmux fit | ⚠️ unsupported |
| **Alacritty** | no tabs / no scripting | ❌ unsupported |

Unsupported host → degrade gracefully: the agents list/status still works (that's
terminal-agnostic); `focus`/`restore`/`new` print a clear "jump needs a supported
terminal" note instead of failing silently.

### Sequence
- **A1** — extract `Terminal` interface + move Ghostty into it (pure refactor, no
  behavior change; tests assert identical output). *Start here.*
- **A2** — host detection (process-ancestry + overrides).
- **A3** — iTerm2 driver (closest to Ghostty, highest value).
- **A4** — kitty + WezTerm + Apple Terminal.

---

## B. Multi-agent (needs-input + notifications)

### Detection — DONE (PR #32)
Process-tree argv detection (`agentInSubtree` / `agentFromCommand`) catches agents
that run as `node …/bin/codex` with no title glyph. **Known rough edge:**
process-tree-detected agents are shown **idle even when working** (we only key
"working" off a title spinner) — fixed in B3.

### needs-input + notifications — generalize the Claude-only hook
Today `gtmux hook` is wired to Claude's `~/.claude/settings.json` and its
`Stop`/`Notification`/`UserPromptSubmit` events; that's what powers ⏸ waiting,
✓ latest, and notifications. Other agents get only working/idle.

**Finding (verified): Codex has a hook mechanism.** `~/.codex/config.toml`:
```toml
notify = ["<program>", "turn-ended"]    # runs <program> on events
```
plus a richer `hooks` system (cf. `codex --help: --dangerously-bypass-hook-trust`).
So Codex can feed gtmux the same way Claude does.

### Generic hook contract
- `gtmux hook` already keys state by `$TMUX_PANE` and reads an event. Generalize:
  `gtmux hook --agent <name>` + a per-agent **event-name → semantics** map
  (`{turn-start, finished, needs-input}`). Claude's
  `UserPromptSubmit/Stop/Notification` is one such map; Codex's `turn-ended`
  (+ any approval event) is another.
- `gtmux install-hooks --agent claude|codex|…` — per-agent installer:
  - **claude** → `~/.claude/settings.json` (existing path).
  - **codex** → `~/.codex/config.toml` `notify`/`hooks` pointed at
    `gtmux hook --agent codex`. **Chain, don't clobber** the user's existing
    `notify` (the config already has one for computer-use).
  - Idempotent + backed-up, same as the settings.json edit.
- Bare `gtmux install-hooks` installs for every agent it detects a config for, and
  reports which agents got needs-input/notifications vs detection-only.

### Working/idle accuracy (B3)
Per-agent working signal: observe each agent's **working** title (Claude animates a
braille spinner; Codex's working title is TBD — needs to be seen running). Until a
signal is known, a process-detected agent stays "idle" when its title has no
spinner.

### Sequence
- **B1** — generalize `gtmux hook` (event-map) + `install-hooks --agent`; keep
  Claude working exactly as today.
- **B2** — Codex integration (config.toml `notify`/`hooks`, chained), event map,
  `install-hooks --agent codex`.
- **B3** — per-agent working-title signal (fix the "always idle" rough edge).

---

## C. Env doctor / setup (later)
After A/B land, a `gtmux doctor`: checks tmux + resurrect/continuum + `set-titles`
+ the **host terminal** (A) + **per-agent hooks** (B); offers consented, backed-up
fixes (managed `tmux.conf` block, per-agent `install-hooks`). Productizes the
maintainer's `ccy-ai-workspace/terminal` installer. Detect → recommend → fix with
confirmation; never silent.

---

## Suggested overall order
A1 (refactor, low-risk foundation) → B1 (hook generalization, Claude unchanged) →
A2+A3 (host detection + iTerm2) → B2 (Codex hooks) → A4 / B3 → C (doctor).
Each is its own PR behind CI; behavior-preserving refactors first.
