# HANDOFF — gtmux 手机远程联动(branch: `claude/tmux-agent-remote-app-kgk3u3`)

> **给在用户 MacBook 上接手的 Claude Code:** 这份文档是你的工作交接。后端(Go)
> 已在云端容器里写好并通过 `make check`,但 **RN 手机 app、真机 APNs 推送、菜单栏
> app 的 Swift 改动都需要在这台 Mac 上才能构建/验证** —— 那是你接手的原因。先读
> 本文件,再读下面「必读契约」,然后按 §5 runbook 执行。

## 0. 这是什么

把 gtmux 从「本地 CLI + macOS 菜单栏 app」扩展出**第三块屏:手机**。用户外出时经
**公司 VPN / Tailscale** 用手机查看挂在 tmux 里的一排 coding agent 的状态,并在 agent
需要输入 / 跑完时收到**锁屏推送**。架构:

```
手机 app (bare RN)  ──HTTPS over VPN──>  gtmux serve (Mac, 只读 HTTP+SSE)
       ▲                                        │ 出站
       └──────── APNs(经 Apple,不依赖 VPN)◄── 推送中继 relay (持 APNs key)
```

完整设计与战略判断见 plan;权威实现状态见本文件 §2。

## 1. 必读契约(动手前先读,别偏离)

- `api/contract.md` —— `v0` HTTP/SSE 契约,server↔app 的边界单一事实源。
- `mobileapp/SPEC.md` —— 手机 app 的权威构建蓝图(你建 RN app 就照它)。
- `docs/design/DESIGN.md` §0–§3 —— 状态语言(色+形+字形)与布局规则。
- `macapp/Sources/GtmuxBar/AgentStore.swift` + `Theme.swift` —— 现有消费者。
  **手机端要镜像它**:同一个 `Agent` 形状、同一套状态色/形/字形、同一分区顺序。
- `relay/README.md` —— 中继的部署、env、HTTP 契约。

## 2. 已完成 vs 待办(诚实的状态)

| 增量 | 内容 | 状态 |
|---|---|---|
| 1 | `gtmux serve` 只读后端:`/api/health`、`/api/agents`(与 `agents --json` 字节一致)、`/api/pane?id=%N`、`POST /api/focus?id=%N`;全 `/api/*` 走常量时间 Bearer token | ✅ `make check` 绿 + 容器内 smoke 过 |
| 2 | `GET /api/events` SSE:~1500ms diff 循环,广播 `agents{rev}` / `alert{pane,kind,…}` / `ping`。alert 由 server 自有 diff 派生(**不** drain `notify/`) | ✅ `make check` 绿 + smoke 过 |
| 3 | 推送:`PushManager`(token 持久化 `~/.config/gtmux/push-tokens.json` 0600)、`POST /api/push/register`、`HTTPRelay` 转发;独立 `relay/` 服务(真 APNs:ES256 JWT + HTTP/2) | ✅ 单测绿(APNs 打**假** Apple);**真机/真 Apple 链路未验证** |
| 4 | 手机 app | ⏳ **只有 `mobileapp/SPEC.md` 蓝图,RN 工程尚未生成** —— 你来建 |
| — | 菜单栏 app「允许手机访问」开关 + 画配对二维码(QR 生产端,Swift) | ⏳ 未做;QR schema 已在 `mobileapp/SPEC.md §6` 定死 |

**新增文件地图**:`internal/server/{server,events,push}.go`(+ 测试)、`internal/app/serve.go`、
`api/contract.md`、`relay/{relay,apns,main}.go`+`README.md`、`mobileapp/{SPEC.md,README.md,.gitignore}`。

## 3. 护栏(不可破坏)

- **CLI 必须 cgo-free**:`CGO_ENABLED=0 go build ./cmd/gtmux` 必须过;`internal/` 不得引入 cgo。
- **每次提交前跑 `make check`**(= gofmt + vet + staticcheck + `go test -race`)。
- **只读不变量**:MVP 不加任何写终端 / 执行命令的端点(`focus` 只本地选中 pane,无 RCE)。
  `send-keys` / 语音是后续 Phase,别提前做。
- **机密绝不入仓**:APNs `.p8`、Key ID、Team ID、relay token 全走环境变量。
- **契约不破**:`gtmux agents --json` 形状、状态路径、bundle id `com.gtmux.menubar`、
  `api/contract.md v0`。改契约要升版本。
- **分支纪律**:在 `claude/tmux-agent-remote-app-kgk3u3` 上开发,**别提交到 `main`**;
  branch → PR → CI 绿 → squash-merge。
- **改菜单栏 app 前先读 `docs/design/DESIGN.md`**(权威设计规范)。

## 4. 哪里需要人(用户)出面

