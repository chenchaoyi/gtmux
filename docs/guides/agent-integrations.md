---
title: How each agent connects to gtmux
description: The implementation, setup and limits of every built-in agent integration.
order: 6
---

**English** · [中文](agent-integrations.zh.md)

This is the central map of gtmux's agent integrations. It describes the implementation
in this checkout, checked against the source on **2026-10-19**. “Supported” has
several meanings: recognising a process, receiving events, reading a conversation and
controlling a session are separate capabilities. The 14 entries below include the
limited integrations, so a registered name is never mistaken for full support.

## Shared architecture

An **agent key** such as `codex` selects an adapter. A **hook** is a callback the agent
runs when something happens. A **bridge plugin** subscribes to an agent's extension
API and forwards those events. A **transcript reader** reads saved dialogue; it does
not operate the agent or call a model.

There are three paths into the same Go core:

1. **A tmux pane:** radar recognises a foreground command or process subtree, then
   combines title, screen changes, CPU samples and hook state. A screen/CPU inference
   is weaker evidence than a structured event. The terminal, menu bar, phone, iPad and
   Web consume the core's results; they do not implement their own agent adapters.
2. **Agent events:** a command hook or plugin calls `gtmux hook --agent KEY EVENT`.
   The classifier translates native events into prompt, working, waiting, resumed,
   success, interruption and failure semantics. It records lifecycle evidence and
   feeds status, notifications and HQ. A tool starting is not automatically a human
   approval request; an interrupted turn is not a successful completion.
3. **Saved conversations:** a reader maps a verified session to its local artifacts.
   Claude, Codex, OpenCode, Kimi and DSH have content readers. Codex and DSH also have
   dedicated external-conversation providers, which discover histories without a
   tmux pane or a loaded hook. Other outside-tmux observations depend on hook traffic.

For interactive **tmux** sessions, input stays in the pane: dispatch checks the
composer, pastes text and submits it. A matching prompt event can confirm receipt;
otherwise verification falls back to screen evidence. Receipt means the prompt
arrived, not that the task succeeded. Readiness has the same positive-evidence rule.
Missing content support leaves structured dialogue and `goal`/`last` absent; screen
capture remains a separate view. Unsupported `spawn --oneshot` is refused.

An external history has no input route merely because it is readable. Codex and DSH
follow grants are scoped to the agent, canonical data home and conversation. HQ
observation, notifications and future knowledge retention require their respective
grants; they grant no takeover permission. Guest links cannot read these histories.

## Where an external conversation was launched

The harness remains the identity: a row can say `Codex · sample-client` without
adding a new agent integration for that client. `launch_host` describes **where a
hook observed the agent being hosted**, not which frontend currently owns the input.
When current input ownership is unconfirmed and there is no host observation, the
subtitle says `Codex · External session`. Details retain “Current input client
unconfirmed” and show the host observation time when available.

The shared Go collector (`internal/launchhost`, called by `internal/hook`) works as follows:

1. At a `SessionStart`, `UserPromptSubmit` or explicit `Resumed` hook, start at the
   hook's parent process. Capture before Codex's worker detaches, since reparenting
   loses the original ancestry. The detached worker receives a bounded, fresh internal
   environment value; it does not try to reconstruct its former parent chain.
2. Read at most eight ancestors, with one 350 ms deadline. Each `ps` query requests
   only the parent PID, process birth time and executable command name (`comm`),
   never the full arguments. Each command's stdout is capped at 4 KiB. This follows
   only the hook's ancestry; it does not enumerate other processes or open files.
3. Match the agent executable against the existing registry. Skip ordinary shells
   and launch shims. After the agent, prefer the outer enclosing macOS `.app` name
   from `CFBundleDisplayName`, then `CFBundleName`, then the bundle basename. The plist
   must be a regular file no larger than 256 KiB. Otherwise use the executable basename.
   Linux uses the basename. Generic runtimes such as `node` or `python`, a tmux/service
   boundary, invalid names, missing parents, cycles or excessive depth leave the host
   unknown. A runtime inside an identifiable macOS app may use that app's name.
4. Re-read the sampled chain before accepting it. Any changed parent, command or birth
   time discards the observation. PID and birth-time snapshots stay in the private
   native record; CLI JSON and HTTP expose only `{name, kind, observed_at}`, with kind
   `app` or `process` and Unix seconds. No executable path or full argument list is sent.

