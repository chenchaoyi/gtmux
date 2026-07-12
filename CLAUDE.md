# gtmux — repo guide for Claude Code

**gtmux** is a command center for tmux sessions and coding agents. Two surfaces
over one Go core (gtmux-core is the single data source):

- **CLI** — `cmd/gtmux` (Go, **must stay cgo-free**). Commands: `agents`,
  `digest`, `hq`, `usage`, `limits`, `events`, `resource`, `overview`, `restore`, `focus`, `new`, `adopt`, `send`, `hook`,
  `serve`, `tunnel`, `doctor`, `update`, `install-hooks`, `uninstall-hooks`,
  `uninstall-app`. Logic lives in `internal/`. `digest`+`hq` = the supervisor
  (中控) MVP: a deterministic per-agent digest (goal/last/ask, zero LLM tokens;
  also `GET /api/digest`) + a supervisor agent session at `~/.config/gtmux/hq/`
  (radar rows carry `role:"supervisor"`; the hook nudges it on waiting events —
  `hqNudge:false` disables). See `openspec/changes/supervisor-mvp`.
- **Native menu-bar app** — `macapp/` (Swift / AppKit + `NSStatusItem` +
  `NSPopover` + SwiftUI). A pure **consumer** of the CLI: polls
  `gtmux agents --json` and shells out to `gtmux focus`. It's also the
  notification click target (`com.gtmux.menubar`; reopen → `gtmux focus --last`).

## Build & verify

- CLI: `make build`. App: `make app` (Swift app + bundled CLI → `Gtmux.app`).
- **Run the gate before every commit** (same as CI): `make check`
  (= gofmt + `go vet` + staticcheck + `go test -race`). For the app:
  `cd macapp && swift build -c release`. For the **mobile app**, the equivalent
  one-command gate is `cd mobileapp && npm run check` (= `tsc --noEmit` + `eslint .`
  + `jest --ci`; 0 errors required, eslint warnings tolerated) — same three checks
  CI's `mobile` job runs. The release tag gate also runs `make check` (not a weaker
  `go test`), so a tag can't ship a regression a PR would have caught.
- The CLI MUST stay cgo-free — `CGO_ENABLED=0 go build ./cmd/gtmux` must pass.
  Only the Swift app is native; nothing in `internal/` may pull in cgo.
- Release: push a tag `vX.Y.Z` → goreleaser ships the CLI tarballs and a macOS
  job runs `macapp/build.sh` to ship `Gtmux-<v>-macos.zip`. CI builds the app
  but **can't see the menu bar** — smoke-test on real macOS before trusting a tag.
- **Signing & notarization:** `build.sh` signs ad-hoc by default (Gatekeeper-blocked
  on other Macs → needs `xattr -dr com.apple.quarantine`). Set `GTMUX_SIGN_ID=
  "Developer ID Application: …"` for a STABLE Developer ID signature (hardened
  runtime; TCC grants then persist across updates), and `GTMUX_NOTARY_KEY`/`_ID`/
  `_ISSUER` (App Store Connect API key) OR `GTMUX_NOTARY_PROFILE` (keychain) to
  **notarize + staple** — then it opens on any Mac with no quarantine dance. **CI
  does this automatically when the release secrets are set** (`release.yml` app job
  imports the cert into a throwaway keychain): `MACOS_CERT_P12` (base64 .p12),
  `MACOS_CERT_PASSWORD`, `MACOS_NOTARY_KEY_P8` (base64 .p8), `MACOS_NOTARY_KEY_ID`,
  `MACOS_NOTARY_ISSUER` (the signing identity is auto-derived from the cert). Without them the release stays
  ad-hoc. One-time setup: `docs/release-signing.md`.
- **Mac App Store is NOT a viable target as built:** the app shells out to
  `gtmux`/`tmux`/`osascript` and reads `~/.local/share/gtmux`, `~/.tmux/…` etc.,
  none of which survive the App Sandbox MAS mandates (and there's no entitlement
  for "drive tmux / control arbitrary terminals"). Ship **Developer ID + notarized
  direct distribution**; MAS would require a sandbox-compatible rearchitect.
