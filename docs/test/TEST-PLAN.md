# gtmux 测试方案（Test Plan）

> 本文是 gtmux 的测试策略与测试设计。每次功能迭代都要**回看本方案**，按需更新测试设计、补/改用例，
> 并更新 [`DESIGN-TRACEABILITY.md`](DESIGN-TRACEABILITY.md)（设计完整跟随情况）。

本文列的是测试要求，不是各平台已通过的验收记录。实际验收须另记版本、设备、场景与结果。

## 0. 目标与分层

gtmux 有五个界面：终端 CLI、macOS 菜单栏、手机、iPad、Web。Go 核心提供状态与 API；
React Native 同时承载手机和 iPad。测试从「机器可判定」走到「人工验收」：

| 层 | 测什么 | 在哪 | 何时跑 |
| --- | --- | --- | --- |
| L1 · Go 单元 | CLI 逻辑：agent 分类/排序、`agents --json` 契约、hook 状态机、ghostty 脚本、设置合并 | `internal/**/_test.go` | `make check` / CI（每 PR） |
| L2 · Swift 单元 | app 纯逻辑：**状态色=DESIGN 权威 hex**、相对时间、分组/过滤/搜索、JSON 解码、monogram | `macapp/Tests/` | `cd macapp && swift test` / CI（每 PR，macOS） |
| L2 · 移动端 JS | 配对、诊断、聊天、无障碍标签及手机/iPad 共用控件 | `mobileapp/src/**/*.test.ts(x)` | `cd mobileapp && npm run check` / CI（每 PR） |
| L2 · Web 交互 | 配对码、连接状态、发送失败恢复 | `internal/server/web/app.test.cjs` | `node --test internal/server/web/app.test.cjs` / CI（每 PR） |
| L2 · Worker | APNs 推送中继与隧道控制面的类型和行为（含 Direct） | `relay-worker/`、`tunnel-worker/` | 各目录 `npm run typecheck && npm test` / CI（每 PR） |
| L3 · 一致性自检 | **设计跟随 + 架构不变量**：状态色与 DESIGN §9 一致、app 不引入 systray、app 只消费不自探测、CLI cgo-free | `scripts/check-design.sh` | CI（每 PR） |
| L4 · 人工验收 | 视觉与交互（无法机器判定）：DESIGN §13 矩阵、浅/深/着色菜单栏、键盘、i18n 即时切换、偏好设置 | 真机 macOS，对照 `docs/design/mockup/` | 发版前 + 收到设计变更时 |

L1–L3 在 CI 全自动；L4 需分别覆盖 Mac、手机、iPad、Web（见 §3）。

## 1. 测试设计原则

- **设计即断言**：把 DESIGN.md 里**可量化**的规范变成断言（颜色 hex、分区顺序、徽章=色+形+字形的存在性、
  相对时间格式、native/tmux 行为差异）。视觉细节（间距、材质观感）留给 L4。
- **纯函数优先**：把可测逻辑抽成纯函数（`relativeTime(_:now:)`、`AgentStore.fuzzy`、`sections(...)`、
  `Agent` 解码、`agentMonogram`），避开 AppKit/UI，单测稳定快。
- **契约锁定**：`agents --json` 的字段（含 §7 的 `source/project/terminal/tab/activity_at`）有契约测试，
  防止字段被悄悄改名/删除而打穿 app。
- **架构不变量自检**：见 L3，把「app 是纯消费方」「CLI cgo-free」「不回退 systray」做成 CI 闸门。

## 2. 持续迭代要求（每次功能迭代必做）

1. **回看测试设计**：本功能触及哪一层？是否需要新纯函数以便单测？
2. **更新用例**：新增/调整 L1/L2 用例；若引入新的 DESIGN 可量化点，加进 L3。
3. **更新跟随矩阵**：在 [`DESIGN-TRACEABILITY.md`](DESIGN-TRACEABILITY.md) 标注该 DESIGN 小节的实现/测试/状态。
4. **架构合理性 review**：确认未破坏不变量（消费方、cgo-free、终端耦合只在 `internal/terminal`
   的 `Terminal` 驱动里（ghostty / iterm2 / warp）、Theme 是唯一 token 权威）。
   `scripts/check-design.sh` 守机器可判定的部分，其余在 PR 描述里自评。
