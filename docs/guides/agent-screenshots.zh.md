---
title: 给本地 agent 使用的截图能力
description: 通过 CLI 截取、查看、标注截图，并投递到准确的 agent 会话。
order: 7
---

[English](agent-screenshots.md) · **中文**

本地 agent 直接通过命令工具运行 `gtmux screenshot` 即可，返回带版本号的 JSON、图片路径和 SHA256。此工作流不需要额外 MCP 服务。原生截图/标注需要兼容且**正在运行**的 Mac App；能力查询不会启动 App 或弹权限申请。更新 App 时，本地接口最多用两秒等待旧实例交接。

## 观察、截取、查看、标记、发送

图片与 sidecar 存在当前任务自己的输出目录，不把 gtmux 配置或状态目录当项目工作区。

```sh
mkdir -p /tmp/my-task-shots
gtmux screenshot capabilities
gtmux screenshot sources --app com.mitchellh.ghostty
gtmux screenshot capture --source-ref REF --rect 40,60,800,500 --unit pt --output /tmp/my-task-shots/terminal.png
```

从窗口清单选择 `REF`；明确需要整个显示器时用 `sources --displays`。引用 30 秒后过期，截取前后都会核窗口所属进程、进程启动时间、边界、显示器映射及像素缩放；即使逻辑边界没变，缩放改变也会让原观察失效。来源改变会拒绝，不会悄悄扩大到整个屏幕。区域坐标是来源左上角的**逻辑点**，支持显示器全局原点为负；超大完整来源、跨不同缩放显示器的区域会拒绝，需要选较小来源或分区截取。

agent 用自己的图片查看工具读取返回的 `preview_path`。sidecar 给出原图/预览尺寸与哈希、capture ID 和坐标变换；标注必须使用该帧。也可导入已有图片：

```sh
gtmux screenshot import --input /tmp/existing.jpg --output /tmp/my-task-shots/imported.png
```

导入结果明确标为 imported，不伪造实时窗口来源。标注 JSON 示例：

```json
{
  "schema_version": 1,
  "capture_id": "从 sidecar 复制",
  "image_sha256": "从 sidecar 复制",
  "coordinate_space": "image_px",
  "annotations": [
    {"type": "rect", "x": 20, "y": 30, "width": 300, "height": 80, "color": "red"},
    {"type": "arrow", "from": [500, 200], "to": [320, 90], "color": "blue"},
    {"type": "text", "x": 20, "y": 130, "text": "检查这个结果"},
    {"type": "redact", "x": 20, "y": 220, "width": 200, "height": 40}
  ]
}
```

也支持 `preview_px`，但必须附相同帧的 `preview_id`。其他标记为 ellipse、mosaic、带 `points: [[x,y], ...]` 的 line。颜色 red/blue/yellow，线宽 thin/medium/thick。`redact` 对相交像素向外取整并用不透明色覆盖，缩小预览也保持这一规则，派生 PNG 中不保留被覆盖像素；mosaic 只是视觉模糊，不能当作同等脱敏。未脱敏原图仍在本地，需妥善保管，发送时明确选择**派生文件**。

```sh
gtmux screenshot annotate --capture /tmp/my-task-shots/terminal.capture.json --spec /tmp/my-task-shots/marks.json --output /tmp/my-task-shots/marked.png
gtmux screenshot edit --document /tmp/my-task-shots/marked.capture.json
```

复用现有原生渲染器及编辑器，保留原图和标记供调整。已有编辑草稿会保留并返回 `editor-busy`。截取、导入、标注、打开编辑器都不会隐式发送，也不覆盖同名输出。已有可编辑派生文档不能再次隐式套一层标记：用 edit 调整，或明确把扁平图片重新 import 为新底图。

## 投递到准确会话

```sh
gtmux screenshot target --pane %9 --session-id EXPECTED_SESSION_ID
gtmux screenshot send --image /tmp/my-task-shots/marked.png --target-ref TARGET_REF --request-id task-shot-00000001 --message-file /tmp/my-task-shots/note.txt
gtmux screenshot status task-shot-00000001
```

目标引用有效十分钟，绑定 session、home、客户端、pane 根进程与 tmux server 的启动身份。首期支持已核验的 managed Codex、DeepSeek Harness 原生绑定，其他客户端继续使用既有 `gtmux send --attach`，不新增向裸终端输入的回退。目标按需解析，不往常态会话列表刷新里增加进程扫描。

回执是带时间的**最近一次观察**：

| 状态 | 含义 |
| --- | --- |
| landed | 输入回执确认已进入对话。 |
| active_turn_accepted | 当前轮次已接纳图片输入。 |
| queued | 后台接受排队，尚未确认进入对话。 |
| blocked / failed | 被拒绝或未取得输入确认前失败。 |
| unknown | 无法确认是否已接受；保留请求 ID 排查，不自动重发。 |

这些都不代表 agent 已读或任务完成。status 只读持久回执，不恢复 thread，不声称之前 queued 的消息现在仍在排队。同 ID/同载荷不会悄悄再次发送；更换图片、说明或目标会返回冲突。过期目标为**新提交**重新解析，旧请求直接按 ID 查状态。

## 边界与落盘

请求最多十秒、JSON 256 KiB、单图 30 MiB/3200 万像素、500 个标记/10000 个路径点；预览长边最多 1600 像素。App 只接本用户 Unix 连接，核宿主进程与实例 nonce，通过已打开文件描述符传图，不接受任意原生输出路径；无 TCP、命令执行、剪贴板修改或隐式录屏权限申请。闲置时监听阻塞等待，最多两个并发请求槽位。超时、调用方断开或宿主停止后，请求失效并关闭连接；图片解码、渲染和文件读写离开 UI 线程。底层系统回调或文件系统调用可能无法中断，此时保留有界工作配额直到实际结束，新工作返回 `native-busy`，迟到结果不会再打开编辑器或发布产物。底层截取挂起时仍可查询 capabilities，但这不代表可以自动重发结果未知的投递。

调用方持有 PNG/preview/sidecar。产品管理的 capture-bridge 只存 socket、宿主身份和锁；capture-requests 存私有目标签名 key、哈希回执，不写图片或说明正文，目录最多 1000 项。满额会明确失败，不自动删未确认回执。本轮不自动迁移或删除已有归档。附件投递沿用既有 uploads 行为。

Linux 可读 CLI 帮助并使用受支持的投递逻辑，原生图片操作返回 platform-unsupported。真实录屏授权、真实窗口截取及签名安装验收，与合成图片/隔离 socket 测试分开记录。
