# Troubleshooting & footguns (living checklist)

Pitfalls we've actually hit during **development, debugging, and release** — with
the check that would have caught each one early. This is a **living document**:
when a new footgun costs real time, add an entry here (symptom → root cause → the
must-check / rule), so the next person (or Claude) trips a checklist instead of the
rake. Keep entries short and action-first.

> Related runbooks live next to their subsystem: remote-access / pairing debug in
> `docs/design/remote-access-tunnel.md`; deploy paths in `CLAUDE.md` (Deploy table).

---

## 菜单栏切不到 Anywhere：GUI 进程的 PATH 没有 Homebrew 前缀

**症状** —— 菜单栏偏好设置里点「任意网络」，确认弹窗出现，点 Enable 后**弹窗直接消失、开关弹回、
屏幕上什么都没有**。同一条命令在终端里跑（`gtmux tunnel --service --yes`）**完全成功**。

**根因（两条，缺一条都还不够解释）**

1. **GUI 进程的 PATH 不是你的 PATH。** 从 Finder/LaunchServices 启动的 app 继承 launchd 的
   `PATH=/usr/bin:/bin:/usr/sbin:/sbin` —— **两个 Homebrew 前缀都不在上面**。`cloudflared` 在
   `/usr/local/bin`，于是 `exec.LookPath` 报「没装」，CLI 接着说「也没装 Homebrew 来帮你装」——
   两句都是假的，两个东西一直都在。`internal/tmux` 早就踩过这个坑并为 tmux 硬编码了兜底路径，
   但 cloudflared / brew 从来没享受到同一课。
2. **失败被吞掉。** `RemoteAccess.run()` 一直有 `lastError`，配对面板一直在显示它，**但偏好设置
   那一栏从来没渲染过**。所以失败的表现就是「弹窗消失，什么都没发生」。

**复现（不需要真的弄坏环境）**
```sh
env -i HOME="$HOME" PATH="/usr/bin:/bin:/usr/sbin:/sbin" gtmux tunnel --service --yes
# → cloudflared isn't installed … Homebrew isn't installed to fetch it
```

**必查**
- 调试任何「app 里不行、终端里行」的问题，**先用上面那行 `env -i` 复现**。这是这一类 bug 的
  分水岭，不先做这一步会往错误的方向查很久（网络？token？权限？）。
- 新增一个 gtmux 要 shell 出去调用的工具时，用 `lookTool()`（`internal/app/toolpath.go`）而不是
  `exec.LookPath`。
- 新增一个能失败的控件时，**在它自己所在的界面**渲染错误。别指望用户去别的面板找原因。


## `gtmux update` 在 Apple Silicon 上装了 x86 版（并且会自我延续）

**症状** —— M 系列 Mac 上，`gtmux update` 打印 `[1/5] Host darwin-amd64`，`~/.local/bin/gtmux`
落成纯 x86 二进制。装完之后**每次再更新还是 amd64**，而且 `file` 一看就是 x86_64。

**根因** —— `install.sh` 用 `uname -m` 判断架构，而 **`uname -m` 报的是「当前进程」的架构，
不是这台机器的**。在 Rosetta 下它在 Apple Silicon 上返回 `x86_64`。所以：

- 从一个被翻译的 shell 跑安装（`sysctl -n sysctl.proc_translated` = 1）→ 拿 amd64 包；
- **装完的 x86 gtmux 自己就是翻译着跑的**，它再调 `gtmux update` → 又看到 x86_64 → **闭环，永远
  出不来**。这就是为什么"装一次不对，以后次次不对"。

**判据** —— `sysctl -n sysctl.proc_translated` 返回 `1` 就说明「你在被翻译，硬件是 arm64」。
`install.sh` 现在据此纠正 `uname -m`。

**必查**
- 怀疑架构问题时，**别信 `uname -m`**，先 `sysctl -n sysctl.proc_translated`。
- `file -b ~/.local/bin/gtmux` 应该是 `arm64`（或 universal），不该是纯 `x86_64`。

