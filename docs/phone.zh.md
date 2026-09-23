# 移动端与远程访问

[English](phone.md) · **中文**

<img src="assets/screenshot-detail.png" width="200" align="right" alt="gtmux 手机端：pane 实时屏幕 + 回复" />

gtmux 有一个 iOS app：同一块 agent 雷达装进手机，agent 需要你或者跑完的那一刻推到锁屏。
可以彩色查看某个 pane 的实时屏幕，回一句话，发控制键（`Enter`、`Ctrl-C` 等），附一张截图。
跑在 tmux 之外的 agent 和菜单栏里一样，只读地列在「不在 tmux」分区里：没有 pane，
所以不能跳过去，也不能回复。

app 连的是 Mac 上的 `gtmux serve`，推送走 Apple 的通知服务。

```sh
gtmux serve --port 8765          # 打印 token 和能连的地址
```

然后配对：跑 `gtmux pair`，扫它打印的二维码（菜单栏 app 的 ⚙︎ → 配对设备… 里是同一个码），
或者手输地址和 token。可以存多台 Mac，在连接页切换（点雷达顶栏的服务器名）。

## 不用开终端：菜单栏 app 里有同样的开关

<img src="assets/menubar-remote.png" width="418" alt="菜单栏偏好设置，远程访问：关 / 局域网 / 任意网络，隧道 Standard / Direct" />

下面讲的远程访问设置，在菜单栏 app 里都是两下点击：点 gtmux 状态图标，⚙︎ → 偏好设置 → 远程访问。
同一个三档开关（关 / 局域网 / 任意网络），任意网络下的隧道类型（Standard / Direct），
开着的时候还显示能连的地址。⚙︎ → 配对设备… 直接给一次性配对二维码或配对码，
远程访问没开的话，它会先带你打开。偏好设置里的「分享」分区管的就是 `gtmux share` 那套访客链接。

配对过的手机（owner 设备）可以远程管分享：「管理这台 Mac」页面能签发、复制、吊销和
`gtmux share` 同一套的访客链接（按 pane 给看、给输入），也能看已配对设备清单，不用走回 Mac 前。
两件事只能在 Mac 上做：吊销一台已配对设备，以及开关远程访问，这样手机丢了，捡到的人也没法给
这台机器重新配钥匙。访客连接看不到这个页面。

<img src="assets/screenshot-servers.png" width="220" alt="gtmux 连接页：已存服务器，切换 / 添加 / 移除" />

在哪儿能做什么，取决于两件事：

- 推送到哪儿都收得到。任何网络（蜂窝、家里 Wi-Fi）都能到，哪怕手机根本连不上那台 Mac。
  Mac 在公司、你在家，「需要你」「跑完了」照样收到。
- 实时视图（雷达、读 pane、focus）需要一条能到 Mac 的网络路径。同一个局域网里直接就行；
  换了网络就要开远程访问，见下。

## 从任意网络：`gtmux tunnel`（推荐）

Mac 主动向外建一条隧道，所以不用开入站端口，NAT 也不碍事。隧道客户端（`cloudflared`）
只跑在 Mac 上，手机只是打开一个普通的 `https://…` 地址。

```sh
gtmux tunnel                  # Standard：稳定的托管地址，配对一次就行
gtmux tunnel --backend self   # Direct：走 gtmux 自己的服务器（付费，见 --redeem）
gtmux tunnel --quick          # 不要账号的临时地址（每次跑都变）
gtmux tunnel --service        # 重启后继续开着（--unservice / --status）
```

它会拉起雷达服务（还没起的话），打开隧道，打印公网地址、token 和配对二维码，另外还有一条
「在电脑上打开」的链接，指向只读网页镜像（浏览器里看雷达和某个 pane，不用装 app）。
手机 app 里「添加服务器 → 扫码」，任何网络下都连上了。没装 `cloudflared` 的话，它会问你要不要
`brew install`。

「任意网络」有两种：

- Standard（默认）：免费、零配置。每台 Mac 拿到一个稳定的 `https://<id>.gtmux.ccy.dev`，
  手机配对一次，重启后照样能用。你这边不需要账号，也不需要域名。
- Direct（`--backend self`）：走 gtmux 自己的服务器、443 端口的隧道，给连不上 Standard 隧道的
  严格网络用（部分公司网）。付费解锁：在 <https://ccy.pub/projects/gtmux/direct> 拿访问码，
  用 `gtmux tunnel --redeem <码>` 兑换（或者在菜单栏「任意网络 → Direct」里按提示输入），
  之后用 `--backend self`。每台 Mac 有自己的地址 `https://tunnel.ccy.dev/p<port>`，在服务器上
  也有自己的账号，只能用到这个地址；一个码最多解锁三台 Mac，同一台 Mac 重复兑换没关系。
  Direct 服务器可能不止一台：`gtmux tunnel --servers` 列出有哪些，以及从这台 Mac 实测的延迟，
  `gtmux tunnel --server <id>` 把这台 Mac 换到另一台，码不变、端口也不变。之前连上来过的手机
  会自己跟过来；只扫过码、还没连上来过的设备要重新扫一次，换之前发出的分享链接会失效。
  想跑自己的服务器，用 `GTMUX_SELFTUNNEL_URL` + `GTMUX_SELFTUNNEL_SECRET` 指过去，
  搭建见仓库里的 `deploy/self-tunnel/`。
