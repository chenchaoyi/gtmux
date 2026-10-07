# App Store 提交流程（手机 + iPad）

维护者手册。每次提交商店按顺序走一遍；上传后要读回版本、build 和截图。
下面的命令块都从仓库根目录运行，子 shell 的 `cd` 不改变外层目录。

## 0. 前提

- 版本号由 `git describe --tags --abbrev=0` 选择当前提交可达的最近 tag，并去掉 `v`：发布流程确定 tag 后，
  运行 `bash mobileapp/scripts/set-version.sh`，更新 `src/version.ts`、pbxproj 的 `MARKETING_VERSION`
  和 `src/releaseNotes.ts`。说明与上一份不同时，才归档两种语言的 What's New 并记录源码哈希。
  改动提 PR 合进 main；戳版本后运行 `check-design.sh`，其中更新说明检查会比较当前源码与归档时的哈希。
- 在运行 fastlane / ASC 脚本的进程中配置 `ASC_KEY_ID`、`ASC_ISSUER_ID`、`ASC_KEY_PATH`；
  本仓库的发布流程使用 **Team** key。不要把某台机器的 shell profile 路径当作通用加载步骤。
- fastlane 要比系统 ruby 新的版本：`brew install ruby` 并把它放进 PATH，然后 `(cd mobileapp && bundle install)`。
  非交互的 shell 通常不会自动加载 profile，所以脚本里要显式 export PATH 和那三个 ASC 变量。
