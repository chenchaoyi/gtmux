# Native Mac usage widget

## Why
The shared usage UI currently compiles on macOS but the shipped menu-bar bundle has no WidgetKit extension. Users cannot add a native gtmux widget on their Mac.

## What changes
- Embed a separately compiled, sandboxed WidgetKit extension in the existing Developer ID app.
- Reuse the display snapshot, provider selection and small/medium views. The parent reads the existing bounded CLI usage report; the extension receives no credentials, host URLs, network access or terminal access.
- Use Apple's supported team-prefixed Mac App Group, derived from the actual signer, independently of the iOS group. Preserve the nonsandboxed parent's existing terminal behavior.
- Build a thin Xcode host around the existing SwiftPM UI library so both host and extension contain the same App Intents configuration and extracted metadata. Keep the SwiftPM executable launcher for development.
- Add build/signature/embedding checks and system registration acceptance. A tap opens the existing usage page with a strictly scoped URL route.

## Surfaces
- terminal: same usage contract; no new command required.
- menubar: publishes a bounded display snapshot while a native widget is configured and handles its usage shortcut.
- phone: existing widget behavior and separate iOS group remain unchanged.
- iPad: same existing iOS widget implementation remains unchanged.
- Web: no widget target; existing usage page unchanged.

## Validation
Test snapshot/privacy and refresh/reload limits, real small/medium rendering, provider selection, bundle contents, universal architectures, sandbox/entitlement boundaries, nested signatures, notarization, registration and click routing. Report actual OS delivery separately from unsigned compilation. Do not replace the installed release or register a test bundle under its identity before controlled verification is ready.
