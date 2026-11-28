# Design: Client Preview URI Parsing, Polling Cadence, and Surface Hardening

## Decisions

1. **RFC 8089 `file://` and `#fragment` parsing without `new URL(...)`**:
   - React Native's `Libraries/Blob/URL.js` polyfill only parses `http(s)` URLs and returns `pathname = "/"` and `hostname = ""` for `file:` URLs.
   - `parseFileURI(raw)` in `references.ts` (and the self-contained mirror in `toolPresentation.ts` so `gen-trace-web.cjs`'s module map remains intact) validates `/^file:\/\//i`, strips any `#fragment` first, rejects `?query` and non-empty authorities other than `localhost`, percent-decodes the slash-prefixed path, and rejects UNC `//` paths and control characters.
   - Non-`file:` local path references in `documentReference` also strip `#fragment` (and reject `?query`) before extension/prefix checks while keeping `gtmux-file:<32hex>` exact. `DocumentPreviewController.openMarkdownLink` strips `#fragment` on local links so child previews display clean filenames.

2. **Mobile polling cadences, screen-focus gating, and SSE lifecycle**:
   - `useScreenFocused()` reads `NavigationContext` safely, subscribing to `focus`/`blur` when present and defaulting to `true` outside a navigator or under partial test mocks.
   - `HQConsoleView` polls `usage`, `hqBoard`, and `hqKnowledge` at `30_000ms`, polls `digest` and `hqEvents` at `6_000ms` only when `!workspaceMode`, and pauses `HQWorkspaceView`'s `10_000ms` workspace poll once initial state is loaded while viewing `console` (refreshing immediately on return to `overview`).
   - `AgentsProvider` closes `unsub` immediately inside `onError` before scheduling exponential backoff, halts SSE retries after `>= 2` consecutive `401`/`403` responses until foreground/credential change, tears down SSE only on `AppState === 'background'` (keeping `inactive` open), removes the duplicate `AppState` listener, memoizes context `value`, and throttles `share()` re-reads on `lastUpdated` for confirmed owners while keeping per-refresh checks for guests.

3. **`ChatView` render isolation, `Composer` session-binding continuity, and `useSendRecovery` protection**:
   - `LiveProgress` owns the 1s `serverNowSec()` clock when `working` and no static `label` override is given, leaving `ChatView`, `MarkdownView`, and `ToolSteps` idle during elapsed ticks. `MarkdownView` and `ToolSteps` memoize their inner render bodies while preserving outer function identity for test tree queries.
   - `<Composer>` keys on `${client.base}|${paneId}` instead of mutable `session_id`. When `draftKey` transitions from `(base, pane, "")` to `(base, pane, session_id)`, `Composer` retains in-memory `text` and `attachments` and migrates any stored draft; switching between distinct non-empty sessions or servers flushes dirty drafts and isolates the new session.
   - Control-key payloads (`key` without `text`) never clear or overwrite a held failed text prompt in `useSendRecovery`.

4. **Web `document.hidden` gating and `ETag` transcript caching**:
   - `isDocHidden()` guards `typeof document !== 'undefined' && !!document.hidden` so Node VM tests remain unaffected. `tilePollPane` checks `t.removed || !WB.on || isDocHidden()`, and `pollWB`, `pollRadar`, `pollPane`, `pollChat`, `tilePollChat`, and `pollOptions` skip background ticks while `visibilitychange` triggers immediate active-view refresh.
   - `pollChat` and `tilePollChat` store `{scope, etag}` per focus pane / tile, send `If-None-Match` when matching the current `chatScope` and no reload is forced, and short-circuit on `304` before `r.json()` and `JSON.stringify`.

5. **macOS off-main execution, sleep/visibility gating, and native iOS/Android parity**:
   - `NotificationManager` dispatches `options(pane)` (`capturePoll`, 3s) and PNG badge rendering onto a background queue before registering categories and submitting `UNNotificationRequest` on the main actor. `Preferences.refreshTabAlert` and `PaneBrowserStore.refresh` (`AgentRefreshGate` + `capturePoll`) run off-main.
   - `AppDelegate` pauses `awakeTimer` and `tabOrderTimer` across workspace/display sleep notifications and gates `RemoteAccess.shared.refreshClients()` / `ShareStore.shared.refresh()` on `popover.isShown` (plus popover open). `SurfaceVisibility` replaces shared `MenuVisibility` collisions via SwiftUI environment injection while keeping `MenuVisibility.shared` compatible.
   - iOS `HQMemory` matches Android's URL/token validation, no-redirect ephemeral `URLSession`, 128 MiB streaming cap, gzip stream validation (`>= 1024` expanded bytes), generation counter, and `.completeUntilFirstUserAuthentication`. `NotificationService` retains `bestAttemptContent` and writes UUID temp files atomically; `LiveActivityModule` serializes `lastToken`/`tokenTask` and awaits ending stale activities; Android `DocumentActivity` retains `ParcelFileDescriptor`, detaches the adapter on destroy, and renders PDF pages asynchronously in `ARGB_8888` with background exception guards.
