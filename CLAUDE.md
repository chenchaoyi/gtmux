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
- Workflow: branch → PR → CI green → squash-merge → tag. Don't commit to `main`.

## Conventions / invariants

- **Contracts (don't break):** the `gtmux agents --json` schema; state paths
  `~/.local/share/gtmux/{active/<pane>, waiting/<pane>, last-finished,
  notify-icon.png, notify/<id>.json}`; hook events
  `Stop`/`Notification`/`UserPromptSubmit`; bundle id `com.gtmux.menubar`. The
  hook→app **notify queue** (`internal/notify` writes JSON; macapp
  `NotificationManager` drains & posts) is the notification channel — there is no
  terminal-notifier/osascript fallback (notifications need the app running).
- **i18n:** every user-facing string is en+zh via `internal/i18n` and `GTMUX_LANG`.
- **Terminal coupling** lives ONLY in `internal/ghostty` (AppleScript). The rest
  (`agents`/`overview`/`hook`/notify) is tmux-only and terminal-agnostic. Project
  direction: generalize `internal/ghostty` into a `Terminal` driver so gtmux is
  not Ghostty-only (iTerm2/kitty/WezTerm/Apple Terminal feasible; Warp/Alacritty
  not). Don't entrench new Ghostty-specific assumptions outside that package.
- **Verifying the status item / popover on macOS** (screen capture is
  permission-blocked): query the accessibility tree, e.g. `osascript -e 'tell
  application "System Events" to get count of menu bar items of menu bar 1 of
  (first process whose name is "GtmuxBar")'`; or run the binary directly with
  `GTMUXBAR_DEBUG=1` (logs to stderr).

---

## 菜单栏 app 设计规范（必读）

实现/改动 macOS 菜单栏 app（`NSStatusItem + NSPopover + SwiftUI`）的任何 UI 时，**先读并遵守
`docs/design/DESIGN.md`**（权威设计规范），参照 `docs/design/mockup/`。要点：

- **状态语言三重编码**（色+形+字形），全表面统一：waiting=红方块·双竖线 / working=青圆·静态加载环 /
  idle=绿圆·✓ / running=灰圆·点。颜色**只**表达状态，状态色用权威值（见下方「代码位置对照」）。
- **层级**：waiting 响、idle 静。分区顺序 needs-you→working→idle→running。
- **agent 身份**用中性单字标，官方图标走 `agents.json` 的 `icon` 字段；**不在代码里绘制第三方商标**。
- **双语** en/zh（跟随 `GTMUX_LANG`），CJK 不换行用省略号；偏好语言三态、即时生效。
- **动效最小**：只允许 idle→waiting 一次脉冲；加载环不旋转；空闲零动画。
- **视觉克制**：无彩虹渐变、无彩色发光阴影；文案平实、禁止营销腔（尤其首次运行权限卡）。
- **支持原生终端**（无 tmux）：行与跳转按 `source: tmux|native` 泛化（DESIGN §7）。
- 与 DESIGN.md 冲突的改动，先提出、不擅自偏离。

### 代码位置对照（重要：DESIGN.md 写于原生化迁移之前）

`docs/design/DESIGN.md` / `HANDOVER.md` 里引用的 Go 文件在迁移到原生 Swift app（v0.0.11）后
已删除。按下表换算到现状，**实现以现状为准、设计意图以 DESIGN.md 为准**：

| DESIGN.md 引用 | 现状位置 |
|---|---|
| `internal/menubar/icon.go`（状态色权威值 `#EF4444/#06B6D4/#22C55E/#8E8E93`） | `macapp/Sources/GtmuxBar/AgentStore.swift` 的 `AgentState.color`。**注意当前用的是 `.systemRed/.systemTeal/.systemGreen/.tertiaryLabelColor` 语义色，尚未对齐 DESIGN 的精确 hex —— 设计重构时需对齐。** |
| `internal/menubar/model.go`（`agents --json` 契约 / Agent shape） | 产出端 `internal/app/agents.go`（`agentJSON`）；消费端 `macapp/Sources/GtmuxBar/AgentStore.swift`（`Agent`）。 |
| `cmd/gtmux-menubar/`（cgo systray 入口） | 已废弃；菜单栏 app 现为 `macapp/`（Swift）。systray 不再使用。 |

`agents.json` 的 `icon` 字段（官方图标）尚未实现，属设计重构待办。
