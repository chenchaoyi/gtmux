# 随处远程访问 —— 隧道设计（2026-06-22）

手机如何**在任意网络、不装 VPN app** 的情况下连上 Mac 上的 agent 雷达。
这是「A1 托管隧道」架构及其决策的权威记录。改 `gtmux tunnel`、
`tunnel-worker/` Worker 或远程访问文档之前，先读这份。

## 问题

`gtmux serve` 在 Mac 上暴露一个只读雷达（HTTP+SSE），token 把关。手机要用，
得先能在网络上够到这台 Mac。三种情形：

- **同一 Wi-Fi** —— 配对到局域网 IP。零配置，但只在家里/办公室有效。
- **Mesh VPN（Tailscale）** —— 哪里都能用，安全性最强（端到端，什么都不公开）。
  但手机上要装 VPN app，而 **Tailscale 一般不在中国大陆 App Store 上架**，
  所以不能作为我们用户的默认方案。
- **任意网络、不装 VPN app** —— 本设计要填的空。

左右选择的约束：

- **必须对*所有*用户可用**，不只是维护者本人 —— 所以「自带域名 / VPS」不能当默认，
  需要托管的基础设施。
- **不能影响 iOS app 的 App Store 上架**（全球 + 中国）。
- **中国大陆可达性**要紧（维护者的用户在这里）。

## 为什么是「出站反向隧道」（不是入站，也不是自建中继）

Mac **主动向外**拨到一个会合点；手机通过公网 URL 到达同一会合点；会合点把两边接起来。
出站意味着**不开入站端口、不需要公网 IP、NAT 问题自动消失**。Cloudflare **免费、全球**
运营这个会合点（它的隧道边缘），所以我们**不**自建、不托管数据中继，只跑一个很小的
*控制面*，负责请 Cloudflare 创建隧道。

否决的备选：

- **自建数据中继** —— 最重：一个由我们运行、付带宽费、还看得到全部流量的有状态桥。
  Cloudflare 已经免费提供了数据面。
- **快速隧道当默认**（`trycloudflare.com`）—— 零基础设施，但 URL **每次运行都换**，
  手机得不停重新配对，而重新配对需要人在 Mac 跟前 —— 这就违背了「人离开 Mac 还能用手机看」。
  保留为 `--quick`，供临时/测试用。
- **每个用户自己的域名（命名隧道）** —— 高级用户很好用，普通用户做不到
  （要 CF 账号 + 域名 + DNS）。托管解决的正是这个。

## 架构（A1：托管命名隧道）

```
gtmux tunnel (Mac)            api.gtmux.ccy.dev (Worker)          Cloudflare API
  │ POST /provision {deviceId} ─────▶ create cfd_tunnel ────────────▶ tunnel
  │   header x-gtmux-reg               set ingress → localhost:8765
  │                                    create DNS <id>.gtmux.ccy.dev
  │ ◀── { url, token } ────────────────┘
  │ cloudflared tunnel run --token <token>     (outbound, http2 — QUIC is often blocked)
  ▼
https://<id>.gtmux.ccy.dev ─CF edge─▶ tunnel ─▶ Mac's gtmux serve :8765
                                                 ▲ phone pairs to this URL, ONCE
```

两个平面，两条信任边界：

- **控制面** —— `tunnel-worker/`，部署在 `api.gtmux.ccy.dev` 的 Cloudflare Worker。
  gtmux 唯一运营的部分。`POST /provision` 幂等地（按每台 Mac 的 `deviceId` 键）创建
  Cloudflare **命名**隧道，把 ingress 指向 `localhost:8765`，创建 DNS 路由，返回连接器
  token。KV（`TUNNELS`）记录 `deviceId → {tunnelId, hostname}`，重跑时复用同一条隧道。
- **数据面** —— Cloudflare 的隧道边缘。Mac 上的 `cloudflared` 向外连到它；手机访问
  `https://gtmux-<id>.ccy.dev`。gtmux 完全不碰这一层。

**iOS app 不变** —— 它照旧配对到一份 `{url, token}` 载荷。传输方式（局域网 / Tailscale /
隧道）对它不可见，所以这套设计**对 App Store 零影响**。

## 稳定地址 = 只配对一次（这就是全部意义）

托管的主机名对每台 Mac 是**稳定**的（`deviceId` 持久化在
`~/.config/gtmux/tunnel-device-id`；provision 幂等）。所以手机**只配对一次**，之后跨
`gtmux tunnel` 重启、跨 Mac 重启都继续有效 —— 不像快速隧道那样 URL 会换。
「重启后仍然在线」还需要一个 launchd 服务（见*尚未构建*）。

## 命名 + 单级 TLS 约束（重要）

用户隧道是**单级**的：`gtmux-<id>.ccy.dev`，**不是** `<id>.gtmux.ccy.dev`。

