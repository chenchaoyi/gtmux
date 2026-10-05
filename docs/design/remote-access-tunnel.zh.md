# 随处远程访问 —— 隧道设计（2026-06-22）

本文记录 2026 年六月选择 A1 托管隧道的原因，并汇总截至 2026-10-06 的实现。
下面的六月背景属于历史决策；架构和交付状态章节描述当前代码。改 `gtmux tunnel`、
`tunnel-worker/` 或远程访问文档之前，先读这份。

## 最初的问题与目标（2026 年六月）

最初的雷达需要一个手机可达的 Mac 地址，又不要求每个用户运行 VPS 或安装手机 VPN。
当时比较了局域网地址、Tailscale 等 mesh VPN 和出站托管隧道。支持中国大陆用户、
保持 iOS App 可发行是设计目标，不能据此认定某种传输适用于所有网络，或对 App Store 审核没有影响。

当前 `gtmux serve` 通过 HTTP、SSE 和 WebSocket 提供读取与获授权的控制；
owner 凭证可以发送终端输入、创建会话，已不再是只读雷达。局域网访问也需要可达的接口和
防火墙规则，仅连接同名 Wi-Fi 并不能证明可达。

## 为什么选择出站反向隧道（六月决策）

Mac 向外连接一个会合点，手机通过它提供的 HTTPS URL 访问 Mac。这样不需要在 Mac 上
开放入站端口，也不需要 Mac 自身有公网 IP；但 Mac 的出站连接和手机到公网入口的路径
仍须被各自网络允许。

Standard 后端由 Cloudflare 提供隧道数据面，gtmux 提供开通控制面。
当时没有把自建数据中继选作默认，是因为它增加服务器和带宽运维；后来交付的 Direct 后端
确实使用另行运营的服务器。

快速隧道（`--quick`）保留作临时访问，URL 是临时的，变化后客户端需要更新地址。
要求每个用户自带域名和 Cloudflare 账号也没有被选作默认；Standard 的托管开通代为提供这些资源。

## 架构（A1：托管命名隧道）

```
gtmux tunnel (Mac)            api.gtmux.ccy.dev (Worker)          Cloudflare API
  │ POST /provision {deviceId} ─────▶ create cfd_tunnel ────────────▶ tunnel
  │   header x-gtmux-reg               set ingress → localhost:8765
  │                                    create DNS gtmux-<id>.ccy.dev
  │ ◀── { url, token } ────────────────┘
  │ cloudflared tunnel run --token <token>     (outbound, http2 — QUIC is often blocked)
  ▼
https://gtmux-<id>.ccy.dev ─CF edge─▶ tunnel ─▶ Mac's gtmux serve :8765
                                                 ▲ phone pairs through this URL
```

两个平面，两条信任边界：

- **控制面** —— `tunnel-worker/`，部署在 `api.gtmux.ccy.dev` 的 Cloudflare Worker。
  `POST /provision` 幂等地（按每台 Mac 的 `deviceId` 键）创建
  Cloudflare **命名**隧道，把 ingress 指向 `localhost:8765`，创建 DNS 路由，返回连接器
  token。KV（`TUNNELS`）记录 `deviceId → {tunnelId, label, hostname}`，重跑时复用同一条隧道。
- **数据面** —— Cloudflare 的隧道边缘。Mac 上的 `cloudflared` 向外连到它；手机访问
  `https://gtmux-<id>.ccy.dev`。provisioner 不代理这条流量；这里的 `<id>` 是随机标签，
  不是 Mac 的 `deviceId`。

当前配对通常使用 `{v:2, url, enrollCode}`（可附显示名称），App 用码换取自己的 token；
旧版 v1 `{url, token}` 配对仍受支持。局域网、Standard 和 Direct 都访问同一套 API，
但这一传输设计本身不能说明 App Store 审核或隐私分类的结论。

## 正常重启时保持地址稳定

Mac 的 `deviceId` 持久化在 `~/.config/gtmux/tunnel-device-id`。复用现有 Standard 注册时，
正常重启保持原地址。确认旧隧道已被删除后重建，会分配新的随机标签并改变地址。
回收器默认回收闲置超过 90 天、或创建超过 24 小时却从未连接过的隧道；被撤销的设备凭证也需要重新配对。
`--service` 已实现后台启动（见下文“常驻”）；固定 URL 本身不会让 Mac 保持唤醒、联网或登录。

## 命名与 TLS 覆盖

