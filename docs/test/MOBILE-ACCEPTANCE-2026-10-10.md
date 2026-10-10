# Mobile acceptance — 2026-10-10

[中文](MOBILE-ACCEPTANCE-2026-10-10.zh.md)

Tested **v1.0.100 (1)** on the connected iPhone 15 Pro Max, iOS 26.6.2,
using Xcode 27 and Appium 3.7.0 / XCUITest 11.17.3 / WDA 15.1.4.
The installation was already complete; this run did not rebuild or reinstall gtmux.
Local evidence: `/private/tmp/gtmux-v100-device-acceptance/`.
Screenshots and XML contain private conversations and are intentionally not committed.

| Scope | Actual result | Evidence |
| --- | --- | --- |
| Screenshot tooling | `screenshotr` unavailable; Device Hub requires iOS 27; signed WDA worked | `screenshot.log`, `device-details.json`, `appium.log`; Device Hub displayed compatibility error |
| HQ long press | Both menu buttons visible, 45pt high, inside the 430×932pt screen, exposed as buttons | `hq-menu.png/.xml` |
| Usage shortcut | Opened from long-press menu; real content visible; close returned to HQ | `current.png/.xml`, `hq-after-usage.png/.xml` |
| Knowledge reader | Opened from HQ header; 759 entries/7 topics and entry rows visible; closed successfully | `knowledge.png/.xml` |
| Radar bottom | Last row ends at y≈773; hidden-count text at y804; no tall blank tail on this fleet | `radar-bottom.png/.xml`, `radar-end.png/.xml`; "1 session in collapsed sections" matches the collapsed errored group |
| Footer spacing | Cramped send-progress edge reproduced; user screenshots also show task/failure cards | `initial.png/.xml`; user-provided screenshots |
| Reader accessibility | Usage close reads `hq-usage-close`; knowledge close reads a glyph; search uses `knowledge-find` | `current.xml`, `knowledge.xml`; source fixes and regression tests added |

## Not accepted yet

- Compact Codex preview and its independent full reader: this run's opened live
  terminal had no recognized pinned instruction; no artificial prompt was sent.
- Knowledge reached directly from the **long-press shortcut**: the reader was
  verified via the HQ header, not this second navigation path.
- Follow switches and stable layout during toggle: no desktop work row was
  visible in this fleet; real authorization was not changed for testing.
- All-folded list state: the last source capture timed out as the phone switched
  to Daxiang (`com.meituan.message`); screenshot is not a gtmux acceptance result.
  Working/idle sections were collapsed during this attempt; restore them when
  resuming, without changing the user's originally collapsed errored section.
- Actual VoiceOver speech/focus/custom actions: requested manual verification;
  iOS 26 does not support the installed driver's iOS 27 VoiceOver commands.
- iPad, landscape, larger text, and Chinese visual acceptance were not run.

Device actions stopped after the foreground app changed. No background unlock
polling or further phone actions were performed. The footer/reader-label source
fixes have not been installed on this device, so their visual/VoiceOver acceptance
remains pending. The reusable procedure is [REAL-DEVICE.md](../../mobileapp/e2e/REAL-DEVICE.md).
