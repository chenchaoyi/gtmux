# App Store 提交流程（手机 + iPad）

维护者手册。每次发版按顺序走一遍；每一步都有「读回来」的检查，因为 deliver 只负责推，不负责确认。

## 0. 前提

- 版本号来自最新的 git tag：先打 tag，再 `bash mobileapp/scripts/set-version.sh`（归档两种语言的 What's New、
  记源码哈希、重生成 `src/releaseNotes.ts`、改 pbxproj 的 `MARKETING_VERSION`），提 PR 合进 main。
  `check-design.sh` 在戳版本**之后**跑才有意义：它比的是当前源码哈希和归档时的哈希。
- ASC 的 API key 在 `~/.zshrc`：`eval "$(grep -E '^export ASC_(KEY_ID|ISSUER_ID|KEY_PATH)=' ~/.zshrc)"`。
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
| iPhone 6.9" | 1320×2868 | `fastlane/screenshots/<locale>/01..07.png` | `appstore-shots` e2e（iPhone 17 Pro 模拟器）+ 锁屏那张是画的（`render-lockscreen.mjs`） |
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

文案在 `scripts/shot-captions.json`（`ipad-en` / `ipad-zh` 两组）。每张都要打开看过再提交：
横屏截图 `simctl` 给的是竖向缓冲，e2e 里已经转正；如果哪天又是侧着的，先查这一步。

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
`xcrun devicectl device install app --device <devicectl-uuid> ios/build/dd/Build/Products/Release-iphoneos/gtmux.app`；
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

## 曾经踩过的

- deliver 每次重试都会把截图传成两份（`docs/TROUBLESHOOTING.md` 有记录），所以去重脚本不是可选项。
- 「上传成功」不等于版本挂上了那个 build；`asc-attach-build.rb --list` 读回来才算。
- 第一次给一个全新版本推文字时 deliver 会在截图前崩（fastlane 的老 bug），用 `skip_metadata:true` 再跑一次推截图。
