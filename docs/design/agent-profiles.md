# Participant profiles: facts and sources for each agent

**English** · [中文](agent-profiles.zh.md)

**Implemented profile collection and mobile presentation**, with source analysis
checked on 2026-10-20. Identity covers all 14 agents in the
[registry](../../internal/agents/registry.go); richer fields require the specific
collectors listed below. [Integration behavior](../guides/agent-integrations.md)
and this document distinguish delivered fields from future candidates.

## Delivered collection

`internal/agentprofile` is the shared Go projection. Native session logs provide
Claude model/version; Codex version/provider, model/effort/collaboration,
approval/sandbox and this session's recorded rate-limit tier; DSH request
model/provider/effort and plan events; Kimi main-agent request and plan/permission
events. Current-only hook snapshots add Claude model/effort/permissions, Cursor
model/version and optional parameters, Gemini model/thinking level/budget, and
OpenCode root-session model/provider/role/variant/explicit reasoning effort.
Optional Claude PostModelSwitch requires CLI >=2.1.251; Gemini BeforeModel/AfterModel
requires >=0.35.3. Installation probes the known CLI once; unknown/older versions
omit these optional bindings. Doctor does not demand metadata-only bindings.
No user status line is replaced. Existing installations need the normal hook
installation/repair flow to acquire new observers; opening a profile installs nothing.

The mobile avatar opens `/api/participant-profile` on demand, outside transcript
polling. Owner and paired-device access only. A historical request must identify
one loaded turn in its verified source session; duplicate/unknown times are refused.
Facts stop at its terminal event or the next prompt. Historical profiles never use
current hook snapshots or current process chains. Other senders retain their own
identity without borrowing the pane's runtime. Hook-only metadata is not historical
storage: older OpenCode/Cursor/Gemini message details can remain unrecorded.

JSONL indexes retain at most 16 files, 1,024 metadata points per file, a 2 MiB initial
tail plus first header, and 1 MiB per record; idle entries expire after ten minutes.
Append reads reuse offsets; replacement/truncation resets them. Limits yield partial
results; I/O failure is marked unavailable. DSH Zstandard reuses its frame cursor:
a cold read can decode the selected session file from its beginning, then appended
frames only. This is not a global event-log scan or a strict 2 MiB compressed-file
limit. Bounded metadata snapshots live privately under `participant-profiles/`
(0700 directory, 0600 files), scoped by agent, home and session.

The other seven integrations keep registry identity and any observed launch chain.
There is no account query, configuration execution, model request, arbitrary CLI
probe, or guessed model/tier for them. The full table includes future adapter
opportunities; it is not a promise that every cell is already collected.

## Presentation

Keep the selected dark centered profile dialog. **The six facts in the Codex mock
are an example, not a fixed template for every agent.** The title identifies the
harness; a subtitle can identify its client or role. Available facts flow through
the same two-column layout without leaving empty slots; larger Dynamic Type uses one column.

- Main facts: model, version, mode, and reasoning options for the relevant turn;
  provider and subscription appear when supported by evidence.
- Agent-specific facts: for example, OpenCode roles/variants or Aider's main/editor models.
- A collapsed execution-chain section expands into recorded programs, paths, and
  sanitized arguments. Long commands scroll horizontally.
- Source and observation time live in a secondary disclosure, rather than making
  every fact read like a diagnostic log.
- Inapplicable fields occupy no space. Missing fields leave no empty slots; an empty profile explains that details
  were not recorded. A failed read has a separate unavailable message.
  This introduces no new radar states or capability scores.

Subscription, task mode, permission policy, and reasoning options are distinct.
`Pro`, `Plan`, `yolo`, and `high` must not become one “Plan” property. Thinking
enabled, token budget, and model variant are not interchangeable effort levels.
Preserve raw provider values; presentation uses OpenAI, Anthropic, DeepSeek, DeepSeek Account and Google for those known IDs, leaving custom IDs literal. Use “Reasoning effort” only for an explicit effort field.

Opening a message avatar describes **that participant at that message**. Current
runtime facts may appear separately with that label; today's installed binary,
model, or pane cannot replace historical facts. Closing preserves reading position
and draft. The human profile keeps its human title, small “碳基决策者 · 自封” line,
and Dune quotation, without the removed humorous paragraph.

