# 多 agent + 多终端 —— 设计（2026-06-18）

## 今天如何阅读这份设计（2026-10-06 补记）

这是 6 月 18 日的计划，驱动表后来有过更新；它不是完整的现行能力表，也不意味着要重新执行下面的实现顺序。

- 原生会话已不再全部推迟：`internal/native` 和雷达能记录 hook 感知到的 tmux 外会话，
  `gtmux adopt` 提供转入路径。原生行本身仍没有可查看或输入的 tmux pane。
  资格与归属边界见[现行原生会话要求](../../openspec/specs/native-agent-sessions/spec.md)；
  有要求不代表每条失败路径都已经验收。
- 实际的 [`Terminal` 接口](../../internal/terminal/terminal.go)比下方草图多了返回值、
  `dryRun` 参数和 `TabOrder`；已注册 Ghostty、cmux、iTerm2、Warp。这是承载 tmux 会话的
  **宿主终端**接口，不是 cmux/Zellij 多路复用器适配器。当前解析最终回退到 Ghostty，
  不要依赖计划中的「上次已知值」缓存或所有不支持宿主都会报出的统一提示。
  Warp 有记录的 tab UUID 时用它聚焦，否则激活 app，并非都按下方草图匹配标题。
- 多 agent hook 与类型化事件分类器已存在。当前入口是
  [`gtmux install hooks --agent …`](../cli.zh.md)，新增接入见[onboarding 指南](agent-onboarding.zh.md)；
  `install-hooks` 旧拼写仍作为别名保留。Codex 安装写 `hooks.json` 和 `config.toml` 的
  `features.hooks`（设置了 `CODEX_HOME` 就使用该目录），保留已有 `notify`；
  下方早期的 `notify` 示例不是当前安装步骤。
- working 判定已不只看标题加载动画：hook 状态和雷达的画面/CPU 证据也参与
  （`internal/radar/agents.go`）。环境 doctor 已实现，C 节是当时的提案。

以下原计划和后来驱动表补记保持不变。文中点名的实测只指当时的检查，不是本轮重新验证了所有终端和 agent。


维护者在 2026-06-18 决定纳入范围的两项工作的设计。实现前先审阅；每节末尾有排好序的计划。

## 范围（已定）

- ✅ **多 agent** —— 呈现并操作 Claude Code 之外的 agent（Codex，…）。
- ✅ **多终端宿主** —— tmux 跑在 **非 Ghostty** 终端下（iTerm2 / kitty / WezTerm /
  Apple Terminal）。
- ❌ **原生（非 tmux）agent** —— 仍然**推迟**。这是另一个问题，别混为一谈。
  `source/project/terminal/tab` 字段保留为潜在的地基。

贯穿始终的不变量：**检测已经与终端无关，并且（自 PR #32 起）由进程驱动。**
只有「远程」一侧和「需要输入/通知」一侧还带着 Claude/Ghostty 的假设，这次就是要把它们泛化。

---

## A. 终端驱动（多终端宿主）

### 问题
`internal/ghostty` 把四个「远程」操作硬编码成 Ghostty AppleScript：`FocusTab`、
`IsViewing`、`OpenWindow`、`SpawnTabs`（由 `focus` / `restore` / `new` 使用）。
`agents` / `overview` 已经与终端无关，那里不用动。

### 接口
```go
// internal/terminal
type Terminal interface {
    Name() string
    FocusTab(session string) error    // bring the tab titled "<session> — …" to front
    IsViewing(session string) bool     // is that tab the frontmost/active one
    OpenWindow(command string) error   // new terminal window running command
    SpawnTabs(sessions []string) error // restore: one tab per session
}
```
`internal/ghostty` 成为第一个实现（行为不变）。一个注册表 + `Resolve()` 返回当前驱动。

### 宿主检测（关键）
`focus`/`restore` 由**菜单栏 app 或 tmux 快捷键**调起，它们没有 `$TERM_PROGRAM`。
用 **tmux 客户端的进程祖先链**解析宿主终端（早先干净地区分原生与 tmux 用的就是这一招）：
`tmux list-clients -F '#{client_pid}'` → 沿父进程链向上 → 终端 app 的 bundle
（`…/Ghostty.app/…`、`…/iTerm.app/…`、`kitty`、`wezterm-gui`，…）。
- 结果缓存（未命中时重新检测）。
- 覆盖项：`GTMUX_TERMINAL=ghostty|iterm2|warp|…`；命令*确实*在终端里运行时也认 `$TERM_PROGRAM`。
- 没有客户端连着（完全 detached）：回退到上次已知 / 配置的值。

### 「按标题找 tab」
每个驱动都靠 **标题 `#S — #W`** 定位会话所在的 tab，标题由 tmux `set-titles` 写入。
Ghostty 的 `focus` 本来就要求这个；现在它成为所有终端的硬前提（`doctor` 必须校验）。

