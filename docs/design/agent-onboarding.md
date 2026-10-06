# Onboarding a coding agent

How to teach gtmux about a new coding agent (Claude Code, Codex, Gemini, Cursor,
opencode, …): the process, the one place identity lives, and the pitfalls that have
cost real time. Read this before wiring a new agent; follow the checklist at the end
before calling it done.

Spec: `openspec/specs/agent-integration/spec.md`. Registry: `internal/agents`.
Codex-specific behavior and diagnostics: [CODEX.md](CODEX.md).

---

## 1. Support tiers and capability boundaries

gtmux supports agents in tiers. These describe integration work, not a single switch:
receipt, readiness, content, headless execution, and usage have separate sources and limits.

| Tier | What lights up | What it needs |
|---|---|---|
| 0 · Sensed | A tmux radar row, focus and input; resume when a resume command is declared. | A manifest with detection commands (+ optionally an idle glyph, icon, resume argv). |
| 1 · Event parity | Hook-fed `waiting`/`done`, receipt-backed send verification, notifications, HQ wakes. | An installer that feeds lifecycle events into gtmux's event stream; the agent must actually run the hooks. |
| 2 · Digest parity | Transcript-derived `goal`/`last` in the digest, and the transcript's first-message time for HQ session age. | A session mapping and a transcript parser. `ask` is read separately from a waiting pane's numbered options. |
| 2+ · Usage parity | Context/burn figures (`gtmux usage`) and the `ctx` criterion of HQ self-rotation. | A resolvable session log with usage records the parser supports: Claude per-message usage or Codex cumulative `token_count` records. Usage does not depend on the driver's `content` switch. |

For HQ, the sensor first needs a resolvable session ID. Its three criteria then have
different evidence sources:

| Criterion | Source | Current coverage |
|---|---|---|
| `turns` | The HQ pane's `UserPromptSubmit` events since the sensor opened this session's window. | Hook-equipped agents whose hooks emit those events. This is not a count recovered from the full transcript. |
| `age` | When opening the window: first transcript message if available, otherwise the time the sensor first observed this session. | Transcript readers for Claude, Codex, opencode and Kimi; the fallback works for other mapped sessions too, but understates an older session's age. |
| `ctx` | Usage parser (`internal/usage/parse.go`). | Claude and Codex logs with usable token records; missing records supply no context figure. |

The wake line omits a non-positive context fraction or age and an unknown (negative)
turn count. It can print `0 turns` once counting has begun. Missing usage must not be
read as proof that a session has spare context.

`rotateInput` knows `/new` for Codex and `/clear` for Claude. For other agents it returns
no command: `gtmux hq --rotate` refuses an unknown reset command rather than typing a
guess. Sensing session age and knowing how to reset that agent are separate capabilities.

**Capability fallback follows the delivered agent-driver contract**
([spec](../../openspec/specs/agent-driver/spec.md)): absent receipt/readiness evidence falls
back to screen checks. Without a content reader, `goal`/`last` are absent; they are not
reconstructed from the screen. `ask` still comes from the pane and usage may still read
the session log. A resolved session whose content reader returns no error is graded
`driver`, even before a log exists; a missing, disabled or failing reader yields `partial`
for a resolved session, and no session mapping yields `screen`. A missing or disabled
headless capability refuses `spawn --oneshot`; it does not launch an interactive agent.
Hook-written state markers are independent of the driver switches.

Codex can deliver a `Stop` from a shared app-server with neither session ID nor cwd and
with another client's inherited `TMUX_PANE`. Match that event to a unique active pane
bound to a rollout that has just logged `task_complete`; never end the inherited pane
on its own. If attribution fails, radar reconciles a bound Codex pane's stale waiting
mark against a later completion in that same rollout. A newer `task_started` keeps the
next turn active. Codex may also omit the session ID on `UserPromptSubmit`, leaving a
plain active marker. A bound rollout's `task_complete` may end that marker only when
it is newer than the marker; a marker naming another session remains untouched.

The same shared app-server may emit `PermissionRequest` without cwd or session ID.
The inherited pane is not evidence of who is asking. Attribute the hook only to
a unique bound session; otherwise suppress the wait and record a diagnostic, without
writing a waiting lifecycle record. The radar senses a
live approval menu in its own pane on the next poll.
If an older hook already left a false waiting marker on an idle Codex pane, its
ready composer lets the radar clear that marker.

### Codex notification ownership

