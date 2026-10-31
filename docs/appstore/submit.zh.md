# App Store 提交流程（手机 + iPad）

[English](submit.md)

维护者手册。每次提交商店按顺序走一遍；上传后要读回版本、build 和截图。
下面的命令块都从仓库根目录运行，子 shell 的 `cd` 不改变外层目录。

可复用构建、断点续跑、上传前校验及统一状态命令见[准备流程](preparation.zh.md)。

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
`appstore-shots-ipad.test.ts` 里的 `shot()`。采集从未配对页进入演示；两份脚本使用 `GTMUX_DEBUG_RESET_SERVERS` 清空专用模拟器内的测试服务器，
并通过 `GTMUX_DEBUG_LANG` 固定语言。仍需确认起始页面，且输出目录没有混入上轮图片。
`frame-shots.mjs` 固定像素比例，并以稍大的 Chrome 视口渲染后裁取画布，避免底部未绘制区域进入成图。

手机和 iPad 两组共用同一批 demo 数据；重拍完记得把 README 配图也重出一遍
（`GTMUX_ONLY=readme bash docs/assets/screenshots/regenerate.sh`），它读的是同一个目录。

## 2b. 页头和搜索结果素材（每次提审都要做）

2026-10-05 起 App Store 多了两个位置：产品页页头（Header）和搜索结果素材（Search Results），
只在 iOS / iPadOS 27 及以上显示。用户定了（2026-10-08）：这两个素材和截图一样，是每次发版提审的一部分，
要跟着界面迭代，每次提交前都要确认是最新的，并上传、配置好。

| 内容 | 位置 |
|---|---|
| 页头：21:9、3840×1646 RGB PNG，无 alpha | `fastlane/creative/<locale>/header-21x9.png` |
| 搜索：3:2、3840×2560 RGB PNG，无 alpha | `fastlane/creative/<locale>/search-3x2.png` |
| 生成 | `scripts/frame-creative.mjs`，读 §2 那批演示截图的原图（iPad「所有 pane」+ 手机 HQ 对话） |
| 文案 | `scripts/creative-captions.json` |
| 上传、放置、检查 | `scripts/asc-asset-library.rb`（App Asset Library API） |

画面要说的是整个产品：Mac 上的每个 tmux 会话（agent 是其中功能最全的），加上替你盯着全部会话的 HQ。
不要写成只讲 coding agent，也不要漏掉 HQ（用户 2026-10-08 看初稿时指出的两点）。

每次提审前：

