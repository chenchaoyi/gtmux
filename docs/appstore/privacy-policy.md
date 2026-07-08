# gtmux — Privacy Policy / 隐私政策

_Last updated / 最后更新: 2026-07-08_

Canonical content for the ccy.dev / ccy.pub privacy pages (App Store URLs). Mirror the
rodi setup: one Astro page under `src/pages/projects/gtmux/`, with a `data-i18n="en"`
and a `data-i18n="zh"` article; ccy.dev leads with English, ccy.pub (SITE_TARGET=cn)
with Chinese.

- **Privacy Policy URL (en):** `https://ccy.dev/projects/gtmux/privacy`
- **Privacy Policy URL (zh / CN storefront):** `https://ccy.pub/projects/gtmux/privacy`
- **Support URL:** `https://ccy.dev/projects/gtmux/support` · `https://ccy.pub/projects/gtmux/support`
- **Marketing URL:** `https://ccy.dev/projects/gtmux`

---

## English

**gtmux** ("gtmux", "the app", "we") is a companion app for monitoring and steering
your own coding-agent sessions running on **your own Mac**. This policy explains what
the app does and does not do with your data. The short version: **your data stays
between your phone and your own Mac. We run no analytics, no accounts, and no server
that holds your content. The only thing that reaches a gtmux-run service is a push
notification — a short status line plus your device's push token.**

### What stays on your devices

- **Your agents, terminals, and chat history.** The app reads a live view of the
  coding agents running in tmux on your Mac. That data is served by `gtmux serve`
  **on your Mac** and travels directly to your phone over your own network (or a
  tunnel you run). It is never sent to us or stored on any gtmux server.
- **The Macs you pair.** Each server you add is stored **on your phone** as an address
  plus an access token. We never receive them.
- **Terminal input you send** (a reply, a keypress) and **photos you attach** go
  **to your Mac** (via its local API) so an agent can act on them. They are not sent
  to us.
- **Camera.** Used only to scan a pairing QR code. Nothing from the camera is stored
  or transmitted.

### What is transmitted, and to whom

- **Push notifications.** So your phone can alert you when an agent needs you (even
  when the app is closed), your Mac sends the **gtmux push relay** a short message —
  a title and one-line body such as "Claude Code needs you" — together with your
  device's **push token** (a random per-install identifier issued by Apple). The relay
  forwards it to **Apple Push Notification service (APNs)**, which delivers it to your
  phone. The relay sees **only** the token and that short status line: **no agent
  output, no terminal content, no chat history, no conversation.** It does not retain
  your notifications.
- **Remote access transport.** When you enable "Anywhere" access, the connection
  between your phone and your Mac is carried over an outbound tunnel (Cloudflare, or a
  gtmux-run relay). This is transport only — it moves the same phone↔Mac traffic
  described above; we do not inspect or store its contents.

### What we do NOT do

- **No analytics or telemetry.** The app contains no analytics SDK. We do not track
  you, profile you, or measure your usage.
- **No accounts.** There is no gtmux sign-in. Pairing is a per-device token you and
  your Mac exchange directly.
- **No advertising, no data sale, no data sharing** for marketing.
- **No central store of your data.** gtmux has no server that holds your agents,
  code, terminals, or messages — that all lives on your own devices.

### Third parties

- **Apple Push Notification service (APNs)** delivers push notifications, per Apple's
  own terms.
- The **gtmux push relay** and the optional **tunnel** run on infrastructure (Cloudflare
  Workers / a VPS). They carry the transient data described above and store none of it
  persistently, aside from the push token your Mac registers so it can be reached.

### Data retention & deletion

Your data lives on your devices; delete the app or remove a paired server to remove it
from the phone. To stop push, disable notifications for the app (or unpair) — your
Mac then stops registering your token with the relay.

### Children

gtmux is a developer tool and is not directed at children under 13.

### Contact

Questions about this policy: **ccy.chenchaoyi@gmail.com**.

---

## 中文

**gtmux**（“gtmux”、“本应用”、“我们”）是一款配套应用，用于监看并操作你**自己 Mac 上**运行的
coding agent 会话。本政策说明本应用如何处理你的数据。简短版本是：**你的数据只在你的手机与你自己的
Mac 之间流转。我们不做任何分析统计、不设账号、也没有任何服务器保存你的内容。唯一会到达 gtmux
所运营服务的，只有一条推送通知 —— 一句简短状态文字，加上你设备的推送 token。**

### 哪些数据留在你的设备上

- **你的 agent、终端与对话历史。** 应用读取你 Mac 上 tmux 里运行的 coding agent 的实时视图。
  这些数据由你 Mac 上的 `gtmux serve` 提供，通过你自己的网络（或你自建的隧道）**直接**传到你的手机，
  绝不发送给我们、也不存放在任何 gtmux 服务器上。
- **你配对的 Mac。** 你添加的每台服务器，作为“地址 + 访问 token”保存在**你的手机上**，我们收不到。
- **你发送的终端输入**（回复、按键）和**你附加的照片**，是发到**你的 Mac**（经其本地接口），
  好让 agent 处理，不会发给我们。
- **相机。** 仅用于扫描配对二维码，相机内容不被保存或传输。

### 哪些数据会被传输、传给谁

- **推送通知。** 为了在 agent 需要你时（即使应用已关闭）提醒你的手机，你的 Mac 会向 **gtmux 推送中继**
  发送一条简短消息 —— 标题和一行正文，例如“Claude Code 需要你”—— 连同你设备的**推送 token**
  （由 Apple 分配的、随安装生成的随机标识）。中继再转发给 **Apple 推送服务（APNs）**送达你的手机。
  中继**只**看到 token 和那句简短状态：**没有 agent 输出、没有终端内容、没有对话历史。** 它不保留你的通知。
- **远程访问传输。** 当你开启“任意网络”访问时，手机与 Mac 之间的连接经由出站隧道（Cloudflare，
  或 gtmux 运营的中继）承载。这只是传输通道，搬运的就是上面所述的手机↔Mac 流量；我们不查看、不存储其内容。

### 我们不做的事

- **不做任何分析或遥测。** 应用不含任何分析 SDK。我们不追踪你、不给你画像、不统计你的使用。
- **没有账号。** gtmux 没有登录。配对是你和你的 Mac 直接交换的、每设备独立的 token。
- **没有广告、不出售数据、不为营销共享数据。**
- **不集中保存你的数据。** gtmux 没有保存你 agent、代码、终端或消息的服务器 —— 这些都只在你自己的设备上。

### 第三方

- **Apple 推送服务（APNs）** 负责送达推送通知，遵循 Apple 自身条款。
- **gtmux 推送中继**和可选的**隧道**运行在基础设施上（Cloudflare Workers / 一台 VPS）。它们承载上述
  临时数据，除了为可达而由你 Mac 注册的推送 token 外，不做持久化存储。

### 数据保留与删除

你的数据存于你的设备；删除应用或移除已配对的服务器即可将其从手机移除。要停止推送，关闭该应用的通知
（或解除配对）—— 你的 Mac 随即停止向中继注册你的 token。

### 儿童

gtmux 是一款开发者工具，不面向 13 岁以下儿童。

### 联系方式

关于本政策的问题：**ccy.chenchaoyi@gmail.com**。
