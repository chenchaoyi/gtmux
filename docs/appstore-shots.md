# App Store 截图：从取材到上架

维护者手册，单语言（和 `TROUBLESHOOTING.md`、`release-signing.md` 同类，不是用户文档）。

商店那七张图**不是手工做的**，整条链路都在仓库里，UI 一动就能重做一遍。这份文档存在的理由是：
2026-09-10 之前它不在 —— 原图来自 e2e 采集脚本，而加标题和机身框那一步是在某个设计工具里手工完成的，
于是那批图在三次改版之后没人能重做，只能眼看着它们过期。

## 一句话的流程

```
demo 数据  →  模拟器采集（6 张）+ 渲染（1 张锁屏）  →  加标题和机身框  →  fastlane  →  回读核对
                        └→  README 配图（docs/assets/screenshots/regenerate.sh）
```

同一批原图有两个下游：商店那七张，和 README 里的配图。README 配图读的就是
`.e2e-artifacts/appstore/` 这个目录，所以**重拍完别忘了把配图也重出一遍**（见本文最后一节）。
iPad 那组的采集命令在 [提交流程](appstore/submit.md)，和这里是一套流程的两半。
下面每个命令块都从仓库根目录运行；子 shell 中的 `cd` 不改变外层目录。

## 0. 先确认 demo 数据是全的

**截图拍的是 demo，App Review 看到的也是 demo。** 所以新做的界面如果读的是 demo 没给的字段，
截下来就是空的 —— 2026-09-10 就有两处这样：HQ 的动作流（demo 一条审计记录都没种）和用量页的
会话行（只有 token 数、没有位置，所以每行没有标题）。

改了界面就回头看一眼 `mobileapp/src/ui/demoData.ts` / `demoClient.ts`。`demoClient.test.ts` 里有几条
守卫：动作必须跨一天（不然分簇和「隔了多久」看不出来）、必须是会渲染的类型、用量的每个字段都要有。

## 1. 采集那六张（模拟器）