5. **跑全闸门**：`make check`、`cd macapp && swift test`、`cd mobileapp && npm run check`、
   Web/Worker 测试和 `./scripts/check-design.sh`，CI 必绿。

## 3. 人工验收清单（L4，发版前）

对照 `docs/design/mockup/` 与 DESIGN §13 矩阵，在真机逐项确认：

- **状态项**：当前 `StatusItemGlyph` 始终画 gtmux pane 网格；waiting 红、working 青、idle 绿，空列表或仅 running 为中性色，计数由 `AppDelegate.renderIcon` 添加。
  浅/深/**着色**菜单栏都应可辨；三种显示模式（点/点+数字/空闲隐藏）切换正确。色+形+字形的完整状态编码在行内徽章另验收；菜单栏网格在无计数的模式下只靠颜色区分状态，是与 DESIGN §2 非颜色编码要求的已知偏离，不能记为该要求已通过。
- **popover**：分区 needs-you→working→idle→running、waiting 标题红+行淡红底；行=头像+状态徽章、session 主/
  window 次、task 省略号、相对时间、跳转记号；hover=选中；`↑↓⏎⎋`；超长 task 与 CJK 不破行/溢出。
- **跳转**：点行/⏎ → tmux 用 pane id 正确切；native 行只显示感知状态，不能直接跳转或输入。可恢复的 native 会话提供「转入 tmux」，另验收其新建 tmux 会话的恢复路径。
- **空状态/首次运行**：文案平实无营销腔；权限卡步骤正确。
- **偏好**：语言三态**即时生效**（状态项/popover 跟随）；刷新间隔、开机自启、显示模式、通知开关生效。
- **动效**：按 DESIGN §10，行内 working 环仅在界面打开且未启用 Reduce Motion 时旋转，周期 2 秒；关闭界面后应换成静态视图。菜单栏网格不旋转；服务器模式的呼吸点另按 §17 验收。idle→waiting 单次脉冲仍是未实现要求，不计为已通过。
- **真机工具**：按 [`REAL-DEVICE.zh.md`](../../mobileapp/e2e/REAL-DEVICE.zh.md) 选择可用入口；安装、截图、控件树、VoiceOver 实际播报分别记录。
- **手机/iPad**：配对、切 Mac、HQ 对话与知识页、VoiceOver/TalkBack 标签；窄屏和分栏各走一遍。
- **Web**：访客链接配对、权限受限、发送失败保留草稿；窄屏和宽屏各走一遍。

## 4. 怎么跑

```sh
# 从仓库根目录运行；子目录命令用子 shell，后续命令仍在根目录。
make check                       # L1 Go: fmt + vet + staticcheck + race tests
(cd macapp && swift test)         # L2 Swift 单元
./scripts/check-design.sh        # L3 设计/架构一致性
(cd mobileapp && npm run check)   # 手机/iPad 共享逻辑与控件
node --test internal/server/web/app.test.cjs  # Web 交互测试
(cd relay-worker && npm run typecheck && npm test)
(cd tunnel-worker && npm run typecheck && npm test)
# L4：构建 app 真机验收
make app                         # 产出 macapp/build/Gtmux.app；不自动安装
```

先准备 Go、Node/npm 和各 JS 目录的依赖；Swift/AppKit 检查及 app 构建需要 macOS 和 Xcode 工具链。
这些命令按所改范围选用；各平台的运行与视觉验收仍须分别完成。

CI（[`.github/workflows/ci.yml`](../../.github/workflows/ci.yml)）：Linux 跑 Go 格式、vet、staticcheck、race/coverage、漏洞检查和设计检查；
另以 `CGO_ENABLED=0` 交叉构建 darwin arm64/amd64 CLI。移动端 job 跑 TypeScript、ESLint、Jest 和 Web 交互测试，两个 Worker 各跑类型和行为检查。
macOS job 跑 release `swift build`、`swift test` 和设计检查；这些检查不等于实际设备的视觉与交互验收。
