# App Store 辅助功能声明

[English](accessibility.md)

**Accessibility URL（辅助功能网址）**是可选的公开说明页链接，用来介绍支持的
辅助功能及限制。它不是 API、权限或认证。没有值得维护的说明页时可以留空，不影响
填写能力标签。参见 [Apple 操作说明](https://developer.apple.com/help/app-store-connect/manage-app-accessibility/manage-accessibility-nutrition-labels/)。

## 控制改动范围

优先核验现有深色界面和不单靠颜色区分信息的能力，复用主题、状态图形与文字、选中
勾号和开关位置。不要为了填标签重做界面或更换状态配色。HQ 和答复按钮使用可理解
的操作名称，测试编号只放在 `testID`。修正个别名称不能证明整个 App 已适配 VoiceOver。

iOS 对话中，长按复制的范围是**整个文本块**；菜单打开时高亮该块，关闭后清除。
“复制全文”仍复制整条回复。这不代表可以拖动选择某个词，或跨段落选择。Markdown
文本和行内链接保留，反馈包装层仅用于对话；Android 保留原有范围选择。原生改动
需要新安装包才能生效，旧安装包配上新 JS 时会回退为原有复制方式。
`scripts/test-copy-text.sh <专用且已启动的模拟器 UUID>` 可运行轻量 UIKit 回归检查，
无需构建整个 React Native App。

## 发布前核验

1. 读取 ASC **当前已上架版本**，不能用本机 Ad Hoc 版或最新 tag 代替。
2. iPhone、iPad 分别检查首次启动/配对、雷达、对话与终端、回复/选项、HQ、服务器
   管理和设置，并覆盖错误、弹层、选中及禁用状态。使用模拟会话和凭据。
3. 深色界面：常用流程均可使用深色外观。不依赖颜色：检查灰度截图，确认状态、连接、
   选择、告警和控件仍可通过文字、形状或位置区分。
4. 记录准确源码/版本、设备类别、测试环境、场景、截图与缺口。真机布局、VoiceOver
   焦点/朗读、推送和硬件交互另行验收，源码、单测及模拟器截图不能代替。
5. 在 ASC → App Accessibility 中，仅保存/发布满足常用流程标准的能力，并独立回读。
   更新标签不等于提交 App 版本，最终 Submit 仍由操作者执行。

[Apple 的常用流程标准](https://developer.apple.com/help/app-store-connect/manage-app-accessibility/overview-of-accessibility-nutrition-labels/)
适用于完整常用操作，不能凭少数控件推断。不要仅因已有标签、字号设置或某个组件
支持减少动画，就勾选 VoiceOver、Voice Control、Larger Text、Sufficient Contrast
或 Reduced Motion。终端的三个字号档位也不等于支持放大到 200%。

本次实测结果和 ASC 写入回执保留在任务的私有证据中；本文描述可重复执行的流程，
不代表未来版本自动取得同样结论。

## 每次上架必须复评

以下命令从 `mobileapp/` 运行，沿用当前进程里的 ASC 凭据。脚本只读 ASC，
不会发布声明或提交 App；公开 PR 的 CI 也不接触真实凭据。

候选包校验完成后，新建一份私有复评：

```sh
bundle exec ruby scripts/asc-accessibility.rb init \
  --version X.Y.Z --build N --package-receipt build/store-package.json
```

默认保存到 Git 忽略的 `build/store-accessibility-review.json`，仅当前用户可读写。
已有文件不会覆盖；新一轮评估可用 `--review /private/path/new-review.json` 指定新路径。
模板读取当前商店版本、build 和全部分页的 PUBLISHED 声明，草稿不算已发布。
ASC 的历史已发布版本也会保留 READY 状态，脚本按数字版本号选择最新已发布的
iOS 版，排除未上线候选版。首次上架或分发情况不明确时须人工核对，不能算作通过。

填写模板。空白项和 `supported: null` 默认不通过：

- `reviewed_by`、`reviewed_at`（ISO 8601）、`change_review`：记录评估人、时间及
  相对商店在线版本的变化。检查新增常用流程、导航、主题/颜色、文字/缩放、动效、
  回复、SDK/RN/原生依赖和扩展。每次提交都复核；结论不变也要有依据。
- `url_review`：确认可选网址继续留空，或已有页面可访问、内容符合在线版本。
- `families.IPHONE`、`families.IPAD`：分别填写 `environment` 中的准确源码/产物、
  系统与设备；记录常用流程证据和 `limitations`。覆盖首次启动/配对、雷达、对话/
  终端、回复/选项、HQ、服务器/设置、错误/弹层、通知/实时活动入口。区分源码检查、
  模拟器截图和真机验收，没测不能填通过。
- 九项 `capabilities`：`supported` 填布尔值，`evidence` 填证据；不声明支持时写明
  原因，例如“尚未验收”。声明支持仍须符合 Apple 的完整常用流程标准。

完成后检查：

```sh
bundle exec ruby scripts/asc-accessibility.rb status \
  --version X.Y.Z --build N --package-receipt build/store-package.json
```

`asc-preparation.rb status|assets` 也会强制执行同一检查，可用
`--accessibility-review FILE` 指定非默认复评。版本、build、包回执哈希不符，证据
缺失，或在线基线/声明已变化，都会阻止“准备完成”；`assets` 在放置素材前检查。
已有能力回退时，先修复；若原声明确实不准确，须在 ASC 更正后重新复评，不能只
承诺上线后再修。网络失败也不能保留旧的通过结果。脚本能核对身份和完整性，
不能证明人工结论属实，更不能代替 VoiceOver 真机验收。

## 上线后核对声明

准确的既有声明可以保留。候选版本新增能力时，填写 `after_release.owner` 和
`after_release.plan`，随私有交接保留到上线后处理；准备检查会列出待发布能力，
不会提前写入。包已上传或审核通过，都不等于已上线。

**确切版本和 build 上线后**，在 ASC → App Accessibility 中按 iPhone、iPad 分别
发布已验证的能力，不顺带修改其他设备类别、隐私或可选网址。随后运行：

```sh
bundle exec ruby scripts/asc-accessibility.rb live-status \
  --version X.Y.Z --build N --package-receipt build/store-package.json
```

非默认复评加 `--review FILE`。命令比较在线 build 和两类设备的 PUBLISHED 能力；
缺失或不一致仍为未完成。将输出与复评、包回执、证据一起留存，再分别预览两类
设备的商店辅助功能区域。API 回读和用户实际看到的页面分开记录，Apple 说明标签
更新可能需要 24 小时显示；两者都不代表真机辅助功能验收。