本仓库手机成图选用 6.9 寸槽位的 1320×2868；建议用 iPhone 17 Pro Max 采集，减少缩放损失。
这是本仓库的制作尺寸，不是所有 App 唯一允许的尺寸；上传前对照
[Apple 截图规格](https://developer.apple.com/help/app-store-connect/reference/app-information/screenshot-specifications/)。

用小一号的 iPhone 17 Pro 也能跑（出 1206×2622），加框那一步会把它缩进同样的机身里，成图尺寸
照样合格，只是细节少一点。**只重出 README 配图的话可以将就**；要传商店就用 Max 重拍一遍。

```sh
(
  set -eu
  : "${AUDIT_SIM_UDID:?指定本次专用、已经启动的手机模拟器 UDID}"
  # 语言要和这一轮的 locale 对上：手机脚本按语言找「看看演示」入口。
  xcrun simctl spawn "$AUDIT_SIM_UDID" defaults write .GlobalPreferences AppleLanguages -array en-US
  xcrun simctl spawn "$AUDIT_SIM_UDID" defaults write .GlobalPreferences AppleLocale -string en_US
  xcrun simctl shutdown "$AUDIT_SIM_UDID"
  xcrun simctl boot "$AUDIT_SIM_UDID"
  xcrun simctl bootstatus "$AUDIT_SIM_UDID" -b
  cd mobileapp
  GTMUX_E2E_UDID="$AUDIT_SIM_UDID" npm run e2e:build
  GTMUX_DEMO_SHOTS=1 GTMUX_SHOTS_LANG=en GTMUX_E2E_UDID="$AUDIT_SIM_UDID" \
    npm run test:e2e -- appstore-shots.test.ts
)
```

中文那轮把三处 `en` 换成 `zh`（`AppleLanguages` 用 `zh-Hans-CN`、`AppleLocale` 用 `zh_CN`、
`GTMUX_SHOTS_LANG=zh`）。原图落在 `mobileapp/.e2e-artifacts/appstore/<lang>/`。
Node、Appium driver 等前置条件见 [e2e 手册](../mobileapp/e2e/README.md)。Jest 的 global setup
会启动 Appium，不要再后台起一个。采集前确认 App 在未配对页面，App 内保存的语言选择也与本轮一致：
手机采集脚本没有强制 `GTMUX_DEBUG_LANG`，两套采集脚本也没有传 `RESET_SERVERS`；重装不保证清空 Keychain。

采集脚本是 `mobileapp/e2e/__tests__/appstore-shots.test.ts`，它走的是「没有配对 → 演示入口 → 各个屏」。
**要加一屏就改它**，别手工补图。
核对六张原图都来自本轮，逐张检查画面是否对应预期页面。测试通过、文件存在或写图命令返回成功，
都不能代替这一步；缺张或画面不对时先修采集路径，不用上轮图片补齐。

## 2. 锁屏那一张是画的

这里用 HTML 和通用示例数据绘制推送、实况活动和锁屏背景，再由 Chrome 导出图片。
它不是 iOS 锁屏截图，也不能证明推送已送达、ActivityKit 已运行或真机显示正确。
这样做避免把操作者的真实会话名放进商店素材；这不表示所有模拟器配置都无法测试实况活动。

```sh
(
  set -eu
  cd mobileapp
  node scripts/render-lockscreen.mjs --lang en --out .e2e-artifacts/appstore/en
  node scripts/render-lockscreen.mjs --lang zh --out .e2e-artifacts/appstore/zh
)
```

`mobileapp/scripts/widget-tokens.mjs` 在渲染时从 `GtmuxWidget.swift` 解析 17 个尺寸和颜色值，
并读取卡片 band 的顺序。背景、推送框和部分字号等仍写在渲染脚本里，不是所有视觉属性都来自 Swift。所以：

- 源码里改一个被解析的尺寸 → 下次出图跟着变（此前实测：把主徽章 26 改成 40，图的哈希就变了）；
- 把那一行重构得认不出来 → 脚本**直接报错**，而不是画出昨天那张卡；
- 增删或调换一条 band → `widget-tokens --check` 变红，这一条接在 `check-design.sh` 里，
  CI 会拦住（实测：多插一条分隔线就红）。

**仍需人工核对**一条 band 内部的结构及没有解析的视觉属性。比如把 PrimaryBand 里两个元素对调，
数字照样解析得出来，而图会悄悄过期。这一条目前只能靠人，写在这里免得下次又被当成「全都有保障」。

## 3. 加标题和机身框

```sh
(
  set -eu
  cd mobileapp
  node scripts/frame-shots.mjs --in .e2e-artifacts/appstore/en --lang en --out fastlane/screenshots/en-US
  node scripts/frame-shots.mjs --in .e2e-artifacts/appstore/zh --lang zh --out fastlane/screenshots/zh-Hans
)
```

标题在 `scripts/shot-captions.json`，按 locale 分组，**顺序就是商店里的顺序**，文件名由脚本按
顺序生成 `01..NN`。每条带一个状态色圆点（红=有人等你 / 青=在跑 / 绿=空闲），那是 app 里
同一套状态语言搬到商店页上。

改顺序 = 改这个 json 的数组顺序，再重新加框。脚本覆盖对应编号，不清除多余的旧成图；
上传前检查目录内实际文件、尺寸和顺序。渲染需要 Node 和脚本指定路径下的 Google Chrome。

## 3.5 先看商店落后了多少，再写 What's New

**大多数真机构建从不提交**，所以商店上在线的版本可能落后很多版 —— 2026-09-11 那次，商店还是
0.68.0，而正在准备的是 1.0.14，中间隔了 36 个版本、104 条改动。那次的 What's New 只写了最后
三版，因为**流水线从来没有把这个跨度摆到人眼前**。

现在摆了：`asc-attach-build.rb --list` 会打出在线版本，落后超过一版就直接告诉你要覆盖到哪里。

写法（规矩原文在 `release-notes/README.md`「When a submission crosses several versions」）：

- 读它的人正在决定**要不要从他手上那一版升级**，所以覆盖整个跨度，不是最后一版。
- 挑**用户真能感觉到的能力**（重做的 HQ 页、长按菜单、知识库、用量页…），其余的收成一句
  「还有一长串打磨和问题修复」。不要把 changelog 全倒进去。
- 写进 `fastlane/metadata/*/release_notes.txt`，然后**不要重跑 `set-version.sh`** ——
  归档要保持一版一条，应用内的「新变化」弹窗会把用户跳过的每一版都回放一遍，
  重跑会让同样的话说两遍。
- 只传文案不动截图：在 `mobileapp/` 运行 `bundle exec fastlane metadata skip_screenshots:true`。

## 4. 上传，然后**回读**

```sh
(
  set -eu
  cd mobileapp
  : "${ASC_KEY_ID:?先配置 ASC Team key}" "${ASC_ISSUER_ID:?先配置 issuer}" "${ASC_KEY_PATH:?先配置 key 路径}"
  : "${STORE_BUILD:?指定本次已经处理完的构建号}"
  bundle exec fastlane metadata                         # 上传文案 + 截图
  bundle exec ruby scripts/asc-attach-build.rb "$STORE_BUILD"
  bundle exec ruby scripts/asc-attach-build.rb --list    # 版本、候选和已挂载 build
  bundle exec ruby scripts/asc-prune-dup-screenshots.rb --list  # 两种语言各槽位的文件与数量
)
```

上传成功后每次都要回读。历史维护记录描述 deliver 几乎每次运行都遇到重复
（曾有 6 张变 10 张、7 张变 9 张）；本轮仍以 `--list` 结果和实际图片为准。
确认存在同名重复且保留的第一张确实正确后，维护者可在 `mobileapp/`
运行 `bundle exec ruby scripts/asc-prune-dup-screenshots.rb`，再用 `--list` 回读。
**不带 `--list` 会删除远端同名的第二张及以后图片，不比较图片内容。**
`release` 和 `metadata` lane 都不负责选择 build；1.0.12、1.0.13 曾挂着上一份 build。
详见 [排障记录](TROUBLESHOOTING.md) 和 [提交流程](appstore/submit.md)。

## 每次改完 UI 该问自己的四句话

1. 新界面读的字段，demo 里有吗？（没有就是空屏截图）
2. 这一屏值不值一张截图？值就改采集脚本，不要手工补。
3. 锁屏那张画的东西，和 widget 源码现在还一致吗？
4. README 的配图重出了吗？`GTMUX_ONLY=readme bash docs/assets/screenshots/regenerate.sh`
   用的就是刚拍的这批原图，不重跑它，README 里还是上一版界面。

采集脚本点的是 testID，**点不到就要让它红**。2026-09-18 之前第 4 步去点一个早已并进「对话」的
标签，失败被 `.catch(() => {})` 吞掉，于是又把上一屏拍了一遍，商店上因此挂了两张一模一样的图。
