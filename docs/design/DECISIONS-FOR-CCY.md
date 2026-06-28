# 需要你拍板的决策（醒来再看，我不会停）

> **已决策 (2026-06-28)：D1=(a) 维持单层 TLS+token，写清信任边界（见 `SECURITY.md`）；
> D2=(a) 维持雷达 + radar++，不做 launcher；D3/D4/D5 实施；D6 暂缓。**
>
> **已交付 (2026-06-28)：**
> - ✅ D1=(a) `SECURITY.md`（PR #170）
> - ✅ D3 对话乐观回显（PR #174）—— 注：字面意义的 Mosh 预测回显**不适配** gtmux 的 composer 输入模型（本地即时回显，批量发送），所以实现为「发送即在对话里乐观显示气泡，等 transcript 追上替换」。
> - ✅ D4 免 hook 的 **CPU 活动**working 信号（PR #172）—— 注：诚实评估：LLM「思考」是**网络 bound**（本地进程空闲），CPU 兜底主要覆盖**本地工具**（编译/测试）无输出的情况；OR 叠加，只会 idle→working，不引入抖动。
> - ✅ D5 `gtmux status` tmux 状态栏段（PR #173）
> - ✅ 顺手修了：二维码中心 logo 对齐品牌（PR #171）
> - 发布：CLI/app 走 **v0.11.5**；移动端改动（D3 + A 的 ChatView）需一次**真机构建**才能上手机（tag 流水线不含移动端）。
>
> **D2(a) radar++ 待办（下个专注会话做，已写好可直接实现的规格）：** 给雷达行补充信息。
> **不在本会话改 `gatherAgents` 核心循环**——那正是「空雷达」bug 所在的关键代码，刚修过，不在超长上下文末尾冒险。
> 实现规格：(1) 在 `gatherAgents` 的 list-panes fields 末尾加 `#{pane_current_path}`（SplitN 9→10）；
> (2) 新增 `gitInfo(cwd)`：直接读 `<cwd>/.git/HEAD` 解析 branch（不起 git 子进程；处理 worktree 的 `.git` 文件 + detached SHA），向上找 `.git` 取 repo 顶层 basename 作 `project`；
> (3) `agentJSON` 加 `branch`（omitempty）、给 tmux agent 填 `project`（契约**附加**，消费端可忽略，安全）；
> (4) 先在**手机行**显示 branch/project（主屏），菜单栏/web 随后；
> (5) 后续 radar++：dev-server 端口/URL、PR 状态、idle 的「待 review」（这些每行成本更高，单独评估）。

> 来自 2026-06-28 的同类项目调研（见 `RESEARCH-prior-art-2026-06.md`）。下面每条我都给了**默认假设/推荐**——在你回复前，我就按推荐继续做，不卡住。你醒来后只需对不同意的条目说一声。

---

### D1. 端到端加密 + 零知识中继（最大的战略问题）
**背景**：最接近的竞品 Happy（~22k star）主打「服务端只存加密 blob，连推送内容都解不开」。gtmux 现在是单层 TLS 隧道 + bearer token，CLAUDE.md 自己写了「token 泄露 = 在 Mac 上执行命令」。
**选项**：(a) 维持现状，只把信任边界写清楚；(b) 先上 **QR 配对（临时密钥对）** 把 token 包起来、改善上手与泄露面（中等工作量）；(c) 完整 E2E（每 session/每机器 AES-256 DEK + 公钥层），工作量大。
**我的推荐 / 默认**：先做 (b) QR 配对（性价比最高，且和「手机扫码连 Mac」体验天然契合），把 (c) E2E 记入 backlog 作为后续大版本。**除非你说做 (c) 或先不动**，我会在做完 A/B/C 后排 (b)。

### D2. gtmux 要不要从「雷达」走向「编排器」（worktree/并行启动）？
**背景**：编排器赛道很热（vibe-kanban 27k / cmux 23k / claude-squad 8k 都用 tmux+worktree），但也在洗牌（vibe-kanban 日落、Crystal 被弃）。
**我的推荐 / 默认**：**核心保持雷达**，只做「radar++」——在行里补 diff 统计 / dev-server 端口 / PR 状态 / 按项目分组 / idle 的「待 review」态。**不**主动做 worktree launcher（最弱的位置）。可选地、以后加一个**很薄**的 `gtmux new --worktree <task>`（复用现有 new/tmux）作为加分项，喂回雷达而非取代它。**默认不做 launcher**，等你明确要再说。

### D3. 终端输入要不要做「预测本地回显」(Mosh/sshx 式)
**背景**：手机隔隧道打字延迟感很差；sshx 用本地预测回显解决。
**我的推荐 / 默认**：值得做，但属于体验优化、非阻塞。**默认排到 A/B/C 之后**。除非你觉得现在手机输入体验是痛点要提前。

### D4. 状态检测加「免 hook 兜底」信号（CPU 采样 / OSC 转义 / buffer 哈希）
**背景**：cmux 翻车证明 hook-only 会让 Codex 行「卡死」。gtmux 已有确定性分类器兜底；可再补 **pane PID 的 CPU 采样**（tap-to-tmux 做法）给非 Claude agent 一个统一的 working/idle 信号。
**我的推荐 / 默认**：**会做**（低风险、强化既有优势），排进 backlog。除非你反对。

### D5. 把雷达渲染进 tmux 自己的状态栏（`#(gtmux status …)` 段）
**背景**：gitmux 证明成本很低，能让雷达在 tmux 里也可见（现在只在菜单栏/手机）。
**我的推荐 / 默认**：**可选小功能**，默认排低优先级 backlog。等你说要不要。

### D6. 手机「快捷回复」升级为可答**权限提示**（exit code 2 拦截）
**背景**：claude-ntfy-hook 证明通知动作能回到 agent 答 Allow/Deny（`PreToolUse` 返回 2=拦截/0=放行），比单纯 `send-keys` 强。
**我的推荐 / 默认**：好点子但要动 hook 协议，**默认记入 backlog**，A/B/C 之后评估。

---

## 我现在的执行顺序（你睡着期间自动推进）
1. ✅ 空 agent 列表 bug 修复 + v0.11.3 发布 + 装机（已完成、已验证）
2. ✅ 同类项目调研（本批文档）
3. ⏳ **A：chat-history**（按 §6 schema 自写 Go parser：Claude + Codex 双 adapter → `GET /api/transcript` → 手机 ChatView 显示「prompt → 折叠的中间步骤 → 最终回复」）
4. ⏳ **B：菜单栏「当前被远程连接」指示**（serve 记录活跃连接 → 状态文件 → 菜单栏显示）
5. ⏳ **C：双隧道防护**（`gtmux tunnel` 检测菜单栏已有 `com.gtmux.tunnel` 在跑 → 打印现有 URL，不再起第二条）
6. 之后按上面 D1(b)/D4 等 backlog 继续

> 凡是「不得不你决策」的，我都会追加到本文件，不阻塞继续做下一个任务。
