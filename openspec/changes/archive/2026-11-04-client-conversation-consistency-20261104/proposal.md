## Why

Browser conversation readers still diverge from the App: turn headers repeat the
agent name, Live uses ordinary page chrome, code actions have inconsistent button
styling, and horizontal indicators cover execution paths. Chat omits the existing
earlier-history affordance. Readable external sessions also bypass the desktop
workbench. The App's HQ overview does not make withheld cards obvious, Diff actions
use an ambiguous glyph, and screenshot delivery retains an unnecessary success delay.

## What changes

- Align Trace turn labels, code actions and expanded Live with the shared App vocabulary.
- Reuse existing bounded transcript cursors and shared history merge logic in browser Chat.
- Let readable external sessions use concurrent, resizable desktop tiles while retaining
  their read-only capabilities, canonical home/session identity and narrow-screen navigation.
- Keep horizontal indicators clear of text, especially profile execution paths.
- Show visible/total and remaining HQ matters with an explicit expansion control.
- Use a file with addition/removal marks for every Diff entry point.
- Close a screenshot editor promptly after a verified successful receipt and keep the
  captured image visible beneath the sending overlay, honoring reduced transparency.

## Surfaces

- terminal (including attach): unchanged protocol and input permissions; no new commands.
- Menu bar (menubar): screenshot sending feedback; no new menu-bar Diff capability.
- phone: shared HQ disclosure and Diff glyph; keep existing conversation semantics.
- iPad: the same HQ component in the regular layout, with a discoverable remaining count.
- Web: Trace, Live, Chat history, code controls, execution paths and external-session tiles.

## Validation and delivery

Use synthetic browser and native fixtures, desktop/narrow and light/dark views,
authorization and stale-response cases, scoped history loading, scroll anchors,
HQ expansion and screenshot success/error lifecycles. Run normal local, candidate
and actual-main gates. Preserve the already published 170; release the new complete
batch under the next verified free version with changed mobile packages. Record
preparation, CI/queue, build/sign/notary, transfer, installation and cleanup separately
and compare only equivalent phases with retained 169/170 evidence.

No permission, pairing, audience, Store-submission or old cleanup boundary changes.