Non-identity hooks preserve the old observation and its original timestamp. A new
identity hook replaces it, including clearing it if the new source cannot be established.
Old records are not backfilled: another natural identity hook is needed. History-only
discovery without hooks cannot supply this evidence, and a process hidden behind an
unidentifiable interpreter stays unknown. No extra source-detection plugin is installed.

Process birth times have the system’s second-level precision. Names and bundle
metadata can be changed by their owner. These are best-effort historical
display hints, not authenticated application identity, proof of current attachment or
liveness, or permission to send, follow, adopt or bind a pane. Multiple conversations can
share the same host process. Creation metadata, host evidence and current control remain
separate. Terminal, menu bar, phone, iPad and Web all consume the same core projection.

## Capability map

“Process” means a built-in radar command profile, not every possible title or custom
profile. “Hook” means an installer exists; it must actually be loaded and emit events.
“Resume” means a registered command recipe, subject to a valid session record and
availability checks. It is not permission to take over an external client.

| Agent key | Process profile | Event integration | Structured history | Resume / one-shot |
| --- | --- | --- | --- | --- |
| `claude` | `claude` | Command hooks | Claude JSONL | Resume; one-shot |
| `codex` | `codex` | Command hooks | Rollout JSONL; external provider | Resume in tmux; one-shot; external Move unavailable |
| `gemini` | `gemini` | Command hooks | None | Resume; no one-shot |
| `cursor` | `cursor-agent`, `cursor` | Command hooks | None | Resume; no one-shot |
| `opencode` | `opencode` | Native JS plugin | gtmux-recorded JSONL | Resume; no one-shot |
| `kimi` | `kimi`, `kimi-code` | Command hooks | Kimi wire journal | Resume; no one-shot |
| `copilot` | None | Command hooks | None | Resume recipe; no one-shot |
| `kiro` | None | V3 global hooks / legacy agent profile | None | Resume recipe; no one-shot |
| `dsh` | Verified terminal-client attachment; installer probes `dsh`/`dst` | Native DSH components | v4 artifacts; shared pane/external reader | dst native input/resume; Web read-only; no one-shot |
| `hermes-agent` | None | Classifier only; no installer | None | Resume recipe only |
| `grok` | None | None | None | Resume recipe only |
| `aider` | `aider` | None | None | Neither |
| `crush` | `crush` | None | None | Neither |
| `amp` | `amp` | None | None | Neither |

## Claude Code — `claude`

**Discovery and events.** Radar matches `claude`, including its process subtree.
The installer merges gtmux callbacks into `~/.claude/settings.json`, preserving
other hooks. Prompt, permission, question, tool, compaction, session and Stop events
drive state. `StopFailure` distinguishes failure; MCP elicitation callbacks expose
questions. There is no installed top-level Interrupt event, and Stop does not cover
every user interruption, so screen/transcript reconciliation still matters.

**Content and control.** The reader finds
`~/.claude/projects/<project>/<session>.jsonl`. It separates user prompts from tool
results and skips sidechains, meta messages and compaction continuation summaries.
Usage reads Claude's message usage records. Interactive input uses tmux; the resume
recipe is `claude --resume ID`. One-shot uses `claude -p … --output-format stream-json
--verbose`, with structured output and exit status determining the outcome.
Machine knowledge is distributed through the managed block in `~/.claude/CLAUDE.md`.

Sources: [installer](../../internal/app/hooks.go),
[reader](../../internal/transcript/claude.go), [one-shot drivers](../../internal/driver/headless.go).

## Codex CLI and desktop histories — `codex`

**Discovery and events.** Radar matches the CLI process. The installer writes the
gtmux entries in `$CODEX_HOME/hooks.json` and enables `features.hooks` in
`config.toml`, leaving an existing `notify` unchanged. The default home is `~/.codex`.
Callbacks are synchronous and bounded; changed hooks may need trust review in
Codex's `/hooks`. The dedicated classifier handles Interrupt and distinguishes
prompt, permission, tools, session and compaction events.

Shared app-server callbacks may inherit the wrong `TMUX_PANE` or omit identity.
Gtmux requires session/rollout binding evidence rather than ending or marking a
guessed pane. A permission callback can precede automatic review; a settled visible
approval menu supplies the user-wait evidence. The bound rollout can reconcile a
missed completion or abort without ending a newer turn.

