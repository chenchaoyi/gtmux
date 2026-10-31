# HQ conversation context and one matter overview

## Why
Talk to HQ currently keeps resource cards outside its disclosure. Its Needs attention tab derives waiting agent sessions from digest, while the outer overview derives persistent HQ matters. The two entrances appear to disagree and leave less space for the conversation.

## What changes
Make the conversation one reading/input surface on workspace-capable servers. Remove its duplicate Calls/Console tabs and waiting-session inspector, return to the authoritative overview for matters, and keep that shared snapshot current while talking. Reuse one matter-count/summary model across both entrances. Fold resources, machine details and HQ updates together; retain a critical machine condition in the compact headline. Bound the expanded context with a scroll area, and align HQ identity/typography with the overview. Older servers retain a clearly named Waiting sessions view.

## Surfaces
- terminal: no CLI, input protocol or state ownership changes.
- menubar: no code change; persistent HQ matters remain the reference model.
- phone: compact conversation context, shared overview navigation/counts and no duplicate attention tab.
- iPad: same components; workspace-backed conversation uses the readable main area, without the legacy waiting-session inspector.
- Web: no code change; existing read contract and permission scope are preserved.

## Boundaries
No automatic send, follow, approval or permission change. Unknown workspace data must not be presented as zero. Transcript, composer, resource readers and session tools remain reachable. Header folding must preserve the mounted chat/draft. Native rendering, React callback actions, native taps, physical devices and VoiceOver are separate evidence.
