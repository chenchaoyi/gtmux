# Fold trace levels independently and inspect recorded context

## Why
The user's DSH Web examples make long agent traces easier to scan: whole turns and
consecutive calls have separate disclosures, each compact call group remains openable,
and detail modes distinguish reading from original content and provenance. Settings
also need clear categories without enlarging the compact browser panel.

## What Changes
- Share turn/call projection and context indexing across browser, phone and iPad.
- Keep bulk and individual choices stable through refresh; search reveals matches temporarily.
- Add bounded explicitly recorded context without expanding guest permissions.
- Offer Overview / Preview / Raw content / Source using actual transcript data.
- Add compact settings category navigation, explanations and appearance icons.

## Surfaces
- Terminal / attach: no keystroke, attach or binding changes; additive transcript context only.
- menubar: no settings redesign; the separate Ghostty title-identity fix is PR #1769. Local browser opens the same reader.
- phone: native virtualized timeline, independent disclosures and four detail modes.
- iPad: shared reader with measured-width split/drill-in behavior and accessible targets.
- Web: shared model, independent disclosures, bounded details and navigable appearance/chat/terminal settings.

## Data boundary
Context comes only from known recorded public harness envelopes or explicit provider
origins, never private reasoning or tool results. Owner/full-scope paired readers receive
it; guest transport strips it and separates cache revisions. Existing message text,
share permissions, source logs and response budgets remain unchanged.

Compatible readers explicitly opt in with context=1 so older App releases retain their existing segment shape and do not render blank context wrappers.