The dialog fits settled content up to its height limit, with scrolling for longer profiles. It reserves a stable placeholder body while the first request is pending. Loading uses a labelled, static placeholder; it never flashes the unrecorded state or a previous target's facts. Failed reads offer Retry; an empty successful result remains unrecorded. The identity header is compact, provenance uses quiet label/value rows, and a reserved close-control band keeps scrolled content clear of Close.

A message's historical facts and its current launch-chain observations are separate views. Opening the chain lazily makes a second current-profile request only when the message and current agent share a source session ID; the normal server binding checks still apply. Only that response's chain and observation time are shown, never its current model, tier or effort. Returned session/agent identity must match the message profile. A header profile reuses its existing current response. Stitched older sessions and other senders do not borrow another runtime; absent observations say so explicitly. A chain describes the latest recorded ancestry, not live ownership or the processes at the message's time.

## Existing integration boundaries

The radar [`agentJSON`](../../internal/radar/agents.go) still carries identity and
lifecycle, not profile facts. The new endpoint composes native metadata separately.
The global [Codex limits overview](../../internal/limits/codex.go) is never used to
assign a tier to a profile. A per-session recorded tier is historical evidence,
not a live account or billing check. Existing OpenCode `octrans` remains text-only.

## Coverage of all 14 agents

| Agent | Model/provider source or candidate | Reasoning/mode source or candidate | Subscription boundary | Profile differences |
| --- | --- | --- | --- | --- |
| `claude` | `SessionStart.model`, `PostModelSwitch.to_model`; assistant message model for turn records | Native `effort.level` and `permission_mode`; plan can identify planning, other values remain permission policy | Generic hooks do not provide a tier; account status needs a separately verified account/billing binding | Model, effort, task mode, permission policy; version only from a matching record/runtime snapshot |
| `codex` | Matching rollout `session_meta.model_provider` and `turn_context.model` | `turn_context.effort`, `collaboration_mode`; approval/sandbox separately | Matching `rate_limits.plan_type` evidence, never a tier borrowed from the global overview | CLI, desktop, and wrappers remain Codex; collaboration and permission facts remain distinct |
| `dsh` | v4 `request/header`, `data.header.config.provider/model` | Same config's `reasoningEffort`; native `plan/mode.active` or verified plan projection | Do not invent a DeepSeek Pro tier from provider or login | DeepSeek Harness title; dst/Web usage; separate harness and client versions |
| `opencode` | Native `chat.message.model.providerID/modelID`; model/provider in `chat.params` | `agent` is a role; `variant` is a model variant; only verified provider reasoning options are effort | Multiple providers; brand, OAuth, or model does not establish a tier | Role (Build/Plan/custom), provider, model, variant; no universal low/high mapping |
| `kimi` | Main-agent wire `llm.request.provider/model/modelAlias` | `thinkingEffort`, `plan_mode.enter/exit/cancel`; permissions from `permission.set_mode` | Request journal has no tier; official account queries are a separate, unconnected path | Model/alias, reasoning options, planning and permissions; exclude other agents |
| `gemini` | Version-gated native `BeforeModel/AfterModel.llm_request.model` | Version-tested thinking options in `llm_request.config`; common hooks have no plan-mode field | Login method, model, and Google account do not establish a tier | Preserve thinking level versus budget; no plan-mode inference from defaults |
| `cursor` | Native `model_id`, legacy `model` fallback | Optional `model_params` effort/thinking/context, with their own meanings; plan needs separate evidence | `user_email` is not a tier and stays out of the DTO | Label `cursor_version` as Cursor version, not an independent CLI/harness version |
| `copilot` | Existing command-hook contract provides no model; recorded launch selection is only a launch fact | No inference of Plan or effort from tool events | GitHub login does not establish a Free/Pro/Business tier | Bound identity, runtime, observed version and launch records first; omit unproven model facts |
| `kiro` | Agent profile `model` is configuration, not an observed request; no verified effective-model hook source | Profile name is not plan mode; permissions are not effort | No tier inferred from brand/login | Recorded configuration profile if available; separate CLI 2.x/3.x adapters, no borrowed IDE facts |
| `hermes-agent` | Partial classification/resume only; no installer, content or model reader | No connected source | No connected source | Observed identity/runtime only; a matching name does not establish a full profile |
| `grok` | Resume command only; no verified readable model protocol | No connected source | No connected source | Do not silently choose a same-named community CLI or xAI model as the protocol |
| `aider` | Process detection only today; future explicit `--model`, `--editor-model`, `--weak-model` launch snapshots | Native code/ask/architect/help modes; edit format separately | API/local-model use and subscription are separate facts | Main/editor/weak model and edit format; no claim to track in-session `/model` changes |
| `crush` | Process detection only; large/small provider configuration is not a session request | Configured thinking/reasoning is not observed runtime state | Do not impose Hyper subscription or API billing on every session | Basic profile now; future slot-aware adapter; never execute `crushrc` to inspect it |
| `amp` | Process detection only; upstream mode routing can change | Mode needs a dedicated source; never derive fixed model/effort from its name | Connected model subscriptions do not establish the Amp account tier | Prefer observed mode; no hard-coded mode-to-model table |

