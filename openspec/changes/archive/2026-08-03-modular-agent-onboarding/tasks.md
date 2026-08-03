## 1. The registry (source of truth)

- [x] 1.1 Add `internal/agents` with the `AgentManifest` type (Key/Aliases/Display/Commands/IdleGlyph/Icon/Resume/ResourceNames/Install/EventSemantics/Transcript/Driver/PromptSignatures) and a `Register`/`For`/`All` API; leaf-only imports so driver/radar/hook/app can consume it without an import cycle.
- [x] 1.2 Author manifests for the current agents (claude, codex, gemini, cursor, cursor-agent, opencode, copilot, kiro) reproducing today's values exactly.
- [x] 1.3 Golden test: assert the registry reproduces every existing per-agent map (driver caps, profiles, installers, semantics, resume, resource, prompt signatures) byte-for-byte before any subsystem switches.

## 2. Migrate subsystems to read the registry (one per commit, tests green each step)

- [x] 2.1 `internal/driver`: derive `hookEquippedAgents` + registry capabilities from manifests; delete the private lists.
- [x] 2.2 `internal/radar`: derive `defaultProfiles` (Name/Commands/IdleGlyph/Icon) from manifests.
- [x] 2.3 RE-SCOPED (kept domain-local): classify semantics tables stay in internal/hook (the `semantic` enum is hook-domain — moving it is over-abstraction); registry-keyed + conformance-checked + documented in the playbook.
- [x] 2.4 `internal/resume` + `internal/resource`: derive resume commands + attribution names from manifests.
- [x] 2.5 RE-SCOPED (kept domain-local): prompt boot/ready signatures stay in internal/prompt; registry-keyed + documented. See design.md over-abstraction note.
- [x] 2.6 `internal/app/agent_hooks.go` (+ codex_hooks/hooks): derive `agentInstallers` from the manifest install specs.

## 3. Generalize the install spec (plugin model)

- [x] 3.1 Make the install spec an interface with `Kind ∈ {jsonHooks, plugin}` that materializes an artifact (path, contents, dedicated); the five current agents return their JSON file unchanged.
- [x] 3.2 Make `install-hooks --agent <key>`, uninstall, and `doctor` treat both kinds uniformly (write / detect-installed / remove-only-what-we-wrote).

## 4. opencode onboarding (Tier 1 acid test)

- [x] 4.1 Fill the opencode manifest: IdleGlyph + Icon + resource names + prompt/ready signatures for its TUI; verify detection via the process subtree.
- [x] 4.2 Ship the opencode plugin artifact (`~/.config/opencode/plugin/gtmux.js` — singular `plugin/`, confirmed load dir on 1.18.11) mapping session.idle→Stop, permission.asked→PermissionRequest, permission.replied→PostToolUse, session.created→SessionStart, session.deleted/error→SessionEnd, session.compacted→PreCompact. UserPromptSubmit (the send-RECEIPT) is DEFERRED — its message.updated payload shape needs verifying on a machine with opencode credits; until then opencode send falls to screen verification (no regression).
- [x] 4.3 Live-verify: `install-hooks --agent opencode`, then confirm a real opencode pane drives waiting/done, a receipt-verified `gtmux send`, and a notification; uninstall leaves user config intact.

## 5. Playbook + conformance

- [x] 5.1 Write `docs/design/agent-onboarding.md`: the step-by-step process (author manifest → register → verify each subsystem) and the pitfalls checklist (subtree identity, hook-must-be-installed, plugin vs JSON model, locale/glyph over daemon PTYs, sparse-events→screen, idle-glyph needs live confirmation, no third-party trademarks, the Swift/RN mark+icon sites).
- [x] 5.2 Reference the playbook from `CLAUDE.md` (the agent-support section).
- [x] 5.3 Extend `scripts/check-design.sh`: fail if a hook-equipped agent has no manifest, or if a migrated subsystem reintroduces a private per-agent list (registry file allowlisted).

## 6. Spec sync

- [x] 6.1 Run `openspec validate --specs --strict` + `make check` + `check-design.sh` green.
- [x] 6.2 sync-specs (fold the `agent-integration` capability into `openspec/specs/`) + archive-change.
