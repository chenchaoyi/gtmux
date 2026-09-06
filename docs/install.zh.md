# 安装

[English](install.md) · **中文**

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

升级用 `brew upgrade gtmux`（app 是 `brew upgrade --cask gtmux-app`）。CLI 也以 cask
形式发布（GoReleaser 已弃用 formula），gtmux 本来就只跑在 macOS 上，所以这不构成限制。
非 macOS 机器请用下面的安装脚本。

## 安装脚本

```sh
curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | bash
```

校验过 checksum 的二进制装到 `~/.local/bin/gtmux`，菜单栏 app 一并装上。可选项：

- `GTMUX_NO_APP=1`：不装菜单栏 app，只要 CLI。
- `GTMUX_APP_LOGIN=1`：开机自启 app。
- `GTMUX_VERSION=vX.Y.Z`：锁定版本。

从源码装：

```sh
go install github.com/chenchaoyi/gtmux/cmd/gtmux@latest
```

之后用 `gtmux update` 升级。卸载：`gtmux uninstall app` 删菜单栏 app，
`gtmux uninstall hooks` 摘掉 agent hook，`gtmux uninstall all` 两个都来。

## 中国大陆 / GitHub 不稳 —— 镜像兜底

如果连脚本本身都拉不下来（`raw.githubusercontent.com` 被挡），先从 CDN 镜像起步。
脚本之后下载自己的资源时也会自动走镜像：

```sh
curl -fsSL https://cdn.jsdelivr.net/gh/chenchaoyi/gtmux@main/install.sh | bash
```

装好之后，`gtmux update` 会用同一条镜像链取脚本（jsdelivr → gh-proxy → ghfast → ghproxy），
所以在国内网络下更新不用再操心这些。

下载安装包时，安装脚本优先走 GitHub，卡住了才自动切镜像链
（`ghfast.top` → `gh-proxy.com` → `ghproxy.net`）。`SHASUMS256.txt` 始终**先从 GitHub 直取**，
所以即使安装包来自镜像，校验值仍锚在 GitHub 上。用 `GTMUX_INSTALL_MIRROR` 可以覆盖：

```sh
GTMUX_INSTALL_MIRROR=ghproxy  curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | bash   # 直接走镜像链
GTMUX_INSTALL_MIRROR=https://my.mirror/  curl -fsSL ... | bash   # 自定义 <前缀><github-url> 代理
GTMUX_INSTALL_MIRROR=github   curl -fsSL ... | bash   # 只走 GitHub，不用镜像
```

## 签名与权限

macOS 把你授予的权限绑在 app 的代码签名上。**Developer ID 签名 + 公证**过的构建，
更新之后权限还在；**ad-hoc** 构建（本地 `make app`，或没签名的发布）每次构建身份都变，
于是 macOS 忘掉授权、重新弹窗。自己构建时设 `GTMUX_SIGN_ID` 就能用你的 Developer ID
签名，见 `macapp/build.sh`。
