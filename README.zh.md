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
go install github.com/chenchaoyi/gtmux@latest
```

## 用法

```
gtmux <命令> [选项]            # 不带参数直接敲 gtmux 显示帮助
```

| 命令 | 作用 |
| --- | --- |
| `overview [--popup]` | session / window / pane 汇总(prefix+g 的弹窗) |
| `agents [--watch\|--json]` | 各 pane 里的 coding agent:⏸ 等输入 / ⠿ 运行中 / ✳ 空闲、在哪、可跳转的 pane id。`--watch` 是 bubbletea 实时面板;`--json` 是给脚本用的结构化输出 |
| `restore [--pick\|--one\|<名字>\|--dry-run]` | 每个 session 一个 Ghostty tab,全部接回(重启后会启动 tmux 并等 continuum 恢复) |
| `focus <名字\|pane-id>` | 跳到该 session 的 Ghostty tab;给 tmux pane id(`%N`)则精确落到那个 window+pane |

输出语言由 `--lang=en|zh`(默认 `en`)或 `$GTMUX_LANG` 控制。

### agent 状态

- **⏸ 等输入** —— 卡在等**你**批准/授权(排最前)
- **⠿ 运行中** —— 忙(pane 里有 spinner 在动)
- **✳ 空闲** —— 完成一轮,轮到你

⏸ 需要安装 Claude Code 通知钩子(它靠事件**时机**而非关键词,区分权限请求与空闲
提醒)。agent 通过前台命令和 pane 标题信号识别;可用 `~/.config/gtmux/agents.json`
扩展 profile。

## 许可

MIT
