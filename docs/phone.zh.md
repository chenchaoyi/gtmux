# 远程访问参考

[English](phone.md) · **中文**

连接方式、设备权限与通知条件。

**第一次连接 iPhone、iPad 或浏览器？** 从[用手机、iPad 和浏览器管理](guides/phone-and-web.zh.md)开始，按步骤完成配对、日常操作和分享。

## 连接方式

实时雷达和终端需要能连接到运行 `gtmux serve` 的 Mac。隧道由 Mac 主动向外建立，不用开放入站端口。

| 方式 | 适用情况 | 地址与要求 |
|---|---|---|
| 局域网 | 同一网络内，设备之间能互相访问 | Mac 的地址，默认端口 8765 |
| Standard 标准隧道 | 跨网络访问 | 免费、固定的 `https://gtmux-<label>.ccy.dev`；Mac 使用 `cloudflared` |
| Direct 直连隧道 | 网络拦截标准隧道 | 访问码付费解锁，经 gtmux 服务器的 443 端口；不需要 `cloudflared` |
| 临时隧道 | 临时连接 | `trycloudflare.com` 地址每次运行都变，需要重新配对 |
| Tailscale 或其他 VPN | 设备之间已有 VPN | 能访问的 VPN 地址，默认端口 8765 |

隧道仍需两端网络允许连接。公司或访客 Wi-Fi 即使名称相同，也可能隔离设备。

### 隧道设置

`gtmux tunnel` 默认使用 Standard，已有常驻隧道时直接复用。`--service` 注册为后台服务，重启后继续运行；`--unservice` 移除服务；`--status` 显示当前方式和地址。Direct 已开启时，`--service` 会保留 Direct；切回 Standard 要明确指定 `--backend cloudflare`。

Standard 沿用同一注册时地址不变；原隧道删除后重新建立会换地址，需要重新配对。

Direct 在 [gtmux Direct](https://ccy.dev/projects/gtmux/direct) 获取访问码，再用 `gtmux tunnel --redeem <码>` 兑换。一个码最多用于三台 Mac，每台有独立地址和账号。`--servers` 列出服务器和实测延迟，`--server <id>` 切换服务器，码和端口不变。此前已连接过的设备会跟随切换；只配对、未连接过的设备需重新扫码，原访客链接也会失效。

完整参数：[gtmux tunnel](cli.zh.md#gtmux-tunnel)。自托管见[隧道设计](design/remote-access-tunnel.zh.md)和 `deploy/self-tunnel/`。

## 配对与访客权限

配对为你的设备生成独立 owner 凭据，可查看和输入 pane、管理访客链接、签发新配对码。这个凭据应当作密码保管。

配对码只能使用一次，五分钟过期，每台设备需生成新码。配对前地址发生变化，要刷新配对码；已经配对成功但首次雷达加载失败，只需重试连接。

访客链接只授权指定 pane。`--view` 允许查看，`--type` 允许输入，也会自动加入可见范围。输入还需总开关 `gtmux share on`。`--expires` 可写 `45m`、`24h` 或 `7d`；不写则不过期。吊销链接后，下一个请求会被拒绝。

访客不能访问 HQ 页面、owner 设置、推送注册、新建会话或 `gtmux attach`。能输入的访客使用 pane 内程序的权限；pane 范围不限制程序对文件和网络的访问。

设备丢失后，在 Mac 上用 `gtmux devices revoke <id>` 吊销。手机界面不提供吊销 owner 设备和开关远程访问，但这些界面限制不约束 owner 凭据的能力。不要公开配对码、设备凭据或访客链接；只有公网地址并不授予访问权限。

完整参数：[gtmux pair](cli.zh.md#gtmux-pair接入你自己的设备全权)、[gtmux share](cli.zh.md#gtmux-share给协作者的受限可吊销访问)和 [gtmux devices](cli.zh.md)。

## 在线与通知条件

Mac 必须保持唤醒，才能远程访问。MacBook 合盖默认休眠；`gtmux awake on` 可让它合盖后继续运行，但不会替你启动 serve 或隧道。电池电量到 30% 时提醒，20% 时恢复休眠。

推送不要求手机直接连接 Mac。它需要醒着、运行 serve 且能访问推送中继的 Mac，以及完成注册、能接收 Apple 推送的 iOS 设备。网络和通知设置可能延迟或阻止送达。实时雷达和回复仍需要能连到 Mac。

tmux 外的 agent 只读：没有 pane，就没有可发送输入的地方。

## 桌面应用会话

ChatGPT 桌面版 Codex 会话显示在「桌面应用」，默认标记为「仅显示状态」。
它们不计入管理总数、HQ 待处理、通知或实时活动；对话内容也不进入摘要和知识采集。

在已配对的手机或 iPad 上，点击会话即可查看实时更新的只读对话，查看不会开启 HQ 跟进。
在长按菜单中选择「跟进设置」，可允许 HQ 读取、分析和汇报。开关修改自动保存，
点击「完成」关闭。「会话通知」和「知识留存」需分别开启，初始均为关闭，HQ 跟进
开启前不可操作；知识采集仅覆盖后续活动。

Mac 确认当前设置后才更新列表标记。发生冲突或回执丢失时，会重新读取实际设置；
读取失败则需「重新加载」成功后再修改。关闭「HQ 跟进」同时关闭两项额外权限，保留
已有记录。再次开启不会自动恢复权限。设置仅对当前 Mac 的这段会话生效，不会扩展到
其他桌面会话或其他 Mac。需处理的问题仍在 ChatGPT 桌面版继续；这些设置不提供终端
输入或批准操作。旧版 Mac 核心需更新后才能提供设置入口；访客没有桌面会话跟进控制。

## 排查连接问题

- **同一个 Wi-Fi 也连不上：** 在设备浏览器里测试 `http://<mac-ip>:8765/api/health`。打不开时检查网络路径，或改用隧道／VPN。
- **配对码被拒绝：** 已使用的码不能再用，每台设备生成新码。
- **访客能看不能输入：** 检查链接的输入范围，以及 `gtmux share on` 总开关。
- **收不到通知：** 检查 Mac 是否醒着、serve 是否运行、设备是否注册，以及 iOS 通知权限。

app 的「设置 → 诊断」可导出本地连接和配对记录；`gtmux doctor --bundle` 收集 Mac 一侧的信息。协议细节见 [API contract](../api/contract.md)。
