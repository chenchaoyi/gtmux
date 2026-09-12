# 知识的三条轴与四个读者：种类 · 出处 · 给谁看

参谋长身上有三处「写着规矩和事实的地方」，它们**看起来都像知识，实际归属完全不同**。
分不清它们，就会出现这样的错觉：把一条本该所有人都拿到的通用改进，留在了一台机器的私有台账里
（2026-09-05 司令就这样问过：「我以为知识库是个性化的本地用户信息，这个看起来是通用的优化建议」）。

这份文档回答四个问题：**知识住在哪、一条知识是什么、它从哪来、谁必须知道它。**
前一个是「三层」，后三个是每条条目身上的「三条轴」（openspec change `hq-knowledge-engine`，
调研依据见 `knowledge-engineering-research.md`）。

## 三层：知识住在哪

| | 出厂章程 `AGENTS.md` | 你的守则 `LOCAL.md` | 这台机器的台账 `knowledge/` |
|---|---|---|---|
| 归属 | **gtmux 产品** | **这个操作者** | **这台机器的参谋长** |
| 谁写 | 代码：`internal/hq/hq.go` 的 `hqInstructions` + `playbook_zh.go` | 你，手写；gtmux 只在你让它「写进去」时追加 | 参谋长，边干边记 |
| 怎么更新 | 改代码 + 升 `hqPlaybookVersion` → 随 `gtmux update` 下发，重新生成 | 你自己编辑；**gtmux 永不覆盖**（种一次） | `gtmux knowledge add/supersede/retire/…`，追加式台账 |
| 何时生效 | **每轮会话都在上下文里** | **每轮会话都在上下文里**（`AGENTS.md` 末尾 `@LOCAL.md` 导入） | **按需**：派活时按仓库名/关键词回声给 worker；其余靠参谋长主动查 |
| 形态 | 一份被管理的文档，不可手改 | 一份你的文档 | 一本带出处的台账（`internal/knowledge`，可 supersede、可退休、有审计） |

**导入顺序是有意的**：`AGENTS.md` 的正文在前、`@LOCAL.md` 在最后 —— 你的守则**扩展并覆盖**出厂章程。

## 三条轴：每条条目身上的三格

台账里的每条条目在三条正交的轴上各占一格。参谋长和人都能答「这是什么、哪来的、给谁」。

**种类（kind）—— 它是什么。** 沿 CoALA 的语义 / 程序记忆二分再细一层：

| kind | 意思 | 例 |
|---|---|---|
| `facts` | 这台机器、这个账号、这个世界是怎样的 | 办公网会 TLS reset wrangler |
| `howto` | 事情怎么做 | 发版：打 tag，等 app job，装机 |
| `pitfalls` | 别这么做 | 装机别指定 udid，锁屏时会挂 |
| `judgment` | 什么情况下该怎么判 | 窗口上限看它实际跑到过多少，不看模型名 |
| `decisions` | 为什么选了 A 不选 B | 分发只放索引不放全文，因为块进每个会话的上下文 |

`topic` 保留为 id 的前缀和自由标签（`--tags`），参谋长自己声明的主题照旧。轴出现之前写的条目，
读取时按一张固定表映射种类并标为「推定」（渲染里带 `?`），`gtmux knowledge kind <id> <kind>` 确认；
台账文件本身永不改写，第一次写新格式时会把旧文件备份成 `.ledger.jsonl.bak-v1`。

**出处（provenance）—— 它从哪来，出现过几次。** `correction`（司令纠正）· `recurrence`（同一个坑又踩）·
`mined`（会话采矿挖出来的）· `capture`（worker 随手记的）· `self`（参谋长自己观察到的），带一个计数。
`gtmux knowledge hit <id>` 让计数涨；采矿器认出台账里已有的报错签名时自动记一笔。**一条已经落库的教训
还在涨计数，说的是载体没被读，不是没记住**——这是 ACE 那个「有用 / 有害计数」，也是整条学习循环回流的信号。
以前 `corrections` 是一个主题，那是按「谁告诉我的」分类，现在它是这一格。

**读者（audience）—— 谁必须知道它。** 这条轴只在晋升时填，四个词，也是两块屏上显示的四个词：

