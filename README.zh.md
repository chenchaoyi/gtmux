<div align="center">

<img src="docs/assets/logo.png" width="104" alt="gtmux logo" />

# gtmux

tmux 会话和 coding agent 的指挥台。

[![Release](https://img.shields.io/github/v/release/chenchaoyi/gtmux?color=06B6D4&label=release)](https://github.com/chenchaoyi/gtmux/releases)
[![CI](https://github.com/chenchaoyi/gtmux/actions/workflows/ci.yml/badge.svg)](https://github.com/chenchaoyi/gtmux/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/go-1.25-00ADD8?logo=go&logoColor=white)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-green)](LICENSE)

[English](README.md) · **中文**

</div>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/readme-hero-dark.jpg" />
  <img src="docs/assets/readme-hero.jpg" width="100%" alt="菜单栏、终端、浏览器、iPad 和 iPhone 锁屏上的 gtmux" />
</picture>

在 tmux 里同时跑几个 coding agent（Claude Code、Codex、Gemini、Cursor），很容易分不清哪个在等你点头，哪个还在跑，哪个已经跑完。gtmux 是这些 pane 上的雷达和遥控器，不管 agent 是谁起的，只要在你的 tmux 里就看得见。

gtmux 的前提是每个 agent 各占一个 tmux pane。我们推荐 [Ghostty](https://ghostty.org) + tmux + gtmux 这套组合，没用过 tmux 可以先看官方的 [Getting Started](https://github.com/tmux/tmux/wiki/Getting-Started)。

## 五个入口

- 在终端里，`gtmux agents` 列出所有 agent，`focus` 跳到 pane，`spawn` 给 agent 派活。
- 菜单栏 app 常驻一个状态点，`⌘⌥G` 唤出面板，agent 等你时弹桌面通知。
- iPhone 和 iPad app（[App Store](https://apps.apple.com/app/id6791144062)）有锁屏推送，能往 pane 里回话，还能用限定范围的链接把一个会话交给协作者。
- 任何浏览器都能打开网页版的雷达和终端镜像，你发给访客的链接也在这里打开。
- 在另一台电脑上，`gtmux attach` 把 Mac 上的 tmux 会话接到眼前的终端里。

远程访问都要求 Mac 醒着。`gtmux awake` 只要一次管理员授权，就能让 Mac 和隧道在合盖后照常运行，电量降到 20% 时自动恢复睡眠。

## 雷达

```
gtmux agent · 7 agent · 1 等输入 · 2 运行中 · 4 空闲

⏸ 等输入   Claude Code  api:0.0                permission to run tests %7
⠿ 运行中   Claude Code  hq:0.0                 api is waiting on you · rest normal %1
⠿ 运行中   Claude Code  web:0.0                refactor auth middleware %11
✳ 空闲     Claude Code  app:0.0                wire up the dashboard %9
✳ 空闲     Codex        worker:0.0             add retry backoff %8  ✓ 最近完成
✳ 空闲     Gemini       docs:0.0               draft the API reference %3
● 运行中   Claude Code  infra:0.0              — %5

跳转：gtmux focus <pane>   （例如 gtmux focus %7）
```

按紧急程度排序。各端用同一套颜色：红色在等你，青色在跑，绿色空闲，灰色是没有 agent 状态的普通进程。

装了 gtmux hook（agent 每轮开始和结束时调用的一小段回调）的 agent 会直接上报状态。在 tmux 里，没装 hook 的 agent 也能从屏幕内容认出来。tmux 之外的 agent 只有装了 hook 才看得到，只读列出，可以用 `gtmux adopt <id>` 转进 tmux。

## HQ（中控）

`gtmux digest` 列出每个 agent 在做什么：你最后给它的指令、它上一条回复的结尾、等待时在问什么，整张表不花一次模型调用。`gtmux hq` 在独立会话里起一个 agent 当 HQ，它读这份 digest，盯着其他 agent，替你往它们的 pane 里输入，谁开始等待就会被叫醒。这样你只要跟 HQ 说话，不用挨个去找 agent。

## 长什么样

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/readme-screens-dark.jpg" />
  <img src="docs/assets/readme-screens.jpg" width="100%" alt="手机上的四个页面：雷达、在 pane 里回复、HQ、用量" />
</picture>

## 快速上手

安装脚本会同时装好 CLI 和菜单栏 app，桌面通知由 app 负责发。

```sh
curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | bash
```

用 Homebrew 的话，`brew install chenchaoyi/tap/gtmux` 装 CLI，`brew install --cask chenchaoyi/tap/gtmux-app` 装菜单栏 app。

然后跑一遍体检。缺的东西它会逐项提出来帮你配上（agent hook、focus 和 restore 依赖的 set-titles、重启后恢复、菜单栏 app），每一步都先问你。

```sh
gtmux doctor
```

之后基本不用管它。菜单栏的点告诉你有没有 agent 在等，点通知就跳到对应 pane，`⌘⌥G` 直接跳到在等你的那个。想用命令行也随时可以：

```sh
gtmux agents --watch         # 终端里的实时看板，回车跳到对应 pane
gtmux app                    # 启动菜单栏 app（别名 menubar）
gtmux update                 # 更新 CLI 和菜单栏 app
```

只想要通知的话，`gtmux install hooks` 只注册 agent hook，Claude Code 以外的 agent 加上 `--agent codex|cursor|gemini|copilot|kiro|opencode|kimi`。

想在手机上看，同一 Wi-Fi 下跑 `gtmux serve`，在别的网络跑 `gtmux tunnel`（不用 VPN），再配对 iOS app，详见 [docs/phone.zh.md](docs/phone.zh.md)。

跳转功能（`focus`、`restore`、`new`）需要 macOS 加 [Ghostty](https://ghostty.org) 1.3+ 或 iTerm2，Warp 尽力支持。`agents` 和 `overview` 在任何跑 tmux 的终端里都能用。身在中国大陆或 GitHub 访问不稳的话，看[安装说明](docs/install.zh.md)。

## 和同类工具的区别

claude-squad、uzi、dmux 这类工具负责启动 agent 并放进 git worktree，也只看得到自己起的那些。gtmux 读你现有的 tmux，手动起的、别的工具起的都看得见，需要时也能用 `gtmux spawn` 派活。它是一个零 cgo 的 Go 二进制，各个 app 读的都是同一份 `gtmux agents --json`。

## 文档

- [CLI 与命令](docs/cli.zh.md)：所有命令、HQ、识别原理、各 agent 的 hook、tmux 按键绑定。
- [手机与远程访问](docs/phone.zh.md)：iOS app、`gtmux serve`、隧道、浏览器镜像。
- [HQ 记住的东西](docs/knowledge.zh.md)：知识库存在哪、分几层、你自己能改什么。
- [安装说明](docs/install.zh.md)：锁定版本、从源码构建、镜像。
- [设计文档](docs/design/README.zh.md)，在途变更在 `openspec/`。

仓库结构和贡献说明在 [CONTRIBUTING.md](CONTRIBUTING.md)（英文）。

## 许可

[MIT](LICENSE) © ccy
