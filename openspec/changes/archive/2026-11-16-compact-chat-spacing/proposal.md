# Compact App conversation spacing

## Why
Chat stacks bubble padding, Markdown block spacing and segment gaps. Avatars and
reply controls add more apparent empty space, separating commentary, Activity and
the final reply more than the content warrants.

## What changes
Tighten shared Chat gutters, turn/segment gaps and date separators. Place incoming
avatar artwork beside the bubble inside its existing target. Count Markdown's
bottom block space once, preserve plain-text preview padding, and bring reply
icons toward the top of their unchanged targets. Keep fonts, selection, receipt
placement, disclosure ownership and sending behavior.

## Surfaces
- Terminal (incl. attach): no terminal rendering or input changes.
- Menubar: no App Chat renderer; unchanged.
- Phone: compact shared Chat and HQ conversation layout in both appearances.
- iPad: the same renderer and spacing, retaining current regular reading width.
- Web: independent browser layout; unchanged by this App-only adjustment.

## Validation boundaries
Reuse existing hosted App checks and native emulator journey evidence. This is a
layout change, with no new dependency or mirrored numeric-style tests. Hosted
Android rendering does not establish iPhone/iPad or spoken VoiceOver acceptance;
CI and review results belong to the PR and delivery report.