Codex has two independent notification paths. Its TUI can send terminal escape
notifications directly to Ghostty through tmux passthrough; gtmux's hook sends
desktop requests to the menu-bar app. The HQ exemption applies to the second
path only, so gtmux launches a Codex HQ with TUI notifications filtered to
`approval-requested` and `plan-mode-prompt`.
The user's global Codex settings and ordinary Codex sessions stay untouched.
An explicit `tui.notifications` option in the HQ agent command takes precedence.
An already running HQ needs a new process to take the launch option. A rotation
sends `/new` inside the old process and does not change its flags.

The hook applies these Codex-specific rules:

| Input | Evidence required | Outcome |
|---|---|---|
| `Stop` with no verified pane | A unique completed rollout and binding are absent | Keep the lifecycle record; do not emit a generic completion banner with no jump target. |
| `Stop` bound to a worker pane | Verified pane binding | Eligible for the ordinary completion notification rules; HQ's routine completion is silent. |
| `PermissionRequest` | A numbered approval menu stays visible after settling | Only then mark waiting and notify. The hook fires before Codex auto-review, so the event alone is not a human request. |
| Ownerless `PermissionRequest` | No verified pane | Do not notify or mark a guessed pane; radar may detect the live menu in its actual pane. |

Suppressed hook notifications are recorded in structured diagnostics with a
reason. The screen check is a conservative fallback until Codex exposes a
post-review, user-facing approval signal; a real menu that appears only after
the check can still be picked up by the radar's next poll.

---

## 2. Identity lives in ONE place: the registry

Historically each subsystem kept its own agent-keyed list, with inconsistent keys and
drifting membership. Now there is one `Manifest` per agent in `internal/agents`, and
each subsystem derives its list from it. Adding an agent's identity is authoring one
manifest.

```go
// internal/agents/registry.go
type Manifest struct {
    Key, Label      string   // "claude", "Claude Code"
    Aliases         []string // alternate keys that resolve here (cursor-agent → cursor)
    Detect          []string // radar process-subtree commands; empty ⇒ no radar profile
    IdleGlyph, Icon string   // the idle marker its TUI paints; optional app/image path
    Resume          []string // resume argv; nil ⇒ not resumable by session id
    Resource        string   // resource-attribution name; "" ⇒ not attributed
    HookDisplay     bool     // registered in the hook-time known-agent gate
    Hooked          bool     // events feed the receipt/ready stream (Tier 1)
    Content         string   // transcript-parser key (Tier 2); "" ⇒ none
    Headless        string   // headless one-shot key; "" ⇒ none
    Semantics       bool     // has a DEDICATED classifier event table (else the generic one)
    Instructions    string   // global instruction-file path; empty ⇒ not a knowledge carrier
    InstructionsEnv string   // optional env var that relocates the instruction-file home
}
```

These subsystems already read the registry; you do not edit them:

| Concern | Accessor | Consumer |
|---|---|---|
| Hook-equipped set | `agents.HookEquippedKeys()` | `internal/driver` |
| Radar detection profiles | `agents.Profiles()` | `internal/radar` |
| Resume command | `agents.ResumeArgv()` | `internal/resume` |
| Resource attribution | `agents.ResourceNames()` | `internal/resource` |
| Hook display name | `agents.DisplayNames()` | `internal/hook` |
| Transcript keys | `agents.ContentKeys()` | `internal/driver` |
| Global instruction carriers | `agents.All()` → `Instructions` / `InstructionsEnv` | `internal/knowledge/distribute.go` |

Headless execution still needs manual wiring in `internal/driver/driver.go`: it explicitly
registers Claude and Codex with `withHeadless`. Adding `Manifest.Headless` alone does not
install a headless driver; `HeadlessKeys()` is currently checked by the registry tests.

The registry's data is pinned by golden tests in `internal/agents/registry_test.go` (copied
verbatim from the legacy maps) and by per-subsystem migration-guard tests.

### Capability conformance and transcript fixtures

Treat each non-empty capability as a promise that must be wired and tested. A registry entry
with `Content` needs a driver parser; `Hooked` needs an installer and display mapping; and
`Semantics` needs a dedicated classifier table. Conformance tests name the missing agent and
capability instead of letting a partial integration appear complete.

Each Tier 2 parser also keeps sanitized fixtures under its package's `testdata/`. Fixtures
record observed event shapes, including a current shape and any legacy shape we continue to
support. `internal/transcript/testdata/codex-current.jsonl` records user input as
`response_item` messages with `role: user` and `input_text` blocks; the parser also supports
`event_msg.user_message`. The Codex parser ignores
injected `AGENTS.md` and environment context, and unknown event records must not block later
recognized turns. Update the fixture and this contract together when an agent release changes
its log.

