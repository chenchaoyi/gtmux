## Context

Per-agent knowledge lives in ~12 sites (see proposal Impact). They use inconsistent keys and
each carries its own agent-keyed map. The import DAG is strict and acyclic — `app → hq →
{radar, dispatchbridge} → leaves` (see `decompose-app-package`), so a registry that the leaf
subsystems (`driver`, `radar`, `hook`, `resume`, `resource`, `prompt`, `transcript`) all read
must sit at or below the leaf layer to avoid a cycle. The CLI must stay cgo-free.

## Goals / Non-Goals

**Goals:**
- One `AgentManifest` per agent, registered once, as the source of truth every subsystem reads.
- Behavior-preserving migration: the existing agents keep working, proven by the current tests.
- A hook-install mechanism that admits a plugin artifact, not only a JSON command-hook file.
- opencode onboarded to Tier 1 through the registry as the acid test.
- A `docs/design/agent-onboarding.md` playbook + pitfalls checklist + a conformance check.

**Non-Goals:**
- Rewriting the Swift menu-bar / RN mobile mark+icon maps into the Go registry (documented in
  the playbook; a possible follow-up feeds them from `agents --json`).
- Building the opencode transcript parser (Tier 2) in this change — it is a listed follow-up
  task; opencode ships Tier 1 here.
- Changing any external contract (`agents --json`, `digest --json`, hook event names).

## Decisions

- **A new leaf package `internal/agents` holds the manifest type + registry.** It has no
  gtmux-internal imports except shared leaves (`transcript`, `i18n`), so `driver`, `radar`,
  `hook`, `app` can all import it without a cycle. The manifest references a transcript parser
  and a driver-capability set by value/func, not by importing those packages back.
  - *Alternative rejected:* putting the registry in `radar` — several non-radar subsystems
    need it, and `hook`/`driver` must not depend on `radar`.
- **Migration is one subsystem per commit, each gated by its own tests.** The registry is
  introduced first (populated for today's agents, asserted equal to the current hardcoded
  maps by a golden test), then each subsystem is switched to read it and its private map
  deleted. This keeps every step reversible and green.
- **`AgentManifest` fields** (the SSOT): `Key`, `Aliases`, `Display`, `Commands` (detect),
  `IdleGlyph`, `Icon`, `Resume` (argv template), `ResourceNames`, `Install` (the install
  spec), `EventSemantics` (native-event → gtmux-semantic table, or "use generic"),
  `Transcript` (parser func or nil), `Driver` (Receipt/Ready/Content/Headless flags),
  `PromptSignatures` (bootBanners / promptGlyphs / selector). A nil/empty field means "this
  agent does not provide it" and the consuming subsystem falls back exactly as today.
- **The install spec is an interface, not a struct of JSON knobs.** `Install` is
  `{ Artifact() (path string, contents []byte, dedicated bool), Kind }` where `Kind ∈
  {jsonHooks, plugin}`. The five current agents return their JSON file; opencode returns a JS
  plugin. `install-hooks --agent <key>` and `doctor` treat both uniformly (write, detect,
  remove). This is the one genuinely new mechanism.
- **opencode plugin** subscribes to opencode's plugin events and shells out via Bun's `$`:
  `session.idle → Stop`, `permission.asked → PermissionRequest`, `permission.replied →
  PostToolUse` (clear waiting), `session.created → SessionStart`, `session.deleted/error →
  SessionEnd`, `session.compacted → PreCompact/PostCompact`, `message.updated` with a user
  part → `UserPromptSubmit`. Written to `~/.config/opencode/plugins/gtmux.js` (dedicated).
  The event names + payload shape are verified against the installed opencode version at
  implementation time, not frozen from memory.
- **Identity resolution is already correct** (`radar.AgentDriverKey`, subtree walk, added in
  the hook-equipped-subtree fix). The registry consumes it; the playbook codifies it as a
  rule so no future site regresses to `pane_current_command`.
- **Conformance check** in `check-design.sh`: assert every `hookEquippedAgents`-equivalent key
  has a manifest, and grep-guard against reintroducing a private per-agent map in the migrated
  subsystems (allowlist the registry file).

## Risks / Trade-offs

- **Cross-cutting refactor risk.** Mitigated by the golden-equality test (registry output ==
  today's maps) landing before any subsystem switches, and by one-subsystem-per-commit.
- **opencode event contract drift.** opencode iterates fast; the plugin's event names may
  change. Mitigated by isolating them in the one plugin artifact + verifying against the live
  version, and by Tier-1 degradation (a missing event just falls to screen verification).
- **Over-abstraction.** A manifest that is too generic becomes its own maze. Mitigated by
  keeping fields concrete (the exact ones the 12 sites need today) and letting "nil = fall
  back" carry the optionality instead of config flags.
- **Swift/RN left out.** The two app-side mark/icon maps stay hand-kept; the playbook lists
  them explicitly so they are not silently forgotten when an agent is added.

## Migration Plan

1. Introduce `internal/agents` with manifests for the current agents; golden test asserts it
   reproduces every existing map exactly. 2. Switch subsystems one at a time
   (driver → radar → hook/classify → resume → resource → prompt), deleting each private map.
3. Generalize the install spec to the plugin kind. 4. Add the opencode manifest (Tier 1) +
   plugin installer. 5. Write the playbook + pitfalls checklist; wire the conformance check.
6. sync-specs + archive.
