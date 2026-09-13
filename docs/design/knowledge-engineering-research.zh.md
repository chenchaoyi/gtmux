# 知识工程调研：中控的知识库该向谁学什么（2026-09-12）

> 背景：中控的知识体系正在从「记规矩」升级成「持续学习」（采矿 → 候选池 → 蒸馏 → 台账 →
> 按读者分发 → 复发回流）。动手改分类和分发机制之前，先看一遍业界现在怎么组织知识，
> 知道了再判断哪些值得借、哪些明确不借。这份文档只做对照和判断，不定实现。
> 现状的三层归属见 `knowledge-layers.md`，采矿机制见 `openspec/changes/archive/2026-09-11-hq-transcript-mining/`。

## 结论先行

看了九种做法。它们在**组织形态**上高度收敛：原子条目、条目之间靠链接而不是靠目录树、
一份不可变的原始材料、编译而不是检索、一个定期体检的 lint。中控的台账已经具备其中四样
（原子条目、`[[同族]]` 链接、带 seq 的出处、确定性渲染），缺的是 lint、相似条目的预分组、
以及分发时的渐进披露。分类框架方面没有一家能直接搬：PARA 和 Johnny Decimal 是给人管项目
和文件的，不是给 agent 沉淀经验的；能用的是 CoALA 的三分（发生过什么 / 世界是怎样的 /
事情怎么做）配上「谁需要知道」这条读者轴。

一句话：**形态上向 Zettelkasten 一系学，分类上用 CoALA，流程上用 Karpathy 的
「编译 + lint」，分发上用 Skills 的渐进披露；PARA / 图数据库 / 向量检索不引入。**

## 九种做法，各一段

### 1. Karpathy 的 LLM Wiki（2026）

