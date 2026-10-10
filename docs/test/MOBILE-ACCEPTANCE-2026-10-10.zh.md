# 手机验收记录 — 2026-10-10

[English](MOBILE-ACCEPTANCE-2026-10-10.md)

实测已连接的 iPhone 15 Pro Max、iOS 26.6.2，App 为 **v1.0.100 (1)**。
工具为 Xcode 27、Appium 3.7.0、XCUITest 11.17.3、WDA 15.1.4。
安装此前已完成，本轮没有重新构建或安装 gtmux。
本地证据：`/private/tmp/gtmux-v100-device-acceptance/`；截图和 XML 含私人对话，不提交仓库。

| 范围 | 实测结果 | 证据 |
| --- | --- | --- |
| 截图工具 | `screenshotr` 不可用；Device Hub 要求 iOS 27；签名 WDA 成功 | `screenshot.log`、`device-details.json`、`appium.log`；Device Hub 的明确版本提示 |
| HQ 长按 | 两个菜单按钮均可见，高 45pt，位于 430×932pt 屏幕内，有按钮角色 | `hq-menu.png/.xml` |
| 用量快捷入口 | 从长按菜单打开，有真实内容；关闭后返回 HQ | `current.png/.xml`、`hq-after-usage.png/.xml` |
| 知识库面板 | 从 HQ 顶部打开；759 条、7 个主题和条目均可见；成功关闭 | `knowledge.png/.xml` |
| 列表末尾 | 最后一行底部约 y773，隐藏计数位于 y804，没有长空白尾部 | `radar-bottom.png/.xml`、`radar-end.png/.xml`；隐藏 1 个会话，与折叠的报错组一致 |
| 底部提示间距 | 复现发送状态紧贴终端下沿；用户截图另显示任务条和失败卡片 | `initial.png/.xml`；用户提供的截图 |
| 面板读屏标签 | 用量关闭暴露 `hq-usage-close`；知识库关闭为符号，搜索为 `knowledge-find` | `current.xml`、`knowledge.xml`；已补源代码修复和回归测试 |

## 仍未验收

- Codex 顶部预览和独立全文面板：打开的真实终端没有识别到吸顶指令；没有为了测试向窗格发送虚构消息。
- 从**长按菜单的知识库按钮**直接进入：面板已从 HQ 顶部验过，第二条快捷导航路径尚未验。
- 跟进开关和切换时布局：本轮列表未出现桌面工作会话；没有为了测试修改真实授权。
- 全部折叠的列表：手机切到大象（`com.meituan.message`）时，最后一次控件树读取超时，截图不是 gtmux 验收结果。
  尝试中折叠了工作中、空闲两组；恢复验收时应展开这两组，保留用户原本折叠的报错组。
- VoiceOver 实际播报、焦点和自定义操作：已请求人工检查；当前驱动的相关命令要求 iOS 27，iOS 26 不支持。
- iPad、横屏、大字号、中文视觉验收均未执行。

发现前台 App 改变后已停止设备操作，没有后台轮询等待解锁，也没有继续操作手机。
底部间距和面板读屏标签修复尚未安装到此设备，不标为视觉或实际读屏通过。
后续可沿用 [REAL-DEVICE.zh.md](../../mobileapp/e2e/REAL-DEVICE.zh.md) 的工具与验收流程。