Cloudflare 免费的 **Universal SSL 只覆盖一级子域名**（`ccy.dev` 与 `*.ccy.dev`）。
像 `<id>.gtmux.ccy.dev` 这样的三级主机名**拿不到边缘证书** → TLS 握手失败。
`*.gtmux.ccy.dev` 的通配证书要付费的 Advanced Certificate Manager。所以隧道保持单级，
用 `gtmux-` **前缀**而不是 `gtmux.` 标签来划命名空间。控制面 Worker（`api.gtmux.ccy.dev`）
用的是 Workers **custom domain**，不论层级深浅都会签发自己的专用证书（首次部署要等几分钟）。

## 安全模型

- **两层相互独立的 token：**
  - **连接器 token**（cloudflared ↔ CF）—— 授权这台 Mac 作为该隧道的连接器。
    由 `/provision` 返回，通过 `cloudflared tunnel run --token` 使用。
  - **serve bearer token**（手机 ↔ serve，穿过隧道端到端）—— 现有的
    `~/.config/gtmux/serve-token`。每条 `/api/*` 路由都校验它（没 token → 401），
    **走公网 URL 时一字不变**。
- **一旦有了公网 URL，bearer token 就是只读雷达的唯一门槛** —— 前面不再有 VPN 层。
  API 仍是只读的（没有 `send-keys`，没有输入注入），但要把 URL + token 当密码看待；
  别把配对二维码截图发到共享渠道。CLI 输出里有这句提醒。
- **`x-gtmux-reg` 软门槛** —— CLI 发给 `/provision` 的注册密钥。它必然随二进制一起发布
  （发版构建时从 CI secret `GTMUX_TUNNEL_REG` 注入），所以**不是**真正的秘密，只是拦一下
  对该端点的随手滥用。真正的防护是下面的加固项。
- **隐私** —— Cloudflare 在其边缘终止 TLS，因此能看到雷达流量明文（任何 CF 隧道都如此）。
  对只读的 pane 元数据可以接受；应用层端到端加密是可能的后续增量。

## gtmux 运营什么（归属 + 成本）

- Cloudflare 上的 `ccy.dev` zone + `gtmux-tunnel` Worker + `TUNNELS` KV。
- Worker 里的 secret：`CF_API_TOKEN`（限定范围：`ccy.dev` DNS:Edit + 账号级
  Cloudflare Tunnel:Edit）和 `REG_SECRET`。
- 成本 ≈ 域名费；这个规模下 Workers + KV + Cloudflare Tunnel 都在免费额度内。
  带宽由 Cloudflare 承担（无出口流量费）。
- **中心化风险** —— 这套基础设施一停，托管远程访问就停。所以**自带路径继续支持**
  （Tailscale；`--quick`；以及 `GTMUX_TUNNEL_API` / `GTMUX_TUNNEL_REG` 覆盖项，
  让自托管者指向自己的 Worker）。

## 自托管

`gtmux tunnel` 运行时读取 `GTMUX_TUNNEL_API` 和 `GTMUX_TUNNEL_REG`（覆盖构建期默认值）。
把 `tunnel-worker/` 部署到你自己的 zone，设好这两个变量，CLI 就走你的控制面而不是 gtmux 的。

## 测试注意 —— 企业 DNS 劫持

在做**透明 DNS 劫持 + 按域名分类**的网络上（例如维护者的办公室，连 `8.8.8.8`/`1.1.1.1`
的应答都会被改写成内网 `172.19.2.x` 代理 IP），**全新的 `ccy.dev` 主机名会被改坏**，
直到代理完成分类为止，所以最后一跳「公网主机名 → 隧道」**在那个网络上没法用 curl 验证**。
控制面（provision）和 Mac→CF 这半段（cloudflared 注册成功）在那里可以验证；最后一跳要从
**走蜂窝/家庭网络的手机**上验（正常网络打到的是真实 CF IP）。这是网络环境的产物，
不是设计缺陷，对真实用户没有影响。

## 常驻（显式选择）

默认情况下 `gtmux tunnel` 在**前台**运行 —— 你有意识地为一次会话打开远程访问，Ctrl-C 即停。
稳定 URL 已经保证手动重启不用重新配对。**常驻**（跨重启可达、不用再手动运行）是一个独立的、
可选的、可逆的模式 —— 永远不是默认，因为一处长期存在的公网暴露应当是有意识的选择，且始终可见：

- `gtmux tunnel --service` —— 先 provision 稳定隧道，再注册两个每用户的 **LaunchAgent**
  （`com.gtmux.serve` → 回环上的 `gtmux serve`；`com.gtmux.tunnel` → 带连接器 token 的
  `cloudflared`），`RunAtLoad` + `KeepAlive`。它会说明这意味着长期暴露，并先征求同意
  （`--yes` 跳过提示 —— 菜单栏开关用这个，它有自己的确认）。