原始材料放 `raw/`，永不修改；LLM 把它们编译成一个互相链接的 markdown 维基 `wiki/`；
之后回答问题查维基而不是查原始材料。三个操作：ingest（进新材料，只改受影响的页）、
query（带引用回答）、lint（查索引完整性、断链、健康度）。核心主张是把工作从查询时
挪到编译时：RAG 每次都在重建关系，维基把关系一次性写下来并持续修。
（[概念](https://denser.ai/blog/llm-wiki-karpathy-knowledge-base/) ·
[一个实现](https://github.com/Astro-Han/karpathy-llm-wiki)）

对照：中控的台账**本来就是编译型**：条目是蒸馏的产物，主题文件是台账的确定性渲染，
`gtmux knowledge` 查的是台账不是事件流。差在两处。一是「raw 不可变、每页引用回 raw」：
我们的出处是事件 seq 和会话 id，采矿候选还带对话片段，但落库后**片段不保留**，条目
只剩指针；二是 **lint**：我们有 self-check 审中控的产出，没有一个专门审台账的体检
（孤儿条目、`[[链接]]` 断了、同义条目、过期未 supersede）。

### 2. Obsidian 生态与 obsidian-mind

Obsidian 本身只是本地 markdown 加 `[[双链]]` 加图视图，它流行的原因是**文件就是数据**，
任何 agent 不用接口就能读写。2026 年出现一批「让 agent 维护 vault」的模板，obsidian-mind
是其中结构最完整的一个：笔记按用途分文件夹（决策记录、事故与复盘、人和团队、
观察到的模式、坑、工作流、参考、草稿），每个文件夹一张 MOC 索引，笔记「住在一个文件夹、
链接到很多地方」，规则是**没有链接的笔记是 bug**。它靠 hook 运转：每条用户输入先分类
（决策 / 事故 / 胜利 / 会议 / 人 / 项目）再给路由提示；每次写完校验链接和 frontmatter，
过大的笔记提示拆分；会话结束、每周、审计各一个例行命令。支持 Claude Code、Codex、Gemini，
其他 agent 只读约定。（[obsidian-mind](https://github.com/breferrari/obsidian-mind) ·
[综述](https://www.stefanimhoff.de/writing/agentic-note-taking-obsidian-claude-code/)）

对照：这套的可取之处不是 Obsidian，是三条纪律：**入库先分类再路由**（我们的候选池
入库时 HQ 手判主题，没有预分类）；**写后即校验**（我们 render 是确定性的，但没校验链接）；
**定期审计孤儿和过期**（同上，缺 lint）。它的笔记类型表也印证了我们缺的两类：
决策记录，和「观察到的模式」这种既不是流程也不是坑的判断依据。
另一个便宜的收获：中控的 `knowledge/*.md` 已经是带 `[[链接]]` 的 markdown，
**用户今天就可以用 Obsidian 打开中控家目录看图**，不用我们做任何事。

### 3. Zettelkasten 与 Evergreen notes

卢曼的卡片盒和 Matuschak 的常青笔记是同一条线：**一条笔记一个概念**（原子）、
**按概念而不是按来源组织**（不按书、按项目、按谁说的）、**密集链接**，并且明确
「偏好关联式本体，不要层级分类」。常青的意思是笔记被持续修订而不是写完归档。
（[Evergreen notes](https://notes.andymatuschak.org/z5E5QawiXCMbtNtupvxeoEX)）

对照：我们的条目是原子的、有链接、可 supersede，这三条都对上了。有一条正相反：
`corrections` 主题是**按来源分类**（「司令纠正的」），这在 Zettelkasten 里是反模式，
一条纠正沉淀出来的可能是坑也可能是流程。来源该是条目上的字段，不是它住的抽屉。

### 4. PARA 与 Johnny Decimal

PARA 按行动性分四层：项目、领域、资源、归档。Johnny Decimal 是给文件夹编号。
两者都是**给人管工作和文件**的，回答的问题是「这东西现在跟我哪件事有关」。
（[对比](https://crystaljjlee.com/blog/two-approaches-to-pkm/)）

对照：不借。中控的台账不是任务管理，它的条目不随项目结束而归档；「归档」在我们这里
叫 retire，判据是「不再成立」而不是「项目完了」。

### 5. A-MEM：给 agent 的 Zettelkasten（2025）

一条记忆七个字段：原文、时间、关键词、标签、一段上下文描述、向量、链接。新记忆进来时
先按向量取 top-k 近邻，再让模型判断有没有真联系；**并且反过来更新近邻**：重算它们的
描述和标签，让「更高阶的模式」从链接里长出来。论文的论据正是 Zettelkasten 的三条：
原子、灵活链接、不预设层级。每次交互 1200 到 2500 token，对比基线 16900。
（[论文](https://arxiv.org/abs/2502.12110) · [代码](https://github.com/agiresearch/a-mem)）

对照：两处可借，一处不借。可借的是**入库前先找近邻**：候选进来时列出台账里最像的几条，
这正是中控在第一批采矿队列里要的能力（「贵的是看出 146 条里哪 11 条是一件事」）；
以及**新条目触发老条目更新**：我们有 supersede，但没有「被链接的条目回写同族」。
不借向量：gtmux 是 cgo-free 的本地 CLI，先用主题 + 关键词重合做近邻，够用了再说。

### 6. CoALA：agent 记忆的标准三分（2023）

情节记忆（发生过什么）、语义记忆（世界是怎样的）、程序记忆（事情怎么做），
外加工作记忆（当前上下文）。这已经是 agent 记忆论文的默认参照系。
（[论文](https://arxiv.org/pdf/2309.02427)）

对照：这是「种类」这条轴的答案。情节记忆在我们这里是事件流和态势板，不进台账；
台账装语义和程序两类。现有六个主题按它归位：environment / accounts 是语义，
workflows / pitfalls / best-practices 是程序（一正一反一优化），corrections 不是种类。
缺的种类：决策（为什么选 A 不选 B）和判断依据（什么情况下该怎么判），前者在 CoALA 里
偏语义，后者偏程序，两者今天都散在 PR 和 proposal 里。

### 7. ACE：可演化的 playbook 及它的两个失效模式（2025）

三角色：Generator 产轨迹、Reflector 从轨迹抽教训、Curator 把教训以**带 id 和
有用 / 有害计数的条目**增量并入，合并是确定性的。它点名两个失效：**短化偏置**
（每次优化都往短了改，领域细节和失败模式被删掉）和**上下文塌缩**（让模型整体重写，
18282 token 一步塌到 122，准确率跟着掉）。（[论文](https://arxiv.org/html/2510.04618v1)）

对照：我们的 distill 就是 Curator，采矿是 Reflector 的输入，这条线在 `hq-transcript-mining`
里已经接上。两个失效模式要写成 distill 的硬约束：**只做增量，永不整篇重写**（render
是确定性的、supersede 保留旧文，这两条已经在防）；「有用 / 有害计数」对应我们的复发回流，
应成为条目上的字段。

### 8. Agent Skills：程序知识的打包与渐进披露

SKILL.md 把「怎么做」打包成可加载的目录：会话开始只把每个 skill 的名字和一句描述
（约 100 token）放进上下文，正文和附属文件按需读。这套设计现在是开放标准。
（[文档](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/overview) ·
[分析](https://www.newsletter.swirlai.com/p/agent-skills-progressive-disclosure)）

对照：这直接决定「本机通则」怎么分发。往每个 agent 的全局指令文件里**内嵌全文**会
随时间膨胀并挤占每个会话的上下文；正确形态是 Skills 那种：指令文件里的托管块只放
**索引**（每条一句话加指向正本的路径），全文在 gtmux 的正本里，agent 需要时自己读。
Codex、opencode、Kimi 不支持 `@` 导入，但都能读文件，所以索引 + 路径对四家都成立。
另一个含义：台账里 `workflows` 这一类如果长成多步流程，它的归宿可能就是一个 skill，
而不是一条 markdown。

### 9. SECI：隐性知识的螺旋

野中的四步：社会化（隐到隐）、外化（隐到显）、组合（显到显）、内化（显到隐）。
（[SECI](https://en.wikipedia.org/wiki/SECI_model_of_knowledge_dimensions)）

对照：不是工具，是这条闭环的命名。采矿和纠正是外化，distill 的合并是组合，分发到各
agent 的指令文件是内化，复发回流是「内化失败」的信号。它提醒一件事：**内化那一步
才是目的**，前面三步只是为它服务；所以「分发」不是可选的收尾，是这个循环存在的理由。

## 对照之后：借什么、不借什么

| 机制 | 出处 | 我们有没有 | 判断 |
|---|---|---|---|
| 原子条目、按概念不按来源 | Zettelkasten / Evergreen / A-MEM | 条目原子；`corrections` 按来源分是反模式 | 借：来源降为字段 |
| 链接优先于层级 | 同上 | 有 `[[同族]]` 链接；主题是单层级 | 借：主题当主分类，跨主题靠链接 |
| raw 不可变 + 引用回 raw | Karpathy | 有 seq 出处，落库不留片段 | 借：条目保留触发它的对话片段作范例 |
| 编译而不是检索 | Karpathy | 已经是 | 已有 |
| lint / 定期审计 | Karpathy / obsidian-mind | 无 | 借：`gtmux knowledge lint`，并入 self-check |
| 入库先找近邻 | A-MEM | 无 | 借：候选入库时列相似条目，给 HQ 判合并 |
| 新条目回写老条目 | A-MEM | 只有 supersede | 借：distill 时更新被链接条目的同族 |
| 有用 / 有害计数 | ACE | 复发计数在采矿台账里，不在条目上 | 借：成为条目字段 |
| 只做增量、不整篇重写 | ACE | render 确定性、supersede 留旧 | 已有，写成硬约束 |
| 渐进披露 | Skills | 无 | 借：分发只放索引和路径 |
| 入库先分类再路由 | obsidian-mind | HQ 手判 | 部分借：采矿候选可预标种类 |
| 记忆三分 | CoALA | 主题大致对应 | 借：作为「种类」轴 |
| 按行动性分层 | PARA / Johnny Decimal | 无 | 不借 |
| 向量近邻、图数据库 | A-MEM / Zep | 无 | 不借：先用主题 + 关键词 |
| Obsidian 作 UI | 生态 | 台账已是带双链的 markdown | 不做事，告诉用户可以直接打开 |

## 由此定下的三条轴，和它们改动现状的地方

一条知识在三条正交的轴上各占一格，中控和人都能答「这是什么、哪来的、给谁」：

1. **种类**（CoALA 三分再细一层）：`facts`（环境、账号并入）、`howto`（工作流）、
   `pitfalls`、`judgment`（判断依据，best-practices 并入）、`decisions`。台账的主题词表
   本来就可扩展，这是改内置词表，不是换系统。
2. **出处**（成为字段，不再是主题）：来自纠正、复发、采矿、自述；带次数。这一格就是
   ACE 的计数，也是 SECI 里「内化失败」的信号。
3. **读者**（分发范围）：中控 / 本机 / 仓库 / 全体。见 `knowledge-layers.md` 的四种落点。

另加一个状态 `hypothesis`：采矿出来、还没证实的线索，给它一个位置，免得要么进库要么丢。

## 明确不做的

- 不引入向量库或图数据库。近邻先用主题 + 关键词重合；哪天不够再说，而且要有实测证据。
- 不引入 Obsidian 作为依赖。台账已经是它能打开的形态，这是免费的。
- 不做 PARA 式的归档。retire 的判据是「不再成立」，不是「项目结束」。
- 不让 distill 整篇重写任何主题文件。这是 ACE 实验证明的塌缩路径。

## 参考

- Karpathy LLM Wiki：[概念综述](https://denser.ai/blog/llm-wiki-karpathy-knowledge-base/) · [实现之一](https://github.com/Astro-Han/karpathy-llm-wiki)
- obsidian-mind：[仓库](https://github.com/breferrari/obsidian-mind)
- Evergreen notes：[Matuschak](https://notes.andymatuschak.org/z5E5QawiXCMbtNtupvxeoEX)
- PKM 对比：[PARA vs Zettelkasten](https://crystaljjlee.com/blog/two-approaches-to-pkm/)
- A-MEM：[arXiv 2502.12110](https://arxiv.org/abs/2502.12110)
- CoALA：[arXiv 2309.02427](https://arxiv.org/pdf/2309.02427)
- ACE：[arXiv 2510.04618](https://arxiv.org/html/2510.04618v1)
- Agent Skills：[Claude 文档](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/overview)
- SECI：[Wikipedia](https://en.wikipedia.org/wiki/SECI_model_of_knowledge_dimensions)
- 另见早先的对照：`hq-transcript-mining` 提案里对 claude-mem、episodic-memory、mem0 一系的判断
