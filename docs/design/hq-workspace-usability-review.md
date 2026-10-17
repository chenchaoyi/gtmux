# HQ workspace usability review · 2026-10-17

Scope: the native HQ Overview, conversation and matter detail, linked conversation readers/management, activity, knowledge, archives, machine and usage. Evidence is the user’s reported behavior, source paths and isolated native fixtures. No live HQ instructions or personal records were changed.

| Priority | Confirmed problem | Change and verification |
| --- | --- | --- |
| P1 | A cached conversation has no first-entry positioning; selecting the same destination returns early. | Initial bottom anchor and an explicit reveal signal. A native scroll-view test opens cached history, reads older content through a refresh, and reselects to return to latest; drafts and records remain. |
| P2 | Activity separates sources, hides summaries in small disclosures and lacks a readable time structure. | One local-time rail, short inline details, bounded per-record inspection and adjacent explicit matter groups. Projection tests cover chronology, filtering, day boundaries, ties, duplicate legacy IDs and missing titles. |
| P2 | Export uses a smaller font than Import; archive forms and linked cards still use compact-era fonts. | Shared workspace typography and matching archive controls. English/Chinese light/dark native render fixtures and existing export height-stability tests cover these components. |
| P2 | Composer exposes internal association language; knowledge actions use abstract promotion/take-away wording. | Plain destination labels and audience/apply/destination actions, preserving the same ledger commands, reasons, confirmation and scope. Existing command/lifecycle tests remain the behavioral gate. |
| P2 | Knowledge uses tiny text glyphs for disclosure; archive information requires hover. | Standard symbol disclosures with expansion values and a clickable shared information popover. Existing sidebar state tests and native fixtures supplement source review; actual VoiceOver speech is not yet accepted. |
| P2 | Resource wording implies that no work is blocked, or that detached processes are necessarily safe to reclaim. | State the resource warning and absence of an active-agent association without those claims. Resource data and actions are unchanged. |
| P2 | A long export failure expands the sheet beyond its parent; password fields lack explicit accessible labels. | Bounded scrolling error text and labelled native fields. A 300-line synthetic failure fits a smaller workspace. |
| P2 | Export rendering tests read the operator’s Keychain. | Inject the password reader; fixtures pass a nil reader. Production still uses the same Keychain lookup. No credential is captured in previews. |
| P2 | Four pending knowledge actions squeeze English labels into broken words in a narrow reader; audience metadata is repeated. | A responsive action layout keeps labels intact and removes duplicate audience text. Native narrow/wide layout tests and bilingual pending-entry fixtures cover this case. |

## Verification boundaries

Native fixtures exercise loading, retained refresh, empty/error content, history, source-reader bounds, narrow/wide windows and both appearances. Synthetic PNGs are retained by the macOS CI job for seven days. Full-window cache captures can omit the system-composited sidebar, so they are not evidence that its real pixels or VoiceOver speech passed; standalone resource/form captures and sizing tests supplement them. Installed-app interaction and real VoiceOver remain separate acceptance work. Phone, iPad, Web and terminal behavior are outside this native UI change; no new release or installation is included.

The history regression navigates by stable message ID through SwiftUI, verifies that the viewport has actually left the tail, then compares nonblank before/after viewport pixels. Direct NSClipView origin changes and raw document offsets are not evidence of a user scroll in a virtualized LazyVStack.
