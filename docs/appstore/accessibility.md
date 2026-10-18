# App Store accessibility declarations

[中文](accessibility.zh.md)

The **Accessibility URL** is an optional link to a public page explaining accessible
features and limitations. It is not an API endpoint, permission or compliance
certificate. Leave it empty until a useful, maintained page exists; labels do not
require one. See [Apple's management instructions](https://developer.apple.com/help/app-store-connect/manage-app-accessibility/manage-accessibility-nutrition-labels/).

## Keep the scope small

Start with existing Dark Interface and Differentiate Without Color Alone support.
These use the existing themes, status shapes/text, selected checkmarks and switch
positions. Do not redesign the UI or change the status palette just to add labels.
HQ and approval buttons use readable action names; test identifiers belong only in
`testID`. Correct labels improve individual controls but do not prove VoiceOver
support throughout the app.

In iOS conversations, long-press selects a **whole block** for Copy and highlights
that block until the menu closes. Copy reply still copies the full response.
This is not drag-to-select a word or selection across paragraphs. The existing
Markdown text and links remain intact; the feedback wrapper is conversation-only.
Android keeps its existing range selection. The native code requires a new app
build; a JS bundle running against an older binary falls back to its original Copy
behavior. `scripts/test-copy-text.sh <disposable-booted-simulator-UUID>` runs small
isolated UIKit regression checks without building the React Native app.

## Evaluate before publishing

1. Read ASC's **live Store version**, not the installed Ad Hoc build or latest tag.
2. Evaluate iPhone and iPad separately across first launch/pairing, radar, conversation
   and terminal, replying/choices, HQ, server management and settings. Include errors,
   sheets and selected/disabled states. Use synthetic sessions and credentials.
3. Dark Interface: verify the selected dark appearance covers common tasks. Non-color
   cues: inspect grayscale screenshots and confirm status, connection, selection,
   warnings and controls remain distinguishable by text, shape or position.
4. Record exact source/version, device family, test environment, scenarios, screenshots
   and gaps. Physical layout, VoiceOver focus/speech, push and hardware interactions
   require their own acceptance; code/tests or simulator captures cannot establish them.
5. In ASC → App Accessibility, save/publish only capabilities whose common-task
   criteria were met. Read declarations back after publishing. Updating labels does
   not submit an app version; the operator still owns final app Submit.

[Apple's common-task standard](https://developer.apple.com/help/app-store-connect/manage-app-accessibility/overview-of-accessibility-nutrition-labels/)
applies to the **whole common workflow**, not a handful of controls. Do not declare
VoiceOver, Voice Control, Larger Text, Sufficient Contrast or Reduced Motion solely
because labels, a font-size setting or one motion-aware component exist. In particular,
the app's three terminal font sizes do not establish the required 200% text support.

Current evaluation results and ASC write receipts belong in the task's private evidence;
this document is the repeatable process, not a claim that a future version is certified.
