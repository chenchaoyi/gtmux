# gtmux — Privacy Policy / 隐私政策

_Last updated / 最后更新: 2026-10-06_

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

**gtmux** ("gtmux", "the app", "we") is a companion app for monitoring and steering coding-agent sessions running on **your own Mac**. Agent execution and session storage stay on that Mac. Remote connections, notifications, and hosted tunnel registration involve additional data flows described below. The app has no analytics SDK or advertising and does not require a gtmux sign-in.

### Data on your devices

- **Agents, terminals, and chat history.** `gtmux serve` on your Mac supplies the live terminal view and conversation history to your paired device. The app sends terminal input and attached images back to that Mac for the running program to use. Across networks, this traffic can pass through a hosted or self-hosted tunnel.
- **Paired Macs and credentials.** The app stores server addresses and access tokens in your device's Keychain. Your Mac keeps its enrolled-device roster and registered push tokens so pairing and notifications can survive a restart.
- **Camera and images.** The camera can scan a pairing QR code or take a photo to attach to a message. An attached image is uploaded to your Mac; the camera is not limited to QR scanning.

### Data sent to services

- **Notifications and Live Activities.** Your Mac sends the gtmux push relay an Apple push token and notification data, which the relay forwards to Apple Push Notification service (APNs). Data can include your Mac's display name, pane identifier, task or session title, status, and the text of choices shown by an agent. These fields can contain text from your work, including command or path fragments. Live Activity updates include activity titles, status counts and timing; silent updates carry badge and notification-dismissal information. This is more than a generic status line, though the push path does not send an entire terminal transcript or conversation history.
- **Remote access.** Standard tunnels use Cloudflare; hosted Direct uses gtmux-operated tunnel infrastructure. These services carry the connection between your device and Mac, including API requests, terminal input/output and attachments. A self-hosted tunnel uses the infrastructure you choose.
- **Hosted tunnel registration.** The registration service receives a generated device identifier and Mac name. It stores the tunnel identifier and hostname for Standard, and per-device account credentials, assigned port, access code and server assignment for Direct. Provisioning also uses IP-based rate-limit counters. These are service records, separate from the session data on your Mac.

- **Direct purchases.** If you buy through Lemon Squeezy, that provider processes the checkout. The order-confirmation service receives the order payload and stores an order-identifier-to-access-code association and claim time. If the code pool is empty, its error log includes the order identifier and purchaser email so the delivery failure can be investigated.

### Use and storage

The app does not use analytics or advertising SDKs, sell data, or share it for marketing. Pairing does not require a personal gtmux account; hosted tunnels still need the service records above. The push relay forwards notifications rather than providing a cloud notification-history feature. It is inaccurate to describe all hosted services as storing nothing: tunnel registration records are persistent, and Apple and hosting providers process data as part of delivering their services.

### Removing a connection and stopping notifications

Remove a saved Mac in the app to remove that pairing from the app's saved server list. This also attempts to unregister its push tokens; if the Mac cannot be reached, removal does not confirm that the Mac has stopped sending. On the Mac, use `gtmux devices` and `gtmux devices revoke <id>` to revoke a device and its associated push registrations. Disabling notifications in system settings controls alerts on that device; it does not itself delete hosted tunnel records. Uninstalling the app is not a substitute for revoking a lost device or deleting server-side records. Contact us about deletion of hosted service records.

### Third parties

Apple delivers notifications through APNs. Cloudflare and the Direct hosting infrastructure provide the optional hosted connections and registration services described above. Lemon Squeezy processes purchases made through its checkout. Their handling of service data is governed by their own applicable policies.

### Children

gtmux is a developer tool and is not directed at children under 13.

### Contact

Questions about this policy or hosted service records: **gtmux@ccy.dev**.

---

## 中文

**gtmux**（“gtmux”、“本应用”、“我们”）是一款配套应用，用于监看并操作你**自己 Mac 上**运行的编码 agent 会话。Agent 的运行和会话存储在这台 Mac 上；远程连接、通知和托管隧道注册还涉及下述数据流。本应用不含分析统计或广告 SDK，也不要求登录个人 gtmux 账号。

### 设备上的数据

- **Agent、终端与对话历史。** Mac 上的 `gtmux serve` 向已配对设备提供实时终端视图和对话历史。应用把终端输入和附加图片发回 Mac，交给运行中的程序处理。跨网时，这些流量可能经过托管或自建隧道。
- **配对的 Mac 与凭据。** 应用把服务器地址和访问 token 存在设备的钥匙串中。Mac 保存已配对设备清单和已注册的推送 token，以便重启后继续使用配对和通知。
- **相机与图片。** 相机既可扫描配对二维码，也可拍摄要附在消息中的照片。附加图片会上传到你的 Mac；相机用途并不限于扫码。

### 发往服务的数据

- **通知与实时活动。** Mac 向 gtmux 推送中继发送 Apple 推送 token 和通知数据，中继再转发给 Apple 推送服务（APNs）。数据可能包含 Mac 显示名称、pane 标识、任务或会话标题、状态，以及 agent 显示的选项文字。这些字段可能含有工作内容，包括命令或路径片段。实时活动更新包含活动标题、状态计数和时间信息；静默更新包含角标和通知消除信息。因此，推送不只是通用状态短句，但这条推送路径不会发送完整终端记录或对话历史。
- **远程访问。** Standard 隧道使用 Cloudflare；托管 Direct 使用 gtmux 运营的隧道基础设施。这些服务承载设备与 Mac 之间的连接，包括 API 请求、终端输入输出和附件。自建隧道使用你选择的基础设施。
- **托管隧道注册。** 注册服务接收生成的设备标识和 Mac 名称。Standard 保存隧道标识和主机名；Direct 保存每设备的账号凭据、分配端口、访问码和服务器分配。注册还使用按 IP 计数的限流记录。这些服务记录与 Mac 上的会话数据是不同的数据。

- **Direct 购买。** 通过 Lemon Squeezy 购买时，由该服务商处理结账。订单确认服务会收到订单载荷，并保存订单标识与访问码的对应关系及领码时间。若码池为空，错误日志会包含订单标识和购买者邮箱，以便排查未交付问题。

### 数据用途与存储

本应用不使用分析或广告 SDK、不出售数据，也不为营销共享数据。配对不要求个人 gtmux 账号，但托管隧道仍需要上述服务记录。推送中继负责转发通知，不提供云端通知历史功能。不能把所有托管服务概括为“不保存任何数据”：隧道注册记录会持久保存，Apple 和托管服务商也会为提供服务而处理数据。

### 移除连接与停止通知

在应用中移除一台已保存的 Mac，会从应用的服务器清单中移除该配对，并尝试向 Mac 注销推送 token；若 Mac 不可达，移除操作并不能确认 Mac 已停止发送。在 Mac 上可用 `gtmux devices` 查看设备，再用 `gtmux devices revoke <id>` 撤销设备及其关联推送注册。系统设置中关闭通知控制的是该设备上的提醒，不会因此删除托管隧道记录。卸载应用不能替代撤销丢失设备的凭据或删除服务端记录。如需删除托管服务记录，请联系我们。

### 第三方

Apple 通过 APNs 投递通知。Cloudflare 和 Direct 托管基础设施提供上述可选的托管连接与注册服务。Lemon Squeezy 处理通过其结账页面进行的购买。它们按照各自适用的政策处理服务数据。

### 儿童

gtmux 是一款开发者工具，不面向 13 岁以下儿童。

### 联系方式

关于本政策或托管服务记录的问题：**gtmux@ccy.dev**。