## 安装布局：哪个 gtmux 是权威的

**权威 CLI = `~/.local/bin/gtmux`**，一个**真实二进制**（不是软链）。`install.sh` / `gtmux update`
就是往这里原子替换的（`mv -f`），所以**把它做成软链没有意义——下次更新会把软链直接覆盖成文件**。

- `~/Applications/Gtmux.app/Contents/MacOS/gtmux` —— app **自带的私有副本**，与 app 版本绑定。
  两个 LaunchAgent（serve / selftunnel）用**绝对路径**指向它，所以清理 PATH 上的副本不会动到服务。
  **不要**让任何东西软链到它：app 可以被替换或删掉。
- `/usr/local/bin/gtmux` —— **不该存在**。那是 Homebrew cask 的地盘（早期 0.9.3 cask 的遗留）。
- `~/.tmux.conf` 里的 `bind g/a/J` 硬编码 `~/.local/bin/gtmux` —— **正确**，因为那正是权威路径。


## Release / git-ops

### Never inline backtick-containing prose into a shell-substituted string
**Symptom:** `gh pr create` / `git commit` prints `foo: command not found`, the PR
body comes out mangled, and — worse — a random process (once a rogue `gtmux serve`)
is now running and squatting a port.
**Root cause:** backticks and `$(…)` inside a **double-quoted** string are command
substitution. Wrapping a heredoc as `--body "$(cat <<'EOF' … EOF)"` re-enables that
substitution around the heredoc, so any `` `word` `` in the markdown body (we fence
identifiers like `` `gtmux serve` `` constantly) gets **executed as a command**. A
`<<'EOF'` quoted delimiter protects the heredoc *body* but not the `"$(…)"` you wrap
it in.
**Rules:**
- Write PR/issue/commit bodies to a **file**, then `gh pr create --body-file <path>`
  / `git commit -F <path>`. Never `--body "$(…)"` or `-m "$(…)"` on text with backticks.
- After any PR-create that warned or errored, run
  `ps aux | grep -E 'gtmux serve|<cmds you backticked>'` and kill stray processes.

### A code change isn't shipped until the right delivery path runs
Four artifacts, **three** paths (git tag ≠ device build ≠ `wrangler deploy`). Editing
`relay-worker/` or `tunnel-worker/` and merging changes **nothing live** until you
redeploy the Worker. See the Deploy table in `CLAUDE.md` and
[[relay-redeploy-footgun]]. Quick check when push behaves oddly:
`cd relay-worker && npx wrangler deployments list` vs. `git log -1 -- relay-worker/`.

### Release tag gate
Tagging `vX.Y.Z` runs the **full `make check`** (not a weaker `go test`), then
goreleaser + the macOS app build. CI can't see the menu bar — smoke-test the app on
real macOS before trusting a tag.

### Menu-bar "click to update" loops — the app reinstalls its OWN version
**Symptom:** the popover shows `New version X — click to update`; clicking it "finishes"
(no error), the app relaunches, and the SAME banner reappears. The CLI + app both stay
on the old version. `~/…/T/gtmux-update.log` shows `Release v<OLD>` / `Installed gtmux
v<OLD>` even though Go logged `Updating <OLD> → <NEW>`. Running `gtmux update` **by hand
in a normal shell works** (installs `<NEW>`).
**Root cause:** `install.sh`'s `open -n "…/Gtmux.app"` used to launch the app with the
installer's env still set, **leaking `GTMUX_VERSION=<OLD>` into the long-lived app
process**. The in-menu update runs `gtmux update`, which inherits that pin; Go honors a
pre-set `GTMUX_VERSION` (`if !LookupEnv(...)`) instead of resolving the latest, so
install.sh reinstalls `<OLD>` — forever. A manual shell has no `GTMUX_VERSION`, so it
resolves `<NEW>` and works. (After a re-login the login LaunchAgent starts the app with
a clean env, which is why a reboot "fixes" it.)
**Fix:** `install.sh` now strips it (`env -u GTMUX_VERSION open -n …`) so the app never
inherits the pin, and `Updater.spawnDetachedUpdate` runs `env -u GTMUX_VERSION gtmux
update` as a belt. **Diagnose** with `ps eww <GtmuxBar-pid> | tr ' ' '\n' | grep GTMUX_`
— a `GTMUX_VERSION=` there is the smell. **Unstick a machine now:** `gtmux update` from
a plain terminal, or just click update twice (the first click relaunches with a clean
env via the fixed install.sh).