**Content and control.** The reader combines the session's verified rollout JSONL
under `$CODEX_HOME/sessions` and `archived_sessions`, filters injected context and
keeps public dialogue. Usage reads token records separately. The external provider
reads verified local histories, including desktop-created conversations, without
resuming them. Creation metadata does not prove which client currently controls a
conversation; **Move, adopt and direct input for external Codex are unavailable**.
Inside tmux the resume recipe is `codex resume ID`; one-shot is `codex exec --json`.
Gtmux's own direct interactive launches preserve scrollback with `--no-alt-screen`
unless the launch already makes an explicit choice. Knowledge uses
`$CODEX_HOME/AGENTS.md`; an owner's `AGENTS.override.md` can take precedence.

Sources: [Codex integration](../design/CODEX.md),
[client identity and ownership](../design/codex-client-lifecycle.md),
[rollout reader](../../internal/transcript/codex_reader.go).

## Gemini CLI — `gemini`

Radar matches `gemini`. The installer merges command hooks into
`~/.gemini/settings.json`: BeforeAgent/AfterAgent, BeforeTool/AfterTool, session,
Notification and PreCompress. Only a Notification with
`notification_type=ToolPermission` establishes a permission wait; ordinary tool
starts do not. AfterTool can clear that wait. There is no dedicated installed
interrupt or terminal API-failure event, transcript reader, usage parser or global
knowledge carrier. Input and its fallback verification use tmux; the resume recipe
is `gemini --resume ID`. No one-shot driver is wired.

Source: [hook contracts and limits](../design/agent-hooks.md).

## Cursor CLI — `cursor`

Radar matches `cursor-agent` or `cursor`; `cursor-agent` is also an alias for the
gtmux key. Hooks live in `$CURSOR_HOME/hooks.json` (default `~/.cursor/hooks.json`).
The installer uses Cursor's flat JSON layout. `beforeSubmitPrompt` supplies receipt
evidence; `stop.status` distinguishes completed, aborted and error. Shell/MCP and
tool-result callbacks, session events and compaction update the lifecycle.
Response text alone is not completion. The bridge does not implement Cursor's
permission-decision callbacks; real waits rely on screen sensing. There is no
structured transcript, usage, knowledge-carrier or one-shot adapter. Tmux input
remains available; the resume recipe is `cursor-agent --resume ID`.

Source: [installer and event bindings](../../internal/app/agent_hooks.go).

## OpenCode — `opencode`

Radar matches `opencode`. This integration uses **gtmux-authored JavaScript**, loaded
through OpenCode's native plugin API. Installation writes
`~/.config/opencode/plugins/gtmux.js`; OpenCode loads it on startup. Only gtmux's owned
legacy `plugin/gtmux.js` is migrated. It is not a JSON command-hook installation.

The plugin observes `event` and `chat.message`, resolves session parentage through
the supplied SDK client, and ignores child or unknown sessions. Busy starts work;
permission/question results resume it. An abort remains interrupted, an error stays
failed, and the following idle event cannot turn either into success. Hook
subprocesses use a pipe or `/dev/null`, never the controlling terminal's stdin.

For this adapter, the plugin records future user prompts and final assistant text
in **gtmux's own** `~/.local/share/gtmux/octrans/<session>.jsonl`. The reader consumes
that file; it does not import OpenCode's entire prior history or query its storage
database. Missing plugin traffic therefore also means missing recorded content.
The resume recipe is `opencode --session ID`; input uses tmux, with no usage or
one-shot driver. Knowledge uses `~/.config/opencode/AGENTS.md`.

Sources: [plugin generator](../../internal/app/agent_hooks.go),
[owned transcript](../../internal/transcript/opencode.go).

## Kimi Code — `kimi`

Radar recognises both the `kimi` launcher and `kimi-code` process. The installer
maintains a marked `[[hooks]]` TOML block in `$KIMI_CODE_HOME/config.toml` (default
`~/.kimi-code`), preserving bytes outside it. Native prompt/TurnStarted, permission
request/result, tool, compaction, session, StopFailure and Interrupt events are
classified separately. A queued prompt has not started work; PermissionResult
resolves a permission wait rather than an unrelated tool result.

The reader resolves `session_index.jsonl`, honouring deletion records, then reads
`sessions/<workspace>/<session>/agents/main/wire.jsonl`. It supports the observed
wire journal's message-origin shapes, separates real user input from injections,
and reads assistant content parts without duplicating partial messages. Input uses
tmux; the resume recipe is `kimi --session ID`. Usage and one-shot drivers are not
wired, even if the upstream product offers such facilities. Knowledge uses
`$KIMI_CODE_HOME/AGENTS.md`.

