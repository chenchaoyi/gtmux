# 分层用例目录

[English](CASES.md) · [测试体系](TEST-PLAN.zh.md) · [完整 CSV](CASES.csv)

目录覆盖 1032 份测试/辅助源码。CSV 每行列出稳定 ID、层次、技术栈、源码/行、原始测试描述、动态标记与设计来源。下表链接分别是各层完整列表，可筛选或导入表格；英文测试名保持源码原文，便于定位。

| 层次 | 静态目录条目 | 完整用例列表 |
| --- | --- | --- |
| L1 · 单元/纯逻辑 | 3726 | [L1.csv](cases/L1.csv) |
| L2 · 组件/契约 | 1905 | [L2.csv](cases/L2.csv) |
| L3 · 运行时集成 | 86 | [L3.csv](cases/L3.csv) |
| L4 · 应用操作流程 | 93 | [L4.csv](cases/L4.csv) |
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
| CT-SCOPE | Only proven non-product RN unit test edits may omit native builds; mixed/shared/unknown paths, test fixtures consumed by journeys and production imports fail closed; all selected checks still aggregate. |
| FOCUS-001 | Literal and decorated versions of the complete session match; differently quoted, truncated, unrelated, wrong-separator and empty identities do not. Existing ordinary/decorated titles and bare trailing-whitespace tolerance remain supported. |
| FOCUS-002 | A literal match wins across all tabs before tolerant decoration fallback. With only a decorated match it is selected. The chosen record, indices and trailing title bytes remain unchanged; no match and empty session return false. Multiple literal tabs retain first-match order. |
| MC-HQ-CHOICE | Choosing a current HQ option immediately prepares and focuses the same-matter draft without IPC or auto-send; edited text is preserved and stale questions have explicit feedback. |
| MC-HQ-WINDOW | The complete native HQ window keeps a long question header, original conversation and composer inside narrow/wide viewports; AX choice activation updates the real NSTextView and focus, independent of read timeout. |
| MC-HQ-READ | Stored matter/overview reads do not perform roster or historical discovery. Slow or failed optional source enrichment cannot block selection/preparing replies or replace newer matters/drafts. Legacy CLI fallback is bounded to explicit unsupported flags. |
| MC-HQ-OVERVIEW-STATE | Overview groups exclude completed/closed/merged matters even with stale projection IDs; a stage-result body never changes an open goal. Valid saved rows fill a bounded three-item preview and hidden counts match unique displayed records. |
| MC-HQ-OVERVIEW-MORE | At the bottom of each truncated native HQ section an explicit remaining/total control opens all matters and can select a hidden record without writes; no extra control when all records fit. Complete minimum/wide light/dark bilingual fixtures preserve text/viewport geometry. |
| MC-HQ-OVERVIEW-BRIEF | The native row presents canonical current assessment before historical receipt text, explicitly names the unfinished goal state and never treats unavailable source metadata as an idle/completed worker; no guessed human decision or completion from titles, Stop or report prose. |
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
| UR-001 | The 34-second HTTP-200-equivalent report is shown; the stalled read is classified as timeout, the next valid report is shown, and the late old report cannot overwrite it. |
| UR-002 | Valid empty reports remain data, 404 is unsupported, other failures throw typed causes; no failure becomes zero usage or a fabricated report. |
| UR-003 | Loading and empty are distinct; retry is available only where it can help; prior usage remains visibly labelled; technical details begin folded; actions and alerts have accessible roles/labels. |
