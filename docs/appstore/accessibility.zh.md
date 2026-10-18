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
