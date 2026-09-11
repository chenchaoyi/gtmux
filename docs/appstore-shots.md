# App Store 截图：从取材到上架

维护者手册，单语言（和 `TROUBLESHOOTING.md`、`release-signing.md` 同类，不是用户文档）。

商店那七张图**不是手工做的**，整条链路都在仓库里，UI 一动就能重做一遍。这份文档存在的理由是：
2026-09-10 之前它不在 —— 原图来自 e2e 采集脚本，而加标题和机身框那一步是在某个设计工具里手工完成的，
于是那批图在三次改版之后没人能重做，只能眼看着它们过期。

## 一句话的流程

```
demo 数据  →  模拟器采集（6 张）+ 渲染（1 张锁屏）  →  加标题和机身框  →  fastlane  →  回读核对
```

## 0. 先确认 demo 数据是全的

**截图拍的是 demo，App Review 看到的也是 demo。** 所以新做的界面如果读的是 demo 没给的字段，
截下来就是空的 —— 2026-09-10 就有两处这样：HQ 的动作流（demo 一条审计记录都没种）和用量页的
会话行（只有 token 数、没有位置，所以每行没有标题）。

改了界面就回头看一眼 `src/ui/demoData.ts` / `demoClient.ts`。`demoClient.test.ts` 里有几条
守卫：动作必须跨一天（不然分簇和「隔了多久」看不出来）、必须是会渲染的类型、用量的每个字段都要有。

## 1. 采集那六张（模拟器）

**必须用 6.9 寸机型** —— 商店要 1320×2868，iPhone 17 Pro Max 的原生分辨率正好是它。

```sh
MAX=$(xcrun simctl list devices available | grep -m1 "Pro Max" | grep -oE '[0-9A-F-]{36}')
xcrun simctl boot $MAX

# 语言要和这一轮的 locale 对上：采集脚本按语言找那个「看看演示」的入口
xcrun simctl spawn $MAX defaults write .GlobalPreferences AppleLanguages -array en-US
xcrun simctl spawn $MAX defaults write .GlobalPreferences AppleLocale -string en_US
xcrun simctl shutdown $MAX && xcrun simctl boot $MAX     # 语言改完必须重启模拟器

cd mobileapp
GTMUX_E2E_UDID=$MAX npm run e2e:build
npm run e2e:appium &
GTMUX_DEMO_SHOTS=1 GTMUX_SHOTS_LANG=en GTMUX_E2E_UDID=$MAX npm run test:e2e -- -t "app store"
```

中文那轮把三处 `en` 换成 `zh`（`AppleLanguages` 用 `zh-Hans-CN`、`AppleLocale` 用 `zh_CN`、
`GTMUX_SHOTS_LANG=zh`）。原图落在 `.e2e-artifacts/appstore/<lang>/`。

采集脚本是 `e2e/__tests__/appstore-shots.test.ts`，它走的是「没有配对 → 演示入口 → 各个屏」。
**要加一屏就改它**，别手工补图。

## 2. 锁屏那一张是画的

实况活动和推送**在这里拍不到**：模拟器构建不带 entitlement（ad-hoc 签名也不带），所以没有
`aps-environment`，ActivityKit 直接拒绝创建活动；`simctl` 也够不到锁屏。拿真机拍又会把操作者
自己的会话名拍到商店上。

```sh
node scripts/render-lockscreen.mjs --lang en --out .e2e-artifacts/appstore/en
node scripts/render-lockscreen.mjs --lang zh --out .e2e-artifacts/appstore/zh
```

它**不抄源码，它读源码**：所有尺寸、颜色都由 `scripts/widget-tokens.mjs` 在渲染时从
`GtmuxWidget.swift` 里解析出来（17 个值）。所以：

- 源码里改一个尺寸 → 下次出图自动跟着变（实测：把主徽章 26 改成 40，图的哈希就变了）；
- 把那一行重构得认不出来 → 脚本**直接报错**，而不是画出昨天那张卡；
- 增删或调换一条 band → `widget-tokens --check` 变红，这一条接在 `check-design.sh` 里，
  CI 会拦住（实测：多插一条分隔线就红）。

**还剩一样它管不了**：一条 band 内部除字面量之外的结构。比如把 PrimaryBand 里两个元素对调，
数字照样解析得出来，而图会悄悄过期。这一条目前只能靠人，写在这里免得下次又被当成「全都有保障」。

## 3. 加标题和机身框

```sh
node scripts/frame-shots.mjs --in .e2e-artifacts/appstore/en --lang en --out fastlane/screenshots/en-US
node scripts/frame-shots.mjs --in .e2e-artifacts/appstore/zh --lang zh --out fastlane/screenshots/zh-Hans
```

标题在 `scripts/shot-captions.json`，按 locale 分组，**顺序就是商店里的顺序**，文件名由脚本按
顺序生成 `01..NN`。每条带一个状态色圆点（红=有人等你 / 青=在跑 / 绿=都停了），那是 app 里
同一套状态语言搬到商店页上。

改顺序 = 改这个 json 的数组顺序，不用重命名任何文件。

## 4. 上传，然后**回读**

```sh
eval "$(grep -E '^export ASC_(KEY_ID|ISSUER_ID|KEY_PATH)=' ~/.zshrc)"
bundle exec fastlane metadata                          # 文案 + 截图
bundle exec ruby scripts/asc-prune-dup-screenshots.rb  # deliver 每次都留重复，必须清
bundle exec ruby scripts/asc-attach-build.rb           # 把新 build 挂到版本上
bundle exec ruby scripts/asc-attach-build.rb --list    # 回读：版本 + build + 截图张数
```

**后三条不是可选的。** deliver 每次都会上传出重复（实测 6 张传成 10 张、7 张传成 9 张），
而且**没有任何东西会把版本指向你刚传的 build** —— 1.0.12 和 1.0.13 都出现过版本上挂着上一个
build 的情况。详见 `TROUBLESHOOTING.md` 里那两条。

## 每次改完 UI 该问自己的三句话

1. 新界面读的字段，demo 里有吗？（没有就是空屏截图）
2. 这一屏值不值一张截图？值就改采集脚本，不要手工补。
3. 锁屏那张画的东西，和 widget 源码现在还一致吗？