### 驱动与可行性
| 终端 | focus / spawn 机制 | 状态 |
|---|---|---|
| **Ghostty** | AppleScript（现有） | 驱动 #1，行为不变 |
| **cmux** | AppleScript 读取面板标题、聚焦及新建工作区 | ✅ 已接入；通过 `CMUX_WORKSPACE_ID` 或 `cmux.app` 进程链识别，优先于内嵌 Ghostty。CLI socket 可能拒绝外部进程，因此菜单栏的 restore/new 使用脚本接口。 |
| **iTerm2** | AppleScript（丰富的 tab/session API） | ✅ 已发布驱动 —— 完整矩阵在 3.6.11 上实机验证（#718） |
| **Apple Terminal** | AppleScript | ✅ 可行（暂无驱动 —— 仅感知） |
| **kitty** | `kitty @ ls`（JSON tab+标题）+ `kitty @ focus-tab` | ✅ 可行，需要 `allow_remote_control`（暂无驱动 —— 仅感知） |
| **WezTerm** | `wezterm cli list` + `wezterm cli activate-tab` | ✅ 可行（暂无驱动 —— 仅感知） |
| **Warp** | 没有 AppleScript 字典 —— 但有 `warp://` URI（`action/new_tab`、按 tab 聚焦的 `session/<uuid>`）+ 启动配置（`warp://launch/<name>`，每个 tab 执行一条命令） | ⚠️ 尽力而为的驱动（已发布）：只有当该 tab 的 `$WARP_TERMINAL_SESSION_UUID` 被记进 tmux 会话环境时才能精确聚焦（gtmux 自己的 attach 脚本会记；现代 macOS 上读不到其他进程的环境），否则只激活 app；IsViewing 认 Warp 的真实进程名 `stable` |
| **Alacritty** | 没有 tab / 没有脚本接口 | ❌ 不支持 |

不支持的宿主 → 平滑降级：agent 列表/状态照常（那部分与终端无关）；
`focus`/`restore`/`new` 明确打印「跳转需要受支持的终端」，而不是静默失败。

### 顺序
- **A1** —— 抽出 `Terminal` 接口 + 把 Ghostty 挪进去（纯重构，行为不变；测试断言输出完全一致）。*从这里开始。*
- **A2** —— 宿主检测（进程祖先链 + 覆盖项）。
- **A3** —— iTerm2 驱动（最接近 Ghostty，价值最高）。
- **A4** —— kitty + WezTerm + Apple Terminal。

---

## B. 多 agent（需要输入 + 通知）

### 检测 —— 已完成（PR #32）
进程树 argv 检测（`agentInSubtree` / `agentFromCommand`）能抓到以 `node …/bin/codex` 形式运行、
标题没有任何符号的 agent。**已知毛边：**进程树检测到的 agent **干活时也显示 idle**
（「working」目前只认标题里的加载动画）—— 在 B3 修。

### 需要输入 + 通知 —— 泛化只认 Claude 的 hook
现在 `gtmux hook` 接的是 Claude 的 `~/.claude/settings.json` 和它的
`Stop`/`Notification`/`UserPromptSubmit` 事件；⏸ waiting、✓ latest 和通知都靠它。
其他 agent 只有 working/idle。

**发现（已验证）：Codex 有 hook 机制。** `~/.codex/config.toml`：
```toml
notify = ["<program>", "turn-ended"]    # runs <program> on events
```
另有更丰富的 `hooks` 系统（参见 `codex --help: --dangerously-bypass-hook-trust`）。
所以 Codex 可以像 Claude 一样给 gtmux 喂事件。

### 通用 hook 契约
- `gtmux hook` 已经按 `$TMUX_PANE` 键存状态并读取一个事件。泛化为：
  `gtmux hook --agent <name>` + 每个 agent 一张**事件名 → 语义**映射表
  （`{turn-start, finished, needs-input}`）。Claude 的 `UserPromptSubmit/Stop/Notification`
  是一张；Codex 的 `turn-ended`（+ 任何审批事件）是另一张。
- `gtmux install-hooks --agent claude|codex|…` —— 按 agent 安装：
  - **claude** → `~/.claude/settings.json`（现有路径）。
  - **codex** → `~/.codex/config.toml` 的 `notify`/`hooks` 指向 `gtmux hook --agent codex`。
    **串接、不要覆盖**用户已有的 `notify`（配置里已经有一条给 computer-use 用的）。
  - 幂等 + 有备份，与 settings.json 的编辑一样。
- 裸的 `gtmux install-hooks` 为所有检测到配置的 agent 安装，并报告哪些 agent 拿到了
  需要输入/通知，哪些只有检测。

### working/idle 准确性（B3）
每个 agent 一套 working 信号：观察各 agent 的 **working 标题**（Claude 会播 braille 加载动画；
Codex 的 working 标题待定 —— 得跑起来看）。在信号明确之前，进程检测到的 agent 在标题没有
加载动画时保持「idle」。

### 顺序
- **B1** —— 泛化 `gtmux hook`（事件映射）+ `install-hooks --agent`；Claude 的行为一字不变。
- **B2** —— Codex 接入（config.toml 的 `notify`/`hooks`，串接）、事件映射、
  `install-hooks --agent codex`。
- **B3** —— 每个 agent 的 working 标题信号（修「永远 idle」的毛边）。

---

## C. 环境 doctor / 安装（稍后）
A/B 落地后，做一个 `gtmux doctor`：检查 tmux + resurrect/continuum + `set-titles` +
**宿主终端**（A）+ **各 agent 的 hook**（B）；提供经同意、有备份的修复（托管的 `tmux.conf`
块、按 agent 的 `install-hooks`）。把维护者的 `ccy-ai-workspace/terminal` 安装器产品化。
检测 → 建议 → 确认后修复；绝不静默。

---

## 建议的总体顺序
A1（重构，低风险地基）→ B1（hook 泛化，Claude 不变）→ A2+A3（宿主检测 + iTerm2）→
B2（Codex hook）→ A4 / B3 → C（doctor）。
每项单独一个 PR、过 CI；保持行为的重构先行。
