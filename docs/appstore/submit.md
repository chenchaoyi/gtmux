# App Store 提交流程（手机 + iPad）

维护者手册。每次发版按顺序走一遍；每一步都有「读回来」的检查，因为 deliver 只负责推，不负责确认。

## 0. 前提

- 版本号来自最新的 git tag：先打 tag，再 `bash mobileapp/scripts/set-version.sh`（归档两种语言的 What's New、
  记源码哈希、重生成 `src/releaseNotes.ts`、改 pbxproj 的 `MARKETING_VERSION`），提 PR 合进 main。
  `check-design.sh` 在戳版本**之后**跑才有意义：它比的是当前源码哈希和归档时的哈希。
- ASC 的 API key 从你的 shell 配置里加载：`eval "$(grep -E '^export ASC_(KEY_ID|ISSUER_ID|KEY_PATH)=' ~/.zshrc)"`
  （换成你自己的那份 profile）。必须是 **Team** key，不是个人 key。
- fastlane 要比系统 ruby 新的版本：`brew install ruby` 并把它放进 PATH，然后 `cd mobileapp && bundle install`。
  非交互的 shell 通常不会自动加载 profile，所以脚本里要显式 export PATH 和那三个 ASC 变量。
- 2026-09-12 起 app 是通用的（iPhone + iPad，三个 target 都是 `TARGETED_DEVICE_FAMILY = "1,2"`）：
  ASC 会要求 13" iPad 的截图，缺了不能提交。

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
cd mobileapp
xcrun simctl boot 49AC4786-86B9-4E5B-A572-7AFF64AB195A        # iPad Pro 13-inch (M5)
GTMUX_E2E_UDID=49AC4786-86B9-4E5B-A572-7AFF64AB195A npm run e2e:build
for L in en zh; do
  GTMUX_DEMO_SHOTS=1 GTMUX_SHOTS_LANG=$L GTMUX_E2E_UDID=49AC4786-86B9-4E5B-A572-7AFF64AB195A \
    GTMUX_E2E_DEVICE='iPad Pro 13-inch (M5)' npm run test:e2e -- appstore-shots-ipad
done
node scripts/frame-shots.mjs --slot ipad --in .e2e-artifacts/appstore/ipad-en --lang ipad-en --out fastlane/screenshots/en-US --prefix ipad-
node scripts/frame-shots.mjs --slot ipad --in .e2e-artifacts/appstore/ipad-zh --lang ipad-zh --out fastlane/screenshots/zh-Hans --prefix ipad-
```

文案在 `scripts/shot-captions.json`（`ipad-en` / `ipad-zh` 两组）。**每张都要打开看过再提交。**
横屏这一张的转向踩过两次坑：以前 `simctl` 给的是竖向缓冲，采集脚本无条件转 270° 补偿；iOS 26.5
给回来的已经是正的，再转一次整组就躺倒了（2026-09-18）。现在脚本先问图片实际是横是竖，只转竖的
那种，所以两种系统都对。要是哪天又歪了，查 `appstore-shots-ipad.test.ts` 里的 `shot()`。

手机和 iPad 两组共用同一批 demo 数据；重拍完记得把 README 配图也重出一遍
（`GTMUX_ONLY=readme bash docs/assets/screenshots/regenerate.sh`），它读的是同一个目录。

## 3. 构建与上传二进制

```sh
cd mobileapp
xcodebuild -workspace ios/GtmuxMobile.xcworkspace -scheme GtmuxMobile -configuration Release \
  -destination 'generic/platform=iOS' -derivedDataPath ios/build/dd \
  DEVELOPMENT_TEAM=2337SY8FRT CODE_SIGN_STYLE=Automatic MARKETING_VERSION=<ver> \
  -allowProvisioningUpdates archive -archivePath build/GtmuxMobile.xcarchive     # 商店包不传 APS_ENVIRONMENT（默认 production）