### Claude Code

Use native-hook extraction with session/prompt/agent scoping.
Session-start selection may be absent and does not track later switches; observe a
verified post-switch event rather than treating a pre-switch request as success.
Keep a reply's recorded model when it differs from session selection.

Status-line JSON is another source for runtime version/model/effort, but profiles
must not replace a user's status line. Any future collection there must compose
transparently; the implemented hook path does not require that change. Subscription
is not promised automatically: even a current account-status result needs to match
the session's billing route, including API or third-party gateways.

### Codex

Read `cli_version` from the physical rollout segment containing the target
message. A logical session can resume under another version into another file;
its earliest creation version is not universal. Scope model/effort/collaboration
to turns; show approval/sandbox separately. Rate-limit tiers need matching
session/account evidence. Desktop or wrapper hosting neither changes `codex`
identity nor grants input permission.

### DeepSeek Harness

Reuse v4 JSONL/Zstandard incremental reading and native `session/event`, extracting
only allowed metadata, not request text, system prompts, or reasoning content.
Absent effort does not establish a default. Adapter-default attribution can be
shown when its explicit marker supports it.

An absent plan capability is not normal mode. Use active/pending semantics only
with verified capability and complete native projection; sparse history remains
unknown. A pending change may annotate the profile row, without adding a radar state.

Show dst as terminal client with its own version. Harness version must identify
the package actually loaded by that process; global dsh and dst's backend dependency
can differ. Web history without a live chain shows only retained origin evidence.
Do not start, mount, or write a session to inspect it. Header `version=4` is a log
format, not a harness version.

### OpenCode and Kimi

Observe allowed fields through the existing OpenCode native plugin in verified
root-session `chat.message`/`chat.params`, without modifying inputs, parameters,
or permissions. Exclude title/compaction/subagent requests. A selected variant is
not proof of API-accepted effort; provider options need their own tested contract.
Old text-only `octrans` cannot reconstruct missing historical metadata, and this
does not require importing the upstream database. Version must describe the
actual server, not its SDK or another CLI on PATH.

For Kimi, incrementally decode selected main-agent wire types and exclude
compaction requests. `llm.request` describes the latest request, whereas
`profile.bind/config.update` describes selection. Preserve that distinction;
without new events the previous request is not a live selection reading.
`plan.revision` content does not prove plan mode: follow enter/exit/cancel semantics.
Journal `protocol_version` is not the CLI version.

### Partial integrations

Gemini model observations require version-tested installer bindings and allowed
field extraction, not serialization of all request messages/configuration.
BeforeModel sees a proposal that later hooks may modify, not server confirmation.
Version-test new optional Cursor fields and keep old clients usable without them.
Upstream SDK features do not imply existing Copilot/Kiro/Hermes/Grok command hooks
provide the same data.

For Aider/Crush/Amp, do not type `/model` or `/status`, start model turns, or ask an
agent to describe itself as a collection technique. Explicit launch flags describe
launch only. Do not infer active choices from defaults; do not execute Crush's
configuration scripts to read them.

### Concrete content layouts

Use the same avatar/name → main facts → execution chain → source-details skeleton.
Prioritize 4–6 useful facts; additional facts continue in the same scrollable
dialog without shrinking type. Do not introduce separate brand pages or navigation.

