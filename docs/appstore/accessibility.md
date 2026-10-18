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

## Required checkpoint for every Store submission

Run from `mobileapp/`, using the current process's existing ASC credentials. The helper
only reads ASC; neither it nor public PR CI publishes declarations or submits an app.

After validating the candidate package, create a **new** private review:

```sh
bundle exec ruby scripts/asc-accessibility.rb init \
  --version X.Y.Z --build N --package-receipt build/store-package.json
```

The default `build/store-accessibility-review.json` is owner-only and Git-ignored.
`init` refuses to overwrite it; use `--review /private/path/new-review.json` for a new
assessment. It reads the live version/build and PUBLISHED declarations, including all
pages. Draft answers do not count. ASC keeps READY states on historical releases;
the helper chooses the numerically latest released iOS version, excluding unreleased
candidates. A first release or ambiguous availability needs manual review.

Fill the generated file; blank fields and `supported: null` deliberately fail:

- `reviewed_by`, `reviewed_at` (ISO 8601), `change_review`: identify the reviewer and
  assess changes since the live Store version, including new common tasks, navigation,
  themes/colors, text/scaling, motion, replies, SDK/RN/native libraries and extensions.
  Review every submission; an unchanged capability still needs a reviewed conclusion.
- `url_review`: confirm the optional URL remains empty, or verify the existing page
  opens and accurately describes the live app. New support must not be advertised early.
- `families.IPHONE` and `families.IPAD`: record the exact source/artifact and OS/device
  in `environment`, common-task evidence references, and honest `limitations`. Include
  first launch/pairing, radar, chat/terminal, replies/choices, HQ, servers/settings,
  errors/sheets and notification/Live Activity entry points. Record which evidence is
  source review, simulator capture or physical acceptance; untested is not passed.
- Each of the nine `capabilities`: choose a boolean `supported` and give `evidence`.
  For false, explain why support is not claimed (for example, not evaluated). True
  requires evidence for Apple's whole-common-task standard, not just one control.

`status` checks the exact package-receipt hash, version/build and current live baseline:

```sh
bundle exec ruby scripts/asc-accessibility.rb status \
  --version X.Y.Z --build N --package-receipt build/store-package.json
```

The same check is **required** by `asc-preparation.rb status|assets`, using the default
review or `--accessibility-review FILE`. Missing evidence, stale package/version/build,
changed live baseline or a regression in a published claim prevents readiness. `assets`
checks it before placement writes. Resolve regressions, or correct demonstrably inaccurate
live claims in ASC and create a fresh review; a promise to fix later is insufficient.
Network failure also prevents a current ready result. Scripts validate identity and
completeness, not the truth of a person's evaluation or physical VoiceOver behavior.

## Publish and close the loop after release

Existing accurate live claims can stay published. If the candidate adds a claim, fill
`after_release.owner` and `after_release.plan`; preparation reports the pending claims
without publishing them. Keep this follow-up with the private release handoff, even
after the operator submits. An approved/uploaded candidate is not a live version.

Once the **exact candidate version/build is live**, publish only verified capabilities
in ASC → App Accessibility, separately for iPhone and iPad. Do not change other families,
privacy answers or the optional URL as a side effect. Then run:

```sh
bundle exec ruby scripts/asc-accessibility.rb live-status \
  --version X.Y.Z --build N --package-receipt build/store-package.json
```

Use `--review FILE` for a non-default review. It compares the live build and both
PUBLISHED capability sets with the reviewed decisions. Missing/mismatched declarations
remain incomplete. Retain the JSON output with the review, package receipt and evidence;
then preview the Store's accessibility section on both device families. ASC readback
and customer-visible rendering are separate results; Apple allows up to 24 hours for
label changes to appear. Never call API readback a physical accessibility acceptance.
