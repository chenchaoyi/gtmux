# gtmux — repo guide for Claude Code

**gtmux** is a command center for tmux sessions and coding agents. Two surfaces
over one Go core (gtmux-core is the single data source):

- **CLI** — `cmd/gtmux` (Go, **must stay cgo-free**). Commands: `agents`,
  `overview`, `restore`, `focus`, `new`, `hook`, `install-hooks`,
  `uninstall-hooks`, `uninstall-app`. Logic lives in `internal/`.
- **Native menu-bar app** — `macapp/` (Swift / AppKit + `NSStatusItem` +
  `NSPopover` + SwiftUI). A pure **consumer** of the CLI: polls
  `gtmux agents --json` and shells out to `gtmux focus`. It's also the
  notification click target (`com.gtmux.menubar`; reopen → `gtmux focus --last`).

## Build & verify

- CLI: `make build`. App: `make app` (Swift app + bundled CLI → `Gtmux.app`).
- **Run the gate before every commit** (same as CI): `make check`
  (= gofmt + `go vet` + staticcheck + `go test -race`). For the app:
  `cd macapp && swift build -c release`.
- The CLI MUST stay cgo-free — `CGO_ENABLED=0 go build ./cmd/gtmux` must pass.
  Only the Swift app is native; nothing in `internal/` may pull in cgo.
- Release: push a tag `vX.Y.Z` → goreleaser ships the CLI tarballs and a macOS
  job runs `macapp/build.sh` to ship `Gtmux-<v>-macos.zip`. CI builds the app
  but **can't see the menu bar** — smoke-test on real macOS before trusting a tag.
- **Signing & notarization:** `build.sh` signs ad-hoc by default; set
  `GTMUX_SIGN_ID="Developer ID Application: …"` for a STABLE signature (hardened
  runtime) so TCC grants persist across updates (ad-hoc changes identity every
  build → macOS re-prompts). It signs the bundled `gtmux` CLI then the bundle
  (no `--deep`) and prints the `notarytool`/`stapler` steps. To sign in CI, add
  the cert + `GTMUX_SIGN_ID` (+ a notarytool keychain profile) as secrets and
  pass them to the macOS release job — until then releases stay ad-hoc.
- **Mac App Store is NOT a viable target as built:** the app shells out to
  `gtmux`/`tmux`/`osascript` and reads `~/.local/share/gtmux`, `~/.tmux/…` etc.,
  none of which survive the App Sandbox MAS mandates (and there's no entitlement
  for "drive tmux / control arbitrary terminals"). Ship **Developer ID + notarized
  direct distribution**; MAS would require a sandbox-compatible rearchitect.
- Workflow: branch → PR → CI green → squash-merge → tag. Don't commit to `main`.

## Spec-driven development (OpenSpec)

This repo uses **OpenSpec** for spec-driven development. `openspec/specs/`
holds the current capability specs (the source of truth for what IS built);
`openspec/changes/` holds in-flight change proposals. For any non-trivial
feature, **propose a change first**, implement against it, then sync/archive:

- `/opsx:propose "<idea>"` — draft a change (proposal + tasks + spec deltas).
- `/opsx:apply-change` — implement an approved change's tasks.
- `/opsx:sync-specs` / `/opsx:archive-change` — fold the deltas into the main
  specs when done. `npx @fission-ai/openspec validate --specs --strict` must pass.

Keep specs aligned with the code: when behavior changes, update the relevant
`openspec/specs/<capability>/spec.md`. Capabilities are backfilled for the major
existing features (agent-radar, terminal-jump, notifications, menu-bar-app,
env-doctor, session-restore, remote-access, push-notifications, mobile-app).

## Conventions / invariants

