# 分层用例目录

[English](CASES.md) · [测试体系](TEST-PLAN.zh.md) · [完整 CSV](CASES.csv)

目录覆盖 1060 份测试/辅助源码。CSV 每行列出稳定 ID、层次、技术栈、源码/行、原始测试描述、动态标记与设计来源。下表链接分别是各层完整列表，可筛选或导入表格；英文测试名保持源码原文，便于定位。

| 层次 | 静态目录条目 | 完整用例列表 |
| --- | --- | --- |
| L1 · 单元/纯逻辑 | 3754 | [L1.csv](cases/L1.csv) |
| L2 · 组件/契约 | 1932 | [L2.csv](cases/L2.csv) |
| L3 · 运行时集成 | 86 | [L3.csv](cases/L3.csv) |
| L4 · 应用操作流程 | 108 | [L4.csv](cases/L4.csv) |
| M · 人工/设备 | 12 | [M.csv](cases/M.csv) |
| G · 质量门禁 | 496 | [G.csv](cases/G.csv) |

数字不是通过数。存量 6,223 个位置来自 4fad6119 盘点（声明、子测试、动态模板），另加明确的新用例、原生过程入口和评审映射入口；动态实例不展开。没有具名用例的辅助文件仍保留摘要。存量层次是结构分类，混合边界在触及时需语义复核，不补造历史评审/运行结果。`reviewed-selector` 也不冒充框架发现的声明。源码/目录/映射漂移由门禁拦截，但测试名不能自动证明断言质量。

索引由 `python3 scripts/test_design.py --refresh` 同步，只接受已评审映射的变更。详见[设计工作流](DESIGN-WORKFLOW.zh.md)。

## 已评审设计用例

