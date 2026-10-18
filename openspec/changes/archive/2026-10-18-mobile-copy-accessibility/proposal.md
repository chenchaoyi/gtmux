# Focused mobile copy feedback and accessibility metadata

## Why

On iOS, long-pressing a conversation paragraph opens Copy without marking what it will copy. React Native 0.86's Fabric paragraph implements whole-block copying, not range selection; changing selectionColor cannot draw a selection band. Separately, the HQ card and approval choices expose automation identifiers as spoken labels, contradicting the existing mobile specification. The Store has no accessibility declarations yet.

## What Changes

- Preserve the current whole-block Copy scope and Markdown renderer. In conversation views only, a native iOS menu wrapper highlights the corresponding block while Copy is open and clears on dismissal, removal or changed text. Keep Android range selection and old-binary fallback.
- Give HQ and approval controls meaningful bilingual labels and button roles, retaining automation identifiers in testID.
- Document conservative Store accessibility evaluation against the live version. Publish only verified device-family capabilities; an optional Accessibility URL and full VoiceOver/Larger Text certification are not prerequisites for these fixes.

## Surfaces

- phone and iPad: shared conversation Copy feedback and spoken control labels; evaluate Store declarations separately per family.
- menubar: no rendering or control changes.
- terminal: no parser, session, selection or CLI behavior changes.
- Web: no rendering changes; browser text selection remains native.

## Validation

JS regression tests cover copy scopes, inline links, outer layout, unchanged non-chat rendering, accessible actions and delivery/test selectors. Isolated UIKit checks cover menu/highlight lifecycle, exact copy, repeat requests, stale callbacks, streaming updates and removal. Existing unsigned iOS CI compiles the full app and extensions. Source/Simulator inspection does not establish physical VoiceOver acceptance.