- `--quick`：什么都不用配，但 `trycloudflare.com` 的地址每次跑都换，每次都得重新配对。
  临时看一眼可以，长期开着不行。

重启后继续开着：`gtmux tunnel --service`（或者菜单栏的「任意网络」开关）把它注册成后台服务；
`--unservice` 关掉，`--status` 看状态。MacBook 合盖就睡，隧道随之断掉；`gtmux awake on` 让 Mac、
隧道和手机在合盖后照常工作（`gtmux awake off` 不用密码，详见 [`cli.zh.md` → `gtmux awake`](cli.zh.md)）。

想自己托管隧道服务的贡献者：`GTMUX_TUNNEL_API` / `GTMUX_TUNNEL_REG` 把 `gtmux tunnel`
指向你自己的实例，见 [`design/remote-access-tunnel.zh.md`](design/remote-access-tunnel.zh.md)。

## 从任意网络：Tailscale 或任何 VPN

如果你的设备之间已经有 Tailscale（或别的 VPN），直接用也行，还能绕开公司 Wi-Fi 的客户端隔离。
Mac 和 iPhone 都装上 Tailscale（Mac 上 `brew install --cask tailscale` 或 App Store），
同一个账号登录，`tailscale ip -4` 拿到 Mac 的地址（一个 `100.x.y.z`），
把 app 配对到 `http://<那个地址>:8765` 加 serve token，其他都不变。

> 同一个 Wi-Fi 也连不上 Mac？公司和访客 Wi-Fi 常常把客户端互相隔离。快速确认：
> 在手机浏览器里打开 `http://<mac-ip>:8765/api/health`，打不开就用 `gtmux tunnel`（或 VPN）。

## 在 iPad 上：侧栏在旁，正事在中间

同一个 app，同一个商店条目。窗口只要有 768×600pt（iPad 任何方向、2/3 分屏、Stage Manager 里差不多大的窗口），
雷达就变成侧栏，你打开的东西（某个会话、gtmux HQ、所有 pane）在旁边的主区里显示；不跳页，点另一行主区就换。
比这窄的（1/2 分屏、Slide Over）就是手机的排布。

- 侧栏可以收起（齿轮旁边的按钮，或 ⌃⌘S），下次还记得。
- 对话按舒服的宽度读；终端占满整个主区。
- HQ 页左边是对话，右边是等你拍板的事和 HQ 做过的事。
- HQ 的知识库（它攒下的经验条目）打开时，列表在左、正文在右。

接了键盘，按住 ⌘ 就能看到全部命令。值得记的几个：

| 按键 | 做什么 |
|---|---|
| ↑ ↓ ⏎ | 在雷达里上下移动、打开选中的会话 |
| ⌘1 … ⌘9 | 打开第 n 行 |
| ⌘⇧H · ⌘⇧P | gtmux HQ · 所有 pane |
| ⌘F | 搜 pane |
| ⌘K | 输入 |
| ⌘[ · ⌘] | 对话 · 终端 |
| ⌘= · ⌘− | 字号 |
| esc | 关掉弹层 |

## 从另一台电脑的终端：`gtmux attach`

手机 app 是看和遥控；在另一台 Mac 或 Linux 的终端上可以更进一步，直接在远端会话里干活：

```sh
gtmux attach http://<mac>:8765 --token <serve-token> %12   # owner（局域网或隧道）
gtmux attach 'https://<mac>.example/#g=<token>' %12        # 访客（分享链接）
```

你本地的 Ghostty / iTerm2 / 终端就变成那个远端 tmux pane，可交互，全屏程序也照常，
走的是和手机同一条连接。`gtmux pair` 还会打印一条现成的 `gtmux attach` 命令，把那个终端登记成
你自己的设备，之后直接 `gtmux attach <host>` 就行。访客只能看到、输入主机放行的 pane
（只读的 pane 就是只读），和网页、手机同一套范围。范围在菜单栏的「分享」分区或者 `gtmux share` 里设：

```sh
gtmux share new --label 张三 --view %1,%2 --type %1 --expires 24h   # 一条链接，自己的范围
gtmux share set <id> --type %2        # 改某一条
gtmux share revoke <id>               # 吊销
```

退出用 tmux 的 `<prefix> d` 或 `Ctrl-]`。完整参考见 [`cli.zh.md` → `gtmux attach`](cli.zh.md) 和
[`design/remote-attach-research.zh.md`](design/remote-attach-research.zh.md)。

## 安全

远程能做的事里，除了往 pane 里打字，全是只读的；而打字这一项只靠配对 token 把关。
公网隧道地址前面没有 VPN 兜底，地址和 token 都拿到的人就能往你 Mac 里打字，
所以**把地址加 token 当密码看待**，别把配对二维码截图发进公共频道。访客链接的范围窄得多
（只有你选的 pane，可设过期，输入还要 `gtmux share on` 放行），随时可以吊销。

出了问题时，手机上留着记录：请求 Mac 失败的情况、每次配对以及失败原因、推送注册、实时
连接的断开和恢复，只存在手机上。「设置 → 诊断」里可以拷贝或分享；Mac 那一侧用
`gtmux doctor --bundle` 打包。

完整协议，贡献者可以看仓库里的 `api/contract.md`。
