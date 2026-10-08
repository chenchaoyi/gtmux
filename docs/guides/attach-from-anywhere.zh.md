---
title: 从任意电脑接回会话
description: 在另一台电脑的终端里用 gtmux attach 进入你 Mac 上的 tmux 会话，局域网或隧道都行，断开时什么都不会停。
order: 4
---

[English](attach-from-anywhere.md) · **中文**

你在公司的 Mac 上跑着一堆 agent，人却在家里另一台电脑前。手机能看能回，但有时你要的是一个真正的终端：原样进到那个会话里接着干。`gtmux attach` 就是干这个的。

## 在 Mac 上开个口子

```sh
gtmux serve                # 另一台电脑和 Mac 在同一个局域网
gtmux tunnel               # 公网 HTTPS 地址，给其他允许访问它的网络用
```

同一个网络里 `serve` 就够。跨网络用 `tunnel`，它经出站隧道给 Mac 一个 `https://…` 地址，不用开路由端口，也不用 VPN；不过你所在的网络仍得允许访问这个地址。隧道有哪几种，见[移动端与远程访问](../phone.zh.md#从任意网络gtmux-tunnel推荐)。

## 在另一台电脑上装 gtmux

另一台是 Mac，就用[安装脚本或 Homebrew](../install.zh.md)。Linux 上用 Go 1.26 或更新版本从源码装 CLI：

```sh
go install github.com/chenchaoyi/gtmux/cmd/gtmux@latest
```

要在交互式终端里用：Ghostty、iTerm2、Terminal，或者你 Linux 桌面上的终端都行。

## 给那个终端配一次对

在 Mac 上跑 `gtmux pair`。它印出的三种形式里有一行 `gtmux attach '…/#c=<code>'` 命令，复制到另一台电脑的终端里运行即可。配对码只能用一次，5 分钟过期，所以每次配对都要新的。

之后这个终端会记住凭据，再连只要：

```sh
gtmux attach <host> %7
```

`%7` 是 pane ID；不写的话，只有一个会话就直接进，否则给你一个带编号的列表挑。不想配对、想自己提供凭据，就明确写出来，并替换掉所有占位符：

```sh
gtmux attach <host> --token <token> %7
```

## 连上之后

你眼前的终端就成了 Mac 上的那个 tmux 会话：Mac 的 tmux 和它的配置、全屏程序、颜色，全部照旧。它是把一个 tmux 客户端接到那个 pane 所在的整个会话上，等于 owner 的完整权限，所以只有你和你配对过的设备能用。

分享链接是给浏览器和手机用的，不能用于 `gtmux attach`：分享链接，或用 `--code` 给的分享码，gtmux 在兑换之前就会拒绝；Mac 上的 `gtmux serve` 也不给访客终端权限。只想把一个 pane 给别人，见[把一个会话开给协作者](phone-and-web.zh.md#把一个会话开给协作者)。

两个值得知道的选项：`--read-only` 只看不输入；`--predict`（实验性）在慢网络上让你打的字立刻显示出来。

## 干完就走

按 tmux 的 `前缀键 d`（默认是 `Ctrl-b d`）或 `Ctrl-]` 断开。会话在 Mac 上照常跑，一点不受影响。**别在 pane 里敲 `exit`**，那会关掉里面的 shell。

---

接下来：只想看看、回一句？手机更顺手，见[用手机和网页远程管理](phone-and-web.zh.md) · 全部参数：[`gtmux attach`](../cli.zh.md#gtmux-attach在另一台机器的终端里进远端会话)
