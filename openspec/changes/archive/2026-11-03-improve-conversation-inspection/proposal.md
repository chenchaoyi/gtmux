## Why

Readers need actionable formatted links and readable previews, stable bookmarks in long conversations, and a clear way to inspect commands inside code-mode input. The menu-bar Restore action can silently reject an inherited tmux environment; screenshot delivery repeatedly scans unrelated histories. Medium usage widgets duplicate headings and can clip their reading time.

## What changes

- Reuse nested Markdown rendering in App messages and browser overview previews.
- Add browser-local, credential/home/session-scoped bookmarks and stable history reading anchors.
- Share a bounded static command-input outline across Chat and Trace readers, preserving Full call and unchanged tool-call counts.
- Reuse the fresh radar pipeline for one pane in screenshot preflight and delivery guards.
- Accept explicit application restore invocations despite inherited TMUX, retain terminal nested guards, and display failure feedback and detached-session meaning.
- Compact shared Apple medium usage widget composition without changing snapshot or refresh semantics.

## Surfaces

- Terminal: agents --json --pane %N exposes fresh target status; restore retains its ordinary nested-terminal guard.
- Menu bar: screenshot checks use target scope; Restore displays progress/failure and opens detached sessions in additional terminal tabs; shared native widget layout improves.
- Phone: shared Markdown links and Chat/Trace code outline; Apple widget composition applies to iPhone, shared RN changes also apply to Android.
- iPad: same RN readers and shared widget views, no second parsing implementation.
- Web: Markdown preview, local bookmarks, stable earlier-history anchors, and generated shared command analysis for Chat/Trace. Native overview navigation has no equivalent map yet.

Android validation also captures bounded device diagnostics independently of WebDriver and fails with honest not-run accounting if that connection is lost. It never restarts shared ADB.

## Boundaries

No input is sent by previews/bookmarks. Bookmarks store opaque identities only within this browser credential namespace; authorization rejection clears local state. Static commands are submitted inputs, never proof of execution or success. No actual restore, service restart, signing, release, installation or private conversation export is required for implementation acceptance. Existing sessions and dirty workspaces remain untouched.
