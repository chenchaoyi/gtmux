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
删除 `/Applications` 里的 app，用 `brew uninstall --cask gtmux` 删除 CLI。

## 中国大陆 / GitHub 不稳：镜像兜底

连脚本本身都拉不下来（`raw.githubusercontent.com` 被挡）的话，从 CDN 镜像取脚本。
脚本跑起来之后，自己要下载的东西会自动切镜像：

```sh
curl -fsSL https://cdn.jsdelivr.net/gh/chenchaoyi/gtmux@main/install.sh | bash
```

这里有两条镜像链，对应两种下载：

- 安装脚本本身。`gtmux update` 先从 GitHub 取，取不到就依次试 jsdelivr、gh-proxy.com、
  ghfast.top、ghproxy.net。装好之后，更新会自动尝试这些备用来源。
- 发布文件（CLI 压缩包和 app 的 zip）。安装脚本先走 GitHub，卡住了就依次试 ghfast.top、
  gh-proxy.com、ghproxy.net。`SHASUMS256.txt` 优先从 GitHub 直取，失败后也可能走镜像。
  SHA256 仍会校验；校验文件也来自镜像时，验证依赖的就是该镜像。安装脚本会打印实际来源。
  app 的 zip 另做压缩包完整性检查，不在 `SHASUMS256.txt` 内。

用 `GTMUX_INSTALL_MIRROR` 可以指定：

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
gtmux hq --export ~/gtmux-hq.tar.gz

# 新机器上，装完 gtmux 并退出正在运行的 HQ agent 之后
gtmux hq --import ~/gtmux-hq.tar.gz
gtmux hq                      # 重启 HQ，让它读到还原后的记录
```

导出时会问你要一个口令，文件用它锁上（`--plain` 不锁）；导入时再输一次。
导入从不就地覆盖：已经在那儿的会被挪到 `hq.replaced-<时间戳>`，路径会打印出来。

在新机器的交互终端里跑 `gtmux doctor --fix`：它会提出缺失的 agent hook、set-titles、
重启后恢复和菜单栏 app 设置，逐项解释并征求确认。手机重新配对一次（`gtmux pair`，或者会打印配对码的 `gtmux tunnel`），
不要拷配对文件，这样新 Mac 就不会接受旧 Mac 发出的 token。这不会撤销旧 Mac 上的访问权限；
停用旧机器时，还需在旧 Mac 上吊销设备。

不要整目录拷贝 `~/.local/share/gtmux/`：其中的标记和快照对应旧 Mac 的 pane。
这里也保存本机事件和用量历史；HQ 导出不会把这两类历史带到新 Mac。

## 签名与权限

macOS 把你授予的权限绑在 app 的代码签名上。正式发布版在 CI 里用 Developer ID 签名并公证，
所以更新后权限还在。自己用 `make app` 构建的是 ad-hoc 签名，每次重新构建身份都变，
macOS 会忘掉授权、再问一次。想给自己的构建签 Developer ID，构建时设 `GTMUX_SIGN_ID`
（见 `macapp/build.sh`）。
