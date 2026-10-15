# Linux 服务器

[English](linux.md) · **中文**

gtmux 运行在 tmux 会话所在的 Linux 主机上，终端、网页、iPhone 和 iPad 共用这台主机的数据。Mac 菜单栏仍管理本机；多台服务器不会自动合并为一个 HQ。

## 安装与连接

提供 amd64、arm64 的纯 Go 发行包，首批面向 Ubuntu 22.04+、Debian 12+。先通过系统包管理器安装 `tmux`、`curl`、`tar` 和 SHA256 校验工具，再运行：

```sh
curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | bash
export PATH="$HOME/.local/bin:$PATH"
gtmux doctor --no-prompt
gtmux install hooks --agent codex --yes
# 也可选 claude、cursor、gemini、kimi、copilot、kiro。
```

安装脚本按架构下载 Linux 包、校验后安装，不会下载 macOS App。hook 需重启 agent 才会加载；agent 本身按各自官方说明安装。源码构建需要 Go 1.26+，不包含官方托管服务凭据。

请用**运行 tmux 的同一用户**执行 gtmux。使用自定义 socket 时，保持对应的 tmux 环境一致，不要用 sudo 跨用户查找会话。

```sh
gtmux serve --bind 127.0.0.1 --port 8765
```

供可信局域网内的设备连接时，可绑定所需网卡，例如 `--bind 0.0.0.0`，再在另一终端运行 `gtmux pair`，用 App 扫码。直接监听的端口应受防火墙或 VPN 保护；owner token 可向终端输入。需要远程连接时：

```sh
gtmux tunnel                        # 标准方式，先安装 cloudflared
gtmux tunnel --backend self         # 直连，需要已有授权或自建配置
```

按[官方说明安装 cloudflared](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/)。gtmux 不会自行运行 sudo 或安装系统软件包。直连使用内置客户端，无需 cloudflared；前台运行不依赖 systemd。授权、配对和 token 的规则保持一致。

## 后台运行

```sh
gtmux serve --service               # 局域网访问
# 或选择一种隧道方式：
gtmux tunnel --service --yes
gtmux tunnel --backend self --service --yes
gtmux tunnel --status
gtmux tunnel --unservice            # 停止并移除远程访问服务
```

Linux 使用 **systemd 用户服务**：`com.gtmux.serve.service`，以及标准方式的 `com.gtmux.tunnel.service` 或直连的 `com.gtmux.selftunnel.service`。文件位于 `$XDG_CONFIG_HOME/systemd/user`，默认 `~/.config/systemd/user`，权限为 0600。配置前先检查用户服务管理器；若无法连接，不会当作安装成功，可改用正常登录会话或前台运行。

用户服务通常随用户管理器启动。若需退出登录后继续运行、系统启动时自动启动管理器，可能需要管理员为该账号开启 lingering。gtmux **不会自动开启**；请先查看 [loginctl 官方说明](https://www.freedesktop.org/software/systemd/man/252/loginctl.html) 和本机策略。服务已启用不等于已验证重启后常驻，也不等于公网可达。

服务记录 HOME、配置时的 PATH，以及明确选择的 tmux socket，不会复制整套环境或凭据。agent 若使用自定义配置目录，请在账号或服务环境中配置。更新 serve 不应结束它创建的 tmux 会话，因此明确使用 `KillMode=process`。关闭远程访问也不会结束本地 agent。

```sh
systemctl --user status com.gtmux.serve.service
journalctl --user -u com.gtmux.serve.service --since today
gtmux logs
gtmux doctor --no-prompt
gtmux update
```

进程输出仍保存在 `~/.local/share/gtmux/logs/`，活动、事件和知识记录沿用 macOS 的路径与格式。更新后会重启正在运行的 gtmux 服务，失败会提示手动处理。自动恢复标准隧道时，不会悄悄覆盖用户手改的服务文件。

## 能力与边界

- 雷达、终端与对话历史、输入、新建会话、派发、恢复、HQ 和知识库复用同一核心。无桌面恢复会保留后台会话，并给出 `tmux attach` 连接方式。`gtmux focus` 可切换本地交互式 tmux 客户端；没有对应客户端时会提示连接方式并返回 2。远程桌面跳转仍不支持。
- 手机推送和实时活动通过已配置的推送中继工作，不依赖桌面通知。官方发行包包含发行用的中继配置；源码构建需自行配置。App 权限和推送限制仍适用。
- macOS 菜单栏、桌面窗口跳转、屏幕标注、合盖与电源控制不属于 Linux 能力。客户端根据主机能力隐藏不适用的操作；旧客户端请求时会收到明确的不支持结果。
- 磁盘采样 gtmux 数据所在分区；内存使用 Linux 的 `MemAvailable / MemTotal`，低于 10% 警戒、低于 5% 告急；负载为一分钟平均负载除以可用 CPU 核数。首批 Linux 服务端不监控电池。应有读数缺失时，不会据此解除告警；尚未采集容器 cgroup 压力。
- HQ 每次管理一台主机；tmux 外会话沿用已有感知和接管边界。真实 agent 账号、手机推送、iPhone/iPad 布局、Linux 登录退出与重启常驻，均需对应环境验收，不能用 CI 模拟结果代替。
- FreeBSD 等其他 Unix 尚未作为支持平台发布；独立的平台模块为后续扩展留出接口。
