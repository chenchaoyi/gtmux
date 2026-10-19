# 安装

[English](install.md) · **中文**

## Agent 插件与 hooks

安装 CLI／App 后，运行 `gtmux install hooks --agent auto` 可识别已有的 agent 目录或安装，
跳过配置中禁用 hook 的 agent。只装 DSH 则用 `gtmux install hooks --agent dsh`；
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
curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | bash
```

校验过 checksum 的二进制装到 `~/.local/bin/gtmux`，菜单栏 app 一并装上。可选项：

- `GTMUX_NO_APP=1`：不装菜单栏 app，只要 CLI。
- `GTMUX_APP_LOGIN=1`：开机自启 app。
- `GTMUX_VERSION=vX.Y.Z`：锁定版本。

这些变量要放在执行安装脚本的 `bash` 前面，不能只传给 `curl`：

```sh
curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | GTMUX_NO_APP=1 bash
```

从源码装（需要 Go 1.26 或更新版本，只安装 CLI）：

```sh
go install github.com/chenchaoyi/gtmux/cmd/gtmux@latest
```

普通源码构建不含官方托管隧道的注册凭据和推送中继凭据。需要这些服务时使用正式发布版，
或按[隧道设计文档](design/remote-access-tunnel.zh.md)和[推送中继说明](../relay/README.md)配置自己的服务。

之后用 `gtmux update` 升级；它也会尝试重启已加载的 serve 和 Direct 隧道服务，让它们使用新二进制。

脚本装在 `~/Applications` 里的 app，用 `gtmux uninstall app` 删除菜单栏 app 及登录项。
`gtmux uninstall hooks` 摘掉 agent hook，`gtmux uninstall all` 两个都做；这些命令会保留
CLI 二进制和 gtmux 的已有记录。Homebrew 安装的还需用 `brew uninstall --cask gtmux-app`
删除 app（通常在 `/Applications`），用 `brew uninstall --cask gtmux` 删除 CLI。

## 中国大陆 / GitHub 不稳：镜像兜底

连脚本本身都拉不下来（`raw.githubusercontent.com` 被挡）的话，从 CDN 镜像取脚本。
脚本跑起来之后，自己要下载的东西会自动切镜像：

```sh
curl -fsSL https://cdn.jsdelivr.net/gh/chenchaoyi/gtmux@main/install.sh | bash
```

`gtmux update` 现在使用本地 Go 安装器，不再下载或执行远端安装脚本。CLI 校验文件
只从官方 GitHub HTTPS 获取；镜像仅提供压缩包，必须匹配官方摘要。App 只从官方 HTTPS
获取，不走镜像。官方请求及重定向限制在允许的 GitHub 主机，无法可信校验时更新失败。

上面的显式 `curl … | bash` 属于独立 bootstrap 路径：脚本和发布文件仍可能使用镜像，
摘要也来自镜像时，验证依赖该镜像；App ZIP 另做完整性检查，不在 CLI 摘要清单中。
脚本会打印实际来源。用 `GTMUX_INSTALL_MIRROR` 选择镜像：

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
