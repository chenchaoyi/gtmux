# release-notes — the per-version archive the in-app What's New reads

The App Store metadata (`fastlane/metadata/*/release_notes.txt`) holds only the CURRENT
submission's notes: fastlane overwrites it every release. That is fine for the store, which
shows one version, but the in-app popup has to answer a harder question — **a user who
skipped three versions must see all three** — so the notes are ARCHIVED here, one pair of
files per shipped version, and `src/releaseNotes.ts` is generated from the whole directory.

- `<version>.en.txt` / `<version>.zh.txt` — one bullet per line, `•` optional.
- `mobileapp/scripts/set-version.sh` copies the current fastlane notes in under the version
  being stamped, then regenerates `src/releaseNotes.ts`. So the normal release flow needs no
  extra step: write the store notes as always, stamp the version, and the archive grows.
- `scripts/check-design.sh` fails when the generated file is stale, by regenerating it and
  diffing — byte-exact, no guesswork.

History before this archive existed lives only in App Store Connect; a user upgrading from
one of those versions sees the notes from here onward, which is everything the app can
honestly claim to know.