### What stays domain-local (and why)

Three things are behavior rather than identity. They stay in their own package, keyed by
the registry's agent key, because moving a domain enum into a pure-data package is
over-abstraction:

- Event semantics, the native-event → gtmux-semantic table: `internal/hook/classify.go`
  (`agentEventSemantics`, else the generic table).
- Prompt / ready signatures (boot banners, prompt/selector glyphs): `internal/prompt`.
- Hook install spec, the file/plugin gtmux writes: `internal/app/agent_hooks.go`.

Go tests check specific registry connections: `internal/app/opencode_installer_test.go`
checks installers and hook display mappings, `internal/driver/registry_wiring_test.go`
checks receipt/readiness and non-nil content wiring, and
`internal/hook/registry_conformance_test.go` checks dedicated classifier tables.
These run with `go test`/`make check`, not `scripts/check-design.sh` alone. Parser fixtures
must also exercise the actual `resolveLog` branch; a non-nil wrapper alone is not proof
that a parser exists. Prompt/ready signatures still need review in `internal/prompt`.

---

## 3. Step by step

### Step 0: Manifest (always)

Add one entry to `manifests` in `internal/agents/registry.go`. Fill `Key`, `Label`,
`Detect` (Tier 0). Add `Resume` if it can relaunch a session by id. Run
`go test ./internal/agents/`; the golden tests will tell you if you disturbed an existing
agent. That's Tier 0: radar detection, focus and input; resume requires `Resume`.

### Step 1: Hook installer (Tier 1)

Set `Hooked: true` and `HookDisplay: true`. Then wire an install spec so the agent
emits events. Three extension models exist; check which the agent supports:

- Command-hook model (Claude, Codex, Cursor, Gemini, Copilot, Kiro): the agent reads a
  JSON/TOML config that runs a shell command on lifecycle events. Add an
  `agentInstaller` entry (`internal/app/agent_hooks.go`) mapping each native event to a
  gtmux token: `beforeSubmitPrompt → UserPromptSubmit`, `afterAgentResponse → Stop`,
  the approval event → `PermissionRequest`, session start/end. Pick or add a `format`.
- Plugin model (opencode): the agent has no command-hook file, only JS/TS plugins.
  The installer writes a small plugin that subscribes to the agent's events and shells
  out to `gtmux hook --agent <key> <event>`. Same `gtmux install hooks --agent <key>` entry
  point; the plugin is a `dedicated` artifact removed cleanly on uninstall.
- Managed-block model (Kimi Code): the agent's hooks live inside a config file
  gtmux does not own, the `[[hooks]]` entries in the same `~/.kimi-code/config.toml`
  that holds the user's providers and keys. Neither of the models above fits: there is
  no file to write whole, and re-serialising someone's hand-written TOML to change four
  lines is not a trade worth taking (gtmux has no TOML library, and should not acquire
  one for this). So `internal/app/kimi_hooks.go` appends a block between sentinel
  comments. Install/uninstall reads the whole file, removes the marked block, and
  writes the remaining text plus the new block when installing; trailing newlines are
  normalized. Uninstall removes the file if only the managed block and whitespace remain.
  It does not parse or validate the surrounding TOML. With valid preceding
  TOML, a new `[[hooks]]` header starts a new table entry; it cannot repair an already
  malformed file. An unterminated managed block is treated as extending to EOF.
  Validate synthetic examples with the agent's own validator before adopting this installer model.

Map the agent's native events onto gtmux's: `UserPromptSubmit`, `Stop`, `PermissionRequest`
(a real user-facing approval → `waiting`), `PostToolUse`/resolve (clears `waiting`),
`SessionStart`, `SessionEnd`, `PreCompact`/`PostCompact`. If the agent's approval signal is
a separate event from its pre-tool event, give it a dedicated semantics table
(`agentEventSemantics`, `Semantics: true`) so the pre-tool event stays telemetry; if its
only signal is the pre-tool event, the generic table's `semToolStartMaybeApproval` escalates
side-effecting tools for you.

Verify identity resolves from the process subtree (see pitfalls; the foreground command
is the wrong source). Install, drive a real session, confirm `waiting`/`done` and a
receipt-verified `gtmux send` (`judged_by: driver`).

### Step 2: Transcript parser (Tier 2)

