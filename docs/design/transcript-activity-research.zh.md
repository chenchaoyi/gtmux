# 对话中的执行过程展示

英文正本：[English](transcript-activity-research.md)。

用户截图里连续的 `js`、`wait` 没有层次，也没有展示调用自带的描述。问题在展示与信息提取，不需要增加 agent 状态。

## 官方参考（2026-10-19 核对）

- [Claude Code Desktop 的视图模式](https://code.claude.com/docs/en/desktop#switch-view-modes)：Normal 把工具调用折叠成摘要，保留完整回复；Verbose 展示每次调用。借鉴按需展开，不再加一个全局模式选择器。
- [Codex 的活动分组](https://github.com/openai/codex/blob/e4568a3538c9cab6627d64ff8cdfc7d98b79b7db/codex-rs/tui/src/history_cell/activity_group.rs)：相邻调用按顺序组织，活动更新时保留细节的位置。借鉴回复段落之间的有序过程分组。
- [Codex 的终端交互记录](https://github.com/openai/codex/blob/e4568a3538c9cab6627d64ff8cdfc7d98b79b7db/codex-rs/tui/src/history_cell/exec.rs)：轮询和输入记录保留在详细/原始历史中，普通对话不反复打印通知。借鉴紧凑摘要，避免一墙不明所以的工具名。

Codex 参考是其官方开源终端界面，并非检查过的私有桌面实现；Claude 参考来自官方文档。本轮未操作这两个桌面 app，没有复制代码或视觉素材。

## 方案

每段工具调用放在低对比度描边的「执行过程」卡片中，与回复气泡分层。标题显示调用总数，收起时按工具名汇总数量（前三种工具，再提示其余种数）；展开后按顺序编号，工具名在上、参数摘要在下。使用中性文字与分隔线，不用青色工具名墙，也不凭调用存在就画成功勾。手机/iPad 延用执行中默认展开、历史默认折叠、用户选择优先的规则；Web 专注对话保留默认折叠；工作台紧凑小窗仍只显示回复。

Codex 优先取字符串 `title`，再取 `description`，否则沿用命令/路径等字段。仍合并空白、限制为 80 个 Unicode 码点加省略号。客户端按可选择的纯文本展示，不把描述当 HTML 或 Markdown。未知工具和旧服务端保留真实名称。

## 边界

调用被记录不证明执行成功或已经完成。现有步骤契约没有结果、退出码、耗时，本轮不推测这些信息；也不展示完整参数、命令正文或 stdout/stderr。只有 `js` 且缺少描述时无法还原用途。旧服务端已经缓存的历史仍使用旧摘要，正常更新重启后才使用新解析器；本轮不清日志、不重启真实 agent。界面验证使用合成数据，不等于实际执行了对应命令。
