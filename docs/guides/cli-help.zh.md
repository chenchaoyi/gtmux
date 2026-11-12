---
title: 命令帮助
description: 查找常用命令、切换帮助语言，以及供脚本读取的命令目录。
order: 8
---

**中文** · [English](cli-help.md)

`gtmux --help` 按用途列出常用命令：状态与用量、会话与终端、HQ、远程访问、安装更新，以及截图与标注。更少使用的命令保留在“更多命令”中。

- `gtmux <命令> --help`：查看该命令的完整参数。
- `gtmux --help --json`：读取结构化命令目录，适合脚本和助手使用。
- `gtmux --lang=zh --help`：使用中文帮助；也可指定 `--lang=en`。

长命令会与说明分行显示，避免参数被截断或文字挤在一起。较窄的终端统一使用上下排列。

截图能力从 1.0.175 起已提供，并非只在 App 中可用：

```sh
gtmux screenshot --help
gtmux screenshot capabilities
```

第一条查看中文操作步骤；第二条检查正在运行的 Mac App 是否支持截图、标注，不会弹出权限申请。截图、标注和发送是独立操作，截图不会自动发送。需要发送时，先用 `target` 核对会话，再用 `send` 显式发送。

完整示例见[给本地 agent 使用的截图能力](agent-screenshots.zh.md)。这些调整只改善帮助文案和显示，不改变命令参数、访问权限或发送行为。