---

### `gtmux doctor --fix` / `gtmux update` hangs right after "menu-bar app launched"
**Symptom:** the app-install step finishes (`[5/5] Menu bar … ✓`, "menu-bar app launched",
the PATH hint all print), then the command NEVER returns to the prompt — no "Restarted
the remote serve" / "Done". The app IS installed and running; only the command is stuck.
**Root cause:** `runInstaller` ends with `restartServeAgents()`, which ran
`launchctl kickstart -k gui/<uid>/com.gtmux.serve` UNBOUNDED. On some Macs that
`kickstart -k` blocks indefinitely, freezing the synchronous `doctor --fix` / `update`
forever. install.sh itself already completed (its final line printed) — the hang is the
best-effort serve-restart, not the install.
**Fix:** every `launchctl` call in `restartServeAgents` is now hard-bounded by a 6s
timeout (`runBounded`); on timeout it skips the restart (the serve refreshes on next
login) instead of hanging. **Unstick a machine now:** press **Ctrl-C** — the app is
already installed; only the trailing restart stalled. (Needs a release to reach an old
`gtmux`.)

---

### `brew upgrade --cask gtmux-app` fails: "App source '/Applications/Gtmux.app' is not there"
**Symptom:** `brew install/upgrade --cask chenchaoyi/tap/gtmux-app` downloads + verifies
the zip, then errors `It seems the App source '/Applications/Gtmux.app' is not there.`
(often on a machine that previously ran `gtmux update`).
**Root cause:** the app has **two install channels that targeted different dirs** — the
Homebrew cask installs to `/Applications/Gtmux.app`, but `install.sh` / `gtmux update`
installed to `~/Applications/Gtmux.app`. If a user did both, `/Applications/Gtmux.app`
goes missing (only the `~/Applications` copy is current), and Homebrew's cask uninstall
step can't find the app it recorded at `/Applications` → the error. NOT a bad zip or
cask stanza (`ditto --keepParent` + `app "Gtmux.app"` are correct).
**Fix:** `install.sh` now **co-locates** — if `/Applications/Gtmux.app` exists (a cask
install) and `~/Applications/Gtmux.app` doesn't, it updates the `/Applications` bundle
in place instead of making a second copy, so the two channels stay on one app.
**Unstick a machine now:** `brew uninstall --cask gtmux-app --force` (forgets the broken
state) then `brew install --cask chenchaoyi/tap/gtmux-app` — or just switch to the curl
installer: `curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | bash`.
(The separate deprecation *warning* `depends_on macos: ">= :ventura"` is cosmetic; the
cask generator now emits `depends_on macos: :ventura`.)

---

## Remote access / pairing / push

### Menu-bar Off / Wi-Fi picker "won't change" from Anywhere — on the Direct backend
**Symptom:** on `Anywhere`, tapping `Off` or `Wi-Fi` in the menu-bar Remote-access picker
snaps straight back to `Anywhere`. Reproduces only when the tunnel backend is **Direct**
(self-hosted); on Standard/Cloudflare the picker works.
**Root cause:** the picker's mode is DERIVED from which LaunchAgents exist
(`groundTruth()`: `cfOn || selfOn ? .anywhere : …`). `serviceRemoveAll()` (Off) and
`serveServiceInstall()` (Wi-Fi) tore down `com.gtmux.serve` + `com.gtmux.tunnel`
(Cloudflare) but **skipped `com.gtmux.selftunnel`** (the Direct agent) — so on Direct it
stayed loaded, `selfOn` stayed true, and the mode re-derived to `.anywhere`.
**Fix:** both teardown paths now remove ALL three agents (serve + tunnel +
**selftunnel**), matching `tunnelServiceRemove` (`gtmux tunnel --unservice`). Pinned by
`TestServiceRemoveAllDropsSelfTunnel`.