| Profile | Main-fact priority | Continuation and omission rules |
| --- | --- | --- |
| Claude Code | Turn model, version, effort, task mode; tier only when bound | Permission policy separately; label launch-only model evidence accordingly |
| Codex | Turn model, version, effort, collaboration, bound tier, runtime | Approval/sandbox below; distinguish desktop client from historical launch host |
| DeepSeek Harness | Model, provider, reasoning, plan mode, harness version, runtime | dst client/version separately; no terminal-client row for Web; unknown plan capability is not normal mode |
| OpenCode | Role, model, provider, variant, server version, runtime | Preserve custom role names; add effort only when explicit; no empty tier slot |
| Kimi Code | Turn model, alias, reasoning, plan mode, version, runtime | Provider/profile/permissions below; explain selection versus last request when both exist |
| Gemini / Cursor | Model, native reasoning parameters, observed version, runtime | Preserve Gemini level/budget and Cursor thinking/context/effort distinctions |
| Other partial integrations | Verified name, runtime, version or explicit launch selection | Naturally shorter without request collectors; retain Aider model roles, Crush slots and Amp modes rather than inventing values |

These are priorities, not mock values. Show two facts when only two are supported
by evidence. A short profile is not an error or an unconfirmed-control diagnosis.

## Shared implementation contract

Use one core fact model with agent-specific decoders, not separate brand switches
in mobile, menu bar, and Web.

1. Derive identity from the registry. Declare collectors and fields per agent;
   new entries default to basic identity, without inheriting Codex capabilities.
2. Scope each fact to agent, opaque canonical-home identity, session, participant,
   optional turn/message, source, observation time and validity interval. Account
   facts also need internal account scope. Cwd, display name or pane alone is insufficient.
3. Internally distinguish unsupported, missing evidence, inapplicable, failed,
   and stale. The UI applies the simpler presentation above. Read failure is not
   unsupported, and absent effort is not thinking disabled. Resolve scope before freshness.
4. Keep trusted native hooks first for lifecycle. For profile facts, reply records,
   applied-mode events, request options and launch snapshots have different meanings;
   do not impose a universal hook-over-log ordering that loses more precise evidence.
5. Persist bounded sanitized fields. Reuse incremental file cursors and invalidate
   on replacement/truncation. No full history/global-event decode on each click or
   two-second poll, and no `--version` subprocess per bubble render.
6. Authorize the read-only profile endpoint with existing session access.
   Executable paths, commands and account facts require existing owner scope;
   avatar taps must not expose them to guests. This design changes no current policy.

## Execution chain and version

Existing [`launchhost`](../../internal/launchhost/launchhost.go) exports only host
name, kind and observation time, deliberately excluding argv. The mock's expanded
chain uses the separate bounded native-hook ancestor collector, rather than relabeling that host field.

- Prefer gtmux-owned launch recipes or an ancestor snapshot of the bound runtime
  during trusted native hooks. Recheck PID/birth/parent identity; missing, reused,
  or ambiguous processes yield unknown facts.
- Show provable process relations, including interpreter/script distinctions.
  Do not reconstruct vanished pre-exec commands, remote clients, or entire desktop
  hosts. A shared server does not prove exclusive session control.
- Separate path from parameters. The flattened `ps` argv string cannot distinguish flags from prompt text.
  Therefore the delivered collector persists names/paths and a recognized absolute
  Node DSH/dst entry script only; it does not reconstruct model/mode/effort flags. Omit arbitrary positional arguments, prompts, inline
  scripts, environment assignments, credentials, and credential-bearing URLs
  before persistence. Copy the displayed sanitized content only, labeled as an
  observation rather than a replayable command.
- Prefer explicit version records for that message/runtime. Otherwise verify the
  bound executable artifact or loaded package. Another PATH install, SDK version,
  protocol version or earlier creation version is not a substitute. Without a
  verified reader, keep it unrecorded; never execute an unknown wrapper to probe it.

## Implementation acceptance

Each declared field needs that agent's own sanitized contract fixtures. Cover
old/missing fields, model changes, absent defaults, cross-version resume,
main/subagent isolation, account switching, shared hosts, historical speakers,
PID reuse, replacement/truncation, sensitive argv, unauthorized guests, and
process-only integrations. Then check phone/iPad reflow, large text, horizontal
commands, 44pt hit targets, VoiceOver and restored reading position. Report source
inspection, author fixtures, independent review, real model turns and physical
device acceptance separately.