Sources: [TOML installer](../../internal/app/kimi_hooks.go),
[wire reader](../../internal/transcript/kimi.go).

## GitHub Copilot CLI — `copilot`

There is no built-in process-command radar profile. The integration provides a
dedicated `$COPILOT_HOME/hooks/gtmux.json` (default `~/.copilot/hooks/gtmux.json`),
using version 1, PascalCase events and `bash`/`timeoutSec` command entries. Its
prompt, stop, session, tool, compaction and error callbacks feed gtmux's hook path;
hook-equipped does not imply hook-free process discovery.

PermissionRequest runs before policy/automatic approval and is telemetry, not proof
of a human wait. Only `ErrorOccurred` with `recoverable=false` ends a turn as failed.
There is no dedicated user-interrupt event, content reader, usage parser, knowledge
carrier or one-shot adapter. The registered resume recipe is `copilot --resume ID`;
it requires a usable session record, rather than discovery being guaranteed.

Source: [hook contracts and limits](../design/agent-hooks.md).

## Kiro CLI — `kiro`

There is no built-in process-command radar profile. Installation selects the hook
engine using existing configuration, then a bounded version probe. V3 uses the
global `$KIRO_HOME/hooks/gtmux.json` (default `~/.kiro`), with
`version=v1`, trigger/action/timeout/enabled entries. Legacy CLI 2.x uses
`$KIRO_HOME/agents/gtmux.json`; that profile must be selected with
`kiro-cli chat --agent gtmux`. An unknown engine retains the legacy-compatible path.
Owned duplicate legacy bindings are removed when migrating to V3.

SessionStart, prompt, pre/post-tool and Stop events feed the bridge. Tool starts
alone do not prove a user wait. There is no documented installed Interrupt,
SessionEnd or StopFailure event, and no content, usage, knowledge-carrier or
one-shot adapter. The registered resume recipe is `kiro-cli chat --resume-id ID`,
subject to valid mapping; a resume recipe is not a complete detection integration.

Sources: [hook contracts](../design/agent-hooks.md),
[version-specific installer](../../internal/app/kiro_hooks.go).

## DeepSeek Harness — `dsh`

**History first.** The external provider reads `$DSH_HOME/sessions` (default
`~/.dsh/sessions`), targeting DSH **0.2.0-rc.2** v4 artifacts. It handles raw JSONL
and concatenated checksummed Zstandard frames. It verifies home/session identity,
excludes child sessions and reads public user text, assistant text and tool steps.
Reasoning, raw tool results, injected instructions and replay/compaction copies are
excluded. A partial tail waits; unknown generations, corrupt data and ambiguous
identities are refused. Discovery is bounded to 128 recent roots; exact ID reads
can open older histories. Reading does not call DSH HTTP APIs, use credentials or
acquire its writer lease, and needs **no plugin**.

**Future events.** Gtmux bundles **a bridge script written by gtmux contributors**,
not the Cordis framework. DSH supplies the Cordis extension runtime. Installation
writes `$DSH_HOME/gtmux/gtmux.mjs` and `gtmux.cordis.yml`; no separate npm package is
required. On the next intentional launch, select the printed
`dsh web --patch '…/gtmux/gtmux.cordis.yml'` overlay. It never restarts running work. For an existing dst profile, installation also manages a marked native-component block while preserving other configuration.

The bridge uses DSH's own `session/event` and `user-questions/request`, **not Codex
compatibility hooks**. It tracks root prompts, turn outcomes and pending questions,
delegates questions unchanged, and never approves or answers them. `approval/asked`
is not proof of a human wait; approval-card detection is not implemented. The queue
and child processes are bounded and stdin is piped. The terminal adapter verifies home, foreground session, process ancestry and tmux epoch before exposing a managed pane; a background mount alone grants no input. Its native command receives the actual target agent and returns an addressed, one-use acceptance receipt. Drafts, menus and switches remain guarded. All consumers retain the DeepSeek Harness label and official icon; dst is only the internal client adapter.

Doctor distinguishes installed configuration from a fresh loaded attachment and input grant. Ordinary `dst --backend dsh`, terminal spawn/resume and explicit HQ public-message review are supported by the adapter. Web remains external/read-only; Web adoption, usage and one-shot are not enabled. Older Web/background intervals are not silently enrolled. Knowledge uses `$DSH_HOME/AGENTS.md`. See the [terminal design and validation boundary](../design/dsh-terminal-clients.md).

