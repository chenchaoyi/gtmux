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

升级用 `brew upgrade gtmux`（app 是 `brew upgrade --cask gtmux-app`）。CLI 以 Homebrew cask
的形式安装。没有 Homebrew 的话，用下面的安装脚本。

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
`gtmux uninstall hooks` 摘掉 agent hook，`gtmux uninstall all` 两个都做。

## 中国大陆 / GitHub 不稳：镜像兜底

连脚本本身都拉不下来（`raw.githubusercontent.com` 被挡）的话，从 CDN 镜像取脚本。
脚本跑起来之后，自己要下载的东西会自动切镜像：

```sh
curl -fsSL https://cdn.jsdelivr.net/gh/chenchaoyi/gtmux@main/install.sh | bash
```

这里有两条镜像链，对应两种下载：

- 安装脚本本身。`gtmux update` 先从 GitHub 取，取不到就依次试 jsdelivr、gh-proxy.com、
  ghfast.top、ghproxy.net。所以装好之后，在国内网络更新不用再操心。
- 发布文件（CLI 压缩包和 app 的 zip）。安装脚本先走 GitHub，卡住了就依次试 ghfast.top、
  gh-proxy.com、ghproxy.net。`SHASUMS256.txt` 始终先从 GitHub 直取，所以即使安装包来自镜像，
  校验值仍锚在 GitHub 上。

用 `GTMUX_INSTALL_MIRROR` 可以指定：

```sh
GTMUX_INSTALL_MIRROR=ghproxy  curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | bash   # 直接走镜像链
GTMUX_INSTALL_MIRROR=https://my.mirror/  curl -fsSL ... | bash   # 自定义 <前缀><github-url> 代理
GTMUX_INSTALL_MIRROR=github   curl -fsSL ... | bash   # 只走 GitHub，不用镜像
```

## 换一台 Mac

值得带走的是 HQ 的档案：HQ 记下的各会话情况、它攒下的知识，以及 `LOCAL.md`，
也就是你自己的偏好文件。这些 gtmux 都不会重新生成。

```sh
# 旧机器上
gtmux hq --export ~/gtmux-hq.tar.gz

# 新机器上，装完 gtmux 之后
gtmux hq --import ~/gtmux-hq.tar.gz
gtmux hq                      # 重启 HQ，让它读到还原后的记录
```

导出时会问你要一个口令，文件用它锁上（`--plain` 不锁）；导入时再输一次。
导入从不就地覆盖：已经在那儿的会被挪到 `hq.replaced-<时间戳>`，路径会打印出来。

其余的重建比拷贝快。新机器上跑 `gtmux doctor --fix`：它会装好 agent hook、set-titles、
重启后恢复和菜单栏 app。手机重新配对一次（`gtmux pair`，或者会打印配对码的 `gtmux tunnel`），
不要拷配对文件，这样旧 Mac 发出的 token 就不再有效。

`~/.local/share/gtmux/` 是运行时状态（标记、事件、快照），留在原地就好。

## 签名与权限

macOS 把你授予的权限绑在 app 的代码签名上。正式发布版在 CI 里用 Developer ID 签名并公证，
所以更新后权限还在。自己用 `make app` 构建的是 ad-hoc 签名，每次重新构建身份都变，
macOS 会忘掉授权、再问一次。想给自己的构建签 Developer ID，构建时设 `GTMUX_SIGN_ID`
（见 `macapp/build.sh`）。