Synthetic contract, historical-boundary, source-binding, privacy and UI interaction
regressions cover the delivered collectors. Real model turns, third-party runtime
versions outside the pinned contracts, physical devices and VoiceOver speech need
separate acceptance. Author tests are not independent review. This implementation
does not itself apply new hooks to user configuration.

## References checked

- Claude: [native hook fields](https://code.claude.com/docs/en/hooks#common-input-fields), [status line](https://code.claude.com/docs/en/statusline#available-data), [account-status command](https://code.claude.com/docs/en/cli-reference). Availability remains version-specific.
- Codex: [0.163.0 protocol](https://github.com/openai/codex/blob/rust-v0.163.0/codex-rs/protocol/src/protocol.rs), [gtmux rollout reader](../../internal/transcript/codex_reader.go), [limits reader](../../internal/limits/codex.go).
- DSH: [0.2.0-rc.2 call config](https://github.com/deepseek-ai/deepseek-harness/blob/dsh-v0.2.0-rc.2/packages/llm/llm/src/call-config.ts), [session types](https://github.com/deepseek-ai/deepseek-harness/blob/dsh-v0.2.0-rc.2/packages/core/session/src/types.ts), [plan module](https://github.com/deepseek-ai/deepseek-harness/tree/dsh-v0.2.0-rc.2/packages/plan/plan-mode). Installed types at the same version and dst 0.16.1 package metadata were inspected read-only; installation does not prove a session's runtime version.
- OpenCode: [1.18.11 plugin types](https://github.com/anomalyco/opencode/blob/v1.18.11/packages/plugin/src/index.ts), [roles](https://opencode.ai/docs/agents/), [current bridge](../../internal/app/agent_hooks.go). Newer upstream APIs are evolving and require separate pinned-version validation.
- Kimi: [pinned wire manifest](https://github.com/MoonshotAI/kimi-code/blob/5710c5496a213742ca0687f69dd8eeb5bde52dbf/packages/agent-core-v2/docs/wire-manifest.d.ts), [plan/permission interaction](https://www.kimi.com/code/docs/en/kimi-code-cli/guides/interaction), [gtmux reader](../../internal/transcript/kimi.go).
- Gemini: [hook contract](https://geminicli.com/docs/hooks/reference/); installed 0.35.3 BeforeModelInput/AfterModelInput types were also inspected without running/changing the CLI.
- Cursor: [common hook fields](https://cursor.com/docs/hooks#input-all-hooks). Current optional online fields are not claimed to exist in the older installed CLI.
- Copilot: [hook payloads](https://docs.github.com/en/copilot/reference/hooks-reference#hook-event-input-payloads); Kiro: [hooks](https://kiro.dev/docs/hooks/types/) and [profile configuration](https://kiro.dev/docs/custom-agents/configuration-reference/).
- Aider: [modes/multiple models](https://aider.chat/docs/usage/modes.html); Crush: [configuration/model slots](https://github.com/charmbracelet/crush/blob/main/docs/config/README.md); Amp: [mode routing](https://ampcode.com/docs/the-dial#models-behind-the-dial). These upstream features are not counted as delivered gtmux integration.

## HQ identity

HQ retains its single name, with “gtmux coordinating agent” as the role subtitle. It is not a separate harness. Its profile explains
coordination, progress follow-up, report review and escalation of user decisions.
On the HQ conversation itself, the recorded reply/profile identifies the harness;
its facts retain the ordinary historical/current scope.

On an HQ sender avatar in another conversation, **Agent at delivery** uses only
the sender's recorded agent, if present. Existing HQ deliveries generally recorded
only the role, so this remains **Not recorded**. **Current HQ** separately uses the
single supervisor row from the same client's connected radar. Disconnected,
ambiguous or other-server state does not supply this identity. This adds no request
or polling, and never uses the receiving pane's agent or presents the current HQ
as the sender at the message's time.

The global reply-fold action retains visible brand-coloured Collapse all / Expand
all wording and a minimum 44pt target. Per-reply and tool disclosures stay independent.