Add `internal/transcript/<agent>.go` reading the agent's session log into `[]Turn`, set the
manifest's `Content` key (that alone auto-wires `driver.Content`; see `agents.ContentKeys()`),
add the `resolveLog` + `normalizeAgent` cases, and add sanitized fixtures for observed log
shapes. Now the digest can populate `goal`/`last`; `ask` remains a separate pane read.
The pane→session mapping is free: the hook writes a `resume` record from the session id, and
`sessionRef` reads it, so a resumable agent whose hook receives the session id needs no extra
wiring.

**When an integration has no supported upstream transcript reader**, it can keep a
gtmux-owned copy. The opencode integration does this: the plugin streams the user prompt and the
final assistant text through `gtmux hook` (piping `{session_id, prompt}` / `{session_id,
assistant}` on stdin), the hook appends them via `transcript.AppendOpencode` as
`{timestamp, role, text}` JSONL under `~/.local/share/gtmux/octrans/<session>.jsonl`, and the
parser reads that. Two subtleties paid for: (a) assistant text arrives as a stream of
`message.part.updated` events (`part.text` is the full text so far), so accumulate per
message-id and flush the newest on `session.idle`; (b) key the file by the agent's own session
id (piped alongside the prompt) so it lines up with the `resume` record `sessionRef` resolves.

---

## 4. Pitfalls checklist (every trap we've paid for)

- [ ] The launcher's name is not the process's name. Kimi's binary is `kimi`, and the
  process it becomes is `kimi-code`. The subtree match is exact, so a manifest carrying
  only the launcher name made a running Kimi pane invisible to the radar (measured: 0
  rows against a live session, with `pane_current_command` reading `kimi` the whole time).
  Put both in `Detect`, and check it on a live pane (`ps -o comm=` on the pane's child)
  rather than on what you typed to start it.
- [ ] Identity comes from the process subtree, never from `pane_current_command`. Claude Code
  renames its process to its version (`2.1.220`); several agents run as bare `node`. Keying
  identity off the foreground command mis-detects the agent and silently disables the
  receipt path. Use `radar.AgentDriverKey` (walks the subtree). This cost a multi-day
  "send stuck forever" hunt.
- [ ] The hook must be installed or the whole event layer stays dark. Being in the
  driver's hook-equipped set is necessary but not sufficient: without an installer that
  actually wires the agent's config/plugin, it emits no events and Tier 1 is a no-op
  (opencode was in the whitelist for months with no installer).
- [ ] Installed ≠ trusted. Some agents gate a newly-registered hook behind a one-time
  user confirmation before it will fire; the Codex ~0.146 integration observed
  "New hook - review required — press t to trust". Until trusted, that hook supplies no
  events even with correct config. Screen detection can still work; transcript-backed
  digest fields also need a resolvable session. The installer must say so; don't debug a
  "silent hook" without checking the agent isn't just waiting to be trusted.
- [ ] Plugin vs command-hook model. Don't assume a JSON "run a command on event" file
  exists. opencode is plugin-only; forcing it into a command-hook format fails.
- [ ] **A plugin that shells `gtmux hook` must redirect its stdin (`< /dev/null`).** A JS
  plugin's subprocess inherits the agent's controlling TTY as stdin, and `gtmux hook`
  drains stdin; `io.ReadAll` on a TTY never EOFs, so the hook hangs in the agent's
  foreground process group and steals its keyboard input (opencode's composer went
  dead after the first send; every subsequent `gtmux send` silently failed `not confirmed`).
  `gtmux hook` now guards this (`stdinIsTerminal` skips a char-device stdin), but the
  plugin must still redirect so an old binary is safe. Pipe-fed calls (`echo … | gtmux
  hook … UserPromptSubmit`) are already safe, because their stdin is the pipe rather than
  the TTY. This cost a full debugging session; the tell is a `gtmux hook` process stuck
  in state `S+`.
- [ ] Locale/glyph loss over daemon-spawned PTYs. A `launchd`-spawned `gtmux serve` has
  no `TERM`/locale, so a PTY it spawns mangles CJK and TUI glyphs to dashes/`_`. Force
  `-u` + `LC_CTYPE` and pass `TERM`. This also breaks the radar's glyph classification.
- [ ] Sparse events fall back to Layer 1, and that's fine. A low-event-density agent (Codex)
  just lowers the receipt hit rate; `NoEvidence` falls to the two-frame screen read. Do not
  treat a missing event as a failure.
- [ ] Idle-glyph classification needs live confirmation. A leftover title glyph on a
  dead shell must not classify as a running agent: the classifier requires the process to
  be live (or the subtree to match), and the title alone is not enough.
- [ ] Agent icons: committed built-in, or the vendor's installed app, else a letter mark.
  §6 now permits a committed official mark for identification (nominative use): drop
  `<key>.png` in `assets/agent-icons/` with provenance in `SOURCES.md`; the serve hands it
  to every surface via `/api/icon`. For agents with a desktop app you can instead point
  `Icon` at `/Applications/<App>.app`. Gotcha: the mobile only fetches `/api/icon` when
  `agents --json` reports a non-empty `icon`, so `radar.IconFor` materializes the committed
  PNG under `~/.local/share/gtmux/cache/agent-icons/<key>.png` and returns that path whenever the
  profile `Icon` is empty; without a hint the phone shows the monogram despite the icon
  shipping. (This is exactly how opencode showed "OC", and Codex's non-tmux rows showed
  "Cx", until fixed.) Because the hint is a real path rather than an opaque token, the
  menu-bar app (which resolves a hint by opening it as a file) gets the committed icon
  too, with no app-side change. Its `~/.config/gtmux/icons/<slug>.png` drop-in is a fallback
  when the hint is empty or its path does not exist; it does not override an existing hint path.
- [ ] Approval event vs pre-tool event. If the agent raises a distinct approval event,
  keep its pre-tool event as telemetry (dedicated table). Otherwise every tool would flag
  "needs you," or real approvals would be dropped (Kiro's lowercase events must be
  registered explicitly).
- [ ] A generic pre-tool event should not flag a read-only tool as "needs you."
  `sideEffectingTools` in `classify.go` is the escalation allowlist; keep Read/Grep/Glob
  out of it. An explicit `PermissionRequest` is a different signal and still means waiting.
- [ ] A generated manifest is a description of the bytes, and it can be wrong. Kimi ships a
  machine-generated wire manifest, and three of its claims did not survive contact with a real
  session: `origin` is documented as a string and written as `{"kind":"user"}`; the
  assistant's reply is not a message record at all but a loop event carrying a
  `content.part`; and a UserPromptSubmit `prompt` is a string for Claude and an array of
  content parts for Kimi. Each one failed silently: a JSON type mismatch fails the
  whole record, and an empty prompt is empty in all four places it is consumed. Thirteen
  fixture tests built from the manifest passed while a real journal parsed to zero turns.
  **Get real bytes before believing a schema**, commit one as a fixture, and prefer
  `json.RawMessage` + a lenient reader for any field two agents might type differently.
- [ ] You do not need an account to get real bytes. Kimi speaks the OpenAI
  chat-completions protocol to any `base_url`, so a ~40-line local stand-in provider
  (`type = "openai"`, `base_url = "http://127.0.0.1:…"`) runs a real session end to end:
  real hooks, a real `wire.jsonl`, real pane identity. Every defect above was found that
  way, with no Moonshot account. Check for the same escape hatch on the next agent
  before settling for fixtures.
- [ ] An unknown field can take the whole config down. Kimi's `[[hooks]]` accepts
  exactly four keys and rejects the entire file on a fifth, so an ownership marker
  written into the entry would have cost the user their providers, far more than one hook.
  Verified with the agent's own validator (`kimi doctor` reported
  `hooks[11]: Unrecognized key: "owner"`). When an installer writes into a file the
  user owns, run the agent's validator over the result, and check the validator
  discriminates by feeding it a bad one.
