# gtmux

**tmux 会话与 coding agent 的指挥台。**

[English](README.md)

`gtmux` 是一个小巧的 Go 命令行,覆盖在你已经在跑的 tmux 会话、以及里面运行的
coding agent(Claude Code、Codex、Gemini、aider……)之上。它告诉你谁在**等你**、
谁在**运行**、谁**空闲**,并一键跳到确切的 Ghostty tab 和 tmux pane。

**它和别的不一样**:claude-squad、uzi、dmux 这类是 agent **spawner**——替你在
git worktree 里生成、隔离 agent;**gtmux 不替你跑 agent**,它是**覆盖你已经在
tmux 里跑的一切的"雷达 + 遥控器"**。非侵入、tmux 原生,连别的工具生成的 agent
也能照看到(它们也在 tmux 里)。名字里的 "g" 取自 Go。

> macOS + [Ghostty](https://ghostty.org) 1.3+。`restore`/`focus` 通过 AppleScript
> 驱动 Ghostty;`agents`/`overview` 在任意 tmux 上都能用。

## 安装

```sh
curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | bash
```

会把经校验和验证的二进制装到 `~/.local/bin/gtmux`。锁定版本:

```sh
GTMUX_VERSION=v0.1.0 curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | bash
```

### 国内 / GitHub 不稳定

安装器**优先走 GitHub,失败时自动回退到镜像链**
(`ghfast.top` → `gh-proxy.com` → `ghproxy.net`)。`SHASUMS256.txt` 始终先尝试
GitHub 直连——所以即使 tarball 走了镜像,用来校验它的校验和依然锚定在 GitHub。
用 `GTMUX_INSTALL_MIRROR` 覆盖:

```sh
# 直接走镜像链(你已知 GitHub 不通)
GTMUX_INSTALL_MIRROR=ghproxy curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | bash

# 自定义 <前缀><github-url> 代理
GTMUX_INSTALL_MIRROR=https://my.mirror/ curl -fsSL ... | bash

# 只走 GitHub,不用镜像
GTMUX_INSTALL_MIRROR=github curl -fsSL ... | bash
```

### 从源码安装

```sh
go install github.com/chenchaoyi/gtmux/cmd/gtmux@latest
```

## 用法

```
gtmux <命令> [选项]            # 不带参数直接敲 gtmux 显示帮助
```

| 命令 | 作用 |
| --- | --- |
| `overview [--popup]` | session / window / pane 汇总;`--popup` 适配 tmux 弹窗 |
| `agents [--watch\|--json]` | 各 pane 里的 coding agent:⏸ 等输入 / ⠿ 运行中 / ✳ 空闲、在哪、可跳转的 pane id。`--watch` 是实时面板;`--json` 是结构化输出 |
| `restore [--pick\|--one\|<名字>\|--dry-run]` | 每个 session 一个 Ghostty tab,全部接回 |
| `focus <名字\|pane-id>` | 跳到该 session 的 Ghostty tab;pane id(`%N`)精确落到那个 pane |

不带参数直接敲 `gtmux` 显示帮助;`gtmux --version` 看版本。输出语言由 `--lang=en|zh`
(默认 `en`)或 `$GTMUX_LANG` 控制。显式调用、不挂任何 shell 钩子,换什么 shell 都能用。

## `gtmux agents`

```
gtmux agents — 6 agents · 1 waiting · 1 working · 4 idle

⏸ waiting  Claude Code  Pica:0.0          等你批准跑测试            %7
⠿ working  Claude Code  ccy-workspace:0.0 Auto-attach tmux sessions %11
✳ idle     Claude Code  Rodi:0.0          Rodi feature dev   %8  ✓ latest
✳ idle     Claude Code  Diting:0.0        —                  %1

jump: gtmux focus <pane>   (例如 gtmux focus %11)
```

一处看清谁在跑、谁空闲、谁刚完成。每行:**状态**、**agent**、位置、任务、**pane id**
—— 按紧急度排序(等输入 → 运行中 → 空闲),表头给出分类统计。三种状态:

- **⠿ 运行中** —— 忙(别打扰)
- **⏸ 等输入** —— 任务进行中,卡在等**你**批准/授权;排在最前,一眼看到谁需要你决策
- **✳ 空闲** —— 完成一轮,你方便时再处理(不紧急)

**`gtmux agents --watch`** 是实时自动刷新面板(基于
[bubbletea](https://github.com/charmbracelet/bubbletea)):约 1.5s 轮询,**↑/↓**
选择,**Enter** 跳到该 pane,**r** 刷新,**q** 退出。观察期间从运行中→空闲的 agent 会
标 `✓ done`。**`--json`** 输出同样数据的结构化数组,给脚本/菜单栏 app 用。

**识别不只针对 Claude:**
- **状态**来自 agent 自己设置的 pane 标题。开头的盲文 spinner(`⠋⠙⠹…`,多数 agent TUI
  都在动这个)= **运行中**;Claude Code 的 `✳` = **空闲**。对会动 spinner 的 agent 通用。
- **哪个 agent**:按前台命令(`claude`、`codex`、`gemini`、`aider`、`opencode`…)或标题里
  的名字匹配。
- 用 **`~/.config/gtmux/agents.json`** 扩展/覆盖 —— 一个 `{"name","commands","idleGlyph"}`
  数组,你的条目优先于内置。
- 只有 agent **进程真的在跑**(前台命令是 agent,或标题在动 spinner)才会列出。普通 shell
  上残留的 agent 标题(如 tmux-resurrect 恢复但没重启 agent 的 session)**不**计入。

> `⏸ 等输入` 和 `✓ latest` 来自 `~/.local/share/gtmux/` 下的状态文件
> (`waiting/<pane>`、`last-finished`),由通知钩子写入 —— [参考实现](#通知钩子)是
> `claude-notify` 钩子,它靠 hook 事件**时机**(而非文案关键词)区分权限请求与空闲提醒。
> 没有该钩子时不会显示 `⏸`,其余功能照常。

## `gtmux restore`

退出 Ghostty 后 tmux server 和所有 session 都还活着 —— 只是 tab 没了。重开 Ghostty 后,
在任意 tab 里跑**一次**:

```sh
gtmux restore            # 每个 tmux session 一个 Ghostty tab,全部接回
```

它为每个 session 开一个 tab(经 Ghostty 1.3+ AppleScript)并全部接回;你运行命令的那个
tab 接回第一个 session。首次运行会弹自动化授权(「想要控制 Ghostty」)—— 点允许。tab 按
session 名顺序创建;原来的 tab↔session 对应关系没有记录,无法完全复现。按 tab 控制:

```sh
gtmux restore --pick     # 列出 session(含 window 和状态)再选:"1 3" / "1,3",
                         # 回车=全部待接回,q=取消
gtmux restore --one      # 把当前 tab 接回下一个无人连接的 session
gtmux restore <名字>      # 按名字把当前 tab 接回指定 session
gtmux restore --dry-run  # 只打印将要做什么,不实际执行
```

**电脑重启后** tmux server 本身也没了。`gtmux restore` 仍可用:它会启动 tmux 并等
[tmux-continuum](https://github.com/tmux-plugins/tmux-continuum) 恢复最近一次自动存档
—— session、window、各 pane 的目录、屏幕文本。**正在运行的程序不会自动重启**;每个 pane
回到原目录的 shell(如用 `claude --resume` 拉起 Claude Code)。需安装 tmux-resurrect/continuum。

## `gtmux overview`

```
gtmux overview — 2 sessions · 3 windows · 5 panes

▶ ccy-workspace        1 window · 1 pane
    0: ccy-workspace *  (1 pane)
● Pica                 2 windows · 4 panes
    0: editor  (1 pane)
    1: claude *  (3 panes)

▶ current  ● attached  ○ detached   * active  Z zoomed  • new output
```

任意 shell 里看 session/window/pane 汇总。**`gtmux overview --popup`** 适配 tmux
`display-popup` 的尺寸,可绑到一个键,在全屏程序之上浮出而不打断它(见
[tmux 集成](#tmux-集成))。

## `gtmux focus`

```sh
gtmux focus Pica         # 把显示 session "Pica" 的 Ghostty tab 切到前台
gtmux focus %11          # 跳到那个确切的 window+pane,再聚焦它所在的 tab
```

这是 tmux `set-titles` 的读取侧:因为每个 tab 标题是 `session — window`,`focus` 找到
匹配的 tab,跑 Ghostty 的 AppleScript `select tab` + `activate`。给 pane id(`%N`)时会
先在 session 内 `select-window` + `select-pane`,于是精确落到那个 pane —— 这也是通知点击
能把你带到刚完成的 agent 的原理。

> 需要 `set-titles on` 且 `set-titles-string '#S — #W'`(让 tab 标题保持 `focus` 匹配的
> 格式)。若有别的工具也写 tab 标题,请关掉它,让标题保持权威。

## 菜单栏 app

gtmux 有两副面孔、一个数据源:终端里的 **CLI**,以及常驻的**菜单栏 app**。这个 app 是
原生 macOS `LSUIElement` 状态栏图标(Swift / AppKit,在 [`macapp/`](macapp/))—— 你盯
coding agent 的「雷达」,一眼看出有多少在 **⏸ 等你 / ⠿ 运行中 / ✳ 空闲**,弹出面板可跳到任意一个。

curl 安装脚本会装好并启动它(`GTMUX_NO_APP=1` 跳过,`GTMUX_APP_LOGIN=1` 开机自启)。源码
构建用 `make app`(构建 `Gtmux.app`:Swift app + 内置的 cgo-free CLI)。删除用 `gtmux uninstall-app`。

```sh
# 安装 / 更新(CLI + app):
curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | bash
```

状态栏图标是一个按最紧急状态着色的圆点 —— **红**等输入 · **青**运行中 · **绿**空闲 ·
灰表示没有运行中的 agent —— 并带计数角标(如两个等你时显示 `2`)。**点圆点或按 ⌘⌥G** 打开
弹出面板;agent 按 **等你 / 运行中 / 空闲** 分组,每行 `‹图标› session · task`,点某行就执行
`gtmux focus <pane>` 跳过去。底部有快捷操作:**概览** 和 **实时面板**(在新 Ghostty 窗口打开
`gtmux overview` / `agents --watch`)、**接回 detached**(`gtmux restore`)、**新建 session**
(`gtmux new`)。

它是 CLI 的纯**消费者** —— 轮询 `gtmux agents --json`(约 1.5s)、调用 `gtmux focus` ——
gtmux 内核始终是数据源。CLI 保持 cgo-free,只有这个 app 是原生 Swift 构建。`gtmux new [name]`
也是一个 CLI 命令。

> 这个 app(`com.gtmux.menubar`)同时也是通知的点击目标:hook 通知 `-activate` 它,其 reopen
> 处理会执行 `gtmux focus --last`。发布会附带 universal、ad-hoc 签名的 `Gtmux-<版本>-macos.zip`;
> 安装时会去掉 quarantine 标记,首次启动不被拦。

## tmux 集成

gtmux 只是个 CLI —— 在 `tmux.conf` 里绑你喜欢的键即可。推荐:

```tmux
set -g set-titles on
set -g set-titles-string '#S — #W'
bind g run-shell -b "gtmux overview --popup"
bind a display-popup -E -w 80% -h 60% "gtmux agents --watch --popup"
bind J run-shell "gtmux focus --last"
```

## 通知钩子

`⏸ 等输入`、`✓ latest` 以及点击跳转的通知,都依赖一个把状态写到 `~/.local/share/gtmux/`
的钩子。gtmux 已经内置这个钩子,无需外部脚本:

```sh
gtmux install-hooks          # 一次性安装(macOS)
gtmux uninstall-hooks        # 撤销
```

`install-hooks` 会在 `~/.claude/settings.json` 的 `Stop`、`Notification`、
`UserPromptSubmit` 事件上注册 `gtmux hook`(幂等;保留其它 hook,并先备份),并缓存
Claude 图标。装了 `terminal-notifier` 通知才可点击(`brew install
terminal-notifier`);没装也会发通知,只是不可点。

`gtmux hook` 是生产者 —— 由 Claude Code 调用,你不用手动跑。它纯靠事件**时机**写
`active/<pane>`、`waiting/<pane>`、`last-finished`,并在你已经盯着该 session 的 tab
时抑制通知。点击通知会 `-activate` 菜单栏 app(`com.gtmux.menubar`),它执行
`gtmux focus --last` 把你带到刚完成的那个 pane —— 所以要保持 `Gtmux.app` 已安装才能点击跳转。
设 `GTMUX_HOOK_DEBUG=1` 可把决策过程写到 `~/.local/share/gtmux/hook.log`。

> 在用 peon-ping?`install-hooks` 会询问是否把它的 `desktop_notifications` 和
> `terminal_tab_title` 设为 `false`(后者是必须的 —— `focus` 需要 tmux 的
> `set-titles` 独占 tab 标题)。加 `--yes` 可非交互式直接接受。

## 许可

MIT