- Workflow: branch → PR → CI green → squash-merge → tag. Don't commit to `main`.
- **git-ops footgun:** never build a `gh pr create` / `git commit` body via
  `--body "$(cat <<'EOF' … EOF)"` or `-m "$(…)"` when the text contains backticks —
  the `"$(…)"` re-enables command substitution, so `` `gtmux serve` `` in prose gets
  **executed** (this once spawned a rogue serve that squatted :8765). Use
  `--body-file <path>` / `git commit -F <path>` instead. After a PR-create that
  warned/errored, `ps aux | grep 'gtmux serve'` and kill strays.
- **Debug/release pitfalls live in `docs/TROUBLESHOOTING.md`** — a living checklist
  (duplicate-serve pairing bug, QUIC-blocked tunnel, relay-redeploy, etc.). Consult
  it when pairing/push/release misbehaves, and **append a new entry whenever a
  footgun costs real time** (symptom → root cause → must-check).

## Deploy — where each artifact ships (DON'T FORGET)

Four artifacts, **three different delivery paths**. A code change isn't "shipped"
until the right one runs:

| Artifact | Ships via | Command / notes |
|---|---|---|
| **CLI** (`gtmux`) | git tag `vX.Y.Z` → `release.yml` (goreleaser) | tarballs + Homebrew. Users get it with `gtmux update` / curl `install.sh`. |
| **macOS app** (`Gtmux.app`) | git tag → `release.yml` builds it; the **notarized** app is published by **`make app-release`** from a Mac (CI ships it only when the 5 signing secrets are set — see Signing below) | `Gtmux-<v>-macos.zip` + Homebrew cask → `~/Applications`. |
| **Mobile app** (`com.gtmux.app`) | **NOT a git tag** — manual device build | `cd mobileapp && bash scripts/set-version.sh` then `xcodebuild -workspace ios/GtmuxMobile.xcworkspace -scheme GtmuxMobile -configuration Release -destination 'id=00008130-001C75142290013A' -derivedDataPath ios/build/dd DEVELOPMENT_TEAM=2337SY8FRT CODE_SIGN_STYLE=Automatic MARKETING_VERSION=<v> APS_ENVIRONMENT=development -allowProvisioningUpdates build` → `xcrun devicectl device install app --device 1BBBCF4D-4207-516C-AB87-B17F911F753B <app>`. Embeds the `GtmuxWidget` + `GtmuxNotificationService` app-extension targets (wired by the `ios/add_*.rb` xcodeproj scripts). **`APS_ENVIRONMENT` controls the aps-environment entitlement AND the reported push env: a dev device build MUST pass `=development` (→ sandbox APNs, matching the dev signing); an App Store/TestFlight ARCHIVE leaves it at the Release default `production`. The app reads it back (LiveActivityModule `apnsEnv` constant from `Info.plist APNS_ENV`) and reports it at push-register so ONE relay routes per token.** |
| **Push relay** (`gtmux-relay.ccy.dev`) | **the LIVE relay is the Cloudflare Worker `relay-worker/` (TS)** — NOT the Go `relay/` (that's the self-host reference impl) | `cd relay-worker && npx wrangler deploy` (wrangler is OAuth-logged-in). **Per-token APNs env:** each push intent carries `env` (`sandbox`/`production`) that the device reported at register (via serve's `DeviceToken.Env`/`PushIntent.Env`), so ONE relay serves dev + App Store; `APNS_ENV` is only the fallback for env-less (old) tokens. (The Go `relay/` reference is single-env — a self-host simplification.) **Keep `relay-worker/src/index.ts` and `relay/apns.go` in sync on payload SHAPE, and REDEPLOY the Worker** — editing only the Go one changes nothing live. |
| **Tunnel provisioner** (`api.gtmux.ccy.dev`) | Cloudflare Worker `tunnel-worker/` | `cd tunnel-worker && npx wrangler deploy`. See [[hosted-tunnel-a1]] / `docs/design/remote-access-tunnel.md`. |

