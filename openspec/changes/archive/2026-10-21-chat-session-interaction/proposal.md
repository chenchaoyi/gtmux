# In-conversation session interaction

## Why
Chat omits transient terminal prompts. Clean numbered menus are supported, but an empty parse hides the mobile card entirely, leaving complex permissions/pickers invisible. The reader should see and operate the live prompt without leaving Chat.

## What changes
- A bounded read-only current-screen API, with the existing hook ask gate for numbered choices; never parse scrollback or invent options.
- A compact, scrollable interaction card in the conversation, including loading/failed/empty states and the existing keyboard controller as fallback.
- Manual inspection when a hook does not identify the wait. This does not infer or change agent status.
- Existing scope, input consent, binding, key result and retry boundaries apply; native external conversations remain read-only.

## Surfaces
- CLI / terminal: existing options and terminal input stay available; no new CLI command.
- menubar: existing options/focus behavior remains; no duplicated screen parser.
- iPhone / phone: automatic waiting card and manual Chat interaction, with readable prompt and keys.
- iPad: the same bounded component in the detail pane; device accessibility acceptance recorded separately.
- Web: the same read-only snapshot and keyboard fallback in Chat, preserving view-only sharing.

## Boundaries
No automatic approval, speculative default, process launch, configuration change, release or installation. Source fixtures are synthetic. Real prompts are private and never committed.
