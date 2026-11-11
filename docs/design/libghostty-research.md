# libghostty terminal replacement research

[中文](libghostty-research.zh.md) · Evaluated: 2026-11-10 · Recorded: 2026-11-11

## 1. Decision and scope

**Do not replace gtmux's terminal surfaces wholesale in this round.** Keep xterm on the Web and the existing mobile native implementation. Some synthetic workloads used less CPU, but the tested Web candidate crashed, changed copied text and failed the scroll-position contract. It did not meet the requirement to preserve every existing capability and improve the experience. This decision applies to the pinned versions below, not every future libghostty release.

The official libghostty-vt 1.0 native and WASM APIs worked in minimal integrations and remain candidates for a terminal state engine, not a proven replacement for a complete UI. Independent review accepted the research evidence and no-replacement conclusion; that was not merge, installation or deployment approval.

“Native macOS terminal” means drawing a terminal directly inside the App using macOS UI technology such as AppKit, rather than displaying a browser page. The baseline Mac App opens a browser for the relevant workspace. It also has native conversation reading, but no equivalent embedded terminal surface to swap directly. A native approach would create a terminal view and still need font rendering, IME, selection, copying, scrolling and accessibility.

|Surface|Decision for this round|
|---|---|
|Terminal / remote attach|Keep the existing path. No host-terminal replacement or real PTY attach validation. The Go CLI/core stays cgo-free.|
|Menu bar / macOS App|Official C core integration is feasible; the custom AppKit PoC is not a complete product view, and there is no equivalent native A/B benefit claim.|
|Phone|Keep RN/native. No iOS/Android device or WebView replacement acceptance.|
|iPad|Keep the current implementation. Split layout, hardware keyboard, selection and lifecycle remain unverified.|
|Web|Reject the tested third-party adapter for adoption; retain xterm as the current backend and comparison.|

## 2. Pinned versions and product facts

Observed remote main and release `v1.0.174` both resolved to baseline **B `094f3d0fcccba3066f37f829847330ea4686f185`**. The initial older tree `aa43ffdf` was not used for product conclusions. The design was sealed at **D `15e2c4886ed1ae28236edcc40a8b84f3ce223c0e`**, before experiment source **Q `2b0de8774f96c36415962483ae8b05317b5fe454`**. Later documentation commits do not change Q. A future evaluation must establish a new authoritative baseline instead of treating B as permanently current.

|Object|Version verified then|Boundary|
|---|---|---|
|Official libghostty-vt|`vt-1.0.0`, tagger 2026-11-06 22:55:16 UTC; commit `0fa3d577a5614150ff8e2c152de5fb9d5c6d2dec`|MIT C ABI for parsing, state, formatting, input encoding, selection, search and render state. No complete Metal, AppKit or browser renderer. Official targets include Linux/macOS/iOS/Android/FreeBSD/Windows/WASM; build support does not supply a finished platform UI.|
|Ghostty App|1.4.0, 2026-10-27|Complete macOS/Linux terminal App with its own release cycle. Its GPU-rendered experience is not a capability supplied by the VT library alone.|
|Third-party `coder/ghostty-web`|npm stable 0.4.0, 2025-12-09; embeds Ghostty `5714ed07a1012573261b7b7e3ed2add9c1504496` plus patches|The tested Web integration: Canvas2D + JS + older WASM, **not official VT 1.0**. Official 1.0 lacks its legacy `ghostty_terminal_write` export; swapping the WASM binary is not an upgrade path.|

