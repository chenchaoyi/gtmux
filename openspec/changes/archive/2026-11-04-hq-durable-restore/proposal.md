# Durable Codex restore and HQ handoff

## Why

A managed HQ switched to an empty Codex thread before reboot. The live attachment
was valid, but no durable thread history existed. Restore replayed the ID and logged
success before Codex reported that it could not find the conversation. The terminal
pane-ID receipt also used two lines per window, obscuring large workspaces. Directory
recency produced binding recommendations despite direct frontend ownership evidence.

## What changes

- Separate live identity from verified recovery checkpoints; inspect bounded canonical
  metadata in the correct Codex home, with legacy/archive/continuation/compressed support.
- Confirm HQ's durable successor and first-turn activity, continuing the existing
  guarded startup handoff once without replaying a reset or overwriting a draft.
- Distinguish command submission, fresh target confirmation and failures in restore
  results; render aligned compact pane mappings and accurate startup statuses.
- Prefer exact frontend binding proof over directory heuristics; unsupported legacy
  proximity observations remain informational, not repair requests.

## Surfaces

- terminal: compact pane table, explicit startup states and shared preflight checks.
- menubar: same restore/doctor core facts; no runtime application restart required.
- phone: existing shared status/Chat identity, no new native UI or input authority.
- iPad: same phone core contract; no independent restore implementation.
- Web: existing status consumers retain their contract; no body or credential logging.

## Safety and validation

No real HQ rotation, restore, client input or shared daemon changes in acceptance.
Use synthetic homes/history and private tmux servers, test missing/corrupt/foreign
history and switched-but-unsaved threads, retain prior checkpoints without silently
resuming them, protect drafts and verify bilingual narrow terminal rendering. Existing
latest-head and actual-main gates remain required; no release/install authorization.
