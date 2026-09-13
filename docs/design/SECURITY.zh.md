# gtmux 远程访问的信任边界 (Security model)

> 决策 D1=(a)（2026-06-28）：维持「单层 TLS 隧道 + bearer token」，**不做端到端加密**，
> 但把信任边界写清楚。QR 配对 / E2E 记为 backlog（见 `DECISIONS-FOR-CCY.md` D1）。

gtmux 的远程功能让手机/浏览器隔着网络看到、并**操作**你 Mac 上的 tmux 会话。这天然是一个
敞口。本文说明：token 意味着什么、隧道能看到什么、以及怎么把风险控制在你能接受的范围。

## 1. token = 密码（最重要的一条）

`gtmux serve` / `gtmux tunnel` 的 **bearer token 等于一把可在你 Mac 上执行命令的钥匙**：
`POST /api/send` 会把内容 `tmux send-keys` 进窗格——这就是远程代码执行（RCE）。

- **像对待密码一样对待它。** 泄露 = 别人能在你机器上跑命令。
- token 存在 `~/.config/gtmux/serve-token`（`0600`），不要贴进聊天/截图/issue。
- 配对二维码里**短期内**含 token（局域网直连场景）；隧道场景用的是**一次性配对码**而非 token（见 §3）。

## 2. 隧道能看到什么（单层 TLS，无 E2E）

托管「任意网络」隧道（`gtmux tunnel`，Anywhere 模式）的链路是：

```
手机 ──TLS──> Cloudflare 边缘 ──加密隧道──> 你 Mac 上的 cloudflared ──loopback──> gtmux serve(127.0.0.1)
```

- TLS 在 **Cloudflare 边缘终止**（单层 Universal SSL，覆盖 `ccy.dev`/`*.ccy.dev`）。
  也就是说：**Cloudflare 在边缘能看到明文 API 流量**（窗格内容、你发送的输入）。目前**没有
  应用层端到端加密**——你信任这条链路，等于信任 Cloudflare + 托管控制面（`api.gtmux.ccy.dev`）。
- 控制面只做「按 deviceId 发一条 per-Mac 命名隧道 + 返回 connector token」；它不代理你的会话流量
  （流量走 Cloudflare 隧道本身），但**边缘可见明文**这一点对任何反向代理隧道都成立。
- **推送**经托管 APNs 中继转发：中继**能看到通知内容**（agent 名 / 任务文本）。不想暴露就关推送。
- 这是 D1=(a) 的明确取舍。要消除「边缘可见明文」，需要 §5 的 E2E（未做）。

## 3. 一次性配对码 ≠ token

浏览器/手机配对链接里的 `…/#c=<code>`：

- 是**一次性、5 分钟过期、单次使用**的 enroll code，**不是**长期 token。
- 在 URL 的 **fragment（`#` 之后）**，浏览器**不会**把它发给服务器——只有前端 JS 读它去换取
  本设备自己的 per-device token。
- 用它配对后，**撤销某台设备**不影响其它设备（master token 仍有效）。

## 4. 把风险降到可接受的实操建议

- **可信网络优先用 Wi-Fi 模式**（`serve`，局域网直连，**完全不走云**）；Anywhere 隧道留给真在外面时。
- **Anywhere 是长期敞口**：菜单栏有「远程开启」绿色指示，且现在还有「设备正在查看」指示
  （v0.11.4），不让它静默。不用了就 `gtmux tunnel --unservice`。
- **丢了手机就撤销那台设备**（per-device token 可撤销）。
- **不信任 ccy.dev 托管控制面/中继？自托管**：`GTMUX_TUNNEL_API` / `GTMUX_TUNNEL_REG` 指向你自己的
  Worker；relay 同理。（见 `docs/design/remote-access-tunnel.md`、[[hosted-tunnel-a1]]）
- 默认选最隐私的：cookie/consent 拒非必要；不把敏感信息放进 URL query。

## 5. 未来（backlog，未做）

- **QR 配对 + 临时密钥对**（D1 选项 b）：把 token 不再以明文落在二维码/链接里。
- **应用层端到端加密 + 零知识中继**（D1 选项 c）：让 Cloudflare 边缘 / 中继**都看不到明文**
  （对标 Happy 的做法）。这是消除 §2「边缘可见明文」的唯一办法。
- 隧道滥用加固（per-device 上限/回收/限流）——目前 `x-gtmux-reg` 只是软门槛。