| `--for` | 词 | 谁读 | 落在哪 |
|---|---|---|---|
| `hq` | HQ | 只有参谋长自己 | `LOCAL.md`，gtmux 追加一节 |
| `machine` | 本机 | 这台机器上所有 agent | 一份正本 `~/.config/gtmux/knowledge/machine.md`，各 agent 全局指令文件里一个索引块 |
| `repo:<path>` | 仓库 | 在那个仓库干活的 agent | 那个仓库的 `AGENTS.md`（只有 `CLAUDE.md` 时用它）里一个块，**不提交** |
| `everyone` | 全体 | 所有 gtmux 用户 | gtmux 产品：一条预填好的 GitHub issue |

判不出「谁」的，就不该晋升，留在台账。

另有一个状态 `hypothesis`：采矿或自述得到、还没证实的线索，`add --hypothesis` 落进自己的一节，
不分发，`confirm <id>` 转正。

## 出口：promote → land，或 withdraw

台账是本机私有的，但会长出比这台机器大的条目。出口是机械的，不靠记性：

1. 参谋长判断某条够大 → `gtmux knowledge promote <id> --why … --for <hq|machine|repo:<路径>|everyone>`
2. gtmux 在 `knowledge/promotions/` 写一份**带走简报**：教训原文、为什么、读者、完整出处、这个读者的出口
3. **落地**。前三种读者 gtmux 替你搬：`gtmux knowledge land <id>` 把它写进那个读者看的地方并关闭。
   `everyone` 是人开 issue（简报里有预填链接，两块屏上是「反馈给 gtmux ↗」），再 `land <id> --ref <issue 链接>`。
   自己搬了也行：`land <id> --ref <哪里>`
4. 参谋长判错了、这条不值得搬：`gtmux knowledge withdraw <id> --why …`，条目回到「活着」。
   不用 `retire`（条目没错），也不用编一个假出处
5. `gtmux doctor` 盯着队列：超过约两周没搬的会标出来；`everyone` 的不计超期，产品的事不该由用户背红线

## 分发：本机知识怎么到每个 agent

`machine` 的正本由 gtmux 渲染，然后往每个已支持 agent 的全局指令文件里维护一个带哨兵和哈希的托管块：
Claude Code `~/.claude/CLAUDE.md`、Codex `$CODEX_HOME/AGENTS.md`、opencode `~/.config/opencode/AGENTS.md`、
Kimi Code `$KIMI_CODE_HOME/AGENTS.md`（路径记在 agent 注册表 `internal/agents`）。

块里**只放索引**（每条一句话，加正本路径），不放全文：块进每个会话的上下文，索引长不成问题，
而四家 agent 都能读文件（Skills 的渐进披露）。块外的内容一律不动；块被手改过的，`sync` 拒绝覆盖，
`--force` 才写。`gtmux knowledge sync` 刷新，`carriers` 看每家状态，doctor 有一行「知识分发」，`--fix` 补上缺的和过期的。

`repo` 的块放全文（小，只有那个仓库的 agent 读），gtmux 写文件不提交，提交是你的。

## 三层之外：候选池与两道闸

`gtmux capture "<一句教训> @<topic>"` 是**成本最低的入口**：worker 随手记一句，落进
`knowledge/.pending-distill.jsonl` 的候选池，**不是**台账条目。候选池的第二个投递方不是人，是**会话采矿器**
（`gtmux knowledge mine`，serve 每天自动跑一轮）：不用模型，从各 agent 的会话日志里减掉机器自己写的一切，
把「人在 agent 回话后紧接着打的纠正」和「跨会话反复出现的报错」投进同一个池子。

参谋长的 distill 是**唯一的质量闸门**：把同一件事的候选并成一条（`knowledge add --capture k1,k2,…`；
`capture --list` 已按家族分好组），或带理由驳回（`dismiss --why …`，驳回同样留痕）。distill 只做增量，
永不整篇重写主题文件，`supersede` 保留旧文（`show <旧 id>` 还能读到）——这是 ACE 实验证明的两个塌缩路径。

第二道闸审台账本身：`gtmux knowledge lint` 报孤儿、断链和过时链接、疑似重复、超期、待确认的种类，只报不改；
它的一行摘要随 self-check 的敲门送到参谋长面前。`neighbours` 找最相近的条目，`add` 写入前会先列出三条，
同一件事就该 `supersede` 而不是再加一条。

## 权威文件

- 台账与全部动词、渲染、分发、lint：`internal/knowledge/`（叶子包，`hq` 只留监督）
- 采矿器：`internal/mine/`
- 设计决策与被否掉的替代方案：`openspec/changes/archive/2026-09-12-hq-knowledge-engine/design.md`（D1–D10）
- 调研：`knowledge-engineering-research.md`