bundle exec fastlane release        # 或 upload ipa:<path>
```

本地真机测试用 `build`（不是 `archive`）加 `APS_ENVIRONMENT=development`，然后
`xcrun devicectl device install app --device <devicectl-uuid> ios/build/dd/Build/Products/Release-iphoneos/GtmuxMobile.app`；
iPad 和 iPhone 装的是同一个包。锁屏的手机也能装（不要用 udid 当 destination，那会等设备 ready）。

## 4. 推文字和截图

```sh
bundle exec fastlane metadata                    # 文字 + 两组截图
bundle exec ruby scripts/asc-attach-build.rb     # 把最新处理完的 build 挂到版本上（deliver 不做这件事）
bundle exec ruby scripts/asc-prune-dup-screenshots.rb   # deliver 重试会重复上传；脚本按文件名去重
bundle exec ruby scripts/asc-attach-build.rb --list     # 读回来：版本 + build
```

去重脚本会逐个语言、逐个槽位打印数量，并对照期望值（iPhone 7 张、iPad 4 张）标出不符的；
`--list` 只看不删。两种语言、两个槽位都对上才算推完。

## 5. 提交前在 ASC 网页上核一遍

- 版本挂的是这次的 build（不是上一个）。
- 两种语言各 7 张手机图、4 张 iPad 图，顺序对，没有重复。
- iPad 截图槽位显示为 iPad Pro 13-inch；「Requires full screen」保持不勾（app 支持分屏）。
- What's New 是这一版的文字（和 `release-notes/<ver>.*.txt` 一致）。
- 隐私政策链接仍然有效（`docs/appstore/privacy-policy.md`）。

## 6. 送审前给审核员一个能用的演示（必须）

gtmux 离开 `gtmux serve` 什么都做不了。审核员手上没有可连的 Mac，基本就按 2.1「无法审核」打回。
给一个受限、可撤销的访客链接：

```sh
# 在一台整个审核期都会开着的 Mac 上（可能要几天）：
gtmux serve
gtmux tunnel                                   # 审核员能访问的公网 https 地址
gtmux share --view %A --view %B --input %A     # A、B 可看，只有 A 可输入
# → 打印访客链接：https://<tunnel-host>/#g=<token>
```

链接贴进 App Review Information → Notes。它是访客范围的（只看得到那两个 pane，只能往一个里打字），
批准后 `gtmux share revoke` 撤掉。**演示 Mac 和隧道要留到审核通过为止**，审核员常常隔一两天才测。

隧道万一中途断了还有兜底：app 自带演示模式（配对页 → 「没有 Mac？看看演示」）。Notes 里要写上这句，
否则链接不通就是一次硬性打回。

## 7. 提交审核

上面几项都读回对上之后：

```sh
bundle exec ruby scripts/asc-submit-review.rb <版本号> <构建号>
```

脚本先核对版本挂的就是这个构建，不是就拒绝；然后把版本放进审核提交单再提交，App Store Connect 偶发的 500 会自动重试。
版本一旦放进提交单，状态就从「准备提交」变成「可以提交审核」，不再算「可编辑版本」，所以之后按版本号查它。
提交完读回状态，应当是 `WAITING_FOR_REVIEW`。

版本跨了好几个商店版本时（比如线上还是 1.0.14，这次提交 1.0.30），「更新内容」写的是汇总文案。
提交后把它存到 `release-notes/store/<版本号>.*.txt`，再把 `fastlane/metadata/*/release_notes.txt` 恢复成这个版本自己的说明，
细节见 `release-notes/README.md`。

## 曾经踩过的

- **归档前不要 `git checkout` 一个改动过的 `Podfile.lock`。** 真机和 e2e 构建会把
  `ios/Podfile.lock` 弄脏，为了「干净归档」把它还原，反而和已安装的 `Pods/Manifest.lock` 对不上，
  归档跑到一分钟左右报 *"The sandbox is not in sync with the Podfile.lock"*，而且是在扩展都编完签完之后，
  看起来像签名失败其实不是。要么 `cd ios && bundle exec pod install` 重新同步，要么就别动那个脏文件。
  开跑前一条命令确认：`diff ios/Podfile.lock ios/Pods/Manifest.lock` 必须为空。
- **真机包不等于商店包。** 真机包是 Development 签名、`APS_ENVIRONMENT=development`（沙盒推送）；
  商店归档是 Distribution 签名，走 Release 配置里的 `production`。所以 `fastlane release` 绝不要传
  `APS_ENVIRONMENT` 覆盖。
- **第一次归档三个 target 时**，如果扩展在无界面环境下签名失败，用 Xcode 打开一次
  `ios/GtmuxMobile.xcworkspace` 让它把三个都配好，再重跑 `fastlane release`。

- deliver 每次重试都会把截图传成两份（`docs/TROUBLESHOOTING.md` 有记录），所以去重脚本不是可选项。
- 「上传成功」不等于版本挂上了那个 build；`asc-attach-build.rb --list` 读回来才算。
- 第一次给一个全新版本推文字时 deliver 会在截图前崩（fastlane 的老 bug），用 `skip_metadata:true` 再跑一次推截图。
- 2026-09-17：Xcode 被自动升级到 27 之后第一次打包，Pods 里部署版本低于 15 的 target 直接报错；Podfile 里已经加了下限，
  详见 `docs/TROUBLESHOOTING.md`。把版本加进审核提交单那一步连续两次返回 500，隔一分钟重试就好了，`asc-submit-review.rb` 已经内置重试。

## 只设一次的字段（ASC 网页）

| 字段 | 值 |
|---|---|
| Category | 主类目 Developer Tools，不需要副类目 |
| Age Rating | 每一问都选 None → 4+ |
| App Privacy | Data Not Collected（没有任何分析或追踪 SDK，token 只在 iOS Keychain 里，什么都不上传；`PrivacyInfo.xcprivacy` 里声明的就是这个） |
| Export Compliance | 豁免，只用标准 HTTPS/TLS；`ITSAppUsesNonExemptEncryption=false` 会跳过上传时的追问 |
| Pricing | 免费 |
| Availability | 除中国大陆外的所有国家和地区。大陆要 app 备案加备案域名，先放着 |
| Sign-in required | 否，没有账号体系，配对配的是用户自己的 Mac |

隐私政策链接 `https://ccy.dev/projects/gtmux/privacy` 必须能打开，Apple 会去抓，404 直接判元数据打回。

## Review Notes 模板

每次提交把这段贴进 App Review Information → Notes，把访客链接换成这次生成的那条：

```
gtmux is a client for "gtmux serve", a small server the user runs on their OWN
Mac. It monitors the user's tmux sessions and coding-agent sessions, and can send
keystrokes to that Mac's terminal — conceptually the same as an SSH / terminal
client (cf. Termius, Blink Shell, Prompt). NO code is downloaded or executed on
iOS; input is sent to the user's own machine over the user's own network, VPN, or
tunnel. Access is gated by a bearer token the user controls and can revoke.

There is no account and no data collection. Camera = scan a pairing QR code;
Photo Library = attach an image to send to an agent; Push = agent status alerts.
Guests (shared links) are scoped: view is limited to an allowlist and typing is
OFF by default and limited to an allowlist (input ⊆ view), enforced server-side.

TO REVIEW THE LIVE APP:
Open the app → "Add a Mac" → paste this guest link into the host field
(the app auto-detects a guest link):
    <PASTE THE gtmux share GUEST LINK HERE>
It is scoped to a couple of demo sessions; you can view them and type into the
one input-allowed pane.

If that link is unreachable, tap "No Mac handy? See a demo →" on the pairing
screen for a built-in sample tour of the UI.
```