Corp-network caveat: `api.cloudflare.com` is intermittently TLS-reset from the
office — wrangler calls may need a retry (see `docs/design/remote-access-tunnel.md`).

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

### RULE — spec ⇄ code ⇄ test consistency (REQUIRED; part of "done")

A 2026-07-12 audit found docs/specs/tests drifting from the code (e.g. the
`self-hosted-tunnel` change was fully implemented — `internal/app/tunnelself.go` —
yet its `tasks.md` sat at 0/14 and it was never archived). To stop that drift, a PR
that changes the **observable behavior of a spec'd capability is NOT done** until,
in the **same PR**:

1. **Spec updated** — edit the relevant `openspec/specs/<capability>/spec.md` (small
   change) or land it via an `openspec/changes/<id>` proposal (non-trivial). A
   behavior change with no spec delta is incomplete.
2. **Tests updated** — add/adjust the test(s) that pin the new behavior (Go
   `*_test.go` / mobile jest / e2e). "It builds" is not coverage.
3. **Docs/memory corrected** — any doc, memory, or `CLAUDE.md` line that cites the
   changed file / flag / behavior is fixed in the same PR. No stale references.

**Historical consistency (the spec lifecycle is not optional).** propose →
implement → **sync-specs + archive-change**. The moment a change in
`openspec/changes/` is implemented + merged, ARCHIVE it (same PR, or the very next),
and keep its `tasks.md` checkboxes truthful as you go. Invariant: `specs/` = what IS
built · `changes/` = ONLY truly in-flight work · `changes/archive/` = the audit
trail. An implemented change left in `changes/` (or unchecked tasks over shipped
code) is exactly the drift we are eliminating.

**Enforced:** `scripts/check-design.sh` (CI's "design + architecture conformance"
step) runs `openspec validate --specs --strict` — a malformed/broken spec fails the
build like a red test. Validation only proves the spec is well-formed; the
spec-matches-code and archive-hygiene points above are a **review-gate checklist**
(a reviewer confirms all three before squash-merge — they can't be fully automated).

## Conventions / invariants

- **Contracts (don't break):** the `gtmux agents --json` schema (incl. the
  additive optional `role` field) + the `gtmux digest --json`/`GET /api/digest`
  shape + `gtmux usage --json`/`GET /api/usage` (usage-watch); state paths
  `~/.local/share/gtmux/{active/<pane>, waiting/<pane>, last-finished,
  notify-icon.png, notify/<id>.json}`; hook events
  `Stop`/`Notification`/`UserPromptSubmit`; bundle id `com.gtmux.menubar`. The
  hook→app **notify queue** (`internal/notify` writes JSON; macapp
  `NotificationManager` drains & posts) is the notification channel — there is no
  terminal-notifier/osascript fallback (notifications need the app running).
- **i18n:** every user-facing string is en+zh via `internal/i18n` and `GTMUX_LANG`.
- **Scope (decided):** gtmux focuses on the **tmux + agent** workflow. Its rich
  view/control surface is **tmux-only** (`agents` scans `tmux list-panes`; focus &
  send need a pane). **Non-tmux ("native") agent sessions are now SENSED**
  (read-only): a hook that fires with no `$TMUX_PANE` records the session by id in
  `internal/native`, and the radar shows it as a `source:"native"` row — sense-only
  (no view/jump/send), in the menu-bar's "Elsewhere / 不在 tmux" category. Resumable
  ones can be **adopted into tmux** (`gtmux adopt <session_id>` resumes the
  conversation in a fresh tmux session). See `openspec/changes/native-agent-sessions`
  + memory `native-agent-sessions`. Still OUT of scope: a live screen/preview or
  in-place input for native sessions, and detecting agents that install **no** gtmux
  hook (no hook = no signal). The per-terminal tab-title scanner (needed for native
  *jump*) remains deferred — `source/terminal/tab` + `focus --terminal/--tab` are
  latent groundwork for it.
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
