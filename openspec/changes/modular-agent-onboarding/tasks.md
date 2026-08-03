## 1. The registry (source of truth)

- [ ] 1.1 Add `internal/agents` with the `AgentManifest` type (Key/Aliases/Display/Commands/IdleGlyph/Icon/Resume/ResourceNames/Install/EventSemantics/Transcript/Driver/PromptSignatures) and a `Register`/`For`/`All` API; leaf-only imports so driver/radar/hook/app can consume it without an import cycle.
- [ ] 1.2 Author manifests for the current agents (claude, codex, gemini, cursor, cursor-agent, opencode, copilot, kiro) reproducing today's values exactly.
- [ ] 1.3 Golden test: assert the registry reproduces every existing per-agent map (driver caps, profiles, installers, semantics, resume, resource, prompt signatures) byte-for-byte before any subsystem switches.

## 2. Migrate subsystems to read the registry (one per commit, tests green each step)

- [ ] 2.1 `internal/driver`: derive `hookEquippedAgents` + registry capabilities from manifests; delete the private lists.
- [ ] 2.2 `internal/radar`: derive `defaultProfiles` (Name/Commands/IdleGlyph/Icon) from manifests.
- [ ] 2.3 `internal/hook`: derive `agentDisplay` + per-agent `classify` semantics from manifests (keep the generic table as the fallback).
- [ ] 2.4 `internal/resume` + `internal/resource`: derive resume commands + attribution names from manifests.
- [ ] 2.5 `internal/prompt`: derive bootBanners / promptGlyphs / selector signatures from manifests.
- [ ] 2.6 `internal/app/agent_hooks.go` (+ codex_hooks/hooks): derive `agentInstallers` from the manifest install specs.

## 3. Generalize the install spec (plugin model)

- [ ] 3.1 Make the install spec an interface with `Kind ∈ {jsonHooks, plugin}` that materializes an artifact (path, contents, dedicated); the five current agents return their JSON file unchanged.
- [ ] 3.2 Make `install-hooks --agent <key>`, uninstall, and `doctor` treat both kinds uniformly (write / detect-installed / remove-only-what-we-wrote).

## 4. opencode onboarding (Tier 1 acid test)

- [ ] 4.1 Fill the opencode manifest: IdleGlyph + Icon + resource names + prompt/ready signatures for its TUI; verify detection via the process subtree.
- [ ] 4.2 Ship the opencode plugin artifact (`~/.config/opencode/plugins/gtmux.js`) mapping session.idle→Stop, permission.asked→PermissionRequest, permission.replied→PostToolUse, session.created→SessionStart, session.deleted/error→SessionEnd, session.compacted→Pre/PostCompact, message.updated(user)→UserPromptSubmit — verified against the installed opencode version.
- [ ] 4.3 Live-verify: `install-hooks --agent opencode`, then confirm a real opencode pane drives waiting/done, a receipt-verified `gtmux send`, and a notification; uninstall leaves user config intact.

## 5. Playbook + conformance

- [ ] 5.1 Write `docs/design/agent-onboarding.md`: the step-by-step process (author manifest → register → verify each subsystem) and the pitfalls checklist (subtree identity, hook-must-be-installed, plugin vs JSON model, locale/glyph over daemon PTYs, sparse-events→screen, idle-glyph needs live confirmation, no third-party trademarks, the Swift/RN mark+icon sites).
- [ ] 5.2 Reference the playbook from `CLAUDE.md` (the agent-support section).
- [ ] 5.3 Extend `scripts/check-design.sh`: fail if a hook-equipped agent has no manifest, or if a migrated subsystem reintroduces a private per-agent list (registry file allowlisted).

## 6. Spec sync

- [ ] 6.1 Run `openspec validate --specs --strict` + `make check` + `check-design.sh` green.
- [ ] 6.2 sync-specs (fold the `agent-integration` capability into `openspec/specs/`) + archive-change.
