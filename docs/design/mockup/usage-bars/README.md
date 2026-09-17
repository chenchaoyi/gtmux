# Usage bars · blue, amber, red (usage-bar-tiers, 2026-09-17)

Five artboards from the design canvas the commander chose from, after pointing at claude.ai's
plan usage page as the reference. `Main` and `PhoneDark` are the phone's usage sheet in light
and dark, `MacLight` and `MacDark` the menu-bar reader's Usage tab, and `TierSpec` the rule:
the bar's anatomy (6pt, round ends, a pale track of the fill's colour with a hairline edge, a
round dot as the least a value draws) and the three tiers with their values. The commander
picked blue for an ordinary window over the grey alternative on `TierSpec`, and deleted the
separate warning line from both Mac artboards, so the shipped reader has none. The four
screens use the live readings of 17:05 that day; the amber row on `TierSpec` is the only
example value. Same format as `../usage-activity/`; values lifted from
`mobileapp/src/ui/theme.ts`, `UsageSheet.tsx` and `macapp/Sources/GtmuxBar/HQReader.swift`.
Re-seed with the `/design` helper to view; the seeded page is not committed.
