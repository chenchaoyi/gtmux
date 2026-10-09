# Design

The selected Product Design concept is the per-conversation settings form (option 3).
See `docs/design/desktop-follow.md` and its Chinese counterpart for wire contract,
collection boundaries, UI feedback and test mapping. A leaf `sessionpolicy` package
owns exact-originator consent and atomic revision-checked storage outside HQ archives.
Unknown agents retain their existing behavior; no cwd/title heuristics or new desktop
control capability is added. Status metadata remains visible; conversation content needs
explicit observation consent, and future mining needs separate knowledge consent.

Codex records without an ordinal use file identity and byte offset for stable provenance; incremental reads retain session identity. On iOS, settings presentation waits for the row menu native dismissal, and switching conversations cancels that pending handoff.
