# HQ 记住的东西

[English](knowledge.md) · **中文**

HQ 是 `gtmux hq` 起的那个参谋会话。它盯着你的 agent 干活，顺手把学到的东西记下来：
这个办公网会掐 TLS 握手、这个仓库里某条命令得带个没人记得住的参数、你说过两次给链接
要原始 URL。这些进了你 Mac 上的一个知识库，下次有 agent 在这台机器上开工，该知道的
那几条已经摆在它面前了。

这页讲的就是它：东西存在哪，一条经验怎么从「HQ 注意到了」走到「这台机器上每个 agent
都读得到」，以及你自己能改什么。除非你主动往外发，这里的东西不出这台机器。

## 存在哪

HQ 的东西都在 `~/.config/gtmux/hq/` 下面，另有一个自动生成的文件在它外面：

| 路径 | 是什么 | 谁写 |
|---|---|---|
| `hq/knowledge/.ledger.jsonl` | 知识库本体：所有条目、所有改动，只追加 | HQ，通过 `gtmux knowledge …` |
| `hq/knowledge/*.md` | 同一批条目按主题渲染出来，给人读的 | gtmux，每次渲染覆盖 |
| `hq/knowledge/promotions/` | 已晋升、等着被带走的条目的交接简报 | gtmux |
| `hq/knowledge/tools/` | HQ 给自己写的脚本，每个都有一条条目指着它 | HQ |
| `hq/AGENTS.md` | HQ 的章程：它怎么干活，随 gtmux 发布 | gtmux，更新时整份换掉 |
| `hq/LOCAL.md` | 你自己的长期规矩 | 你手写。gtmux 从不覆盖 |
| `hq/notes/board.md` | HQ 当前对舰队的判断，不算知识 | HQ |
| `knowledge/machine.md` | 这台机器上每个 agent 都该知道的那几条 | gtmux，自动生成 |

这下有两个都叫 `knowledge` 的目录，绊人的就是这里。`hq/knowledge/` 是仓库，几百条条目，
只有 HQ 读写。`~/.config/gtmux/knowledge/machine.md` 是出货口，只装「写给整台机器」
那一档的副本，删掉它下次同步会重新生成。

## 三层，哪一层是你的

规矩从三个地方到达一个 agent，三者不能互相顶替：

| | HQ 章程 `AGENTS.md` | 你的规矩 `LOCAL.md` | 本机知识库 `knowledge/` |
|---|---|---|---|
| 归谁 | gtmux 的 | 你的 | 这台机器的 |
| 谁写 | 产品写，随版本发 | 你写 | HQ 边干边写 |
| 什么时候生效 | 每次会话都在 | 每次会话都在 | 相关时才取，或 HQ 主动查 |
| 你能不能改 | 不能，它会被重新生成 | 能，这就是给你改的地方 | 走下面的命令，不要手改 |

`LOCAL.md` 在章程末尾被引入，所以你写在里面的东西会延伸并覆盖 gtmux 发的那份。想让
HQ 每次都照办的，写这儿；想让它记住自己摸索出来的，那是知识库。

## 一条经验怎么走完全程

```
出了点事  →  HQ 记成一条  →  （可选）晋升  →  落地
```

HQ 一学到能复用的东西就写一条：发生了什么、怎么认出它又来了、该怎么办。
任何 agent 都可以用 `gtmux capture "<一句教训> @<主题>"` 随手丢一条候选，由 HQ 判断哪些
成条目。gtmux 每天还会不用模型地读一遍各 agent 的会话日志，把你纠正别人的地方、以及
反复失败的工具调用排进同一个队列。

多数条目就待在原地。当一条经验不再只关于这台机器，而是关于你本人、关于
这里所有 agent、关于某个仓库、或者关于 gtmux 本身时，HQ 会晋升它，并说清给谁看：

| 读者 | 谁会读到 | 落在哪 |
|---|---|---|
| HQ | 只有 HQ 自己 | 追加进你的 `LOCAL.md` |
| 本机 | 这台 Mac 上每个 agent | `knowledge/machine.md`，外加各 agent 全局指令文件里一个短块 |
| 某个仓库 | 在那个仓库里干活的 agent | 那个仓库 `AGENTS.md` 里一个块，只写文件不提交 |
| 所有人 | 全体 gtmux 用户 | 一个填好的 GitHub issue，等你去开 |

晋升只是写好简报，落地才是 gtmux 真把它送到位。日常真正用得上的是「本机」这一档：
你的 Claude Code、Codex、opencode、Kimi Code 各自的全局指令文件里都会有一个块，列出这几条
并指向全文，新开的会话不用谁去粘贴就知道。`gtmux knowledge carriers` 能看到每个 agent 的
文件和是否最新，`gtmux doctor --fix` 补上过期的。

后来发现记错了的条目，退休时要写理由，理由会留在台账里：将来谁想弄明白这条错在哪，
只有那儿说得清。

## 怎么看，怎么改

手机和 iPad 上打开 HQ，点知识那一行。菜单栏里 HQ 卡片的 `KNOWLEDGE` 行打开的是同一个
列表。两边都把等着你的那几条排在最前面。

终端里：

```sh
gtmux knowledge list                 # 所有在库条目
gtmux knowledge list --topic pitfalls
gtmux knowledge show <id>            # 看某一条全文
gtmux knowledge lint                 # 给知识库做体检：只报该修什么，从不替你改
gtmux knowledge carriers             # 哪些 agent 装了本机块，是不是最新的
gtmux knowledge promotions           # 已晋升、还等着被带走的
```

改是 HQ 的活，那些动词是它的工具。其中三条你可能会想自己用：

```sh
gtmux knowledge retire <id> --why "办公网已经修好了"
gtmux knowledge sensitive <id> --confirmed "<你自己的原话>"   # 只留本机
gtmux knowledge sync                                          # 把本机块重新推一遍
```

日常改 HQ 知道什么，最省事的办法是直接在 HQ 的会话里跟它说，让它自己写条目。这样理由、
出处和两种语言的两半都还在，手改丢掉的正是这些。

## 两种语言

每条条目都写两遍，中英各一半，各按各自语言的读者写，不是逐字翻译。各个界面都给你你那
半份，只有另一半时会标出来。缺一半只是一个待补的计数，绝不因此就不记。

## 你自己的信息

你可以让 HQ 记你个人的东西：常用的账号、只有你有的路径、某个偏好。这类条目会被标成敏感，
gtmux 把这个标记当硬规矩：不渲染进 `machine.md`、不写进任何仓库的块、不进导出简报，
每个界面上都带一把锁。HQ 只有在你说了之后才标，而且会把你的原话记下来。

知识库里的东西不会被上传到任何地方。要换机器时 `gtmux hq --export` 把整个家目录打成一个
文件，`--import` 还原，`--records` 告诉你它长到多大了。

## 想再往下读

[docs/cli.zh.md](cli.zh.md) 里有全部命令和参数。背后的设计，包括三层为什么要分开、
每道闸各管什么，在 [docs/design/knowledge-layers.zh.md](design/knowledge-layers.zh.md)。