1. **判断要不要重出。** §2 重拍过截图、iPad「所有 pane」或手机 HQ 页变了、或者文案要改，就重新生成：

   ```sh
   (
     set -eu
     cd mobileapp
     node scripts/frame-creative.mjs --lang en --ipad .e2e-artifacts/appstore/ipad-en \
       --phone .e2e-artifacts/appstore/en --out fastlane/creative/en-US
     node scripts/frame-creative.mjs --lang zh --ipad .e2e-artifacts/appstore/ipad-zh \
       --phone .e2e-artifacts/appstore/zh --out fastlane/creative/zh-Hans
   )
   ```

   **按原尺寸打开看过**再提交，看缩小的预览会误以为糊。内容要求：4+；不放价格、网址、版权符号、
   其他平台的 logo 或 Apple 奖项。`creative-layout.mjs` 采用 Apple 官方 PSD 的 Art Safe Area：
   页头 `(1097,493,1646,661)`、搜索 `(836,765,2168,1030)`，依次是 x、y、宽、高。
   排版器测量实际文字和完整设备画面，越界或文字溢出即停止。裁切检查仍需结合 Apple 网页预览；
   不能把几何检查当成所有设备、方向和外观已验收。通用 5244×2950 PNG 仍符合 Apple 规格，
   但只保护上下约350px不能避免左右裁切，不作为缺图时的静默备用。
   规格与模板见 [Apple 创意素材规范](https://developer.apple.com/help/app-store-connect/reference/app-information/creative-assets-specifications/)
   和[设计指引](https://developer.apple.com/app-store/asset-best-practices/)。

   这些是按展示位置制作的图，不是每款 iPhone 一份 App 包。界面一致时，iPhone、iPad 各提供
   所需最高分辨率截图，Apple 可自动缩放到较小屏幕；布局确实不同或缩小后难读时再增加专用截图。
   界面和文案都没变就不用重出：图片字节不变，库里那张素材继续用。
2. **放到这个版本上**：§4 上传文字、关联 build 后，按 §4b 完成辅助功能复评，
   再运行 `asc-preparation.rb assets`。它按内容哈希复用库里的素材，为中英文放置页头与搜索结果图。
3. **读回**：统一准备命令独立核对四个位置；也可用
   `asc-asset-library.rb creative-status --version "$STORE_VERSION"` 排障。
   中英各两行都为 `ok` 才算配好，有 `TODO` 时退出码为 1。

版本一旦送审或上线，API 就不能再往上面加放置。那时只能在网页的 Asset Library 里单独送审素材，
批准后在产品页「Header and Search Results」里选上。1.0.97 就是这样：素材是版本提交之后才做出来的。

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
  : "${STORE_VERSION:?指定本次商店版本}"
  bundle exec fastlane metadata
  bundle exec ruby scripts/asc-attach-build.rb "$STORE_BUILD"
  bundle exec ruby scripts/asc-attach-build.rb --list
)
```

`fastlane metadata` 只推文字（它也负责在 ASC 建出这个版本）。素材统一通过 §4b 的
`asc-preparation.rb assets` 放置；单独的 place 命令只用于恢复排障，不能代替准备检查。截图从 2026-10-08 起不再由 deliver 上传：
App Store Connect API 4.5.1 把它用的截图接口标为弃用，而且它几乎每次都会重复上传几张，以前得靠去重脚本收拾。
现在由 `asc-asset-library.rb place-screenshots` 通过 App Asset Library 放截图：
`fastlane/screenshots/<locale>/01..07.png` 是 iPhone 6.9" 组，`ipad-01..04.png` 是 iPad 13" 组，
库里已有的同一张图（按内容哈希命名）直接复用，某一组已经是这些图、这个顺序时就不动它。
不放心时先加 `--dry-run` 看计划。§4b 统一读回后，每组、每个语言都应为 `ok`；有 `TODO` 时退出码是 1，并列出差异。

新接口还是第一次用时出了问题，可以退回旧路：`bundle exec fastlane metadata deliver_screenshots:true`
（在弃用接口下线前仍可用），然后照样用 `screenshot-status` 读回。

## 4b. 辅助功能复评与统一状态检查

关联 build 后、放置素材前，每次提交均按[辅助功能流程](accessibility.zh.md)完成候选包绑定的私有复评，不能直接
复制上一版的声明。iPhone、iPad 分开记录证据；可选 Accessibility URL 可以继续留空。
按[准备流程](preparation.zh.md#4-放置素材并独立读回)执行 `asc-preparation.rb assets/status`，
只有 `ready_for_user_review` 才能交给操作者。新能力留到确切版本/build 上线后发布，
保留跟进负责人和计划，随后运行 `asc-accessibility.rb live-status` 并记录商店预览。
这些脚本不发布辅助功能声明，也不提交 App。

## 5. 提交前在 ASC 网页上核一遍

- 版本挂的是这次的 build（不是上一个）。
- 两种语言各 7 张手机图、4 张 iPad 图，顺序对，没有重复：`asc-asset-library.rb screenshot-status --version <版本>` 全是 `ok`。
- 页头和搜索结果素材：`asc-asset-library.rb creative-status --version <版本>` 四行都是 `ok`（§2b），网页上用预览看一眼中英两种语言。
- iPad 截图槽位显示为 iPad Pro 13-inch；「Requires full screen」保持不勾（app 支持分屏）。
- What's New 覆盖商店在线版本到这次提交的跨度；只跨一版时对应那版归档，跨多版时见 §7 的汇总规则。
- 辅助功能复评绑定当前候选包，线上声明与实测一致；新增声明有明确的上线后跟进。
- 隐私政策链接仍然有效（`docs/appstore/privacy-policy.md`）。

## 6. 审核员怎么体验：只用内置演示

用户定了（2026-10-07）：审核只用 app 自带的演示模式，不搭演示服务器，也不建访客链接。
Review Notes 写明入口：配对页上的「No server handy? See a demo」（中文「尚未连接？看看演示」；以 app 里显示的文字为准，
`PairingScreen.tsx`）。演示用合成数据、不联网，覆盖雷达、终端和回复按钮、HQ、知识库、用量和 iPad 布局；
App Store 认可用功能完整的演示模式代替演示账号。
联系人信息和「需要演示账号 = 否」在 App Review 信息里，会从上一版带过来，提交前看一眼。

## 7. 提交审核

交付准备工作的 agent 到此停止，由用户在 ASC 预览并点击 Submit。以下命令是操作者
明确选择的独立提交入口，准备脚本不会调用它。

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
| Availability | 从 ASC 读回当前国家和地区的可用性并保持不变；不要把历史地区列表当作当前分发策略 |
| Sign-in required | 否，没有账号体系，配对配的是用户自己的 Mac |

隐私政策链接 `https://ccy.dev/projects/gtmux/privacy` 必须能打开，Apple 会去抓，404 直接判元数据打回。

## Review Notes 模板

基于 1.0.97 的审核说明，演示入口按 1.0.121 的实际标签更新。改了演示入口、相机或推送的说法时，同步改这里；英文与中文采集说明见 [assets-ci.md](assets-ci.md) / [assets-ci.zh.md](assets-ci.zh.md)：

```
gtmux is a companion app for a server that users run on their own Mac (`gtmux serve`). It shows the user's tmux and coding-agent sessions and lets them reply from their phone. There is no account or login. Users pair with a QR code or code shown on their Mac. Terminal input goes only to the Mac they paired. The app does not offer a VPN, and it collects no data: push notifications are sent by the user's own Mac through a relay that forwards them to Apple's push service without storing them.
REVIEW WITHOUT A SERVER
On the first pairing screen, tap "No server handy? See a demo" ("尚未连接？看看演示" in Simplified Chinese). The self-contained demo uses sample data and needs no credentials or network. It shows the agent radar, terminal and reply controls, HQ, knowledge, usage, and the iPad layout. You can navigate the whole demo without connecting to a server.
Camera access scans the pairing QR code, or takes a photo to send to the user's own Mac. Photo Library access attaches an image to an agent message. Push notifications report agent status.
```