### "Pairing code expired" that never clears — check for a DUPLICATE serve on :8765
**Symptom:** menubar "refresh code" → phone scans → *invalid or expired enroll code*,
no matter how fresh the code, across app reinstalls and `gtmux update`.
**Root cause:** two `gtmux serve` processes on 8765. The menubar mints via
`POST 127.0.0.1:8765` (IPv4 → serve A); the tunnel ingress `http://localhost:8765`
resolves to `::1` (IPv6 → serve B). **Enroll codes are in-memory per process**, so a
code minted on A is absent on B → "expired". (The same split corrupts push-token state.)
**Must-check (run this FIRST when pairing/push misbehaves):**
```
lsof -nP -iTCP:8765 -sTCP:LISTEN     # MUST show exactly one PID
ps aux | grep 'gtmux serve' | grep -v grep
```
Expect ONE serve — the app's `com.gtmux.serve` LaunchAgent
(`/…/Gtmux.app/Contents/MacOS/gtmux serve --bind 127.0.0.1 --port 8765`). Any second
`gtmux serve` (especially bare, binding `*:8765`) is a squatter → kill it. With only
`127.0.0.1` listening, cloudflared's `localhost` falls back to IPv4 and hits the same
serve the menubar mints on.

### Don't restart `gtmux serve` between mint and scan
Enroll codes (TTL 5 min) live only in memory; a serve restart (incl.
`launchctl kickstart`, and the `launchctl unload/load` that `gtmux tunnel --service`
does) wipes every pending code → a just-minted QR reads as "expired". Mint → scan
without bouncing serve in between.

### Tunnel silently offline on a corp network — QUIC is blocked
**Symptom:** phone gets Cloudflare **1033 / HTTP 530**; `tunnel.log` loops
`failed to dial to edge with quic: timeout` / `no free edge addresses left to resolve to`.
**Root cause:** cloudflared defaults to QUIC (UDP/7844); many corp/campus nets block it.
**Fix:** `--protocol http2` (TCP/443) — now the gtmux default for all cloudflared
launch paths (override with `GTMUX_TUNNEL_PROTOCOL`). An **old** service plist keeps
QUIC, so after `gtmux update` re-run `gtmux tunnel --service` to regenerate it.
Diagnose with `tail ~/.local/share/gtmux/tunnel.log`. See
`docs/design/remote-access-tunnel.md`.

### Corp-DNS hijack ≠ dead tunnel
The office net rewrites brand-new `ccy.dev` answers to internal `172.19.x` IPs, so the
Mac's own reachability probe fails on a *healthy* tunnel (returns HTTP 530). Verify the
last hop from a **phone on cellular**, not from the office LAN. `api.cloudflare.com` is
also intermittently TLS-reset here — retry `wrangler`.

### The app classifies enroll failures — read the phone's message
Since the enroll-error split, the phone names the failure class: *can't reach* /
*tunnel offline* / *code expired* / *no token*. Use that to jump straight to the right
section above instead of guessing.

---

## HQ attention system / perception feed

### `gtmux hq` "Focused the running supervisor" but the HQ session is dead
**Symptom:** you quit the HQ agent but left its tmux window open (a bare shell). Later
`gtmux hq` says "Focused the running supervisor" and jumps to that window — which holds
only a shell prompt, no agent. Confusing.
**Root cause:** `findHQPane()` detects HQ by a pane STAMP that survives the agent
exiting, so `gtmux hq` treated a stamped-but-dead pane as "running" and focused it.
**Fix:** `gtmux hq` now checks the pane's foreground command (`hqAgentAlive` →
`pane_current_command`): a shell means the agent exited, so it RELAUNCHES the agent in
that same pane instead of focusing a dead prompt (`agentAliveByCmd`, pinned by
`TestAgentAliveByCmd`).