Sources: [setup, upstream contracts and official logo](deepseek-harness.md),
[plugin source](../../internal/app/dsh_plugin.mjs), [artifact reader](../../internal/dsh/log.go).

## Hermes — `hermes-agent`

This is **partial groundwork**, not a turnkey integration. The registry contains
a display identity, `hermes --resume ID` recipe and dedicated event semantics for
session, LLM, tool and approval callbacks. There is no built-in process profile,
hook installer or receipt/readiness driver, and no transcript, external provider,
usage, knowledge-carrier or one-shot adapter. A classifier accepting events does
not install a bridge that emits them. `--agent auto` does not install Hermes hooks.

Sources: [manifest](../../internal/agents/registry.go),
[classifier](../../internal/hook/classify.go).

## Grok — `grok`

Only the display identity and `grok -r ID` resume recipe are registered. There is
no built-in process profile, hook installer, dedicated event classifier, content
reader, external provider, usage, knowledge carrier or one-shot driver. An existing
record can use the recipe where eligible; this is not automatic Grok onboarding.

Source: [manifest](../../internal/agents/registry.go).

## Aider — `aider`

Radar recognises the `aider` process subtree and attributes its process resources.
Status relies on the shared title/screen/CPU sensing path. A recognised tmux pane
can be focused and receive input, with screen-based verification. There is no
gtmux hook installer, structured history, usage-token reader, resume recipe,
knowledge carrier or one-shot driver. Process visibility is the integration.

Source: [manifest](../../internal/agents/registry.go).

## Crush — `crush`

Radar recognises `crush` and attributes process resources. The shared tmux sensing,
focus, input and screen verification apply. No command-hook/plugin, structured
history, usage-token reader, resume recipe, knowledge carrier or one-shot driver
is registered. No hook installation is needed for this process-only integration.

Source: [manifest](../../internal/agents/registry.go).

## Amp — `amp`

Radar recognises `amp` and attributes process resources, using the shared tmux
status, focus and input paths. It has no gtmux hook installer, transcript reader,
usage-token reader, resume recipe, knowledge carrier or one-shot driver. The
existence of an upstream API does not imply that gtmux has wired it.

Source: [manifest](../../internal/agents/registry.go).

## Installation, diagnosis and implementation ownership

For the nine integrations with an installer, select the exact key above:

```sh
gtmux install hooks --agent codex
gtmux install hooks --agent auto
gtmux doctor
gtmux doctor --fix
gtmux uninstall hooks --agent codex
```

`auto` detects supported homes or executables and skips recognised disabled-hook
settings. It only selects agents that have an installer. Omitting `--agent` retains
the Claude default. Doctor diagnoses configuration; `--fix` uses the corresponding
agent's writer, preserving foreign hooks and refusing unclear ownership. Configured
does not prove loaded: the agent must run the new hooks, Codex trust may need review,
and DSH must explicitly load its overlay. No automatic agent restart is implied.
Desktop notifications additionally need the menu-bar app and notification permission;
phone notifications need pairing, push registration and the relevant consent.

| Source | Owns |
| --- | --- |
| [Agent manifests](../../internal/agents/registry.go) | Keys, aliases, process/install detection, resume recipes and declared capabilities |
| [Hook installers](../../internal/app/agent_hooks.go), [Claude](../../internal/app/hooks.go), [Kimi](../../internal/app/kimi_hooks.go), [DSH](../../internal/app/dsh_hooks.go) | Files written, configuration format and owned migrations |
| [Hook classifier](../../internal/hook/classify.go) | Native event meaning, including limits of waiting/cancellation evidence |
| [Radar](../../internal/radar/agents.go), [drivers](../../internal/driver/driver.go) | Process/state sensing, receipt/readiness, content and one-shot wiring |
| [Transcript readers](../../internal/transcript/transcript.go), [external providers](../../internal/conversation/provider.go) | Content parsing and home/session verification |
| [Onboarding checklist](../design/agent-onboarding.md), [hook contracts](../design/agent-hooks.md) | How to extend an integration and the terminal agents' upstream event contracts |

Capability entries are source-checked documentation, not a claim that every current
upstream release, terminal, desktop client or device was live-tested. Registry and
regression tests check wiring and synthetic contracts; actual hook traffic and
runtime acceptance are separate evidence. Update this page and its Chinese twin
when an integration's files, capabilities or limits change.
