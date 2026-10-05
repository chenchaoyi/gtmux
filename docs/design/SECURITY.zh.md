# gtmux 远程访问的信任边界 (Security model)

> 决策 D1=(a)（2026-06-28）：维持「单层 TLS 隧道 + bearer token」，**不做端到端加密**，
> 但把信任边界写清楚。QR 配对 / E2E 记为 backlog（见 `DECISIONS-FOR-CCY.md` D1）。

上面记录的是六月的决策。一次性配对码此后已交付；临时密钥配对和应用层端到端加密仍未实现。

gtmux 的远程功能让手机/浏览器隔着网络看到、并操作你 Mac 上的 tmux 会话。这天然是一个
敞口。本文说明：token 意味着什么、隧道能看到什么、以及怎么把风险控制在你能接受的范围。

## 1. token = 密码（最重要的一条）

`gtmux serve` 的 **master token 或已配对 owner 设备的 token，可以在你的 Mac 上执行命令**：
`POST /api/send` 会把内容 `tmux send-keys` 进窗格，这就是远程代码执行（RCE）。

- 像对待密码一样对待它：泄露就等于别人能在你机器上跑命令。
- master token 存在 `~/.config/gtmux/serve-token`（`0600`），不要贴进聊天/截图/issue。
- 当前局域网和隧道配对都优先使用一次性配对码（见 §3）。旧版 v1 二维码载荷含长期 bearer token；
  菜单栏配对面板在铸码失败时可能回退到该格式，因此不能把所有显示出来的二维码都当成五分钟过期的凭证。
  `gtmux pair` 则要求新配对码，铸码失败时直接报错。

## 2. 隧道能看到什么（单层 TLS，无 E2E）

Standard Cloudflare 隧道的链路是：

```
手机 ──TLS──> Cloudflare 边缘 ──加密隧道──> 你 Mac 上的 cloudflared ──loopback──> gtmux serve(127.0.0.1)
```

- TLS 在 Cloudflare 边缘终止。
  也就是说：**Cloudflare 在边缘能看到明文 API 流量**（窗格内容、你发送的输入）。目前没有
  应用层端到端加密，你信任这条链路，等于信任 Cloudflare + 托管控制面（`api.gtmux.ccy.dev`）。
- 对 Standard，控制面负责开通或修复 Mac 注册的隧道，并返回连接器 token。它不代理会话流量，
  流量走 Cloudflare 隧道本身；这条在代理处终止 TLS 的路径会让边缘看到明文。
- Direct / 自托管走另一条路径：客户端通过 HTTPS 连接所选服务器的反向代理，再经 chisel 回到 Mac。
  该服务器的运营者同样能看到 API 流量；换隧道后端不会增加应用层端到端加密。
  见[自托管隧道配置](../../deploy/self-tunnel/README.md)。
- 推送单独经过所配置的 APNs 中继和 Apple。中继收到通知内容、元数据，以及启用时的 Live Activity 更新。
  关闭某台 Mac 的通知，需要设置传到那台 Mac 后才会停止转发；Mac 离线时，App 的更改可能仍待同步。
- 这是 D1=(a) 的明确取舍。要消除「边缘可见明文」，需要 §5 的 E2E（未做）。

## 3. 一次性配对码 ≠ token

浏览器/手机配对链接里的 `…/#c=<code>`：

- 五分钟后、兑换一次后或 serve 重启后失效。有效期内，它可以注册拥有全权的 owner 设备，因此同样要保密。
- 在 URL 的 fragment（`#` 之后），浏览器不会把它发给服务器，只有前端 JS 读它去换取
  本设备自己的 per-device token。
- 用它配对后，撤销某台设备不影响其它设备（master token 仍有效）。

## 4. 把风险降到可接受的实操建议

- 「局域网」模式让会话 API 流量直达 Mac；推送转发是独立路径，仍可能经过托管中继。
- 在菜单栏的远程访问设置或 `gtmux tunnel --status` 查看状态。用 `gtmux tunnel --unservice`
  停止后台远程访问服务；若隧道还在终端前台运行，也需要在那里停止。
- 丢了手机就撤销那台设备（per-device token 可撤销）。
- 自托管 Standard 控制面时，把 `GTMUX_TUNNEL_API` 设为自己 Worker 的 URL，
  `GTMUX_TUNNEL_REG` 设为它的注册门槛值。同时把 `GTMUX_TUNNEL_API_FALLBACK` 设为同一 URL
  或自己的备用入口；只改主入口仍会保留托管服务的备用入口。推送则配置 serve 的 `--relay-url` / `--relay-token`。
  见[provisioner README](../../tunnel-worker/README.md)和[relay README](../../relay/README.md)。
- 不要把 token 或其他敏感信息放进 URL query。

## 5. 未来（backlog，未做）

- 临时密钥配对（D1 选项 b）。一次性配对码已实现，它缩短凭证的有效期，但不提供 API 流量的端到端加密。
- 应用层端到端加密 + 零知识中继（D1 选项 c）：让会话和通知内容经过 §2 中的中间服务时仍保持加密。
  这需要新协议和配套客户端，当前隧道及推送中继并不具备这一能力。

## 6. 已有的 provisioner 防护

Standard provisioner 已有尽力而为的每 IP / 全局新建上限和定时闲置隧道回收；阈值与保留边界见
[provisioner README](../../tunnel-worker/README.md)。`x-gtmux-reg` 仍只是软门槛，并非客户端私密凭证。