- 2026-09-12 起 app 是通用的（iPhone + iPad，三个 target 都是 `TARGETED_DEVICE_FAMILY = "1,2"`）：
  需要准备 13" iPad 槽位截图。尺寸以
  [Apple 截图规格](https://developer.apple.com/help/app-store-connect/reference/app-information/screenshot-specifications/)为准。

## 1. 文字

`mobileapp/fastlane/metadata/{en-US,zh-Hans}/`：

| 文件 | 说明 |
|---|---|
| `description.txt` | 商店描述；两种语言各写，不互译。iPad 那一行在 ALSO / 此外 里。 |
| `release_notes.txt` | What's New；戳版本时归档成 `release-notes/<ver>.{en,zh}.txt`，app 里的更新弹窗读的是归档。 |
| `keywords.txt` `subtitle.txt` `promotional_text.txt` | 一般不动。 |

## 2. 截图

两组，都从演示模式画出来，都在同一个语言目录里（deliver 按尺寸分槽位）：

| 槽位 | 尺寸 | 文件 | 来源 |
|---|---|---|---|
| iPhone 6.9" | 1320×2868 | `fastlane/screenshots/<locale>/01..07.png` | `appstore-shots` e2e（iPhone 17 Pro **Max** 模拟器，整条流程见 `docs/appstore-shots.md`）+ 锁屏那张是画的（`render-lockscreen.mjs`） |
| iPad 13" 横屏 | 2752×2064 | `fastlane/screenshots/<locale>/ipad-01..04.png` | `appstore-shots-ipad` e2e（iPad Pro 13" 模拟器） |

生成 iPad 那组（两种语言从同一台模拟器出，`GTMUX_DEBUG_LANG` 强制 app 语言）：

```sh
(
  set -eu
  cd mobileapp
  : "${AUDIT_IPAD_UDID:?指定本次专用、已经启动的 iPad 模拟器 UDID}"
  GTMUX_E2E_UDID="$AUDIT_IPAD_UDID" npm run e2e:build
  for L in en zh; do
    GTMUX_DEMO_SHOTS=1 GTMUX_SHOTS_LANG="$L" GTMUX_E2E_UDID="$AUDIT_IPAD_UDID" \
      GTMUX_E2E_DEVICE='iPad Pro 13-inch (M5)' npm run test:e2e -- appstore-shots-ipad.test.ts
  done
  node scripts/frame-shots.mjs --slot ipad --in .e2e-artifacts/appstore/ipad-en --lang ipad-en --out fastlane/screenshots/en-US --prefix ipad-
  node scripts/frame-shots.mjs --slot ipad --in .e2e-artifacts/appstore/ipad-zh --lang ipad-zh --out fastlane/screenshots/zh-Hans --prefix ipad-
)
```

文案在 `scripts/shot-captions.json`（`ipad-en` / `ipad-zh` 两组）。**每张都要打开看过再提交。**
横屏这一张的转向踩过两次坑：以前 `simctl` 给的是竖向缓冲，采集脚本无条件转 270° 补偿；iOS 26.5
给回来的已经是正的，再转一次整组就躺倒了（2026-09-18）。现在脚本先问图片实际是横是竖，只转竖的
那种；仍需检查实际方向，不能把宽高判断当成所有系统都已验过。要是哪天又歪了，查
`appstore-shots-ipad.test.ts` 里的 `shot()`。采集从未配对页进入演示；这份脚本不传 `RESET_SERVERS`，
重装也不保证清空 Keychain，因此每轮开始前要确认起始页面，且输出目录没有混入上轮图片。

手机和 iPad 两组共用同一批 demo 数据；重拍完记得把 README 配图也重出一遍
（`GTMUX_ONLY=readme bash docs/assets/screenshots/regenerate.sh`），它读的是同一个目录。

## 3. 构建与上传二进制

```sh
(
  set -eu
  cd mobileapp
  : "${ASC_KEY_ID:?先配置 ASC Team key}" "${ASC_ISSUER_ID:?先配置 issuer}" "${ASC_KEY_PATH:?先配置 key 路径}"
  bundle exec fastlane release
)
```

`release` lane 自己构建、归档、导出并上传，不需要先手工 `archive`。它从 ASC 取下一构建号，
读取工程中的版本，使用仓库配置的 team；商店包不传 `APS_ENVIRONMENT` 覆盖，Release 默认是 production。
已有 IPA 可在 `mobileapp/` 运行 `bundle exec fastlane upload ipa:"${STORE_IPA:?指定已导出的 IPA 路径}"`。
这两条 lane 都不等待处理完成，也不提交审核。

本地真机开发包的 `build`、development 推送和安装步骤见 [mobileapp README](../../mobileapp/README.md)。
安装时明确选一台测试设备的 UDID，不能把 `idevice_id -l` 的多行结果直接当一台设备。
产物叫 `gtmux.app`，iPad 和 iPhone 装同一个包。
2026-09-15 曾因 devicectl 挂载开发者磁盘镜像受阻而改用 ideviceinstaller；这是一次环境排障记录，
不保证锁屏设备、重启后的设备或任意系统版本都能安装。

## 4. 推文字和截图

```sh
(
  set -eu
  cd mobileapp
  : "${ASC_KEY_ID:?先配置 ASC Team key}" "${ASC_ISSUER_ID:?先配置 issuer}" "${ASC_KEY_PATH:?先配置 key 路径}"
  : "${STORE_BUILD:?指定本次已处理完的构建号}"
  bundle exec fastlane metadata
  bundle exec ruby scripts/asc-attach-build.rb "$STORE_BUILD"
  bundle exec ruby scripts/asc-attach-build.rb --list
  bundle exec ruby scripts/asc-prune-dup-screenshots.rb --list
)
```

去重脚本会逐个语言、逐个槽位打印数量，并对照期望值（iPhone 7 张、iPad 4 张）标出不符的；
`--list` 只看不删。确认有同名重复且应保留第一张后，才在 `mobileapp/` 运行不带 `--list` 的
`bundle exec ruby scripts/asc-prune-dup-screenshots.rb`：它按文件名保留第一张、删除后续同名图，
不比较图片内容。随后再次列出；两种语言、两个槽位的画面、顺序和数量都对上才算推完。

## 5. 提交前在 ASC 网页上核一遍

- 版本挂的是这次的 build（不是上一个）。
- 两种语言各 7 张手机图、4 张 iPad 图，顺序对，没有重复。
- iPad 截图槽位显示为 iPad Pro 13-inch；「Requires full screen」保持不勾（app 支持分屏）。
- What's New 覆盖商店在线版本到这次提交的跨度；只跨一版时对应那版归档，跨多版时见 §7 的汇总规则。
- 隐私政策链接仍然有效（`docs/appstore/privacy-policy.md`）。

## 6. 送审前给审核员一个能用的演示（必须）

App 内置演示不需要 Mac；真实连接功能需要可达的 `gtmux serve`。
提供真实连接供审核时，使用只含合成数据和专用测试 pane 的隔离演示环境，准备可撤销的访客链接：

```sh
# 先在隔离演示环境运行 serve，并让 tunnel 建立可达的公网地址。
# 前台 serve/tunnel 会持续运行；不要把它们串在同一个 shell 中等待退出。
(
  set -eu
  : "${REVIEW_PANE_A:?第一个专用演示 pane}" "${REVIEW_PANE_B:?第二个专用演示 pane}"
  gtmux share on
  gtmux share new --label 'App Review' --view "$REVIEW_PANE_A,$REVIEW_PANE_B" --type "$REVIEW_PANE_A"
  # 输出 #code= 分享链接及其 ID；保留 ID，供结束审核后撤销。
)
```

链接贴进 App Review Information → Notes。App 的 HTTP 接口按访客范围检查（这两个 pane 可看，一个可输入），
输入还要求分享输入总闸开启、pane 授权未因 tmux 服务重建而过期。
审核结束后用 `gtmux share revoke "${REVIEW_SHARE_ID:?本次分享的 ID}"` 撤掉。
**演示 Mac 和隧道要留到审核结束**，并在 Notes 中说明内置演示的入口。

隧道万一中途断了还有兜底：app 自带演示模式（配对页 → 「没有 Mac？看看演示」）。Notes 里要写上这句，
避免审核员把连接失败误认为没有可用的演示。

## 7. 提交审核

上面几项都读回对上之后：

```sh
(
  set -eu
  cd mobileapp
  bundle exec ruby scripts/asc-submit-review.rb "${STORE_VERSION:?本次商店版本}" "${STORE_BUILD:?本次构建号}"
)
```

脚本先核对版本挂的就是这个构建，不是就拒绝；然后把版本放进审核提交单再提交，App Store Connect 偶发的 500 会自动重试。
版本一旦放进提交单，状态就从「准备提交」变成「可以提交审核」，不再算「可编辑版本」，所以之后按版本号查它。
提交完读回状态，应当是 `WAITING_FOR_REVIEW`。

版本跨了好几个商店版本时（比如线上还是 1.0.14，这次提交 1.0.30），「更新内容」写的是汇总文案。
提交后把它存到 `release-notes/store/<版本号>.*.txt`，再把 `fastlane/metadata/*/release_notes.txt` 恢复成这个版本自己的说明，
细节见 `release-notes/README.md`。

## 曾经踩过的

- **归档前不要 `git checkout` 一个改动过的 `Podfile.lock`。** 真机和 e2e 构建会把
  `mobileapp/ios/Podfile.lock` 弄脏，为了「干净归档」把它还原，反而和已安装的 `mobileapp/ios/Pods/Manifest.lock` 对不上，
  归档跑到一分钟左右报 *"The sandbox is not in sync with the Podfile.lock"*，而且是在扩展都编完签完之后，
  看起来像签名失败其实不是。要么 `(cd mobileapp/ios && bundle exec pod install)` 重新同步，要么就别动那个脏文件。
  开跑前确认：`diff mobileapp/ios/Podfile.lock mobileapp/ios/Pods/Manifest.lock` 必须为空。
- **真机包不等于商店包。** 真机包是 Development 签名、`APS_ENVIRONMENT=development`（沙盒推送）；
  商店归档是 Distribution 签名，走 Release 配置里的 `production`。所以 `fastlane release` 绝不要传
  `APS_ENVIRONMENT` 覆盖。
- **第一次归档三个 target 时**，如果扩展在无界面环境下签名失败，用 Xcode 打开一次
  `mobileapp/ios/GtmuxMobile.xcworkspace` 让它把三个都配好，再从仓库根目录运行 `(cd mobileapp && bundle exec fastlane release)`。

- deliver 重试曾留下重复截图（[排障记录](../TROUBLESHOOTING.md)）；每次回读检查，有重复再按 §4 处理。
- 「上传成功」不等于版本挂上了那个 build；`asc-attach-build.rb --list` 读回来才算。
- 第一次给一个全新版本推文字时 deliver 会在截图前崩（fastlane 的老 bug），用 `skip_metadata:true` 再跑一次推截图。
- 2026-09-17：Xcode 被自动升级到 27 之后第一次打包，Pods 里部署版本低于 15 的 target 直接报错；Podfile 里已经加了下限，
  详见 `docs/TROUBLESHOOTING.md`。把版本加进审核提交单那一步连续两次返回 500，隔一分钟重试就好了，`asc-submit-review.rb` 已经内置重试。

## 只设一次的字段（ASC 网页）

| 字段 | 值 |
|---|---|
| Category | 主类目 Developer Tools，不需要副类目 |
| Age Rating | 每一问都选 None → 4+ |
| App Privacy | **Data Not Collected**（用户 2026-10-07 定）。依据是 app 的实际数据流：手机只和用户自己的 Mac 通信；推送由 Mac 发给 relay（`relay-worker`），relay 实时转给 APNs，不用 KV、不打日志，Worker 也没开 observability 或 logpush；Standard 和 Direct 隧道的标识、账号由 Mac 上的 CLI 写进 tunnel Worker，不是 app 写的；相机拍的照片只发到用户自己的 Mac。Standard 隧道的流量会经过 ccy.dev 这个 Cloudflare zone，free 计划的 HTTP 分析只有聚合数据，没开 Logpush。没有分析或追踪 SDK，token 只在 iOS Keychain 里。`PrivacyInfo.xcprivacy` 的 collected types 保持为空。以后要是 relay 或 Worker 开始留存数据或开日志，这一项要重新判断。 |
| Export Compliance | 豁免，只用标准 HTTPS/TLS；`ITSAppUsesNonExemptEncryption=false` 会跳过上传时的追问 |
| Pricing | 免费 |
| Availability | 除中国大陆外的所有国家和地区。大陆要 app 备案加备案域名，先放着 |
| Sign-in required | 否，没有账号体系，配对配的是用户自己的 Mac |

隐私政策链接 `https://ccy.dev/projects/gtmux/privacy` 必须能打开，Apple 会去抓，404 直接判元数据打回。

## Review Notes 模板

隐私句按上面 App Privacy 一行（Data Not Collected，2026-10-07）。把访客链接换成本次演示环境的链接后，
填入 App Review Information → Notes。

```
gtmux is a client for "gtmux serve", a small server the user runs on their OWN
Mac. It monitors the user's tmux sessions and coding-agent sessions, and can send
keystrokes to that Mac's terminal — conceptually the same as an SSH / terminal
client (cf. Termius, Blink Shell, Prompt). NO code is downloaded or executed on
iOS; input is sent to the user's own machine over the user's own network, VPN, or
tunnel. Access is gated by a bearer token the user controls and can revoke.

There is no account and no data collection. Camera = scan a pairing QR code, or
take a photo to send to the user's own Mac; Photo Library = attach an image to
send to an agent; Push = agent status alerts, sent by the user's Mac through a
relay that forwards them to APNs without storing them.
Guests (shared links) are scoped: view is limited to an allowlist and typing is
OFF by default and limited to an allowlist (input ⊆ view), enforced server-side.

TO REVIEW THE LIVE APP:
On first launch the app opens its pairing screen (later: Servers → "Add a
server"). Paste this guest link into the Host field; the app recognizes a
guest link and connects with its limited scope:
    <PASTE THE gtmux share GUEST LINK HERE>
It is scoped to a couple of demo sessions; you can view them and type into the
one input-allowed pane.

If that link is unreachable, tap "No Mac handy? See a demo →" on the pairing
screen for a built-in sample tour of the UI.
```