### A dispatched worker shows `done` in `gtmux tasks` but never ran
**Symptom:** you `gtmux spawn` a task; `gtmux tasks` (and HQ/the digest) show it `done`,
but the worker's tmux pane is actually sitting at the "Do you trust the files in this
folder?" startup gate (or holds the goal UNSUBMITTED in the composer — a long paste
swallowed the Enter). Not one step ran.
**Root cause:** `waiting` (needs-you) was HOOK-marker-driven ONLY. The startup gate and
an unsubmitted composer fire NO gtmux hook, so the radar read the pane `idle`, and
`taskStatusFor("idle")` mapped idle → `done` unconditionally — no `waiting` wake either.
**Fix (v0.28.9, stuck-dispatch-waiting):** a narrow screen-content guard — for a TRACKED
dispatch whose capture shows a startup/permission gate (`prompt.IsStartupGate`, per-agent)
or a structured non-empty draft (`dispatch.DraftOf`) — reclassifies it `waiting` (kind
`startup`/`draft`), never `done`. The serve slow-tick writes the marker + fires a
`waiting` wake so HQ unblocks it; `wakeDone` also skips `done` when the post-Stop screen
is a gate/draft. All other waiting stays hook-driven. **Unstick now:** answer the gate /
press Enter in the pane.

### HQ's startup briefing typed into the input box but never sent
**Symptom:** `gtmux hq` starts the agent, a long "Startup briefing — make this your very
first output…" prompt sits in the input box UNSENT, and HQ stalls waiting.
**Root cause:** the briefing used to be a huge multi-line prompt PASTED into the pane and
submitted — fragile (a long paste + a single Enter can land as typed-but-not-submitted,
especially on a just-started agent) and Claude-Code-specific.
**Fix (v0.28.8, playbook v6):** the briefing CONTENT + format now live in the seeded
playbook (`AGENTS.md` "## First turn"), read by any agent via its own convention file;
gtmux injects only a MINIMAL one-line trigger — `» gtmux·startup` — which submits
reliably and is agent-agnostic. (Unstick a stalled one: just press Enter in that pane.)

### `feed-degraded` in HQ — the perception feed is down
**Symptom:** HQ surfaces `⚠ perception feed down — on the 5-min polling backstop`, or a
`[CRITICAL gtmux:feed-degraded]` line appears in `gtmux hq-feed --tail`.
**Root cause:** the `gtmux hq-feed` daemon died and mechanical self-heal failed twice
(the no-LLM watchdog lives in the `gtmux serve` slow-tick — if serve is OFF, nothing
restarts it automatically).
**Must-check / fix:** `gtmux hq-feed --status` (running? heartbeat age ≤ 90s? cursor lag?).
If down, `gtmux hq-feed --daemon &` restarts it (singleton-guarded), or just re-attach
HQ's `gtmux hq-feed --tail` — the tail auto-starts the daemon. Confirm `gtmux serve` is
running so the watchdog can supervise it going forward. Files:
`~/.local/share/gtmux/hq-feed/{pid,cursor,heartbeat,spool.jsonl}`.

### HQ went quiet — is it the feed or the surfacing threshold?
**Symptom:** HQ stopped printing routine updates.
**Root cause:** by design. The feed is SILENT (gtmux no longer types low-value receipt
nudges into the pane); HQ only PRINTS CRITICAL/NORMAL and ledger-records QUIET. Quiet
mode raises the bar to CRITICAL-only.
**Must-check:** `gtmux quiet status` (the resolved threshold). QUIET items are in
`gtmux tasks --verbose`, not lost. A `feed-degraded` CRITICAL is never quieted, so
silence there means the feed is healthy, not broken.

