# libghostty 终端替换调研

[English](libghostty-research.md) · 调研：2026-11-10 · 归档：2026-11-11

## 1. 决策与适用边界

**本轮不把 gtmux 的终端页面整体换成 libghostty。** 保留 Web 的 xterm 和移动端原生实现。部分合成负载更省 CPU，但被测 Web 候选存在崩溃、复制失真和滚动位置不兼容，未达到“现有能力全部保留且体验更好”的要求。这是对下文固定版本的结论，不是对未来版本的永久否定。

官方 libghostty-vt 1.0 的原生与 WASM API 已通过最小接入实证，可以继续作为终端状态核心研究；没有证明它能直接替换整个 UI。研究证据和整体不替换结论获独立审查认可，不代表合入、安装或部署批准。

这里的“原生 macOS 终端”是指用 AppKit 等 macOS 界面技术在 App 内直接绘制终端，而不是显示浏览器网页。基线中的 Mac App 相关工作区会打开浏览器；另有原生会话阅读界面，但没有同等的嵌入终端面可直接换库。原生方案意味着另建终端界面，仍需字体绘制、输入法、选择复制、滚动和无障碍等宿主能力。

|形态|本轮决定|
|---|---|
|终端 / remote attach|保持现有链路；未改变宿主终端，也未验证真实 PTY attach。Go CLI/core 保持 cgo-free。|
|菜单栏 / macOS App|官方 C 核心可接入；自绘 AppKit PoC 不是可替换产品界面，尚无同类原生 A/B 收益结论。|
|手机|保持现有 RN/native 实现；未验证 iOS/Android 真机或 WebView 替换。|
|iPad|同样保留现状；分栏、外接键盘、选择和生命周期未验。|
|Web|被测第三方适配层不采用；xterm 继续作为对照和现行后端。|

## 2. 固定版本与产品事实

本次观察到的远端 main 与发行 `v1.0.174` 同为基线 **B `094f3d0fcccba3066f37f829847330ea4686f185`**。起始旧树 `aa43ffdf` 不作为产品基线。设计先封存于 **D `15e2c4886ed1ae28236edcc40a8b84f3ce223c0e`**，实验源码固定在 **Q `2b0de8774f96c36415962483ae8b05317b5fe454`**。后续文档提交不改变 Q；重新调研应另选当时的权威基线，不把本文 B 当成永远的最新版。

|对象|当时核实的版本|不能混称的边界|
|---|---|---|
|官方 libghostty-vt|`vt-1.0.0`，tagger 2026-11-06 22:55:16 UTC；commit `0fa3d577a5614150ff8e2c152de5fb9d5c6d2dec`|MIT，C ABI 的解析/状态/格式化/输入编码/选择/搜索/render-state 核心；不附完整 Metal、AppKit 或浏览器渲染器。官方列出 Linux/macOS/iOS/Android/FreeBSD/Windows/WASM 等支持，平台可构建不等于该平台 UI 已完成。|
|Ghostty App|1.4.0，2026-10-27|完整 macOS/Linux 终端 App；其版本和 GPU 渲染体验不能直接算作 VT 库的能力。|
|第三方 `coder/ghostty-web`|npm 稳定版 0.4.0，2025-12-09；嵌入 Ghostty `5714ed07a1012573261b7b7e3ed2add9c1504496` 及补丁|本轮 Web 接入对象：Canvas2D + JS + 旧 WASM，**不是官方 VT 1.0**。官方 1.0 缺少它依赖的旧 `ghostty_terminal_write` 导出，不能直接换 WASM 文件升级。|

