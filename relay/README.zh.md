# gtmux 推送 relay

[English](README.md)

这是供自建的 Go 参考实现。项目线上 `gtmux-relay.ccy.dev` 使用的是
`relay-worker/` 中的 Cloudflare Worker；修改 Go 版本不会改变线上服务。
两份实现共用 `api/testdata/apns-contract.json` 校验 APNs 请求格式。

## 用途

iOS 应用关闭后，通过 Apple APNs 接收等待输入、完成提醒、静默角标和 Live Activity 更新。
路径是 Mac 的 `gtmux serve` → 持有 APNs 密钥的 relay → Apple → 手机。
通知可能包含任务、pane、选项、命令或路径，并非只有抽象状态；自建仍会经过 Apple，
也需要应用 bundle id 对应的 Apple Developer 凭据。打开会话界面则另需能访问 Mac。

Go 与 Worker 均支持普通通知、静默角标、Live Activity update/end。
请求的 `env` 优先选择 sandbox 或 production，缺省时使用 relay 的 `APNS_ENV`。
APNs 拒绝状态以结构化错误返回，serve 因此能停用失效 token。
合同测试验证请求格式，不代表已验证 Apple 实际投递。

## 运行

在仓库根目录构建，用自己的凭据替换占位符：

```sh
go build -o ./bin/gtmux-relay ./relay
PORT=8080 \
GTMUX_RELAY_TOKEN=<shared-secret> \
APNS_KEY_PATH=/secrets/AuthKey_XXXX.p8 \
APNS_KEY_ID=XXXXXXXXXX \
APNS_TEAM_ID=YYYYYYYYYY \
APNS_TOPIC=com.gtmux.app \
APNS_ENV=production \
./bin/gtmux-relay
```

Mac 使用 `gtmux serve --relay-url https://<relay-host>/push --relay-token <shared-secret>`。
密钥只从环境指定的路径读取，不要提交 `.p8` 或真实标识。

| 环境变量 | 含义 |
| --- | --- |
| `PORT` | 监听端口，默认 8080 |
| `GTMUX_RELAY_TOKEN` | 可选；设置后 `/push` 要求对应 bearer |
| `APNS_KEY_PATH` | PKCS#8 EC P-256 `.p8` 文件路径 |
| `APNS_KEY_ID` | Apple 密钥标识 |
| `APNS_TEAM_ID` | Apple Developer 团队标识 |
| `APNS_TOPIC` | 应用 bundle id |
| `APNS_ENV` | 请求未带 `env` 时的默认值；sandbox 为开发环境，其余为生产环境 |

未配置 APNs 凭据时仍能启动并返回健康状态，但 iOS 推送返回不支持该平台。

## HTTP 合同

`GET /health` 返回 `200 {"service":"gtmux-relay","status":"ok"}`。

`POST /push` 在配置 token 后要求 `Authorization: Bearer <GTMUX_RELAY_TOKEN>`。
请求包含 `token`、`platform`（默认 ios）、`title`、`body`、`pane`、`kind`；可选字段为
`subtitle`、`options`、`silent`、`badge`、`collapseId`、`env`、`liveActivity`、`event`、
`contentState`、`staleDate`。

成功返回 `200 {"status":"ok"}`；无 token 或坏请求返回 400，错误 bearer 返回 401。
APNs 拒绝返回 `502 {"error":"apns","status":410,"detail":"…"}`，其中 status 保留 Apple
原状态；传输或其他网关错误返回 `502 {"error":"push failed: …"}`。

当前 ios 使用 APNs。Android（FCM）、HarmonyOS（HMS）可通过 `Pusher` 接口扩展，
尚未实现对应网关。