Primary sources: [official release](https://github.com/ghostty-org/ghostty/releases/tag/vt-1.0.0), [pinned API](https://github.com/ghostty-org/ghostty/blob/0fa3d577a5614150ff8e2c152de5fb9d5c6d2dec/include/ghostty/vt.h), [official platforms/install](https://lib.ghostty.org/vt/installing.html), [license](https://github.com/ghostty-org/ghostty/blob/vt-1.0.0/LICENSE), [App 1.4](https://ghostty.org/docs/install/release-notes/1-4-0), [third-party v0.4.0](https://github.com/coder/ghostty-web/releases/tag/v0.4.0). These record what was verified in this round; recheck versions when reopening the decision.

Adoption is not compatibility evidence: the [JetBrains announcement](https://blog.jetbrains.com/platform/2026/10/jetbrains-terminal-now-uses-ghostty/) concerned **2026.3 EAP**, with rendering and higher-level interaction retained by the IDE. Observed [coder/xum](https://github.com/coder/xum/blob/main/package.json) 0.32.1 used `ghostty-web 0.4.0-next.7.g03ead6e` alongside xterm headless/serialize. [cmux](https://github.com/manaflow-ai/cmux/blob/main/.gitmodules) used the `manaflow-ai/ghostty` fork, with observed submodule `ef4f1570d62117ebc51e3b0f07ad4f34032267b6`. Those two main links move; captured snapshots are referenced by the full report. None proves official VT 1.0 supplies every UI capability gtmux requires.

## 3. Data path and historical lessons

```text
tmux (VT already interpreted) -> capture-pane -e -> authorized Go API -> B append/reset -> xterm
                                                                     -> isolated ghostty-web comparison
Composer drafts/permissions/identity -> /api/send -> delivery confirmation and receipt (not renderer-owned)
Official VT 1.0 C/WASM -> terminal state -> platform rendering and interaction implemented by the host
```

Entry points at B are `internal/tmux/tmux.go:CapturePaneColor` and `internal/server/web/app.js:ensureTerm/writePane`. Web uses xterm 5.5, fit and Unicode11, scrollback 5000 for the main view and 4000 for tiles, with stdin disabled. Prefix matches append; other snapshots reset/write. Scrolling depends on `baseY - viewportY`, with an approximately 900ms gesture freeze. **This is not a raw PTY stream; parser throughput cannot stand in for product-path performance.** Chat, Trace, Diff, attachments, waiting/permissions, exact session identity, drafts and receipts are higher-level capabilities that a renderer migration must preserve.

|Historical evidence|Lesson to retain|
|---|---|
|`24aa887e` (#126, 2026-06-25) and `bdf942ea` (#131)|Introduced xterm 5.5 in a WebView, then tried WebGL for scrolling.|
|`6f369f36b06b927ee1b8f30cb28766226d34d5ec` (#145, 2026-06-26)|Device WebGL black screens caused a Canvas fallback. Simulator success did not establish real GPU stability.|
|`10b16656859d7da4b1a0db08022ae0f4b6c7aec9` (#208, 2026-06-29)|Keyboard popups, black screens and canvas fragility motivated the mobile RN native replacement.|
|`365f30842fbd8c9a0f95608c04e0d7aad51a36c3` (#209, same day)|Native also had large nested-Text hitches/crashes, polling that erased selection, scroll freezing and history limits. Native does not automatically mean faster.|
|`284a6795` (#194), `4b34e6f7` (#345), `b405dbc5` (#346)|Web scroll preservation/reflow, native symbol/emoji fixes, and removal of the dead WebView respectively.|

The search at evaluation time found no earlier libghostty integration record. Ghostty **host App** focus/title, tmux tab and restore identity-chain failures are not evidence of a libghostty renderer defect. Older uncommitted worktrees were neither borrowed nor modified.

## 4. Results, gaps and capabilities that must survive

|Observation|Result and boundary|
|---|---|
|Minimal VT/ANSI/cursor/alt screen|Both Web backends and official C/WASM passed minimal probes, not a full protocol/TUI conformance suite.|
|Wide-character boundary|120 writes completed at 40 columns. A fresh single 80-column terminal, without clipping or preceding reset/selection, failed inside the third-party WASM write with an out-of-bounds access; xterm completed. Official 1.0 WASM completed 120 writes from the same generator at 80×24. The old third-party core/patch/bridge failure is not attributed to official 1.0.|
|Scroll and history|xterm retained line 40. Candidate compatibility getters returned zero for `baseY/viewportY`, and write returned to the bottom, breaking B's preservation contract.|
|Selection, copying, Unicode|xterm round-tripped Chinese/combining/ZWJ text. Candidate selection inserted CJK spaces and returned old ALT text after reset; source showed the selection manager was not rebound to the recreated wasmTerm. Text readback was also incomplete.|
|Resize/zoom/local input|Candidate standalone resize click timed out after reset/selection/scroll; the B narrow-page check passed. This is a state-sequence failure, not a claim that every resize fails. Read-only paste emitted no onData; OS keyboard/IME and full mouse protocols were not validated.|
|B page|Terminal tile/focus, Chat/Trace/Diff with draft preservation, snapshot replacement plus narrow viewport, and no fixture sends/unexpected exceptions: 4/4 per backend. Snapshot checks only required UPDATED to appear, not removal of all stale content or real network reconnection.|
|Native drawing|Official C API linked to Cocoa, custom AppKit PNG rendering, direct synthetic keyDown, resize and alt restore assertions passed. This was not Ghostty's Metal renderer and did not validate OS IME, system clipboard, complete selection or accessibility.|
|Product capabilities still unverified|Attachments/image preview, terminal search, hyperlink/OSC8/52 policies, real permissions and agent waiting, exact identity/delivery/receipts, system fullscreen, mobile devices and long-duration stability lacked complete acceptance. Unchanged source is not proof those paths passed regression checks.|

Final standalone results were xterm 8/8 and candidate 4/8; failures remain in the evidence. Further work must preserve exact session/home/process birth/TTY, draft protection, queued-versus-landed distinction, delivery receipts and screenshot scope/lifecycle. Do not connect terminal.onData directly to real sends around Composer, or weaken permissions and identity to improve a score.

## 5. Performance record and interpretation

M4 Pro / 14 CPU / 24GiB, macOS27.2, Chrome157.0.8103.39; DPR2, 1440×900, Hack14px, 40×16, 5000 history lines. All 1/4/8 terminals fit. The same B, workload and synthetic HTTP transport were used in two ABBA rounds: four trials per backend/workload/count, 96 valid trials. Ordinary workloads had 120 steps; history used eight 5000-line writes. This extract shows eight terminals, A=xterm and C=ghostty-web, in ms.

|Workload|parse A/C|draw A/C|CDP task A/C|Pre-write through callback and two rAF opportunities, p95 A/C|
|---|---:|---:|---:|---:|
|Scroll|34.90 / 28.00|168.15 / 490.50|845.11 / 818.93|15.20 / 25.40|
|History|214.80 / 219.50|14.30 / 34.40|267.86 / 300.40|85.00 / 51.10|
|Local refresh|21.25 / 28.25|181.10 / 543.40|1047.11 / 885.73|24.50 / 25.50|
|Wide text|20.55 / 18.35|464.30 / 494.25|1946.66 / 760.89|18.10 / 25.50|

The first three metrics are medians of four per-trial accumulated times. The last is the **pooled step-sample p95** across those trials: 480 samples for ordinary workloads, 32 for history. Timing starts **before calling write** and includes callback waiting plus two subsequent rAF opportunities, not just time after callback completion. Callback semantics differ, with the candidate scheduling its callback on rAF. This proxy is neither physical presentation nor real input latency; p99 was exploratory. Do not repeat the corrected “all columns are trial medians” or “completion-to-frame” labels.

At eight terminals, wide-text CDP task fell about 61%, refresh about 15%, scroll about 3%, while history rose about 12%. These are not whole-experience wins: functionality differs; parse wrappers include different bridge responsibilities; draw measures synchronous JS, not all GPU work; absent Canvas Paint/Layout trace event names do not mean zero GPU cost. HTTP timing includes main-thread scheduling. The suite is completion-paced, not a fixed arrival-rate or saturation-throughput test.

Browser RSS samples were approximately 714–828MiB for A and 599–737MiB for C. At eight terminals, C also had 44.5–86.4MB of WASM memory; JS heap alone is misleading. New-context navigation was mostly 36–50ms (some initial samples 82–91ms), not a whole-App cold-start or OS cold-cache result. gtmux, Chrome and security services were active; no other person's tasks were stopped, and this was not a dedicated benchmark host. Reliable per-renderer GPU utilization and long-duration leak stability were not established.

Earlier headless samples and clipped 80-column multi-tile samples are excluded from this table. The latter retained 12 candidate wide-text failures among 96 trials as sensitivity evidence. Uncompressed distribution file sizes were 308750B for xterm plus CSS/fit/Unicode11 and 1104963B for third-party ESM+WASM, about 3.58×; this was not a compressed-transfer comparison. Official small WASM was 844279B before a bridge and renderer. Selection/IME/accessibility/scroll/lifecycle maintenance remains an adoption cost.

## 6. Conditions for reopening the decision

1. Recheck the then-current release/main, exact official and third-party versions, API, license and adopter dependencies. Establish a new B, preserve this record and Q, and do not count product-version differences as renderer gains.
2. Reproduce the minimal failures when a candidate fixes selection/reset, viewport preservation and the 80-column crash, or when a maintained Web adapter targets the official stable ABI. An upgrade alone is not capability parity.
3. Rebuild the full capability matrix first: Chinese IME; width/combining/emoji; selection/copy; search/links/clipboard policies; keyboard/mouse/paste/resize/zoom; reconnect/history/scroll; Chat/Terminal/fullscreen; Trace/Diff/attachments/images; waiting/permissions/drafts/identity/receipts. Native and Web may use different renderers; sharing a core does not supply every UI.
4. After parity, compare matched B/configuration/arrival rates, separating parse/transport/layout/paint, actual input paths and synthetic proxies. Record frames, longtasks, CPU/RSS/WASM/available GPU metrics, cold/warm startup, sustained stability, size and maintenance. Mobile needs real devices; screenshots cannot substitute for measurements.
5. Only then produce a product candidate with fallback. Continue using isolated processes, profiles, bundles and synthetic fixtures, without replacing production installations or typing into real sessions. This record does not preapprove implementation or merging.

## 7. Reproduction and evidence retention

Experiment source is retained at fixed commit Q on the separate `research/libghostty-20261110` branch; it is not merged into main by this documentation-only change. Paths relative to Q are `research/libghostty-20261110/README.md` (instructions), `research/libghostty-20261110/DESIGN.md` (sealed design), and `docs/test/designs/libghostty-eval-20261110.json` (test design). Q also contains the generator, localhost harness, B-page probes, native source, locked dependencies and summary script. Confirm Q is available before reopening; these are not paths promised to exist on main. Raw files and caches are not copied into the repository. Versions, counterexamples, metric semantics, decision and reopening conditions remain available here even if temporary evidence is later reclaimed.

The delivery archive is in project-owned `gtmux-deliveries/libghostty-eval-20261110/` (relative to the local project storage root, not this repository). It contains the full matrix and 1/4/8 results in `REPORT.zh.md`, derived `summary.csv/json`, current `SHA256SUMS`, and `REPORT.P3.diff` plus `history/` preserving the two wording corrections and original report. Raw/build/download files live only under `/private/tmp/gtmux-libghostty-20261110/`. These are machine-local retention locations, not portable download URLs or permanent availability promises. Check files and hashes when reopening; if absent, rerun Q as new evidence rather than presenting it as the original run.

|Evidence|SHA256|
|---|---|
|Corrected formal report|`8a2ab7ff7bf9d822e2be9ecaf6e11413c895bcbac9234d79d6c71273a19c7c0d`|
|Original formal report|`503d5da7d52c6cf1da927342f4850ba703e730a0570c072012073605b8159c35`|
|Primary 96-trial data, `evidence/bench-fit.jsonl`|`0922079e2c060f7c283cbfd4e268d7e8018ea3e9f52e208a5d1efe9c214d934f`|
|Earlier 80-column data, `evidence/bench-final.jsonl`|`9af443c8e9f242a8ee06886b2aab6c32425427d2a9b3374821c55729a7f7b65d`|

Retain the original gate limits: native/WASM probes, source design checks and B-to-Q test-history ordering passed. `make -k check` did not fully pass: the native child temporary-HOME guard failed, and the state temporary-path prefix assertion failed with the explicit task TMPDIR. Exact-term private-policy checks were skipped when that policy was unavailable. No assertions were removed or weakened. Recording this research does not turn failures or unverified capabilities into passes.
