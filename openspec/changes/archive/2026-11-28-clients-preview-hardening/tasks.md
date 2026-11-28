## 1. Implementation

- [x] 1.1 Implement RFC 8089 `parseFileURI` and `#fragment` stripping in `references.ts`, `toolPresentation.ts`, and `DocumentPreviewController.swift`; regenerate Web `documents.js` and `trace-model.js`; add Mobile and Web unit tests.
- [x] 1.2 Decouple `HQScreen` / `HQWorkspaceView` polling cadences, add `useScreenFocused()` + `startReadPolling` gating across `HQScreen`, `DetailScreen`, and `RadarPanel`, and harden `AgentsContext` SSE teardown, auth retry cap, `AppState` handling, and memoization.
- [x] 1.3 Isolate `ChatView`'s 1s `nowSec` clock inside `LiveProgress`, memoize `MarkdownView` / `ToolSteps` / `readingColors`, gate/cache `paneLines`, preserve `<Composer>` drafts/attachments across empty-to-bound `session_id` transitions, and protect `useSendRecovery` text failures from control keys.
- [x] 1.4 Gate Web `app.js` pollers on `WB.on` and `!document.hidden` with `visibilitychange` refresh and per-pane/per-tile `ETag` `304` transcript caching, with Node VM tests.
- [x] 1.5 Harden macOS `NotificationManager`, `Preferences`, `PaneBrowser`, `AppDelegate`, and `SurfaceVisibility`, plus iOS `HQMemory`, `NotificationService`, `LiveActivityModule`, and Android `DocumentActivity`, with Swift unit tests.
- [x] 1.6 Stamp mobile version `1.0.208` with bilingual Apple and Android release notes, map test selectors in `docs/test/implementations.json`, refresh test catalogue, and run all verification gates.
