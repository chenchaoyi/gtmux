# `gtmux attach` —— PTY over WebSocket 调研（2026-07-14）

一轮调研（深度调研流水线：28 个来源 → 116 条断言 → 25 条对抗式核验，24 条成立 / 1 条推翻），
支撑 `remote-terminal-client` 这个 change：把远端 tmux pane 双向流到本地一个裸终端，
走现有的 `gtmux serve` HTTP 面（WebSocket 穿 Cloudflare 隧道 —— WS/TCP，不是 SSH/UDP），
遵守 owner/guest 的 token 范围。

## 可借的模板 —— gotty + ttyd + creack/pty

- **架构：gotty**（Go）—— 一个 WebSocket 中继（输出→客户端，输入→PTY，一条 WS）。
  注意：gotty 为浏览器 xterm 做了 base64 编码；我们发**原始字节**。借它的中继形态，不借编码。
- **线上协议：ttyd** —— 二进制 WS 帧，首字节是操作码，载荷从下标 1 开始
  （客户端→服务端 INPUT/RESIZE/PAUSE/RESUME；服务端→客户端 OUTPUT）。gtmux 照搬的就是这个
  （`internal/connect/frame.go`）：OUTPUT 里是原始 PTY 字节，无 base64。
- **PTY 原语：creack/pty** —— `pty.Start(cmd)` → master `*os.File`；读写它就是字节泵。
  远程改尺寸：`pty.Setsize(ptmx, &Winsize{Rows,Cols})`（TIOCSWINSZ）——
  服务端没有本地 tty 可供 `InheritSize`。
- **输入默认拒绝（gotty `--permit-write`）** → 扩展为**按 token 范围**：
  对只读的 guest pane，服务端丢弃 INPUT/RESIZE 帧。永远不信任客户端。

## 桥接 tmux —— 在服务端 PTY 里 attach（不用 control mode）

在 `creack/pty` 的 PTY 里起 `tmux attach-session -t <session-of-pane>`，把 master 逐字节流出去。
**不用** control mode（`-CC`）：那是八进制转义、按行分帧的**文本**（`%output`、`%begin/%end`，
<32 的字符 → `\ooo`），必须解析/反转义，不适合原始透传（iTerm2 用它做原生 tab 映射 ——
目标恰好相反）。**也不用** `pipe-pane`+`send-keys`（gtmux 今天的做法 —— 不是真正的 attach）。

## 坑（设计时就要防）

核心认识：**走 TCP/WS 的原始透传就是 SSH 那一类**，继承 SSH 的洪水/背压问题和 Ctrl-C 延迟。
Mosh 靠 UDP 上的屏幕状态 diff 避开了它（跳过中间状态；Ctrl-C ≤1 RTT 内生效）——
我们做不到（Cloudflare 隧道是 WS/TCP），所以要明确缓解：

1. **输出洪水 / 背压** —— 狂刷输出的 pane（`yes`、吵闹的构建）跑赢慢客户端 → 无界缓冲/OOM/卡顿。
   MVP 缓解：同步 `WriteMessage` 天然形成背压链（客户端慢 → WS 写阻塞 → 服务端停读 PTY →
   tmux 阻塞），输入跑在**独立** goroutine 里，Ctrl-C 永远不会排在输出后面。ttyd 式的显式
   PAUSE/RESUME 已在协议里定义，留给将来的异步（浏览器）客户端。
2. **按键延迟 / 没有本地回显** —— 在稳定的宽带隧道上代价很小（Mosh 的 503ms→4.8ms 是 3G 场景）。
   预测式本地回显需要客户端有终端状态模型，裸客户端没有 —— 是后面的杠杆，不是 MVP。
3. **改尺寸** —— 客户端捕获 SIGWINCH → RESIZE 帧 → 服务端 `pty.Setsize`。tmux 多客户端的尺寸协商
   是个褶皱（`window-size` 可配置；「钳到最小客户端」的说法已被**推翻**）。MVP 用
   `attach-session`（不泄漏）；若干扰到 owner 的客户端，后续改 `new-session -t`（独立尺寸）。
4. **WS over TCP 的队头阻塞 + 不能漫游 IP**，在丢包链路上 —— TCP 的根本限制（没有 UDP 修不了）。
   稳定隧道上少见；蜂窝网络上会劣化。缓解：重连 + 重同步（tmux 保着会话；新客户端重绘）。
5. **服务端按 token 的输入门控** —— 对不可查看的 guest pane 拒绝 WS 升级（不起 PTY）；
   对只读 pane 丢弃写帧。

块边界切断 UTF-8/ANSI 序列对原始透传**不是问题**（我们从不解析；本地终端跨次写入自行拼回）——
值得做一次实测确认。

## 建议（已在本 change 中实现）

gotty 中继 + ttyd 二进制分帧 + creack/pty；在服务端 PTY 里通过 `tmux attach-session` 桥接
（原始透传）；服务端做范围门控 + 丢弃输入；天然背压 + 独立输入 goroutine。推迟：预测式本地回显、
control mode（`-CC`）路径、重连/重同步、`new-session -t` 尺寸。

## 待定问题

Go 的 WS 库（选了 gorilla；对比 coder/websocket，在洪水下压测）；共享会话的 tmux `window-size`
策略；重连/重同步机制；attach 粒度（会话 vs 指定 window/pane）。覆盖空白：sshx / VibeTunnel /
tmate / wetty / xterm attach addon 没有产出存活的断言（VibeTunnel 是直接竞品；sshx = 主机 PTY +
加密 WS 到无状态中继 + xterm.js —— 值得以后再看）。

主要来源：gotty（github.com/yudai/gotty）、ttyd（github.com/tsl0922/ttyd）、creack/pty、
tmux Control-Mode wiki、Mosh 论文（USENIX ATC '12）。