| ID | 预期行为（源码英文） |
| --- | --- |
| CORE-CODEX-FOREIGN-ACTIVE | Only a verified current Codex home/session/cwd, a named foreign active marker, a completed/aborted own rollout and a ready nonworking nonapproval frame can contradict false working. Own/empty/unknown/stale binding or actual start/work/wait remains conservative. Reads never remove a foreign marker or mint a completion. |
| CORE-CODEX-FOREIGN-GATHER | The real radar gather with isolated HOME, a completed own rollout, another session marker and recent window activity reports idle without changing marker content/mtime, binding or completion records; genuine own turns, approvals and legacy unknown ownership still work. |
| CT-HTTP | Real Go handler responses agree with independent RN/Web fake servers for transcript history/context, HQ overview/detail/cursor, document metadata/download, null/empty and permission errors. |
| CT-DRAFT | Actual Detail and HQ composers isolate text across servers and reused panes/session identities, ignore delayed old draft loads, and persist/send to the current target only. |
| CT-HQ | A current human question opens its original matter, prepares but does not auto-send a reply; an authoritative answer removes it from pending without completing the matter. Old client responses cannot replace current data. |
| CT-SUB | Mounted server mode consumers refresh after actual provider notifications and do not display late results from a retired client. |
| CT-ANDROID | The unchanged 31 Android journey assertions run as a full journey or independent capability blocks with idempotent setup and dependency-aware blocked results; driver disconnect preserves the first cause. |
| CT-ANDROID-POLICY | The unchanged 31 Android journey assertions run as a full journey or independent capability blocks with idempotent setup and dependency-aware blocked results; driver disconnect preserves the first cause. |
| CT-TIMING | Stage and operation timing attributes setup/action/evidence cost separately, including failures and blocks; no silent rerun or suppressed assertions. |
| CT-BROWSER-POLICY | Reuse one browser process per test worker while every case retains a fresh context/server and isolated storage/failure traces; closed browsers are replaced, resources close on setup/assertion failure. |
| CT-BROWSER | Reuse one browser process per test worker while every case retains a fresh context/server and isolated storage/failure traces; closed browsers are replaced, resources close on setup/assertion failure. |
| CT-SCOPE | Only proven non-product RN unit test edits may omit native builds; mixed/shared/unknown paths, test fixtures consumed by journeys and production imports fail closed; all selected checks still aggregate. The Duo candidate also selects the iOS native and Duo simulator jobs for iOS application changes while retaining conservative selection for mixed and unknown paths. |
| FOCUS-001 | Literal and decorated versions of the complete session match; differently quoted, truncated, unrelated, wrong-separator and empty identities do not. Existing ordinary/decorated titles and bare trailing-whitespace tolerance remain supported. |
| FOCUS-002 | A literal match wins across all tabs before tolerant decoration fallback. With only a decorated match it is selected. The chosen record, indices and trailing title bytes remain unchanged; no match and empty session return false. Multiple literal tabs retain first-match order. |
| MC-HQ-CHOICE | Choosing a current HQ option immediately prepares and focuses the same-matter draft without IPC or auto-send; edited text is preserved and stale questions have explicit feedback. |
| MC-HQ-WINDOW | The complete native HQ window keeps a long question header, original conversation and composer inside narrow/wide viewports; AX choice activation updates the real NSTextView and focus, independent of read timeout. |
| MC-HQ-READ | Stored matter/overview reads do not perform roster or historical discovery. Slow or failed optional source enrichment cannot block selection/preparing replies or replace newer matters/drafts. Legacy CLI fallback is bounded to explicit unsupported flags. |
| MC-HQ-OVERVIEW-STATE | Overview groups exclude completed/closed/merged matters even with stale projection IDs; a stage-result body never changes an open goal. Valid saved rows fill a bounded three-item preview and hidden counts match unique displayed records. |
| MC-HQ-OVERVIEW-MORE | At the bottom of each truncated native HQ section an explicit remaining/total control opens all matters and can select a hidden record without writes; no extra control when all records fit. Complete minimum/wide light/dark bilingual fixtures preserve text/viewport geometry. |
| MC-HQ-OVERVIEW-BRIEF | The native row presents canonical current assessment before historical receipt text, explicitly names the unfinished goal state and never treats unavailable source metadata as an idle/completed worker; no guessed human decision or completion from titles, Stop or report prose. |
| DUO-001 | cold notification opens the correct server and selection routes to the selected server. |
| DUO-002 | selected session and draft survive a simulator orientation change. |
| DUO-003 | iPad retains sidebar and selected session through rotation. |
| DUO-004 | cold and warm notification actions send the selected response exactly once. |
| DUO-005 | warm and cold scene links open the usage sheet. |
| DUO-006 | first server selection and server switching follow the same observable route on main and candidate. |
| DUO-007 | fake request diagnostics expose routing metadata without credentials or query values. |
| DUO-008 | explicit hosted simulator configuration preserves local defaults and bounds driver startup. |
| DUO-009 | Unacknowledged responses remain retrievable; acknowledged responses disappear, duplicate identity/action is queued once, and only the newest 128 entries remain. |
| DUO-010 | Initial fallback proceeds despite a stalled drain or permission request, foreground retry retrieves a retained response, and shared identity permits at most one reply. If the initial notification is absent and the first drain stalls, a later active transition delivers and acknowledges the retained response once. |
| DUO-011 | Trace and Diff remain open with their current content across window resizing. |
| DUO-012 | resizing preserves mounted detail and selection while compact navigation remains usable. |
| DUO-013 | usage modal resolves all safe edges in its native presentation. |
| DUO-014 | both adaptive shell modes compose the shared radar panel. |
| DUO-015 | inline workspace selection retains identity without pushing a second detail route. |
| DUO-016 | root layout updates available window size without remounting the reader. |
| DUO-017 | Duo acceptance verifies ad-hoc simulator signatures while native compilation remains unsigned and no production signing credentials are used. The isolated Duo job installs and verifies published driver 12.20.0 and WDA 16.17.1 for simulator hinge/display commands. |
| DUO-018 | Background handling starts before the event; the visual tree starts after it without remounting background handling. |
| DUO-019 | One listener stays active, uses current owner state, and guest or parameterized links remain rejected. Changing the client retains one listener and uses its latest client; a guest-rejected link cannot open after switching to owner. |
| DUO-020 | Measured hinge reaches open and closed positions, the application window changes, the selected conversation and the same visible reading prompt survive, and the typed draft remains after fold and unfold. |
| DUO-021 | Trace keeps its selected session and search text; Diff remains open and both can be exited after posture changes. |
| DUO-022 | The dictionary includes the browsing activity type under the supplied UIKit key and retains the original activity and URL; actual UIKit delivery remains a device test. |
| DUO-023 | The size setting reports accessibility-large and the selected session and tools are reachable on both panels; save actual audit findings without claiming a VoiceOver or physical-device pass. |
| DUO-024 | Scene URLs remain pending until acknowledged, the consumer opens an authorized usage link once, guest-rejected links do not become later owner intents, and foreground retry can recover the retained link. A stalled first drain cannot block active-state recovery, and a later duplicate is suppressed. |
| DUO-025 | The previously visible compact list returns after a round trip; an explicit expanded selection opens its reader, and a first wide automatic selection still shows its reader on folding. |
| DUO-026 | A guest- or demo-rejected initial intent is consumed once; no sheet opens or usage request occurs after the owner transition. |
| DUO-027 | The native visible-content anchor is active only while reading history; live-tail following stays active at the tail. Actual reflow is separately verified on the Duo simulator. |
| DUO-028 | Each queued intent is acknowledged once, a repeated availability event cannot reopen a consumed intent, guest rejection does not replay on owner, and unrelated URLs keep their normal routing. Native signal and actual scene delivery require separate checks. |
| DUO-029 | A fully visible turn remains at its prior viewport position across measured reflow; user gestures and active selection are not overridden and the tail continues following. Actual fold remains independently verified on Duo. |
| DUO-030 | Only one reply starts; receipt and native completion remain pending until settlement or deadline; default taps finish normally and an ambiguous send is never automatically replayed. |
| DUO-031 | Background work starts without a scene, no fake metrics are supplied, visual work opens with actual current measurements and updates when native insets change, and old native shells retain their normal provider behavior. Device cold-link startup remains separately verified. |
| DUO-032 | Permission progresses despite a stalled initial response, the initial action is still processed exactly once without waiting for permission, and late callbacks after teardown are ignored. |
| DUO-033 | The first visible prompt retains its viewport offset as parent and nested layout settle; a new drag stops correction, and promptless histories fall back to a visible turn. Real fold remains separately tested. |
| DUO-034 | Teardown remains pending until the original operation settles; later execution cannot race it, and rejected journeys release the guard without concealing their failure. |
| DUO-035 | Current revision/scale records exist for visible text, UIFont points are positive, measured height and final character rectangle fit internal bounds, and raw audits retain every finding; normal parent scrolling is not mistaken for internal clipping. The selected session and tools remain reachable on both panels at accessibility-large, preserving the entire previous DUO-023 journey. |
| DUO-036 | The control has an opaque fill and visible border without changing its floating bar, safe-top offset, minimum touch size, bounded label scale, native transform or expand/collapse semantics. Device text legibility is separately checked by real Duo screenshots. |
| DUO-037 | VoiceOver reports enabled, speaks the selected session title and Session tools within 48 focus moves and 90 seconds on each panel, and reports disabled after cleanup. Missing/unsupported behavior fails rather than counts as a pass; physical and subjective speech quality remain unverified. |
| DUO-038 | Every evidence failure still attempts VoiceOver disable and disabled-state verification plus hinge/display/fake restoration. Initial speech and late successful moves cannot pass outside the same panel budget; no detached driver command remains. On-budget target speech succeeds. |
| DUO-039 | A transient stale frame causes the whole observation to be discarded; fresh complete stable frames and layouts pass; permanent failures, absent identities, missing/clipped layouts and results outside the fixed budget fail. |
| DUO-040 | The same fixture returns identical messages and times across polls; a new fixture captures its own epoch; explicit world.transcripts is returned unchanged. |
| STOR-001 | Every carrier separates project-owned and task-temp output from gtmux configuration/runtime trees; no promoted entry required; user text and hand-edit protection preserved; second sync idempotent. |
| STOR-002 | Both playbooks reserve gtmux-owned state and only reusable HQ knowledge tools for its home, direct project scripts/reports/builds to project/task storage, require external evidence references, and never grant automatic migration/deletion; upgrade backs up old charter, preserves LOCAL/notes and converges. |
| TD-001 | Reject a changed/new test without a reviewed design; accept a mapped implementation |
| TD-002 | Reject a tampered review seal, missing assertion description, invalid layer or unsafe path |
| TD-003 | Reject missing/changed catalogue files and missing named implementation selectors |
| TD-004 | PR history requires the sealed design in an earlier commit than test edits; main validates the resulting snapshot after squash |
| TD-005 | Explicit removals and renames require reviewed rationale; legacy baseline does not imply historical review |
| ND-001 | Run existing Swift HTML oracle: local image/srcset/entity rewriting, deduplication, raw script/style/comment exclusion |
| ND-002 | Run existing Swift Markdown oracle: structures, unsafe links/images/scripts, scoped resources and font cap |
| ND-003 | Native runner compiles production Swift without -O and propagates compilation/oracle failures; CI calls it |
| DG-001 | Default Appium and RN discovery exclude manual diagnostics/capture; explicit tools retain failure reporting; knowledge capture does not perform promotion actions. |
| DG-002 | Send repro fails before device access without exact pane, owned-fixture declaration, loopback endpoint/token and a matching fake-server fixture identity; nonfixture endpoints are rejected. |
| DG-003 | Send repro selects the configured pane, never the first agent; absence/mismatch prevents sending |
| CP-001 | Five agent Trace fixtures render matching semantic counts, original ordinals and tool/context/reply labels in RN and browser DOM |
| CP-002 | HQ conversation provenance and full request remain readable in RN and browser Chat; opening HQ profile preserves sender identity. Workspace-only cards are not claimed equivalent to browser Chat. |
| CP-003 | Markdown and code in document links and tool previews preserve content; local file actions use the paired preview adapter, not OS file or javascript navigation. |
| CP-004 | Browser consumes the same synthetic expectation fixture as RN; granting no access does not expose content and no test sends real input. |
| TD-006 | Known test documentation is policy-only; mixed Go test changes still select complete Go; unknown metadata still selects all. |
| DG-004 | Rejected verification never calls global device setup; successful verification calls setup exactly once afterwards. Existing DG-002 regressions cover the wrapper policy. |