- **Apple Developer 账号**($99/年):建 App ID(bundle id + Push 能力)、APNs Auth Key(`.p8` + Key ID + Team ID)。真机推送的前置。
- **一台真机 iPhone**:相机扫码 + APNs 推送在模拟器上都不可用。
- **决策**:先建 app(§5 阶段 5)还是先搞真机推送(阶段 4);要不要现在做菜单栏 QR 生产端。
  拿不准就用 `AskUserQuestion` 问用户,别擅自展开大改。

## 5. 执行 runbook(逐阶段,过了检查点再走)

### 阶段 1 · 构建 + 门禁
```sh
make check
make build                              # → ./gtmux
CGO_ENABLED=0 go build ./cmd/gtmux      # 确认 cgo-free
```
✅ `make check` 全绿。

### 阶段 2 · 起后端,本机 curl(无需 Apple)
先确保 tmux 里有几个 agent 在跑。
```sh
./gtmux serve --port 8765 --bind 127.0.0.1      # 打印 token;也在 ~/.config/gtmux/serve-token
# 另一终端:
TOKEN=$(cat ~/.config/gtmux/serve-token)
curl -s localhost:8765/api/health; echo
curl -s -H "Authorization: Bearer $TOKEN" localhost:8765/api/agents | jq .
curl -s -H "Authorization: Bearer $TOKEN" "localhost:8765/api/pane?id=%2512" | jq .   # %25='%'
curl -sN -H "Authorization: Bearer $TOKEN" localhost:8765/api/events                  # 留着看 alert
```
✅ `/api/agents` 与 `./gtmux agents --json` 一致;改某 agent 状态时 `/api/events` 冒出 `alert`。

### 阶段 3 · 手机浏览器先验证「连得上」
```sh
ipconfig getifaddr en0          # Mac 的内网/VPN IP
./gtmux serve --port 8765       # 默认 --bind 0.0.0.0
```
手机连同一 Wi-Fi/VPN,浏览器开 `http://<IP>:8765/api/health`。
✅ 看到 `{"status":"ok"}`。打不开 → 客户端隔离/防火墙,改 Tailscale 兜底。

### 阶段 4 · 🍎 中继 + 真机推送(需 Apple,见 §4)
```sh
go build -o relay ./relay
PORT=8080 GTMUX_RELAY_TOKEN=$(openssl rand -hex 16) \
APNS_KEY_PATH=/secure/AuthKey_XXXX.p8 APNS_KEY_ID=XXXX APNS_TEAM_ID=YYYY \
APNS_TOPIC=com.gtmux.app APNS_ENV=sandbox ./relay        # 细节见 relay/README.md
# serve 指向中继:
./gtmux serve --port 8765 --relay-url http://127.0.0.1:8080/push --relay-token <RELAY_TOKEN>
```
✅ `curl :8080/health` ok;启动日志 `relay: APNs enabled`。端到端推送要等阶段 5 装上 app 拿到 device token。

### 阶段 5 · 生成并跑 RN app(你的主要工作)
**完整照 `mobileapp/SPEC.md`**。概要:
```sh
npx @react-native-community/cli@latest init GtmuxMobile --directory mobileapp --pm npm
cd mobileapp
npm i @react-navigation/native @react-navigation/native-stack react-native-screens \
      react-native-safe-area-context react-native-sse react-native-webview \
      react-native-keychain react-native-vision-camera \
      @react-native-community/push-notification-ios @react-native-async-storage/async-storage
cd ios && pod install && cd ..
npx react-native run-ios        # 模拟器用「手动输入 host+token」配对(阶段 2 的 token)
```
按 SPEC 实现 Pairing / Radar / Detail / Settings 四屏,状态语言严格对齐菜单栏 app。
✅ app 里雷达的色/形/字形/分区顺序与菜单栏一致;详情看到 pane 内容;状态实时更新;
真机后台 alert → 锁屏推送 → 点击跳到对应 agent(手机离线 VPN 也收得到)。

> ⚠️ `init` 会在 `mobileapp/` 生成 `ios/`、`android/`、`package.json` 等;`mobileapp/.gitignore`
> 已忽略 `node_modules/`、`ios/Pods/` 等生成物。把源码 + 锁文件提交,别提交构建产物。

## 6. 建议顺序

阶段 1→2→3 一口气跑通(半小时、零 Apple 依赖,确认 Go 后端在本机 work)。然后**先做阶段 5
把雷达跑出来**(成就感最大、零账号),真机推送(阶段 4🍎)留最后。菜单栏 QR 生产端按需再做。

每完成一块:`make check`(Go 侧)/ RN 构建过 → 提交到本分支 → 按需开 PR。卡住就把报错贴给用户。