用户隧道使用 `gtmux-<id>.ccy.dev`，不是 `<id>.gtmux.ccy.dev`。
仓库中的 `ZONE_NAME` 为 `ccy.dev`，自托管时应换成自己的 zone。

Cloudflare 的完整 DNS 接入模式下，Universal SSL 覆盖根域名和一级子域名；更深的名称需要
另行配置证书覆盖，部分 CNAME 接入模式的规则不同。见
[Cloudflare Universal SSL 限制](https://developers.cloudflare.com/ssl/edge-certificates/universal-ssl/limitations/)。
`wrangler.toml` 的控制面路由使用 Workers Custom Domain，Cloudflare 会为它签发证书，
也支持多级名称；见 [Custom Domains](https://developers.cloudflare.com/workers/configuration/routing/custom-domains/)。
这些是配置要求，不是对生产账号证书状态的回读证明。

## 安全模型

- **两层独立的凭证：** `/provision` 返回的连接器 token 授权 `cloudflared` 把 Mac 连到隧道；
  serve bearer 凭证授权客户端的 API 请求。已配对设备通常使用自己的 token，而不是 Mac 的 master token。
- **API 包含写操作。** owner 凭证和有效的 owner 配对码都应当作 Mac 终端的钥匙保管。
  `/api/health` 公开，`/api/enroll` 用码本身作凭证，不要求 bearer token；其他 API 路由
  校验 bearer，并各自应用调用者权限。权限细节见 [API 合同](../../api/contract.md)。
- **`x-gtmux-reg` 是软门槛。** 发版时注册值注入二进制，因此不是客户端私密凭证。
  Worker 还有新建上限和闲置隧道回收，见下文。
- **TLS 不等于应用层 E2E。** Cloudflare 终止 Standard 的 TLS，可以看到终端输出、
  提交的输入等会话 API 流量；Direct 的 TLS 代理运营者同样在信任路径中。
  推送另走中继路径。见[安全模型](SECURITY.zh.md)。

## 运营组件与自托管

Standard 使用 `ccy.dev` zone、`gtmux-tunnel` Worker 和 `TUNNELS` KV。Worker 需要
`CF_API_TOKEN`（zone DNS:Edit 和账号级 Cloudflare Tunnel:Edit）及 `REG_SECRET`。
它还提供 Direct 开通路由；Direct 隧道服务器和 APNs 推送中继是另行运营的组件。
原始设计按免费额度估算成本，不是对当前用量或账单的保证。

托管控制面不可用时，开通和修复可能失败；已经运行的隧道数据路径与控制面分开。
Cloudflare 或所选 Direct 服务器不可用，则可能中断现有远程访问。

自托管 Standard 控制面时，用自己的账号、zone、路由和 KV ID 部署
[tunnel-worker](../../tunnel-worker/README.md)，并配置 CLI 的三个值：

- `GTMUX_TUNNEL_API`：自己的 Worker URL。
- `GTMUX_TUNNEL_REG`：它的注册门槛值，不是 URL。
- `GTMUX_TUNNEL_API_FALLBACK`：自己的备用 URL；设为与主 URL 相同可省略第二入口。
  只改主入口仍保留托管备用入口；环境变量设为空会回到编译时默认值。

前台 shell 的环境变量不是 launchd 服务配置。使用后台服务时，要确保服务进程收到所需配置；
不能假设只在安装终端 export 的变量会在下次登录后保留。

## 网络核验

DNS 劫持和网络策略既会影响测试，也会影响真实用户。开通响应成功，或 Mac 到边缘的连接器
注册成功，都不能证明手机到公网主机名的路径可用。最后一跳需要从实际客户端网络验证。
与另一个网络比较有助于定位，但蜂窝网络成功不能证明受限办公网络也能到达。

## 常驻（显式选择）

全新配置下，`gtmux tunnel` 在**前台**运行，Ctrl-C 停止这条前台隧道。
若常驻隧道已经加载，命令改为打印已保存的配对地址后退出；URL 文件缺失时则提示运行
`gtmux tunnel --status`。后台远程访问需要显式开启，
在用户登录时启动，并依赖网络可用：

- `gtmux tunnel --service` —— 对 Standard，先开通隧道，再注册两个每用户的 **LaunchAgent**
  （`com.gtmux.serve` → 回环上的 `gtmux serve`；`com.gtmux.tunnel` → 带连接器 token 的
  `cloudflared`），`RunAtLoad` + `KeepAlive`。它会说明这意味着长期暴露，并先征求同意
  （`--yes` 跳过提示 —— 菜单栏开关用这个，它有自己的确认）。
- `gtmux tunnel --unservice` —— 卸载并删除共用的 serve agent 和任一后端的隧道 agent；
  不会停止另行启动的前台隧道。
- `gtmux tunnel --status` —— 开/关 + 稳定 URL。
- 连接器 token 放在隧道 plist 里（0600）。菜单栏 App 提供远程访问控制和状态。
  未显式指定后端时，重跑 `--service` 保持已安装的 Direct；
  `--backend cloudflare --service` 则显式选择 Standard。

## 已交付防护与剩余工作

Standard provisioner 已有尽力而为的每 IP / 全局新建上限，以及每日闲置隧道回收。
回收会移除符合条件的隧道和 DNS 资源，不删除注册 KV 记录。仓库中的阈值见
[provisioner README](../../tunnel-worker/README.md)。目前没有 `DELETE /provision` 路由。
菜单栏配对面板和常驻控制已交付；应用层端到端加密仍未实现。

## 后端：Standard 与 Direct / 自托管

`--backend cloudflare|self` 或 `GTMUX_TUNNEL_BACKEND` 选择后端，全新配置默认 Cloudflare。
两者都依赖 DNS、入口可达性及网络策略；Direct 使用 HTTPS 不等于无法被阻断。

- **Standard（`cloudflare`）** 使用上面的托管 Cloudflare 路径。
- **Direct / 自托管（`self`）** 用进程内 jpillora/chisel 客户端连接所选 HTTPS 服务器，
  Mac 不需要独立 chisel 客户端程序。服务器的 TLS 代理把每台 Mac 的 `/p<port>` 路径
  经 chisel 转发到 serve；版本化的 Caddy/nginx 配置见
  [自托管配置](../../deploy/self-tunnel/README.md)。使用自己的服务器时，配置
  `GTMUX_SELFTUNNEL_URL` 和 `GTMUX_SELFTUNNEL_SECRET`（`user:pass`），或把它们
  存入 `~/.config/gtmux/selftunnel.conf`。安装的 `gtmux tunnel-client` 服务也需要收到这份配置，
  生成的 plist 不含 secret。托管 Direct 通过 `gtmux tunnel --redeem <code>` 获取每台 Mac 的账号。

[七月自托管 proposal](../../openspec/changes/archive/2026-07-12-self-hosted-tunnel/proposal.md)
记录了 P1 及当时延期的工作。此后已交付进程内 chisel、付费 Direct 兑换、服务器选择和
配对客户端的路由发现。二维码仍只放一个地址；配对客户端通过 `/api/addresses` 学习备用
Direct 地址。这不是 Cloudflare 到 Direct 的自动切换。原计划中的跨后端故障切换和双 URL
二维码仍未实现。

## 调试手册（配对 / 可达性）

带日期的故障记录和详细检查见 [TROUBLESHOOTING.md](../TROUBLESHOOTING.md)。当前可从这里查起：

1. **注册码兑换失败。** 可能已过期、已使用，或在 serve 重启前铸造。等 serve 稳定后生成新码。
   仍失败时，可用 `lsof -nP -iTCP:8765 -sTCP:LISTEN` 查冲突监听；IPv4 和 IPv6 路径应到达
   同一个预期 serve。停止进程前先确认归属。
2. **连接器反复报 QUIC 错误。** 检查选择的协议。gtmux 默认 `http2`，可用
   `GTMUX_TUNNEL_PROTOCOL` 覆盖；旧 plist 保留旧参数，直到重新安装服务。
   仅看到“离线”不能诊断为 QUIC 被封。
3. **开通成功，手机却连不上。** 除连接器状态外，也要从手机网络检查 DNS 和公网
   `/api/health` 路径。另一网络成功有助于定位，不代表第一个网络已经恢复。

## 代码地图

| 组件 | 位置 |
|---|---|
| CLI 托管 + 快速模式、cloudflared 运行器、二维码 | `internal/app/tunnel.go` |
| 构建期 API URL + 注册门槛（可用环境变量覆盖） | `internal/app/tunnelconfig.go` |
| 控制面 Worker（经 CF API 做 provision） | `tunnel-worker/src/index.ts` |
| 部署配置（账号/zone/KV id、域名） | `tunnel-worker/wrangler.toml` |
| 注册门槛注入 | `Makefile`、`.goreleaser.yaml`、`.github/workflows/release.yml` |
| 能力规格 | `openspec/specs/remote-access/spec.md` |