- [ ] Keys must be consistent. One canonical `Key`; use `Aliases` for alternate command
  names (cursor-agent → cursor). Don't invent a per-subsystem key.

### App-side fallback marks

The apps consume the `icon` hint from Go but keep local fallback monogram maps.
Review both when adding an agent:

- Menu-bar: `macapp/Sources/GtmuxBar/Components.swift` (`agentMonogram`, `AgentIcons`).
- Mobile: `mobileapp/src/ui/agentMark.ts`.

(The fallback marks remain hand-kept; icon hints already come from `agents --json`.)

---

## 5. Done checklist

- [ ] Manifest added; `go test ./internal/agents/` green (golden tests undisturbed).
- [ ] Tier targeted is actually wired (installer for Tier 1, parser for Tier 2).
- [ ] Identity verified via the process subtree on a real pane.
- [ ] `gtmux install hooks --agent <key>` writes the integration; uninstall removes only what
  gtmux wrote, leaving user config intact.
- [ ] A live session drives `waiting`/`done` and a receipt-verified `gtmux send`
  (`judged_by: driver`), for Tier 1.
- [ ] Menu-bar + mobile mark/icon updated.
- [ ] `make check` + `scripts/check-design.sh` green.
- [ ] CLAUDE.md / `docs/cli.md` mention the agent if it's user-facing; spec updated if
  behavior changed.
