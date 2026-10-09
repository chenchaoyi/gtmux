# Read desktop Codex conversations

## Why
Desktop Work sessions already have local Codex rollouts, but gtmux shows only identity and follow settings. Owners want to read prompts, intermediate commentary and replies from their phone as activity happens.

## What changes
- A verified desktop conversation opens a read-only chat reader independently of HQ follow permissions.
- Go resolves its session ID to verified local rollouts and exposes bounded turns with conditional revisions; no arbitrary file paths, private analysis, runtime input or resume.
- Phone/iPad reuse ChatView; a separate reader polls serially while visible and active, retains history after transient failures, and ignores late responses after switching session/server.
- Mac opens a native utility reader using a core CLI read. Follow settings retain their separate entry.
- Missing/unsupported/offline/error states are explicit. Reading never opts into notifications, knowledge or HQ follow.

## Surfaces
- Terminal: a read-only transcript command for a verified desktop conversation.
- Menu bar: row opens a native conversation reader; gear/context menu opens follow settings.
- Phone: tap opens read-only chat, live commentary included; follow settings stay in the long-press menu.
- iPad: same reader in the existing detail/split-view container and the same lifecycle rules.
- Web: existing status diagnostic list remains; this first batch does not add a web desktop reader.

## Verification
Owner/guest identity and error contract tests, resumed-rollout/conditional-update tests, UI request cleanup and read-only tests, Swift model/render tests, full repository gates. Physical iPhone/iPad/VoiceOver and real desktop runtime handoff are separate acceptance work. No release/install in this change.
