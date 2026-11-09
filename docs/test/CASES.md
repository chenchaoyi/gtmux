# Case catalogue by layer

[中文](CASES.zh.md) · [Test system](TEST-PLAN.md) · [Complete CSV](CASES.csv)

The catalogue covers 1050 test/harness source files. Each CSV row lists stable ID, layer, stack, source/line, original description, dynamic flag and design origin. Layer links contain the complete lists for filtering/spreadsheets; names remain verbatim for source lookup.

| Layer | Static index entries | Complete case list |
| --- | --- | --- |
| L1 · Unit / pure logic | 3749 | [L1.csv](cases/L1.csv) |
| L2 · Component / contract | 1930 | [L2.csv](cases/L2.csv) |
| L3 · Runtime integration | 89 | [L3.csv](cases/L3.csv) |
| L4 · Application journeys | 94 | [L4.csv](cases/L4.csv) |
| M · Manual / device | 12 | [M.csv](cases/M.csv) |
| G · Qualification guards | 496 | [G.csv](cases/G.csv) |

Numbers are not passes. The legacy 6,223 positions came from the 4fad6119 inventory (declarations, subtests, dynamic templates); explicit new cases, native oracles and reviewed selectors are indexed separately. Dynamic instances are not expanded. Harnesses without named cases retain their hashes. Legacy layers are structural triage; mixed boundaries need semantic review when touched, not fictional historical review/results. A reviewed-selector is not a framework-discovered declaration. Gates catch source/index/mapping drift; names cannot prove assertion quality.

Refresh with `python3 scripts/test_design.py --refresh`; only reviewed/mapped changes are accepted. See [design workflow](DESIGN-WORKFLOW.md).

## Reviewed design cases

| ID | Expected behavior |
| --- | --- |
| ACP-001 | Fresh enabled Deny elements only; ignored first click can use one further explicit click in the same camera dialog, maximum two; no click starts after 10s; no unrelated permission acted upon; original command error preserved; no permission grant or settings mutation. |
| ACP-002 | New case survives a single lost OS denial touch; old frozen callback fails. Missing alert, grant=true, wrong/stuck system dialog still fail. Original App alert 30s remains; read-only QA CAMERA granted=false is required. Other 30 journeys and case orchestration unchanged. |
| ACP-003 | Both sizes retain complete suite and existing expectations; no denied permission granted, hidden blocked results or inflated timing budget. Failed driver evidence remains separate from product assertions. |
| CET-001 | Return the latest kind, time and explicit turn ID; tied conflicting metadata is unusable as exact completion proof. Legacy kind/time callers still work and scoped facts stay fresh. |
| CET-002 | Idle since actual boundary; active content/mtime and turn ID remain untouched; no fabricated completion, latest flag or done notification evidence. |
| CET-003 | Only verified same-session exact-ended-turn ready evidence overrides working. Real waiting/start/work survives, missing identity follows existing legacy behavior; no input authority or real marker mutation. |
| CET-004 | An explicit callback matching structured completion and current turn is ignored without clearing completion or minting events. Actual next start and unknown/missing legacy IDs are not fenced; approvals are preserved. |
| CET-005 | Continuation leaves marker start intact; actual new prompt/turn start and ownership change still refresh active time. |
| CET-006 | A real manual PostCompact receipt is recorded once, active stays absent and exact completion/current turn remain intact. Only actual rearm decisions are eligible for the completed-turn fence. |
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
| HRS-001 | Only first exact prompt after lower bound matches. First record remains stable across append; rewrite/replacement/continuation changes invalidate. Complete lines only; no result bodies retained. Cancelled/error reads do not grant attribution. |
| HRS-002 | Only accepted exact canonical HQ reply in correct Codex home/thread at its first source record gets from.kind=hq. Wrong identity, receipt state, body, missing clock/record or later human copy remains unattributed; existing sender preserved; no new sends or ledger writes. |
| HRS-003 | App/Web API turn JSON exposes identical HQ sender; new receipt invalidates prior human snapshot, unchanged snapshot is cacheable; old pages keep correct first-only attribution; policy/ledger/source unchanged by reads; CLI-origin and desktop-origin both work. |
| HQP1-F1-001 | The empty read leaves cursor 10 and persists the gap notice. The next read durably queues exact event-11 with its own type/body/conversation, then cursor 11. Repeated reads leave the database byte-for-byte unchanged. Old code must fail without source mutation or sleeps. |
| HQP1-F1-002 | Activation does not ingest history and writes an explicit events cursor even at zero. The new event at boundary+1 is queued with its actual identity, persists across reopen, and is never treated as historical on a second activation. |
| HQP1-F1-003 | The empty gap keeps cursor 10 and one stable gap record; retry changes no bytes. Actual retained event 16 is queued atomically with cursor 16, and neither gaps nor duplicate reads erase it. Source epoch-reset repair remains out of scope. |
| HQP1-F1-004 | First pass advances to 510, creates no prohibited or unregistered observations and does not claim sequence 511. Next pass queues event-511 and advances to 511; repeat has identical stored bytes. No policy broadening or hidden replay occurs. |
| HQP1-001 | Late/duplicate reports cannot reopen completed matters or erase the current question; observation saved exactly once. |
| HQP1-002 | State transitions require the current matter revision; wrong targets, stale and unversioned writes fail visibly; explicit current user transitions work. |
| HQP1-003 | Question resolution matches immutable question identity and current revision; old replies cannot remove newer questions; legacy exact-ID replies remain supported. |
| HQP1-004 | Durable queue survives Store reopen and event-read watermark; expired lease can be reclaimed and old generation cannot assess. |
| HQP1-005 | Assessment receipt and projection commit together; repeated command converges, changed payload conflicts; dry-run writes nothing and no instructions are replayed. |
| HQP1-006 | Source ingestion uses exact conversation/time attribution, records ambiguous/unassigned observations and source gaps; cursor advances only with durable inbox. |
| HQP1-007 | CLI operations enforce HQ authority and expose pending reconciliation; periodic ingestion/wake is independent from delivery and restart-safe. |
| HQP1-008 | Schema 1 reads migrate without invented history, schema 2 prevents old writer loss, and seeded bilingual playbook explains safe processing. |
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
| TCV-001 | Polls contain only bounded selected inputs and call metadata, never result bodies; owner reads recover recorded arguments/results, reject rewritten/foreign/oversized records and redact structured credentials. |
| TCV-002 | One call remains, original input/ref stays intact, result and status remain available by exact native ref; unrelated IDs are distinct. |
| TCV-003 | Queries expose named parameters and readable structured/text results; commands keep original source, files retain changes, unknown tools have factual fallback; pending/failed/empty/unrecorded are distinct, raw contains recorded sanitized fields and does not silently truncate. |
| TCV-004 | Tool-specific name, provider/status/duration, parameters/results are visible; no false summary-only warning, stale responses cannot cross sessions, original source remains verbatim, copy is complete and disabled while incomplete. |
| TCV-005 | Both surfaces show the same facts and nonempty recorded values; controls remain reachable, no horizontal document overflow or console errors, pending/error states are actionable and old-server fallback remains meaningful. |
| UR-001 | The 34-second HTTP-200-equivalent report is shown; the stalled read is classified as timeout, the next valid report is shown, and the late old report cannot overwrite it. |
| UR-002 | Valid empty reports remain data, 404 is unsupported, other failures throw typed causes; no failure becomes zero usage or a fabricated report. |
| UR-003 | Loading and empty are distinct; retry is available only where it can help; prior usage remains visibly labelled; technical details begin folded; actions and alerts have accessible roles/labels. |
