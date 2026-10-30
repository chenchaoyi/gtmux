# 安装、更新与卸载

[English](install.md) · **中文**

第一次安装并连接设备，按[首页三步](../README.zh.md#安装并连接)操作。本页用于选择安装方式、处理 doctor 结果和维护已有安装。

## 选哪个下载入口

| 需要什么 | 用哪个 |
| --- | --- |
| Mac 上的 CLI 和菜单栏 App | 下面的官方安装器 |
| 用 Homebrew 管理安装 | 下面的两条 Homebrew 命令 |
| iPhone 或 iPad App | [App Store](https://apps.apple.com/app/id6791144062)，主机配置好后再配对 |
| 只装 CLI 或指定版本 | 安装器选项，或 [GitHub Releases](https://github.com/chenchaoyi/gtmux/releases) |

## 安装脚本

```sh
curl -fsSL https://ccy.pub/gtmux/install | bash
```

另一个官方入口是 `https://ccy.dev/gtmux/install`。两者都会验证发行清单签名，再校验安装器和下载文件；签名不符时停止安装。原始发行包保留在 GitHub。

CLI 装到 `~/.local/bin/gtmux`。macOS 上会同时安装并打开 `~/Applications` 中的菜单栏 App；若已有 `/Applications` 安装，则更新原位置。脚本不安装 tmux，Mac 上可用 `brew install tmux` 补齐。你使用的 coding agent 也需要先安装并登录。

出现 `gtmux: command not found` 时，按安装器提示把下面一行加入 shell 配置，再开新终端：

```sh
export PATH="$HOME/.local/bin:$PATH"
```

安装选项要传给执行脚本的 `bash`：

```sh
curl -fsSL https://ccy.pub/gtmux/install | GTMUX_NO_APP=1 bash
curl -fsSL https://ccy.pub/gtmux/install | GTMUX_VERSION=vX.Y.Z bash
```

将 `vX.Y.Z` 换成已发布的标签。`GTMUX_NO_APP=1` 只装 CLI，`GTMUX_APP_LOGIN=1` 会开启 App 登录自启。手动下载时按系统和架构选择压缩包；`Gtmux-<版本>-macos.zip` 是 Mac App，CLI 有单独的包。

## 用 doctor 配置

```sh
gtmux doctor
```

检查结果按所影响的功能分组。交互终端中发现问题时，会询问「现在查看可修复项？」；确认后逐项解释并询问是否应用，也可直接运行：

```sh
gtmux doctor --fix
```

它可以配置 tmux 标题和 pane 标签、历史缓冲与会话恢复、快照插件和自动保存，以及本机已安装 agent 的受支持 hook；也会提出相关 agent 设置和缺失 App／依赖的处理步骤。每项改动都先解释再确认，需单独授权或手动完成的事项会留在报告里。gtmux 管理块以外的设置会保留，修改 tmux 配置前会保存 `.gtmux.bak`。

tmux 未安装时先安装；没有运行中的服务时，用 `tmux new -s work` 创建会话再重试。socket 权限错误需先解决，不能改用另一用户运行 gtmux 来绕过。

修复后再跑一次 `gtmux doctor`。**✗** 是影响功能的问题，**⚠** 是建议，**✓** 是检查通过，**·** 是信息。看清剩余检查及其操作提示，「已应用修复」仍可能有未处理的建议。hook「已配置」只说明文件已保存，agent 要在下次正常启动时加载；doctor 不会重启正在运行的会话。

只检查、不询问时用 `gtmux doctor --no-prompt` 或 `gtmux doctor --json`。Mac App 也有「偏好设置 → 通用 → 运行环境检查」。仍有问题可先看 `gtmux logs --since 1h`；`gtmux doctor --bundle` 会在本地生成诊断包，发送前可先检查内容。

只配置某个 agent 可用 `gtmux install hooks --agent codex`，将 key 换成所需 agent；`--agent auto` 为已安装且受支持的 CLI 配置，不带 `--agent` 默认是 Claude Code。key 和加载要求见 [Agent 接入](guides/agent-integrations.zh.md)。DSH 自带的 Cordis 模块需在下次启动 Web 时使用打印的覆盖参数。

## Homebrew（macOS）

```sh
brew install chenchaoyi/tap/gtmux
brew install --cask chenchaoyi/tap/gtmux-app
```

CLI 以 Homebrew cask 分发。App 默认装到 `/Applications`，可通过 Homebrew 的 `--appdir` 调整。升级用 `brew upgrade gtmux` 和 `brew upgrade --cask gtmux-app`。

<a id="platform-support"></a>

## 平台支持与源码构建

同一安装器支持 macOS、Linux 的 amd64 和 arm64，Linux 会自动跳过 App。tmux 通过系统包管理器安装，gtmux 与 tmux 会话使用同一用户运行。桌面跳转、菜单栏 App 和合盖控制仅适用于 macOS。Linux 发行包仍可从 [GitHub Releases](https://github.com/chenchaoyi/gtmux/releases) 下载。

源码构建需要含安全修复的稳定版 Go（1.26.10／1.27.3 或更新）：

```sh
go install github.com/chenchaoyi/gtmux/cmd/gtmux@latest
```

源码构建只装 CLI，不含官方托管隧道的注册凭据和推送中继凭据。需要这些服务时使用正式发行包；自建方式见[隧道设计](design/remote-access-tunnel.zh.md)和[中继参考](../relay/README.zh.md)。

## 更新或卸载

`gtmux update` 更新 CLI 和菜单栏 App，并尝试重启已加载的 serve、Direct 隧道服务以使用新二进制。组件版本更高时保留，未知版本不自动替换。`GTMUX_VERSION` 可明确指定版本，包括降级。详见[更新规则](cli.zh.md#安装与更新命令)。

CLI 安装后返回退出码 3，表示可选的 App 步骤失败。旧 App 可重试 `gtmux update`；未安装时用 `gtmux install app`。

`gtmux uninstall app` 删除脚本安装的 App 和登录项，`gtmux uninstall hooks` 删除 gtmux 的 agent hook。`gtmux uninstall all` 两者都做，保留 CLI 二进制和已有记录。Homebrew 安装则用 `brew uninstall --cask gtmux-app` 和 `brew uninstall --cask gtmux` 删除对应组件。

## 下载失败

先尝试另一个官方安装域名。短入口需要 OpenSSL／LibreSSL，macOS 自带 LibreSSL。确需使用 GitHub 原安装器或代理时，可用：

```sh
curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | GTMUX_INSTALL_MIRROR=github bash
curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | GTMUX_INSTALL_MIRROR=ghproxy bash
```

`github` 禁用镜像兜底；`ghproxy` 下载发行包时使用镜像链，校验文件仍先尝试 GitHub。这些选项不影响管道前面的 `curl`。取得签名清单后，兜底文件也必须符合其摘要和大小。校验及运维边界见[发行托管说明](release-hosting.zh.md)。

## 换一台 Mac

值得带走的是 HQ 的档案：HQ 记下的各会话情况、它攒下的知识，以及 `LOCAL.md`，
也就是你自己的偏好文件。这些 gtmux 都不会重新生成。

```sh
# 旧机器上
gtmux hq --export ~/gtmux-hq.tar.gz.age

# 新机器上，装完 gtmux 并退出正在运行的 HQ agent 之后
gtmux hq --import ~/gtmux-hq.tar.gz.age
gtmux hq                      # 重启 HQ，让它读到还原后的记录
```

导出时会问你要一个口令，文件用它锁上（`--plain` 不锁）；导入时再输一次。
导入从不就地覆盖：已经在那儿的会被挪到旁边一个名为 `hq.replaced-…` 的文件夹，路径会打印出来。

在新机器的交互终端里跑 `gtmux doctor --fix`：它会提出缺失的 agent hook、set-titles、
重启后恢复和菜单栏 app 设置，逐项解释并征求确认。手机重新配对一次（`gtmux pair`，或者会打印配对码的 `gtmux tunnel`），
不要拷配对文件，这样新 Mac 就不会接受旧 Mac 发出的 token。这不会撤销旧 Mac 上的访问权限；
停用旧机器时，还需在旧 Mac 上吊销设备。

不要整目录拷贝 `~/.local/share/gtmux/`：pane 标记对应旧 Mac 的 pane；`hq-snapshots/`
保存的是 HQ 家目录归档，与 HQ 导出的内容重复。这里也保存本机事件和用量历史；
HQ 导出不会把这两类历史带到新 Mac。

## 签名与权限

macOS 把你授予的权限绑在 app 的代码签名上。正式发布版在 CI 里用 Developer ID 签名并公证，
所以更新后权限还在。自己用 `make app` 构建的是 ad-hoc 签名，每次重新构建身份都变，
macOS 会忘掉授权、再问一次。想给自己的构建签 Developer ID，构建时设 `GTMUX_SIGN_ID`
（见 `macapp/build.sh`）。

## Standard 隧道安全更新

可选组件 `cloudflared` 必须满足 Cloudflare 的证书安全版本要求：2026.10.2 或更新版本；
macOS 非 Homebrew 安装需 2026.10.3 或更新版本。版本未知或预发布版本不能启动新的
Standard 或临时隧道。Direct 使用 gtmux 自身的传输实现，不使用这个程序。

`gtmux install cloudflared --yes` 可单独安装或更新这个组件。
macOS 上已有的 Homebrew 安装会通过其所属的 Homebrew 更新，避免 Intel 和 Apple Silicon
两套安装互相覆盖。其他安装渠道和 Linux 包管理器需用原渠道更新；未完成时命令会说明
原因并返回失败，不会自动更换安装渠道，也不会关闭 TLS 校验。

安装脚本、`gtmux install all` 和普通 `gtmux update` 会检查并更新已安装的客户端；
没有安装时不会强行添加。`update --check` / `--json` 仍然只读，`--cli-only` 跳过依赖更新。
旧版更新器无法提前拥有这个能力：从尚未接入此流程的旧版升级后，请额外运行一次
`gtmux install cloudflared --yes`。只用 Homebrew 更新 gtmux cask 也不会执行这项修复。

更新文件不代表运行中的进程已更新。`doctor` 会通过 gtmux 所管理连接的本机指标区分
安装版本与运行版本。Standard 仍运行旧版或无法确认运行版本时，不会报告更新全部完成；
请运行 `gtmux tunnel --service --recover`，随后检查连接。重启会断开已有长连接。
Mac 运行进程的安装渠道无法确认时，按更严格的 2026.10.3 判断。
旧的登录启动服务、其他主机上的连接仍需分别检查，这不是整个 Cloudflare 账号的审计。

正式 CLI 和 Mac 包内 CLI 的构建必须使用含 GO-2026-6809 修复的稳定版 Go
（1.27.3／1.26.10 或更新版本）；自行编译也应使用这些版本。操作系统信任库仍需保持更新，
受影响的 HTTPS 源站证书应更换，不要用 `noTLSVerify` 绕过报错。依据：
[Cloudflare 事件说明](https://blog.cloudflare.com/emudhra-certificate-compromise/)、
[更新方法](https://developers.cloudflare.com/tunnel/guides/update-cloudflared/)和
[Go 安全公告](https://pkg.go.dev/vuln/GO-2026-6809)。