- **Contracts (don't break):** the `gtmux agents --json` schema; state paths
  `~/.local/share/gtmux/{active/<pane>, waiting/<pane>, last-finished,
  notify-icon.png, notify/<id>.json}`; hook events
  `Stop`/`Notification`/`UserPromptSubmit`; bundle id `com.gtmux.menubar`. The
  hook→app **notify queue** (`internal/notify` writes JSON; macapp
  `NotificationManager` drains & posts) is the notification channel — there is no
  terminal-notifier/osascript fallback (notifications need the app running).
- **i18n:** every user-facing string is en+zh via `internal/i18n` and `GTMUX_LANG`.
- **Scope (decided):** gtmux focuses on the **tmux + agent** workflow — it only
  detects agents running **inside tmux** (`agents` scans `tmux list-panes`).
  Agents started directly in a terminal tab (no tmux) are intentionally **out of
  scope** for now. The `source/project/terminal/tab` fields + `focus
  --terminal/--tab` are latent groundwork kept for a possible future expansion —
  do NOT build the native-detection scanner without an explicit decision to widen
  scope (status + jump both need per-terminal tab-title reading).
- **Terminal coupling** goes through the `internal/terminal.Terminal` interface
  (`FocusTab`/`IsViewing`/`OpenWindow`/`SpawnTabs`); `internal/ghostty.Driver`
  and `terminal.iterm2` are the two impls. `terminal.Active()` resolves the host
  driver via `detect.go` (`GTMUX_TERMINAL` override → `$TERM_PROGRAM` → tmux
  client process ancestry → Ghostty fallback). Callers
  (`focus`/`restore`/`new`/`hook`) use `terminal.Active()`, never a terminal
  package directly (except the still-deferred native `ghostty.FocusTerminalTab`).
  The radar side (`agents`/`overview`/notify) is tmux-only and terminal-agnostic.
  **iTerm2 gotchas (verified on real iTerm2):** the AppleScript target is
  `"iTerm"` (NOT `"iTerm2"` — that loads no scripting dictionary), the macOS
  *process* name is `"iTerm2"`, and the iTerm session `name` carries the tmux
  title (often suffixed `" (tmux)"` → drivers prefix-match). iTerm2's AX window
  title is empty, so `IsViewing` asks iTerm directly (`frontmost` + current
  session `name`) instead of System Events. Add new terminals as drivers
  (kitty/WezTerm/Apple Terminal feasible; Warp/Alacritty not) — see
  `docs/design/multi-agent-multi-terminal.md`.
- **Verifying the status item / popover on macOS** (screen capture is
  permission-blocked): query the accessibility tree, e.g. `osascript -e 'tell
  application "System Events" to get count of menu bar items of menu bar 1 of
  (first process whose name is "GtmuxBar")'`; or run the binary directly with
  `GTMUXBAR_DEBUG=1` (logs to stderr).

---

## gtmux 设计规范（必读 —— 后续每次 UI 迭代都必须遵循）

gtmux 是一个产品、三块屏（CLI · 菜单栏 · 手机），共用同一套状态语言。**改动任何 UI 前，先读对应
权威设计规范，并严格遵循；不得擅自偏离。**

- 改 **菜单栏 app**（`NSStatusItem + NSPopover + SwiftUI`）→ 先读 `docs/design/DESIGN.md`。
- 改 **移动端 app**（bare React Native，`mobileapp/`）→ 先读 `docs/design/MOBILE.md`。
- 落地总入口 / 顺序 / 验收 → `docs/design/HANDOFF.md`；可视参照 `docs/design/mockup/`。

要点（两块屏统一）：

- **「等你输入」= 仅 `waiting`（红）；`working`（蓝）永不等输入。** 结构化 `1/2/3` 回应只挂 waiting。
- **状态语言三重编码**（色+形+字形），全表面统一：waiting=红方块·双竖线 / working=青圆·静态加载环 /
  idle=绿圆·✓ / running=灰圆·点。颜色**只**表达状态，状态色用权威值（见下方「代码位置对照」/
  `mobileapp/src/ui/theme.ts`）。
- **层级**：waiting 响、idle 静。分区顺序 needs-you→working→idle→running。
- **agent 身份**用中性单字标，官方图标走 `agents.json` 的 `icon` 字段；**不在代码里绘制第三方商标**。
- **双语** en/zh（跟随 `GTMUX_LANG` / 设备语言），CJK 不换行用省略号；语言三态（跟随系统/EN/中文）即时生效。
- **动效最小**：只允许 idle→waiting 一次脉冲；加载环不旋转；空闲零动画。
- **视觉克制**：无彩虹渐变、无彩色发光阴影；文案平实、禁止营销腔（尤其首次运行/权限卡）。
- **支持原生终端**（无 tmux）：行与跳转按 `source: tmux|native` 泛化（DESIGN §7 / MOBILE §2）。
- **连接指示**用 server 名 + 状态点（已连接绿 / 重连琥珀 / 离线红），不用 "live" 字样；离线不清屏、留缓存置灰。
- **移动端**：监控 + focus + 推送 + **终端输入**（`POST /api/send` → `tmux send-keys`，仅
  bearer token 把关，默认开启 —— token 泄露即可在 Mac 上执行命令,务必当密码看待）。语音输入仍属 P3。
- **命名**统一小写 `gtmux`。
- **与 `DESIGN.md` / `MOBILE.md` 冲突的改动，先提出、不擅自偏离；改了设计就同步更新这两份规范。**

### 代码位置对照（重要：DESIGN.md 写于原生化迁移之前）

`docs/design/DESIGN.md` / `HANDOFF.md` 里引用的 Go 文件在迁移到原生 Swift app（v0.0.11）后
已删除。按下表换算到现状，**实现以现状为准、设计意图以 DESIGN.md 为准**：

| DESIGN.md 引用 | 现状位置 |
|---|---|
| `internal/menubar/icon.go`（状态色权威值 `#EF4444/#06B6D4/#22C55E/#8E8E93`） | `macapp/Sources/GtmuxBar/AgentStore.swift` 的 `AgentState.color`。**注意当前用的是 `.systemRed/.systemTeal/.systemGreen/.tertiaryLabelColor` 语义色，尚未对齐 DESIGN 的精确 hex —— 设计重构时需对齐。** |
| `internal/menubar/model.go`（`agents --json` 契约 / Agent shape） | 产出端 `internal/app/agents.go`（`agentJSON`）；消费端 `macapp/Sources/GtmuxBar/AgentStore.swift`（`Agent`）。 |
| `cmd/gtmux-menubar/`（cgo systray 入口） | 已废弃；菜单栏 app 现为 `macapp/`（Swift）。systray 不再使用。 |

`agents.json` 的 `icon` 字段（官方图标）**已实现**（v0.0.22+）：profile 的 `icon`
经 `agents --json` 透传，菜单栏 app `AgentIcons` 解析为头像图标 —— `.app` 路径走
`NSWorkspace` 取**用户已装应用的真实官方图标**（不在仓库内置/绘制第三方商标，符合
§6），图片路径直接加载，亦支持 `~/.config/gtmux/icons/<agent-key>.png` 免配置投放；
取不到时回退中性单字标。内置默认把 Claude/Codex/Cursor 指向 `/Applications/*.app`。
