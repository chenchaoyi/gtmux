## Implementation
- [x] Implement verified conversation-link resolution and lazy snapshots with persistent revocation.
- [x] Add bounded parent-document resource resolution and authorization tests.
- [x] Render formatted native Markdown with source toggle and original sharing.
- [x] Verify native preview, state cleanup and generated output with synthetic fixtures.
- [x] Sync bilingual docs, API and specs, then archive.

## Acceptance
No installed binaries, gtmux app or real agent input were replaced by implementation tests.

- Go race checks cover document/app resolution, identities, historical Codex links, relative Claude links, revisions, source deletion, persistent removal, parent resources, symlink escapes and HTTP authorization. The complete artifact and server suites pass; server fixtures need loopback-listener permission outside the restricted sandbox.
- The cgo-free CLI builds to a private temporary directory. TypeScript reference/metadata tests pass, including bare relative Markdown links.
- The standalone UIKit app compiles the production Markdown renderer, preview and transport. Pure Swift checks cover semantic output and unsafe links/content.
- Authorized iPhone and iPad simulators display headings, lists, tables, code and a verified local image; DOM checks find no application bridge or horizontal page overflow and confirm scripts remain disabled. iPhone source/preview switching was exercised through the native button.
- iPad also passes light appearance, maximum accessibility text size and native share presentation/toolbar-anchor checks. The image check scrolls lazy images into view; share is checked after loading rather than against a fixed startup delay. The share action is invoked by the fixture because the simulator window offset prevents reliable automated toolbar taps. No share destination is selected.
- Physical devices, VoiceOver, a full React Native app build and end-to-end conversation-to-bridge navigation were not exercised. Android/browser document viewers are outside this change. These checks do not imply an installed release update.
