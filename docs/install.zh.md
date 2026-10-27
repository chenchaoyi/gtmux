# 安装

[English](install.md) · **中文**

## Agent 插件与 hooks

安装 CLI／App 后，运行 `gtmux install hooks --agent auto` 只为本机找到的 Agent CLI 自动配置 Hook，
跳过配置中已禁用 Hook 的 Agent。只有旧配置目录、没有 CLI 时不会误报已安装，也不改动已有配置。只装 DSH 则用 `gtmux install hooks --agent dsh`；
不带 `--agent` 保留默认 Claude。`gtmux doctor` 只检查，`gtmux doctor --fix` 逐项提出修复。
DSH 的 Cordis 插件自带于 gtmux，不需要 npm 包。已有对话无需插件即可读取；
生命周期事件需在下次启动 Web 时带生成的 `--patch`，详见 [DSH 指南](guides/deepseek-harness.zh.md)。

## Linux

同一安装脚本支持 Linux amd64、arm64，自动跳过 macOS App。tmux 通过系统包管理器安装。配对、前台运行、systemd 用户服务和能力边界见 [Linux 服务器](linux.zh.md)。

## Homebrew（macOS）

```sh
brew install chenchaoyi/tap/gtmux             # CLI
brew install --cask chenchaoyi/tap/gtmux-app  # 菜单栏 app（可选）
```

也可以先 tap 一次，之后按名字装：

```sh
brew tap chenchaoyi/tap
brew install gtmux
brew install --cask gtmux-app
```

升级用 `brew upgrade gtmux`（app 是 `brew upgrade --cask gtmux-app`）。CLI 以 Homebrew cask
的形式安装。app cask 默认装在 `/Applications`，自定义 Homebrew 的 `--appdir` 会改变这个位置。
没有 Homebrew 的话，用下面的安装脚本。

## 安装脚本

```sh
curl -fsSL https://ccy.pub/gtmux/install | bash
```

校验过 checksum 的二进制装到 `~/.local/bin/gtmux`，菜单栏 app 一并装上。可选项：

- `GTMUX_NO_APP=1`：不装菜单栏 app，只要 CLI。
- `GTMUX_APP_LOGIN=1`：开机自启 app。
- `GTMUX_VERSION=vX.Y.Z`：锁定版本。

这些变量要放在执行安装脚本的 `bash` 前面，不能只传给 `curl`：

```sh
curl -fsSL https://ccy.pub/gtmux/install | GTMUX_NO_APP=1 bash
```

从源码安装（需含安全修复的稳定版 Go 1.26.10／1.27.3 或更新版本，只安装 CLI）：

```sh
go install github.com/chenchaoyi/gtmux/cmd/gtmux@latest
```

普通源码构建不含官方托管隧道的注册凭据和推送中继凭据。需要这些服务时使用正式发布版，
或按[隧道设计文档](design/remote-access-tunnel.zh.md)和[推送中继说明](../relay/README.md)配置自己的服务。

之后用 `gtmux update` 升级；它也会尝试重启已加载的 serve 和 Direct 隧道服务，让它们使用新二进制。

自动更新按组件保留更高或未知版本；缓存索引可能落后于 GitHub。现有 `GTMUX_VERSION`
仍可明确指定版本重装或降级。可选 App 失败后，已装好的 CLI 保留，退出码 3 表示
App 步骤未完成；旧 App 重试 `gtmux update`，未安装时用 `gtmux install app`。详见[更新规则](cli.zh.md#安装与更新命令)。


脚本装在 `~/Applications` 里的 app，用 `gtmux uninstall app` 删除菜单栏 app 及登录项。
`gtmux uninstall hooks` 摘掉 agent hook，`gtmux uninstall all` 两个都做；这些命令会保留
CLI 二进制和 gtmux 的已有记录。Homebrew 安装的还需用 `brew uninstall --cask gtmux-app`
删除 app（通常在 `/Applications`），用 `brew uninstall --cask gtmux` 删除 CLI。

## 官方镜像与 GitHub 兜底

两个官方短网址现已可用：`https://ccy.pub/gtmux/install` 和 `https://ccy.dev/gtmux/install`。短 bootstrap 从自有下载域读取并验证签名清单，再校验安装脚本和每个压缩包；签名异常停下，不自动降低验证标准。包含这项实现的原生更新器使用同一公钥在 Go 内校验，从不执行远端脚本；已有二进制在升级前仍沿用原来的更新通道。官方源网络不可用时保留原有 GitHub 通道；已有签名清单时，兜底下载仍必须匹配其摘要和大小。

构建、存储、来源与运维边界见[托管说明](release-hosting.zh.md)。短入口需要 OpenSSL／LibreSSL，macOS 自带 LibreSSL。旧版安装器、Homebrew 和显式覆盖仍沿原通道运行。需要强制 GitHub 或第三方代理时，从 GitHub 取得完整安装器：

```sh
curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | GTMUX_INSTALL_MIRROR=ghproxy bash
curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | GTMUX_INSTALL_MIRROR=https://my.mirror/ bash
curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | GTMUX_INSTALL_MIRROR=github bash
```

`ghproxy` 下载发布压缩包时直接走镜像链，校验文件仍先尝试 GitHub。
将 `https://my.mirror/` 换成自己的代理前缀；此模式先尝试 GitHub，再尝试 `<前缀><github-url>`。
`github` 禁用镜像兜底。这些选项只影响安装脚本发起的下载，不影响管道前面的 `curl`。

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