### Seed is generated ONCE — a live HQ home won't auto-update
The attention-system behavior lives in the HQ playbook (`hq.go` `hqInstructions` →
`~/.config/gtmux/hq/AGENTS.md`), which is seeded once and never overwritten. A FRESH hq
home gets it automatically; the commander's EXISTING HQ needs a deliberate re-seed
(back up and remove/replace AGENTS.md, then `gtmux hq`) to pick up the feed/threshold/
self-check instructions.

---

## Driving a pane (dispatch / `gtmux send`)

### One instruction pasted 2–3× and submitted in pieces
**Symptom:** a dispatched message appears in the agent's box twice or three times, is
submitted line by line (the tail lines land as "queued messages"), the Enter looks
swallowed and needs a manual re-press — and `gtmux send` still reports `NOT delivered`.
**Root cause:** two, and they compound.
1. `paste-buffer` was **not bracketed** (`-p`), so the payload went in raw and every
   `\n` reached the TUI as a bare Return — submitting each line as its own message.
2. The fragment retry called `ClearDraft` (C-u) and re-pasted **without checking the
   clear worked**. C-u kills only the line the cursor is on; against a multi-line draft
   a second C-u (and Escape) do nothing at all. So the retry pasted onto the leftover
   and concatenated a copy — `PasteRetries: 2` → up to three copies.
**Rules:**
- Any tmux paste into an agent TUI is `paste-buffer -p`. Test with a **multi-line**
  payload — single-line text hides both bugs completely.
- Never re-paste into a box you have not SEEN go empty. Clearing a draft is not
  reliable; failing loudly with evidence beats duplicating an instruction.
- The frame right after a paste is not evidence — the TUI redraws on its own schedule.
  Let a paste settle before judging it a fragment (a stale frame read as a fragment is
  what triggered the destructive retry).
**Must-check:** reproduce against a real agent pane, not a fake — `tmux new-session -d
-s lab; tmux send-keys -t lab claude Enter`, then send a 3-line instruction and read
the box. A unit test with single-line fixtures passes either way.

---

## Disk / storage

### gtmux state dir balloons to GB (disk red line)
**Symptom:** `~/.local/share/gtmux` grows to hundreds of MB or GB; a disk-space alarm
fires. `gtmux doctor`'s `Storage` row shows red (`✗ very large`).
**Root cause:** it is almost never the event log — `events.jsonl` (20 MB) and the HQ
spool (8 MB) already self-rotate. The culprit is an **unrotated launchd log**:
`serve.log` / `tunnel.log` / `selftunnel.log` / `restore.log` are plain
`StandardOutPath`/`StandardErrorPath` redirects launchd never rotates, and the gtmux
process can't `SetOutput` a redirect it doesn't own. A chatty daemon — classically
`cloudflared` retrying forever against a **QUIC-blocked** corp network — writes with no
ceiling. Secondary: the `uploads/` dir (phone images) and the per-pane churn markers
(`frame/`, `cpu/`, `goalchanged/`, `sends/`) that never cleaned up a dead pane's leftover.
**Fix / must-check:**
- `du -ah ~/.local/share/gtmux | sort -rh | head` — find the big file. A multi-hundred-MB
  `tunnel.log` confirms cloudflared churn (check the tunnel is actually up; see the
  QUIC-blocked entry).
- The slow-tick hygiene sweep (`internal/hq/diskhygiene.go` `diskHygieneSweep`) caps each
  log to its recent tail (8 MB → last 2 MB), age-prunes + LRU-trims `uploads/`, and ages
  out dead-pane churn markers, every 30 min while `gtmux serve` runs. If serve isn't
  running, nothing trims — start it, or manually `: > ~/.local/share/gtmux/tunnel.log`.
- `events.seq` is a single monotonic integer — never delete it to reclaim space; a reset
  would break every consumer's durable cursor.