主源：[官方 release](https://github.com/ghostty-org/ghostty/releases/tag/vt-1.0.0)、[固定 API](https://github.com/ghostty-org/ghostty/blob/0fa3d577a5614150ff8e2c152de5fb9d5c6d2dec/include/ghostty/vt.h)、[官方平台/安装](https://lib.ghostty.org/vt/installing.html)、[许可证](https://github.com/ghostty-org/ghostty/blob/vt-1.0.0/LICENSE)、[App 1.4](https://ghostty.org/docs/install/release-notes/1-4-0)、[第三方 v0.4.0](https://github.com/coder/ghostty-web/releases/tag/v0.4.0)。这些是本轮核实记录；后续重评必须重新核对版本。

采用方不能替代适配验证：JetBrains 的[官方公告](https://blog.jetbrains.com/platform/2026/10/jetbrains-terminal-now-uses-ghostty/)对应 **2026.3 EAP**，IDE 保留渲染和上层交互；当时 [coder/xum](https://github.com/coder/xum/blob/main/package.json) 0.32.1 使用 `ghostty-web 0.4.0-next.7.g03ead6e`，还依赖 xterm headless/serialize；[cmux](https://github.com/manaflow-ai/cmux/blob/main/.gitmodules) 使用 `manaflow-ai/ghostty` fork，读取的子模块为 `ef4f1570d62117ebc51e3b0f07ad4f34032267b6`。后两者链接随 main 变化，原始快照见正式报告；不能推导为官方 VT 1.0 已在 gtmux 所需 UI 上全能力落地。

## 3. 链路与历史教训

```text
tmux（已解析 VT）→ capture-pane -e → Go 授权 API → B 页面 append/reset → xterm
                                                      └→ 隔离 ghostty-web 对照
Composer 草稿/权限/身份 → /api/send → 投递确认与回执（不交给 renderer）
官方 VT 1.0 C/WASM → 终端状态 → 自行实现平台渲染与交互
```

B 的 `internal/tmux/tmux.go:CapturePaneColor`、`internal/server/web/app.js:ensureTerm/writePane` 是入口。Web 用 xterm 5.5、fit、Unicode11，主窗 scrollback 5000、tile 4000、禁 stdin。快照前缀匹配则追加，否则 reset/write；用 `baseY - viewportY` 保滚，手势后约900ms冻结。**这里不是原始 PTY 流；解析器吞吐不能代表当前产品链路的总收益。** Chat、Trace、Diff、附件、等待/权限、精确会话身份、草稿和发送回执属于上层能力，不能因更换 renderer 被省略。

|历史证据|后续需要记住的事|
|---|---|
|`24aa887e`（#126，2026-06-25）与 `bdf942ea`（#131）|曾引入 xterm 5.5 WebView，再尝试 WebGL 优化滚动。|
|`6f369f36b06b927ee1b8f30cb28766226d34d5ec`（#145，2026-06-26）|WebGL 真机黑屏，退到 Canvas；模拟器通过不代表真机 GPU 稳定。|
|`10b16656859d7da4b1a0db08022ae0f4b6c7aec9`（#208，2026-06-29）|键盘弹出、黑屏与 canvas 脆弱性促成移动端 RN 原生替换 WebView。|
|`365f30842fbd8c9a0f95608c04e0d7aad51a36c3`（#209，同日）|原生也有巨大嵌套 Text 卡顿/崩溃、轮询擦除选择、滚动冻结与历史上限等问题；“原生”不自动等于更快。|
|`284a6795`（#194）、`4b34e6f7`（#345）、`b405dbc5`（#346）|分别记录 Web 保滚/reflow、原生符号 emoji 修复和删除死 WebView。|

当时搜索未找到此前 libghostty 集成记录。Ghostty **宿主 App** 聚焦/标题、tmux tab、restore 身份链故障不能当成 libghostty renderer 故障。旧未提交工作树未借用或改动。

## 4. 实证、缺口与不可放宽的能力

|观察|实测结果及边界|
|---|---|
|最小 VT/ANSI/光标/alt-screen|两 Web 后端与官方 C/WASM 最小探针通过；不是完整协议/TUI 一致性认证。|
|宽字符边界|40列连续120次完成；全新单终端80列、无裁切也无先前reset/selection时，第三方 WASM write 越界；xterm完成。官方1.0 WASM在80×24同源120次输入完成。未把第三方旧核心/补丁/bridge的问题归到官方1.0。|
|滚动与历史|xterm第40行保滚通过；候选兼容 API 的 `baseY/viewportY` 返回0，write还会回到底部，无法满足 B 的保滚逻辑。|
|选择、复制、Unicode|xterm中文/组合字/ZWJ复制通过；候选增加CJK空格，reset后选择读到旧ALT；源码有 selection manager 未随重建 wasmTerm 换绑的证据。文本读回也不完整。|
|resize/缩放/本地输入|候选 standalone reset/selection/scroll 后点击resize超时；B窄屏检查通过。是特定状态序列失败，不能说所有resize都失败。只读paste无onData通过；未验OS键盘/IME及完整鼠标协议。|
|B 页面|Terminal tile/focus、Chat/Trace/Diff切换与草稿保留、快照替换+窄屏、零fixture发送/无意外异常，两后端各4/4。快照只断言UPDATED出现，不等于所有残留消除或真实断网重连通过。|
|原生绘制|官方C API链接Cocoa，AppKit自绘PNG，直接synthetic keyDown、resize、alt restore断言通过。不是Ghostty Metal renderer，也没验OS IME、系统剪贴板、完整选择或无障碍。|
|未覆盖的产品能力|附件/图片预览、终端搜索、hyperlink/OSC8/52策略、真实权限与agent等待、精确身份/投递/回执、系统全屏、移动真机、长时稳定性没有完整验收。未改源码不等于这些能力已回归通过。|

最终 standalone：xterm 8/8，candidate 4/8；失败全部保留。继续研究必须保留共享策略：精确 session/home/process birth/TTY、草稿保护、queued 不等于 landed、发送回执、截图 scope/lifecycle。不得把 `terminal.onData` 直接接到真实发送绕过 Composer，不得为性能放宽权限和身份。

## 5. 性能记录及正确读法

M4 Pro / 14 CPU / 24GiB，macOS27.2，Chrome157.0.8103.39；DPR2、1440×900、Hack14px、40×16、5000行历史，1/4/8终端均完整显示。同一B、负载和合成HTTP传输，ABBA重复两轮：每个backend/负载/终端数4次，共96有效trial。普通负载120步，history为8次×5000行。此表摘录8终端结果，A=xterm，C=ghostty-web，单位ms。

|负载|parse A/C|draw A/C|CDP task A/C|write前至callback及两次rAF机会 p95 A/C|
|---|---:|---:|---:|---:|
|滚屏|34.90 / 28.00|168.15 / 490.50|845.11 / 818.93|15.20 / 25.40|
|长历史|214.80 / 219.50|14.30 / 34.40|267.86 / 300.40|85.00 / 51.10|
|局部刷新|21.25 / 28.25|181.10 / 543.40|1047.11 / 885.73|24.50 / 25.50|
|宽字符|20.55 / 18.35|464.30 / 494.25|1946.66 / 760.89|18.10 / 25.50|

前三列是每trial累计时间的4次中位数；末列是4次逐步样本**合池p95**，普通负载480个样本、history32个。计时**从write调用前开始**，含callback等待与随后两次rAF机会，不是从callback完成才开始。两后端callback语义不同，候选自身排到rAF；这个代理指标不是物理呈现或真实输入延迟。p99仅探索性。不要重复已纠正的“四列都是trial中位数”或“completion-to-frame”标签。

8终端宽字符CDP task约降61%，刷新约降15%，滚屏约降3%，历史约升12%。这不构成全体验收益：功能未对齐；parse包含不同bridge责任；draw是同步JS调用，不含全部GPU；Canvas缺少同名Paint/Layout trace事件不等于GPU零成本。HTTP耗时也含主线程调度。测试按完成节奏推进，不是固定外部到达率或饱和吞吐。

RSS样本A约714–828MiB、C599–737MiB；8终端候选另有44.5–86.4MB WASM memory，不能只比JS heap。新context导航约36–50ms（首次部分样本82–91ms），没有整App冷启动/OS冷缓存结论。背景有gtmux、Chrome、安全服务等负载；未停止他人任务，无专用跑分机置信度。未取得可靠逐renderer GPU利用率，也未证明长时无泄漏。

旧headless样本与80列多tile裁切样本不进入主表；后者96次中12个候选宽字符失败保留为敏感性证据。包体原文件：xterm及CSS/fit/Unicode11合计308750B；第三方ESM+WASM合计1104963B，约3.58倍，非压缩传输对比。官方small WASM844279B还需bridge和renderer。维护选择/IME/a11y/滚动与生命周期适配是采用成本。

## 6. 何时值得重评

1. 先核当时发行/main和官方/第三方精确版本、API、许可证、采用方依赖；另建B，保留本文与Q，不把新旧版本差当renderer收益。
2. 有修复复制/reset、viewport保滚、80列越界的候选，或有人愿意维护官方1.0稳定ABI的新Web适配层，再复跑这些最小反例。升级成功不等于所有能力保持。
3. 先重新填全能力矩阵，覆盖中文IME、宽度/组合字/emoji、选择复制、搜索/链接/剪贴板策略、键鼠粘贴resize缩放、重连/历史/滚动、Chat/Terminal/全屏、Trace/Diff/附件/图片、等待/权限/草稿/身份/回执。原生与Web可选不同renderer，共享核心不等于共享全部UI。
4. 功能达标后，才做同B、同配置、同到达率的配对性能比较，区分parse/transport/layout/paint、真实输入链路与合成代理；记录帧、longtask、CPU/RSS/WASM/GPU可得性、冷热启动、长时稳定性、包体和维护成本。移动需要真机，截图不能替代测量。
5. 有完整收益证据才形成带回退的产品候选；继续保持独立进程、profile、bundle和合成fixture，不安装替换生产App、不向真实会话输入。本记录不预先批准实施或合入。

## 7. 复现与证据保留

实验源码保留在独立研究分支 `research/libghostty-20261110` 的固定提交Q，未随本次纯文档变更合入main。以下路径均相对Q：`research/libghostty-20261110/README.md`（实验说明与命令）、`research/libghostty-20261110/DESIGN.md`（封存设计）、`docs/test/designs/libghostty-eval-20261110.json`（测试设计）。Q还包含generator、localhost harness、B页面探针、native源码、锁定依赖与汇总脚本；重评时先确认能取得Q，不能把这些路径当成main已有文件。本文不复制raw或缓存进repo；即使临时证据以后回收，版本、反例、指标解释、结论与重评门槛仍可从此处找回。

交付归档位于项目拥有的 `gtmux-deliveries/libghostty-eval-20261110/` 目录（相对本机项目存储根，不是本仓库路径）。其中 `REPORT.zh.md` 为完整能力矩阵与全部1/4/8结果，`summary.csv/json` 为汇总，`SHA256SUMS` 为当前清单，`REPORT.P3.diff` 与 `history/` 保留两处标签修订及原报告。raw/build/download仅在 `/private/tmp/gtmux-libghostty-20261110/`；这些是本机保留位置，不是可移植下载地址，不能承诺永久存在。重评时先核文件及SHA；缺失时按Q复跑并生成新证据，不能冒充旧结果。

|证据|SHA256|
|---|---|
|修订后正式报告|`8a2ab7ff7bf9d822e2be9ecaf6e11413c895bcbac9234d79d6c71273a19c7c0d`|
|原正式报告|`503d5da7d52c6cf1da927342f4850ba703e730a0570c072012073605b8159c35`|
|主96次数据 `evidence/bench-fit.jsonl`|`0922079e2c060f7c283cbfd4e268d7e8018ea3e9f52e208a5d1efe9c214d934f`|
|旧80列数据 `evidence/bench-final.jsonl`|`9af443c8e9f242a8ee06886b2aab6c32425427d2a9b3374821c55729a7f7b65d`|

原门禁边界也要保留：native/WASM探针、源码设计检查、B→Q测试历史顺序通过；`make -k check`未全绿，native临时HOME子进程保护断言失败，显式task TMPDIR下state临时路径前缀断言失败；私有policy缺失时精确词检查跳过。没有删改断言来通过。文档归档不把这些失败或未验范围改写为通过。