- `gtmux tunnel --unservice` —— 卸载并删除两个 agent。
- `gtmux tunnel --status` —— 开/关 + 稳定 URL。
- 连接器 token 放在隧道 plist 里（0600）。菜单栏 app 提供开/关切换和可见指示，
  常驻永远不会悄无声息。

## 尚未构建（已跟踪）

- **防滥用加固** —— 每 `deviceId` 上限、回收 N 天未用的隧道、`DELETE /provision`、限速。
  `x-gtmux-reg` 门槛只是拦一下。
- **应用层端到端加密** —— 让 Cloudflare 看不到雷达明文。
- **菜单栏「允许手机访问」** —— 由 app 直接生成配对二维码，数据来自隧道地址。

## 后端：Cloudflare（默认）vs 自托管（P1）

`gtmux tunnel` 的后端可插拔（`--backend cloudflare|self`，或 `GTMUX_TUNNEL_BACKEND`）：

- **`cloudflare`**（默认）—— 上面的零配置托管地址。大多数网络能用，但敌意网络可以对
  Cloudflare 的边缘（`*.argotunnel.com`）做 DNS 劫持，无论什么协议都会被掐断（见调试手册）。
- **`self`** —— 走 443 的 WebSocket 隧道（Chisel），连到**你自己的 VPS + 域名**，
  与普通 HTTPS 无法区分，所以能扛住那种劫持。服务端由你自己跑（专用 VPS 上 chisel + Caddy
  做 TLS）—— 版本化的配置和安装/迁移脚本见 **`deploy/self-tunnel/`**。配置是手动的
  （毕竟是你自己的服务器）：`GTMUX_SELFTUNNEL_URL`（`https://tunnel.example.com`）+
  `GTMUX_SELFTUNNEL_SECRET`（chisel 的 `user:pass`）。客户端是**进程内运行的 jpillora/chisel
  库** —— Mac 上没有独立二进制（不用下载、不用管理，也不会多出一个文件让终端安全扫描器标成
  双用途黑客工具；就是 gtmux 自己签过名的二进制发起一条出站 WebSocket）。常驻模式
  `--service` 把它跑成 `gtmux tunnel-client` LaunchAgent，secret 从 `selftunnel.conf` 读
  （所以永远不出现在 plist 或 `ps` 里）。手机配对 `{url, token}` 的方式与 Cloudflare 完全一样。

P1 是手动选择；Cloudflare→self 的自动切换和双 URL 配对二维码是 P2
（见 `openspec/changes/.../self-hosted-tunnel`）。隧道是计划中的付费档。

## 调试手册（配对 / 可达性）

已汇总到 `docs/TROUBLESHOOTING.md`；本子系统的要点：

1. **「配对码已过期」怎么也清不掉 → :8765 上有重复的 serve。** 菜单栏通过
   `127.0.0.1:8765`（IPv4）铸码，而隧道的 `localhost:8765` 解析到 `::1`（IPv6）；
   若有第二个 `gtmux serve` 绑了 `*:8765`，铸码和兑换会打到不同进程，注册码（内存态）
   对不上。检查：`lsof -nP -iTCP:8765 -sTCP:LISTEN` 必须只有一个 PID（app 的
   `com.gtmux.serve`）。杀掉任何裸跑的、占着 `*:8765` 的 `gtmux serve`。
2. **铸码和扫码之间别重启 serve** —— 码在内存里（TTL 5 分钟）；
   `launchctl kickstart`/`unload+load` 会清掉它 → 「已过期」。
3. **公司网络上隧道离线 = QUIC 被封。** `tunnel.log` 反复出现 `failed to dial to edge
   with quic`；手机看到 CF 1033/530。解法 = `--protocol http2`（现已是默认；
   `GTMUX_TUNNEL_PROTOCOL` 可覆盖）。旧的服务 plist 仍是 QUIC —— `gtmux update` 之后重跑
   `gtmux tunnel --service`。
4. **公司 DNS 劫持**把 `ccy.dev` 改写到 `172.19.x` → 隧道明明健康，Mac 自己的探测却失败；
   用**走蜂窝的手机**验。

## 代码地图

| 组件 | 位置 |
|---|---|
| CLI 托管 + 快速模式、cloudflared 运行器、二维码 | `internal/app/tunnel.go` |
| 构建期 API URL + 注册门槛（可用环境变量覆盖） | `internal/app/tunnelconfig.go` |
| 控制面 Worker（经 CF API 做 provision） | `tunnel-worker/src/index.ts` |
| 部署配置（账号/zone/KV id、域名） | `tunnel-worker/wrangler.toml` |
| 注册门槛注入 | `Makefile`、`.goreleaser.yaml`、`.github/workflows/release.yml` |
| 能力规格 | `openspec/specs/remote-access/spec.md` |
