# Troubleshooting & footguns (living checklist)

Pitfalls we've actually hit during **development, debugging, and release** — with
the check that would have caught each one early. This is a **living document**:
when a new footgun costs real time, add an entry here (symptom → root cause → the
must-check / rule), so the next person (or Claude) trips a checklist instead of the
rake. Keep entries short and action-first.

> Related runbooks live next to their subsystem: remote-access / pairing debug in
> `docs/design/remote-access-tunnel.md`; deploy paths in `CLAUDE.md` (Deploy table).

---

## 通用：`| tail` 会吞掉退出码，于是失败看起来像成功

**症状:** 一条命令明明失败了，你却以为它成功了 —— 因为你看到的是它的**尾巴几行**，而尾巴
往往是无害的收尾输出。更糟的是 `cmd | tail && echo 成功` 会**照样打印"成功"**。

**根因:** 管道的退出码是**最后一个命令**的退出码。`tail` 只要读到了输入就返回 0，前面那个
命令炸成什么样都与它无关。同理 `| head`、`| grep`、`| cut` 全都一样。

**这个坑在同一天咬了两次，场景完全不同：**

- `fastlane release | tail -40` —— 上传其实**成功了**，但结尾有一段无关的 Ruby 报错，我
  只看到那 40 行就判定"上传失败"，又去查 ASC（新构建要几分钟才会出现在列表里）"印证"了
  这个错判。真相是重传时 altool 报 *Redundant Binary Upload* 才露出来的。
- `git rebase main | tail -2 && echo "已 rebase"` —— rebase 因为工作区有冲突**根本没执行**，
  但"已 rebase"照样打印了出来，我据此往下走了一步。

**规则:**

- **在意成败时，永远不要把命令接进 `| tail`/`| head`。** 重定向到文件再 grep：
  `cmd > /tmp/x.log 2>&1; echo "exit=$?"; grep -E '…' /tmp/x.log`
- 非要用管道，就先 `set -o pipefail`，或者读 `${PIPESTATUS[0]}`。
- **`&& echo 成功` 不是验证。** 它只证明管道最后一环没崩。要验证就去查**动作真的发生了**
  的证据 —— 文件变了、进程在了、接口返回了 —— 而不是查命令"看起来"跑完了。

**同源的更大教训：判断对 ≠ 动作做了。** 探针本身也会骗人 —— 同一天我用"数子进程"判断
菜单栏 app 有没有在轮询，结论是"完全没有"；后来拿一个**明确每 1.5 秒真的会调一次**的对照
组去验这个探针，对照组同样显示"没有"。**下结论前先用一个已知结果的对照组验一下你的量具。**

---

## 长中文句子发送被截断:换行插入的空格打断了指纹匹配(2026-08-03)

**症状:** 手机上发一条**较长的中文**消息,app 弹 "Not sent — the input box didn't
confirm the full message",但 agent 又**收到了被截断+重复**的一段(开头出现两次、尾巴丢
失)。短消息、纯英文长消息都正常 —— 只有**无空格的长中文单行**触发。之前几次都复现不出来
(用 ASCII 测到 37K 字符都没事)。

**根因:** dispatch 的粘贴确认 `draftHasDelivery` 要求草稿里**同时**出现正文的 head(前 40
runes)和 tail(后 40 runes)指纹。一行**没有空格的中文**在 composer 里会**折行**,而
`normalizeSpace` 把折行的 `\n` 变成一个**空格** —— 于是草稿里是 "我们 正在",而 tail 指纹是
"我们正在"(无空格),tail 跨越折行点就永远匹配不上。确认失败 → 触发**清空重粘**的补偿逻辑 →
把好草稿清了、重粘叠加 → 截断+重复。英文因为有空格会**按词折行**,tail 落在下一行仍连续,所以
不中招;中文没有断点 → 单行折在正文中间。

**修复(内部 dispatch,`internal/dispatch/deliver.go`):** head+tail 普通匹配失败后,再做一次
**去掉所有空白**的 head+tail 匹配(`containsSpaceless`),折行插入的空格被抹掉、跨折行的指纹就
恢复了 —— 与图片路径早就用的 whitespace-free 技巧同源。40-rune 指纹去空白后原样重现不会误判。
回归测试:`TestPasteAndSubmit_WrappedCJKLine_ConfirmsNotChurns`。

**MUST-CHECK:** 任何"发送/粘贴确认"逻辑改动,都要用一条**长中文无空格单行**验证(不是只用英文)。

---

## 菜单栏切不到 Anywhere：GUI 进程的 PATH 没有 Homebrew 前缀

**症状** —— 菜单栏偏好设置里点「任意网络」，确认弹窗出现，点 Enable 后**弹窗直接消失、开关弹回、
屏幕上什么都没有**。同一条命令在终端里跑（`gtmux tunnel --service --yes`）**完全成功**。

**根因（两条，缺一条都还不够解释）**

1. **GUI 进程的 PATH 不是你的 PATH。** 从 Finder/LaunchServices 启动的 app 继承 launchd 的
   `PATH=/usr/bin:/bin:/usr/sbin:/sbin` —— **两个 Homebrew 前缀都不在上面**。`cloudflared` 在
   `/usr/local/bin`，于是 `exec.LookPath` 报「没装」，CLI 接着说「也没装 Homebrew 来帮你装」——
   两句都是假的，两个东西一直都在。`internal/tmux` 早就踩过这个坑并为 tmux 硬编码了兜底路径，
   但 cloudflared / brew 从来没享受到同一课。
2. **失败被吞掉。** `RemoteAccess.run()` 一直有 `lastError`，配对面板一直在显示它，**但偏好设置
   那一栏从来没渲染过**。所以失败的表现就是「弹窗消失，什么都没发生」。

**复现（不需要真的弄坏环境）**
```sh
env -i HOME="$HOME" PATH="/usr/bin:/bin:/usr/sbin:/sbin" gtmux tunnel --service --yes
# → cloudflared isn't installed … Homebrew isn't installed to fetch it
```

**必查**
- 调试任何「app 里不行、终端里行」的问题，**先用上面那行 `env -i` 复现**。这是这一类 bug 的
  分水岭，不先做这一步会往错误的方向查很久（网络？token？权限？）。
- 新增一个 gtmux 要 shell 出去调用的工具时，用 `toolpath.Look()`（`internal/toolpath`）而不是
  `exec.LookPath`。它原本住在 `internal/app`，而下一个需要它的工具（`gh`，在 `internal/dispatch`
  这个 leaf 里）**够不着** —— 于是同一课被重学了一遍，代价是 `gtmux reap` 把已合并的分支报成未合并
  （见下方 reap 条目）。**共享的教训必须放在每一层都能取到的地方。**
- 新增一个能失败的控件时，**在它自己所在的界面**渲染错误。别指望用户去别的面板找原因。


## `gtmux update` 在 Apple Silicon 上装了 x86 版（并且会自我延续）

**症状** —— M 系列 Mac 上，`gtmux update` 打印 `[1/5] Host darwin-amd64`，`~/.local/bin/gtmux`
落成纯 x86 二进制。装完之后**每次再更新还是 amd64**，而且 `file` 一看就是 x86_64。

**根因** —— `install.sh` 用 `uname -m` 判断架构，而 **`uname -m` 报的是「当前进程」的架构，
不是这台机器的**。在 Rosetta 下它在 Apple Silicon 上返回 `x86_64`。所以：

- 从一个被翻译的 shell 跑安装（`sysctl -n sysctl.proc_translated` = 1）→ 拿 amd64 包；
- **装完的 x86 gtmux 自己就是翻译着跑的**，它再调 `gtmux update` → 又看到 x86_64 → **闭环，永远
  出不来**。这就是为什么"装一次不对，以后次次不对"。

**判据** —— `sysctl -n sysctl.proc_translated` 返回 `1` 就说明「你在被翻译，硬件是 arm64」。
`install.sh` 现在据此纠正 `uname -m`。

**必查**
- 怀疑架构问题时，**别信 `uname -m`**，先 `sysctl -n sysctl.proc_translated`。
- `file -b ~/.local/bin/gtmux` 应该是 `arm64`（或 universal），不该是纯 `x86_64`。

## 安装布局：哪个 gtmux 是权威的

**权威 CLI = `~/.local/bin/gtmux`**，一个**真实二进制**（不是软链）。`install.sh` / `gtmux update`
就是往这里原子替换的（`mv -f`），所以**把它做成软链没有意义——下次更新会把软链直接覆盖成文件**。

- `~/Applications/Gtmux.app/Contents/MacOS/gtmux` —— app **自带的私有副本**，与 app 版本绑定。
  两个 LaunchAgent（serve / selftunnel）用**绝对路径**指向它，所以清理 PATH 上的副本不会动到服务。
  **不要**让任何东西软链到它：app 可以被替换或删掉。
- `/usr/local/bin/gtmux` —— **不该存在**。那是 Homebrew cask 的地盘（早期 0.9.3 cask 的遗留）。
- `~/.tmux.conf` 里的 `bind g/a/J` 硬编码 `~/.local/bin/gtmux` —— **正确**，因为那正是权威路径。


## restore：四类症状、一个共同成因（没有可执行的契约）

一天之内 restore 暴露四类症状（丢会话 / 同一个会话连丢两次 / pane 布局变了 / 终端窗口顺序），
每次都只修被人注意到的那一个。**这不是四个独立 bug，是「这个子系统没有可执行契约」的特征。**

**跑契约**（默认 SKIP，不会碰你的东西）：
```sh
GTMUX_RESTORE_E2E=1 go test ./internal/app/ -run TestRestore -timeout 12m
```

它在一个**私有 tmux server**（`TMUX_TMPDIR`）+ **私有 HOME**（resurrect 存档目录）里
save → kill-server → restore → 逐维断言。**不 mock tmux**——要抓的失败恰恰活在
gtmux × resurrect × 真 server 的交互里，mock 会把它删掉。

**它第一次跑就抓到的**
- **活跃窗口/pane 从来就没恢复过**，而且**四类症状里没人报过它**。resurrect 用
  `tmux switch-client` 放置活跃窗口，而**没有 client 附着时它什么都不做、也不报错**——
  gtmux 是无头驱动 restore 的。修法：自己用 `select-window`/`select-pane` 回放（见
  `restoreactive.go`）。
- **缺失会话找回被一个反了的条件挡住**：`shouldRecover` 要求**所有**存档会话都不在才找回。
  重启后总有某个终端标签自己带起一个 session → 条件不成立 → **其余会话永远回不来**，
  下一次 autosave 还会把「它们不存在」这件事记下去。

**它没抓到的，也要说清**
- **pane 布局在干净路径上不丢**。第一版测试报红过，**是我的比较写错了**：恢复出来的是新
  pane，layout 字符串里的 pane id 当然会变（`...,0,0,7` → `...,0,0,8`），几何完全一致。
  比较必须去掉**校验和**和**每个叶子末尾的 pane id**。差点把它当 bug 报出去。
- **终端窗口顺序无法在这里验证**（要真终端 + 辅助功能树）。列在契约里，标注「人验」。

**必查**
- 改任何 restore 相关代码，跑上面那条命令。
- 加一条 restore 行为时，**先往契约里加一维断言**，再写实现。
- 断言 tmux layout 字符串时，**永远先归一化 pane id**，否则你测的是 pane 编号不是布局。


## restore 往「本来没有 agent 的 pane」注入 `claude --resume`（2026-08-04）

**症状**：重启 + restore 之后，某些一直是普通 shell 的 pane 里凭空出现 `{ cd -- '…'; } &&
claude --resume '<uuid>'`，停在信任门。一次重启把舰队从 **10 个 agent pane 变成 16 个**；多出来的
会话 goal 都是几天前的旧事，其中一个自带 33.7M token，直接触发 `usage·warn burn`。**同一个 pane
会跨多次重启反复中**。

**根因**：resume 记录由 agent 的 hook 写入、**从不清理**，它只能证明「这个 locator 历史上跑过
agent」。restore 却把它当成「这里当时正在跑 agent」，于是任何跑过一次 agent 的 pane 就成了永久的
注入目标。而且它**自我延续**：注入出来的会话会重新写一遍那条记录（下一次 autosave 还会把它写进
`pane_full_command`），于是下次重启的「证据」更充分了 —— 任何基于「记录多旧」的启发式都拦不住，
因为记录的时间戳恰恰是被上一次注入刷新的。

**修法（v0.45.x，`internal/app/restoresave.go`）**：判据改成 tmux-resurrect 存档里那一行 pane 记录的
`pane_current_command` / `pane_full_command` —— 存档是重启前几分钟的快照，也是**唯一还活着的证人**
（进程早就没了）。存档说是 shell 的 pane 一律不碰；「无法判定」（存档没记全）才放行并记日志（宁可
多恢复一个，也不能丢一个真在跑的会话）。

**⚠️ 必查：tmux-resurrect 存档有两种字段布局（空 pane 标题会整体左移一格）**
resurrect 的 `save.sh` 用 `while IFS=$d read …` 回读自己 dump 的行，分隔符是 **TAB**，而 tab 属于 IFS
**空白**字符 —— bash 会把连续的空白分隔符**合并成一个**。于是**标题为空的 pane 那一行会少一个字段，
后面所有字段左移一格**：固定列上读到的「命令」其实是 **pane 的 pid**，而末尾的 full command 是
resurrect 拿错 pid 算出来的垃圾。事故里 6 个幽灵会话有 4 个就在这种行上（包括被报的那个）：按固定列
读，`日常更新:0.0` 的命令是 `77304`，不是 shell，于是照样放行 —— **不处理这个位移，这个 bug 修不掉**。
判别方法：格式里目录字段带 `:` 前缀，正常在 index 7、位移后在 index 6；位移行的 full command 必须丢弃。
（同一个位移也让 resurrect **恢复不出这些 pane 的目录**，它们会回到 `/` —— 那是上游行为。）

**不用重启就能验**：
```sh
mkdir -p /tmp/probe/tmux/resurrect && cd /tmp/probe/tmux/resurrect
cp ~/.local/share/tmux/resurrect/tmux_resurrect_<戳>.txt . && ln -sf tmux_resurrect_<戳>.txt last
XDG_DATA_HOME=/tmp/probe gtmux restore --plan   # 只读；列出的就是会被恢复的会话
# 对照存档里真正在跑 agent 的 pane：
awk -F'\t' '/^pane/{ if (substr($8,1,1)==":") print $2":"$3"."$6" "$10" "$11; else print $2":"$3"."$6" "$9" (shifted)" }' \
  ~/.local/share/tmux/resurrect/tmux_resurrect_<戳>.txt
```
端到端契约（真 tmux + 真 resurrect，私有 server）：
`GTMUX_RESTORE_E2E=1 go test ./internal/app/ -run TestRestoreResumesOnlyPanesThatWereRunningAnAgent`

**已经被注入出来的僵尸会话怎么办**：它们是真的在跑（在烧额度），gate 只防新的。手动 `tmux kill-pane`
或在 pane 里退出即可；`~/.local/share/gtmux/resume/` 里的历史记录不必手工清（gate 之后无害）。

## 重启后的三件事：存档是旧的、pane 编号串了船、布局坏了没人吭声（2026-08-18）

一次重启同时暴露三件，它们**是同一个毛病的三个出口**：gtmux 在信「别人告诉它的」（一段配置、
一条记录、一个名字），而不是「它自己看得到的」（文件的修改时间、当前活着的 pane、窗口的形状）。

### ① 存档几小时不动，而兜底存档一次都没触发

**症状** —— 恢复回来的是几十分钟前甚至几小时前的现场；关机前的改动全没了。
**根因** —— 自动存档那行命令挂在 tmux **状态栏右侧**，只有「有终端连着、状态栏在重画」时才跑。
Mac 一睡就不存 —— 配置再正确也不存。实测 3 天半 **76 次断档，最长近 6 小时**，而设置写的是 5 分钟。
gtmux 的兜底早就有，但它的开关问的是**「状态栏里写没写那行命令」**（`shouldBackstopSave(statusRight)`）
—— 而问题恰恰是**写着 ≠ 在跑**，所以兜底从头到尾一次都没触发。
**修法** —— 判据换成**存档文件多久没更新**（`internal/app/resurrectsave.go`）：10 分钟没动就自己存;
状态栏里有触发器时放宽到 20 分钟（让自动存档先动手），并在真正开存前**让一小会儿再复查一次**
——「醒机瞬间两边同时开存」是唯一还剩的碰撞窗口。旧那条防并发的教训没丢，只是换成用证据表达:
**在跑的存档器会把文件刷新，兜底就永远不会在它旁边醒来**。
**必查** —— `gtmux doctor` 的 `resurrect autosave` 行现在会报**真实的存档年龄**;看到
「已装,但 6h 没存过」就是这个毛病,不要因为它写着「已装」就当健康。

### ② `%N` 被重发,一批状态文件串到了别的船上

**症状** —— 重启后 gtmux 把某个会话的目标安在另一个会话头上;对没派过活的 pane 发「有草稿卡住」的警报。
**根因** —— **tmux 的 pane id 是 server 上的序号,不是身份**。server 一重启就从 `%1` 重新发,旧号
归了别的 pane。gtmux 有十几个目录按这个号命名,却只清过三个。重启后实测:`enrolled` 50/52、
`goal` 27/29、`sends` 31/32、`hqwake` 93/103 是死号,最老追到两周前 —— 而活着的 pane 正好顶着这些号。
**修法** —— `state.ReapDeadPaneState(live)`:恢复完成时(`afterRestore`)和 serve 慢节拍里(5 分钟一次),
按当前活着的 pane 列表清掉所有按 pane 命名的记录。**三重保险**:只删名字长得像 pane id 的文件、
只扫列出来的那几个家族、**live 集合为空时一个都不删**(读不到 `list-panes` 必须当「不知道」,
绝不能当「没有 pane」)。`resume/`(按 locator 命名,restore 正是在「所有 pane 都没了」那一刻读它)
和 `usage/`(按对话 id)**永远不碰**。
**顺带** —— 两周没送达的派工记录不再驱动截屏判断(`dispatch.Task.StaleUndelivered`);记录本身留着,
`gtmux tasks` 照旧说实话,只是它对一个被重发的号没有发言权了。

### ③ 布局坏了,两头都不说话

**症状** —— 某扇窗恢复回来排布不对,而 `restore.log` 里什么都没有。
**根因** —— gtmux 只数过「会话名字回来没有」,从没看过窗口长什么样;而 tmux-resurrect 那边
`restore_window_properties >/dev/null 2>&1`,`select-layout` 失败(典型是 `have 3 panes but need 2`
—— 恢复出来的窗口比存档多一个 pane)**错误被直接丢掉**,那扇窗就停在默认堆叠排布。两头都哑,
所以只能靠人几天后自己看出来。已经这样坏过两次(8/15、8/18)。
**修法** —— 恢复完把存档里每扇窗的**窗格数 + 排布**跟实际比一遍,不一致就写进 `restore.log`
并在终端提示(`internal/app/restorecheck.go`)。另外每次恢复都印出**存档时间和年龄**
(「恢复的是 09:57 存下的布局(37m前)」)—— 原来的陈旧告警门槛是 24 小时,而真正丢工作的那次
存档只有 37 分钟旧。

**⚠️ 比较 tmux layout 字符串前必须归一化**:去掉开头 4 位校验和 + 每个叶子末尾的 pane 号。
server 一重启 pane 号就全变,不归一化的话**每扇窗都会报"变了"**。生产用的
`normalizeLayout`(`restorecheck.go`)和端到端契约现在共用同一个实现,免得两边漂移。

**端到端验**(真 tmux + 真 resurrect,私有 server,不碰你的会话):
```sh
GTMUX_RESTORE_E2E=1 go test ./internal/app/ -run TestRestoreContract -timeout 12m -v
```
契约里新增两维:忠实恢复时对账必须**沉默**(会喊狼来了的检查比没有检查更糟),
以及给某扇窗多切一个 pane 后它必须**点名那扇窗**。

## release 里拿不到 tag message（`{{ .TagBody }}` 变成了 PR 描述）

**症状** —— tag 上明明写了 `user:` 段（`git tag -l --format='%(contents:body)' vX` 本地看得到），
但 GitHub Release 的正文里是**那次 squash merge 的 commit body（也就是 PR 描述）**，`user:` 段
根本没出现，于是 `gtmux update` 的「本次更新」什么都不印。

**根因** —— `actions/checkout` 把 tag 留成**轻量 ref**。`%(contents:body)`（GoReleaser 的
`{{ .TagBody }}` 就取这个）在轻量 tag 上会**回退到 commit message**，而 squash merge 的 commit
body 正是 PR 描述。整条链路不报错，只是悄悄换了内容。

**修法** —— checkout 之后补一句：
```yaml
- run: git fetch --force --tags
```

**必查**
- 发版后**确认 release 正文里有 `user:` 段**，别只看 workflow 绿了。
- 任何依赖 tag message 的 CI 逻辑，都要先 `git fetch --force --tags`。


## iOS 上架：`fastlane release` 在这台 M4 上归档失败的一长串坑（2026-07-24）

一次 `bundle exec fastlane release` 连撞五个坑才把 build 传上去。全部源于**工具链是 Intel x86
（Rosetta）**——同一台机器 `uname -m` 在 Rosetta 下返回 x86_64，ruby/cocoapods 都是 intel。

**根因链（按撞到的顺序）**
1. **`bundle` 找不到 bundler 4.0.8** —— `/usr/bin/bundle` 是系统 ruby 2.6。项目的 gem 在
   `vendor/bundle/ruby/4.0.0`，要 ruby 4.x。修：用 Homebrew ruby 的 bundle。
2. **gym 归档「秒失败、gym 日志只有一行」** —— gym 把 xcodebuild 管道给 **xcpretty**，而
   xcpretty 在新 ruby 下把 xcodebuild 的管道 SIGPIPE 掉，一行就死。**裸跑 xcodebuild 正常**。
   修：`build_app(xcodebuild_formatter: "")`（xcpretty 已废弃）。**这是让所有后续真错误现形的关键。**
3. **`Signing … requires a development team`** —— release lane 没往归档传 `DEVELOPMENT_TEAM`
   （可靠的真机 build 一直有传）。修：xcargs 加 `DEVELOPMENT_TEAM=<TEAM> CODE_SIGN_STYLE=Automatic`。
4. **`Build input file cannot be found … ReactCodegen/*-generated.mm`** —— `clean: true` 把
   `ios/build/generated`（RN 新架构 codegen 落点）清掉，codegen script phase 不保证在消费它的
   编译前重生成。修：`clean: false` + `pod install`（真机 build 也从不 clean）。
5. **`option '-authenticationKeyPath' may only be provided once`** —— 这个版本的 gym 把
   `xcargs` **同时**用于归档和导出，再加 `export_xcargs: auth` 就把 auth 传了两遍。修：删掉
   `export_xcargs`，auth 只放 `xcargs`（一份就同时到达两边）。

**彻底修法：换 arm ruby**
- `arch -arm64 /opt/homebrew/bin/brew install ruby`（arm 4.0.6）。
- `.zshrc`：`export PATH="/opt/homebrew/opt/ruby/bin:/opt/homebrew/lib/ruby/gems/4.0.0/bin:$PATH"`
  （ruby bin + gem-exec bin），删掉 RVM 与 `/usr/local/opt/ruby`。
- `gem install cocoapods`（arm）；`bundle install`（重编 native gem 为 arm）；`Gemfile.lock`
  的 `BUNDLED WITH` 跟 arm ruby 的 bundler 走（4.0.16）。
- 移除 RVM：`rvm implode` + `rm -rf ~/.rvm` + 清 rc 里的 rvm 行。

**必查**
- 调 iOS 归档报错时**先把 `xcodebuild_formatter` 关掉**再看日志——formatter 一崩，真错误全被吞。
- `bundle`/`ruby`/`pod` 必须都是 arm（`file $(which ruby) | grep arm64`）。
- 首次上架某版本时 `fastlane metadata` 会在传完文案后、传截图前撞 fastlane 的 `No data` bug，
  用 `fastlane metadata skip_metadata:true` 单独补截图（Fastfile 注释里已写）。


## 「我的 Mac 不睡了」：`disablesleep` 是个隐形开关（2026-07-31）

**症状** —— 合盖不休眠、电池莫名掉光、机器在包里发烫。翻遍 系统设置 › 电池 也看不出问题，
`pmset -g` 也一切正常。

**根因** —— `sudo pmset -a disablesleep 1` 设的是内核标志 `SleepDisabled`，它:

1. **跨重启保留**（写在 `/Library/Preferences/com.apple.PowerManagement.plist`）;
2. **在 `pmset` 的任何报告命令里都看不到** —— `-g` / `-g custom` / `-g live` 三个都不显示这一项，
   开着关着都不显示。所以「没有异常输出」根本不能证明它是关的。

于是任何设过它又没恢复的工具（或你自己手跑的一条命令），都会让这台 Mac **永久不睡且无人察觉**。

**必查（按可信度排序）**

```sh
# ① 内核活状态 —— 唯一权威、实时、不需要 root
ioreg -r -c IOPMrootDomain -d 1 -w0 | grep SleepDisabled     # = Yes 就是不会睡

# ② 落盘设置 —— 回答「重启后还算不算数」，但【滞后】于刚做的修改
plutil -extract SystemPowerSettings.SleepDisabled raw -o - \
  /Library/Preferences/com.apple.PowerManagement.plist

# ③ 恢复
sudo pmset -a disablesleep 0
```

**三个会咬人的细节**

- **别信 `pmset` 的退出码。** 非 root 跑它会打印 "must be run as root" 然后 **exit 0**。写完必须
  用上面 ① 读回确认，否则你会以为已经恢复了。
- **关闭态是 `false`，不是「key 不存在」。** 恢复过的机器 key 还在、值为 false；从没设过的机器才没有
  这个 key。用「key 在不在」判断会得出相反结论。
- **`pmset -c`（只写 AC 档）不被这个设置尊重** —— 值落在 plist 的 `SystemPowerSettings` 顶层，
  是全局的，电池供电时同样生效。所以别指望「拔掉电源内核会自动恢复睡眠」。

`gtmux doctor` 现在会报告这个状态（只在这台机器真的碰过它时才出现），`gtmux awake`
会同时给出活状态与落盘值。**gtmux 只回滚自己设的那一个** —— 没有 gtmux 归属戳的设置只报告、给出
上面的手动命令，绝不替你改。

---

## `make check` 本地绿、CI 的 `test` 红:Go 测试跑在 **Linux** 上（2026-07-31）

**症状** —— `make check` 在你的 Mac 上全过,PR 上的 `test` job 却挂了,报的是一个"这台机器应该
支持 X"之类的断言。

**根因** —— gtmux 是 macOS 产品,但 CI 的 `test` job 是 `runs-on: ubuntu-latest`
（只有 `menu-bar app build` 在 macos-latest）。任何**读真实系统**的测试 —— `ioreg`、`pmset`、
`sw_vers`、`/Library/...`、`runtime.GOOS` —— 在那里的行为和你的 Mac 完全不同,而本地永远测不出来。

**必查 / 写法**

- 读系统的测试要么**按 `runtime.GOOS` 分支**,要么用纯函数 + fixture(首选:解析逻辑独立成纯函数,
  喂真实输出的字符串)。
- 分支时**两个方向都断言**,而不是在非 macOS 上 `t.Skip` —— CI 的 Linux 环境是免费的"不支持
  平台"样本,正好验证你的兼容性门禁真的会**拒绝**,而不是瘸着继续跑。`servermode.Supported()`
  的测试就是这么写的。
- 提 PR 前想验证:`GOOS=linux go vet ./...` 能抓编译期问题;行为差异只能靠上面两条写法预防。

---

## 生成 shell 脚本时,内容必须走 base64 —— 又一次反引号事故(2026-07-31)

**症状** —— 菜单栏点「开启服务器模式」,输入密码后弹「authorization declined — nothing was
changed」。但用户**确实**输了密码、也点了 OK。

**根因(两个叠加)** —— 特权 payload 是「一段 shell 脚本,用来写出另一段 shell 脚本」。内层内容
当时是用 Go 的 `%q` 插进 shell 双引号里的:

1. **`%q` 是 Go 语法,不是 shell 语法。** 它把换行写成 `\n` 两个字符,而 shell 双引号**不解释**
   `\n` —— 写出去的守护脚本会变成带字面 `\n` 的一整行,彻底坏掉。
2. **被写的脚本注释里有反引号**(`` `gtmux awake on` ``)。shell 在双引号内会**执行**反引号里的
   内容。配合 `set -e`,那一行一失败整个安装就中止。

于是 osascript 返回非零,而调用方把**所有**非零都归类成"授权被拒绝",把排查引向完全错误的方向。

**规则**

- **把文件内容传进 shell,一律用 base64**:`echo <base64> | base64 -d > file`。base64 的字母表里
  没有任何 shell 特殊字符,这类 bug 结构上不可能再发生。永远不要用 `%q`/手工引号拼 shell。
- **区分"用户拒绝"和"脚本失败"**:osascript 的 `-128` / "User canceled" 才是拒绝;其余是执行
  失败,必须把真实报错显示出来。把两者压成一个错误会让人查错地方。
- **测 round trip,不测引号写法**:让 payload 真的跑一遍,再把落盘的文件和原文逐字节比对
  (`TestInstallPayloadReproducesTheGuardExactly`)。任何"看起来对"的引号都骗不过它。

这是 CLAUDE.md 里 `--body "$(…)"` 那条反引号 footgun 的同族 —— 换了个场景又踩一次。判据一样:
**只要有一段文本要穿过 shell,就问它里面有没有反引号/换行/引号;有就 base64。**

---

## Release / git-ops

### Never inline backtick-containing prose into a shell-substituted string
**Symptom:** `gh pr create` / `git commit` prints `foo: command not found`, the PR
body comes out mangled, and — worse — a random process (once a rogue `gtmux serve`)
is now running and squatting a port.
**Root cause:** backticks and `$(…)` inside a **double-quoted** string are command
substitution. Wrapping a heredoc as `--body "$(cat <<'EOF' … EOF)"` re-enables that
substitution around the heredoc, so any `` `word` `` in the markdown body (we fence
identifiers like `` `gtmux serve` `` constantly) gets **executed as a command**. A
`<<'EOF'` quoted delimiter protects the heredoc *body* but not the `"$(…)"` you wrap
it in.
**Rules:**
- Write PR/issue/commit bodies to a **file**, then `gh pr create --body-file <path>`
  / `git commit -F <path>`. Never `--body "$(…)"` or `-m "$(…)"` on text with backticks.
- After any PR-create that warned or errored, run
  `ps aux | grep -E 'gtmux serve|<cmds you backticked>'` and kill stray processes.

### GoReleaser deletes every apostrophe from the release notes (2026-08-09)
**Symptom:** `gtmux update` prints the release notes with apostrophes missing —
"gtmux HQs own pull", "a real non-tmux agents work" — while the tag message reads
correctly. Only `'` is affected: em dashes, quotes, `·` and `%` all survive.
**Root cause:** GoReleaser reads `{{ .TagBody }}` with
`git tag -l --format='%(contents:body)'`, passing those quotes as **literal characters**
(it execs git, no shell to strip them), then removes `'` from the output to undo them —
which takes the body's own apostrophes with it. The mangled text lands in the GitHub
release body, which is exactly what `gtmux update` / `gtmux whatsnew` read back.
**Fix (shipped):** `release.yml` REPAIRS the notes after GoReleaser runs — the true tag
body (shell quotes this time, so git gets a clean format string) + GoReleaser's generated
`## Changelog` section, written back with `gh release edit --notes-file`. Deliberately
`continue-on-error`: the header is already there and merely mangled, so a failed repair
degrades to the old behavior instead of failing a release whose assets already shipped.
**Must-check:** this is upstream behavior we work around, not something we control — if
the workflow step is ever removed or GoReleaser's header stops being `{{ .TagBody }}`,
apostrophes go missing again. To repair a release by hand:
`git tag -l --format='%(contents:body)' vX.Y.Z` → prepend to the `## Changelog` section →
`gh release edit vX.Y.Z --notes-file <file>`.

### A code change isn't shipped until the right delivery path runs
Four artifacts, **three** paths (git tag ≠ device build ≠ `wrangler deploy`). Editing
`relay-worker/` or `tunnel-worker/` and merging changes **nothing live** until you
redeploy the Worker. See the Deploy table in `CLAUDE.md` and
[[relay-redeploy-footgun]]. Quick check when push behaves oddly:
`cd relay-worker && npx wrangler deployments list` vs. `git log -1 -- relay-worker/`.

### Release tag gate
Tagging `vX.Y.Z` runs the **full `make check`** (not a weaker `go test`), then
goreleaser + the macOS app build. CI can't see the menu bar — smoke-test the app on
real macOS before trusting a tag.

### Menu-bar "click to update" loops — the app reinstalls its OWN version
**Symptom:** the popover shows `New version X — click to update`; clicking it "finishes"
(no error), the app relaunches, and the SAME banner reappears. The CLI + app both stay
on the old version. `~/…/T/gtmux-update.log` shows `Release v<OLD>` / `Installed gtmux
v<OLD>` even though Go logged `Updating <OLD> → <NEW>`. Running `gtmux update` **by hand
in a normal shell works** (installs `<NEW>`).
**Root cause:** `install.sh`'s `open -n "…/Gtmux.app"` used to launch the app with the
installer's env still set, **leaking `GTMUX_VERSION=<OLD>` into the long-lived app
process**. The in-menu update runs `gtmux update`, which inherits that pin; Go honors a
pre-set `GTMUX_VERSION` (`if !LookupEnv(...)`) instead of resolving the latest, so
install.sh reinstalls `<OLD>` — forever. A manual shell has no `GTMUX_VERSION`, so it
resolves `<NEW>` and works. (After a re-login the login LaunchAgent starts the app with
a clean env, which is why a reboot "fixes" it.)
**Fix:** `install.sh` now strips it (`env -u GTMUX_VERSION open -n …`) so the app never
inherits the pin, and `Updater.spawnDetachedUpdate` runs `env -u GTMUX_VERSION gtmux
update` as a belt. **Diagnose** with `ps eww <GtmuxBar-pid> | tr ' ' '\n' | grep GTMUX_`
— a `GTMUX_VERSION=` there is the smell. **Unstick a machine now:** `gtmux update` from
a plain terminal, or just click update twice (the first click relaunches with a clean
env via the fixed install.sh).

---

### `gtmux doctor --fix` / `gtmux update` hangs right after "menu-bar app launched"
**Symptom:** the app-install step finishes (`[5/5] Menu bar … ✓`, "menu-bar app launched",
the PATH hint all print), then the command NEVER returns to the prompt — no "Restarted
the remote serve" / "Done". The app IS installed and running; only the command is stuck.
**Root cause:** `runInstaller` ends with `restartServeAgents()`, which ran
`launchctl kickstart -k gui/<uid>/com.gtmux.serve` UNBOUNDED. On some Macs that
`kickstart -k` blocks indefinitely, freezing the synchronous `doctor --fix` / `update`
forever. install.sh itself already completed (its final line printed) — the hang is the
best-effort serve-restart, not the install.
**Fix:** every `launchctl` call in `restartServeAgents` is now hard-bounded by a 6s
timeout (`runBounded`); on timeout it skips the restart (the serve refreshes on next
login) instead of hanging. **Unstick a machine now:** press **Ctrl-C** — the app is
already installed; only the trailing restart stalled. (Needs a release to reach an old
`gtmux`.)

---

### `brew upgrade --cask gtmux-app` fails: "App source '/Applications/Gtmux.app' is not there"
**Symptom:** `brew install/upgrade --cask chenchaoyi/tap/gtmux-app` downloads + verifies
the zip, then errors `It seems the App source '/Applications/Gtmux.app' is not there.`
(often on a machine that previously ran `gtmux update`).
**Root cause:** the app has **two install channels that targeted different dirs** — the
Homebrew cask installs to `/Applications/Gtmux.app`, but `install.sh` / `gtmux update`
installed to `~/Applications/Gtmux.app`. If a user did both, `/Applications/Gtmux.app`
goes missing (only the `~/Applications` copy is current), and Homebrew's cask uninstall
step can't find the app it recorded at `/Applications` → the error. NOT a bad zip or
cask stanza (`ditto --keepParent` + `app "Gtmux.app"` are correct).
**Fix:** `install.sh` now **co-locates** — if `/Applications/Gtmux.app` exists (a cask
install) and `~/Applications/Gtmux.app` doesn't, it updates the `/Applications` bundle
in place instead of making a second copy, so the two channels stay on one app.
**Unstick a machine now:** `brew uninstall --cask gtmux-app --force` (forgets the broken
state) then `brew install --cask chenchaoyi/tap/gtmux-app` — or just switch to the curl
installer: `curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | bash`.
(The separate deprecation *warning* `depends_on macos: ">= :ventura"` is cosmetic; the
cask generator now emits `depends_on macos: :ventura`.)

---

## Remote access / pairing / push

### Menu-bar Off / Wi-Fi picker "won't change" from Anywhere — on the Direct backend
**Symptom:** on `Anywhere`, tapping `Off` or `Wi-Fi` in the menu-bar Remote-access picker
snaps straight back to `Anywhere`. Reproduces only when the tunnel backend is **Direct**
(self-hosted); on Standard/Cloudflare the picker works.
**Root cause:** the picker's mode is DERIVED from which LaunchAgents exist
(`groundTruth()`: `cfOn || selfOn ? .anywhere : …`). `serviceRemoveAll()` (Off) and
`serveServiceInstall()` (Wi-Fi) tore down `com.gtmux.serve` + `com.gtmux.tunnel`
(Cloudflare) but **skipped `com.gtmux.selftunnel`** (the Direct agent) — so on Direct it
stayed loaded, `selfOn` stayed true, and the mode re-derived to `.anywhere`.
**Fix:** both teardown paths now remove ALL three agents (serve + tunnel +
**selftunnel**), matching `tunnelServiceRemove` (`gtmux tunnel --unservice`). Pinned by
`TestServiceRemoveAllDropsSelfTunnel`.

### "Pairing code expired" that never clears — check for a DUPLICATE serve on :8765
**Symptom:** menubar "refresh code" → phone scans → *invalid or expired enroll code*,
no matter how fresh the code, across app reinstalls and `gtmux update`.
**Root cause:** two `gtmux serve` processes on 8765. The menubar mints via
`POST 127.0.0.1:8765` (IPv4 → serve A); the tunnel ingress `http://localhost:8765`
resolves to `::1` (IPv6 → serve B). **Enroll codes are in-memory per process**, so a
code minted on A is absent on B → "expired". (The same split corrupts push-token state.)
**Must-check (run this FIRST when pairing/push misbehaves):**
```
lsof -nP -iTCP:8765 -sTCP:LISTEN     # MUST show exactly one PID
ps aux | grep 'gtmux serve' | grep -v grep
```
Expect ONE serve — the app's `com.gtmux.serve` LaunchAgent
(`/…/Gtmux.app/Contents/MacOS/gtmux serve --bind 127.0.0.1 --port 8765`). Any second
`gtmux serve` (especially bare, binding `*:8765`) is a squatter → kill it. With only
`127.0.0.1` listening, cloudflared's `localhost` falls back to IPv4 and hits the same
serve the menubar mints on.

### Don't restart `gtmux serve` between mint and scan
Enroll codes (TTL 5 min) live only in memory; a serve restart (incl.
`launchctl kickstart`, and the `launchctl unload/load` that `gtmux tunnel --service`
does) wipes every pending code → a just-minted QR reads as "expired". Mint → scan
without bouncing serve in between.

### Tunnel silently offline on a corp network — QUIC is blocked
**Symptom:** phone gets Cloudflare **1033 / HTTP 530**; `tunnel.log` loops
`failed to dial to edge with quic: timeout` / `no free edge addresses left to resolve to`.
**Root cause:** cloudflared defaults to QUIC (UDP/7844); many corp/campus nets block it.
**Fix:** `--protocol http2` (TCP/443) — now the gtmux default for all cloudflared
launch paths (override with `GTMUX_TUNNEL_PROTOCOL`). An **old** service plist keeps
QUIC, so after `gtmux update` re-run `gtmux tunnel --service` to regenerate it.
Diagnose with `tail ~/.local/share/gtmux/tunnel.log`. See
`docs/design/remote-access-tunnel.md`.

### Corp-DNS hijack ≠ dead tunnel
The office net rewrites brand-new `ccy.dev` answers to internal `172.19.x` IPs, so the
Mac's own reachability probe fails on a *healthy* tunnel (returns HTTP 530). Verify the
last hop from a **phone on cellular**, not from the office LAN. `api.cloudflare.com` is
also intermittently TLS-reset here — retry `wrangler`.

### The app classifies enroll failures — read the phone's message
Since the enroll-error split, the phone names the failure class: *can't reach* /
*tunnel offline* / *code expired* / *no token*. Use that to jump straight to the right
section above instead of guessing.

---

## HQ attention system / perception feed

### `gtmux hq` said it focused the supervisor but the HQ session is dead
**Symptom:** you quit the HQ agent but left its tmux window open (a bare shell). Later
`gtmux hq` said it had focused the running supervisor and jumped to that window — which
held only a shell prompt, no agent. Confusing.
**Root cause:** `findHQPane()` detects HQ by a pane STAMP that survives the agent
exiting, so `gtmux hq` treated a stamped-but-dead pane as "running" and focused it.
**Fix:** `gtmux hq` now checks the pane's foreground command (`hqAgentAlive` →
`pane_current_command`): a shell means the agent exited, so it RELAUNCHES the agent in
that same pane instead of focusing a dead prompt (`agentAliveByCmd`, pinned by
`TestAgentAliveByCmd`).

### A dispatched worker shows `done` in `gtmux tasks` but never ran
**Symptom:** you `gtmux spawn` a task; `gtmux tasks` (and HQ/the digest) show it `done`,
but the worker's tmux pane is actually sitting at the "Do you trust the files in this
folder?" startup gate (or holds the goal UNSUBMITTED in the composer — a long paste
swallowed the Enter). Not one step ran.
**Root cause:** `waiting` (needs-you) was HOOK-marker-driven ONLY. The startup gate and
an unsubmitted composer fire NO gtmux hook, so the radar read the pane `idle`, and
`taskStatusFor("idle")` mapped idle → `done` unconditionally — no `waiting` wake either.
**Fix (v0.28.9, stuck-dispatch-waiting):** a narrow screen-content guard — for a TRACKED
dispatch whose capture shows a startup/permission gate (`prompt.IsStartupGate`, per-agent)
or a structured non-empty draft (`dispatch.DraftOf`) — reclassifies it `waiting` (kind
`startup`/`draft`), never `done`. The serve slow-tick writes the marker + fires a
`waiting` wake so HQ unblocks it; `wakeDone` also skips `done` when the post-Stop screen
is a gate/draft. All other waiting stays hook-driven. **Unstick now:** answer the gate /
press Enter in the pane.

### `gtmux spawn` starts a session but the goal never arrives — and `tasks` says `done`
**Symptom:** `gtmux spawn --goal-file …` prints what reads like a normal startup log,
the session is up, and the composer is **empty** — the goal was never entered. `gtmux
tasks` shows the entry green `✳ done` with the full goal text beside it. `digest`'s
goal/last for that pane are blank. `gtmux send --message-file <the same file>` into that
pane lands first try. Hit on 2026-08-01, 08-03, 08-06, and twice on 08-09.
**Root cause (two independent defects, and you need both fixes):**
1. The readiness gate listed `MCP servers need authentication` as a **boot banner**. On a
   Mac that permanently carries `⚠ 10 MCP servers need authentication · run /mcp`, that
   line is not startup noise — it never clears — so `hasBootBanner` matched forever, the
   gate could never settle, and every spawn timed out into `NOT delivered`. (#652
   narrowed the MATCHING but left the phrase in, fixing only the "transcript mentions it"
   variant — which is why "✅ fixed in v0.44.10" was wrong and cost two more incidents.)
2. `gtmux tasks` derived status from the pane ALONE (`taskStatusFor`: idle → `done`). A
   ready-timeout leaves a live, empty, **idle** agent pane, so the failed dispatch
   rendered as finished work. The ledger recorded `delivered:false` the whole time —
   nothing but the resume lookup ever read it.
**Fix (spawn-readiness-persistent-banner):** a **standing notice** — a bottom line naming
an action only the user can take — is no longer a boot banner; only chrome that RESOLVES
BY WAITING (`Connecting…`/`Loading…`) holds the gate, with the two-frame settle covering
the transient window. The ledger stores the dispatch `state`, and `tasks`/`digest` read
it: a never-landed dispatch is `✗ undelivered`, sorted first, never `done`. A `gtmux
send` that lands in that pane back-fills the entry.
**Must-check when a spawn looks wrong:** read the FIRST line of the failure, not the
screen dump under it — it now says `blocked by: <the line that said no>`. spawn DOES
report this on **stderr with exit 1**; the reason it reads as "no error" is that the old
evidence appended the full capture (measured: 224 lines / 11.8 KB), so the verdict was
the head of a wall that looks like a boot log. **Rule:** a spawn whose `tasks` row is
`undelivered` was never dispatched — re-send with `gtmux send %N --message-file <path>`,
or fix the blocker the evidence names (`/mcp` to authenticate, answer the trust gate, …).

### `gtmux reap` kills the session and worktree but leaves the branch behind
**Symptom:** you squash-merge a PR, run `gtmux reap`, and it reclaims the session and the
worktree but reports the branch as **not merged** and keeps it. Meanwhile `gh pr view
<branch>` from your terminal says `MERGED`. Hit 2026-08-09 on `fix/app-hq-history`.
**Root cause — TWO faults that compound, which is why it looked like "squash detection is
broken":**
1. `defaultBranch` stripped the `origin/` prefix and returned the LOCAL branch name, so
   both git-side probes judged against a local `main` that nothing had pulled — `gh pr
   merge` doesn't fetch, and under a worktree layout the local `main` belongs to another
   checkout that may be pinned for other work. Measured on the branch that hit this: the
   squash-equivalence scan had an **empty commit range** against local `main` and said no;
   against `origin/main` the branch tip's tree matched the squash commit exactly.
2. `prMerged` folded every `gh` failure into `false`. `gh` lives in `/opt/homebrew/bin`;
   a gtmux started by launchd inherits `/usr/bin:/bin:/usr/sbin:/sbin`. So the last
   remaining probe was invisible, and its silence became a confident "not merged".
**Fix (reap-merged-detection):** the base is the remote-tracking ref, `gtmux reap`
refreshes it before judging (bounded + best-effort; the hook-driven reap-suggest sweep
still does no network), `gh` resolves through the shared `internal/toolpath` search, and
the result is three-way — "could not establish" fails the gate CLOSED but says so and
names the remedy instead of asserting unmerged commits. Verified end-to-end on the two
real leftover branches: both judge merged, **and still do with `gh` hidden** — the single
point of failure is gone.
**Must-check:** `gh` being on YOUR PATH proves nothing about the PATH the failing process
had. Before concluding a shelled-out tool is "not installed", check it through
`internal/toolpath` (or `env -i PATH=/usr/bin:/bin sh -c 'command -v <tool>'`), and never
let a tool's *unavailability* stand in as its *answer*.

### HQ's startup briefing typed into the input box but never sent
**Symptom:** `gtmux hq` starts the agent, a long "Startup briefing — make this your very
first output…" prompt sits in the input box UNSENT, and HQ stalls waiting.
**Root cause:** the briefing used to be a huge multi-line prompt PASTED into the pane and
submitted — fragile (a long paste + a single Enter can land as typed-but-not-submitted,
especially on a just-started agent) and Claude-Code-specific.
**Fix (v0.28.8, playbook v6):** the briefing CONTENT + format now live in the seeded
playbook (`AGENTS.md` "## First turn"), read by any agent via its own convention file;
gtmux injects only a MINIMAL one-line trigger — `» gtmux·startup` — which submits
reliably and is agent-agnostic. (Unstick a stalled one: just press Enter in that pane.)

### An `event-sequence gap` warning on a pull — events rotated away unread
**Symptom:** `gtmux events --since-seq <n>` prints a CRITICAL warning about a sequence
gap.
**Root cause:** the journal retains a bounded window (8 MB × 2 generations); events
between HQ's watermark and the retained tail were rotated away before being read —
usually after HQ was down or silent for a long stretch.
**Must-check / fix:** rebuild from `gtmux digest --json` FIRST, then
`gtmux events --ack <latest>` — acking over the gap without the snapshot forgives the
loss silently. If gaps recur, check that `gtmux serve` is running (without it the wake
and unread knocks that keep HQ consuming never fire).

### HQ went quiet — is it the feed or the surfacing threshold?
**Symptom:** HQ stopped printing routine updates.
**Root cause:** by design. The feed is SILENT (gtmux no longer types low-value receipt
nudges into the pane); HQ only PRINTS CRITICAL/NORMAL and ledger-records QUIET. Quiet
mode raises the bar to CRITICAL-only.
**Must-check:** `gtmux quiet status` (the resolved threshold). QUIET items are in
`gtmux tasks --verbose`, not lost. A read-time gap CRITICAL is never quieted, so
silence there means perception is healthy, not broken.

### The playbook upgrades itself — never hand-edit AGENTS.md
The supervisor's charter (`~/.config/gtmux/hq/AGENTS.md`) is a MANAGED file: it carries a
version+language marker, and `gtmux hq` regenerates it automatically whenever the shipped
version is newer OR `GTMUX_LANG` names a different language than the installed edition —
the prior file is backed up beside it (`AGENTS.md.bak-v<N>-<lang>`) first. There is
nothing to re-seed by hand, and edits to AGENTS.md are displaced to a backup on the next
upgrade; your customizations belong in `LOCAL.md`, which no upgrade ever touches.

---

## Reclaiming a dispatch (`gtmux reap`)

### `reap` 删了 session + worktree，分支还在，而且**一个字都没说**（2026-08-09）
**Symptom:** `gtmux reap <id>` 输出 `✓ reaped:` 两行（killed session / removed worktree），
**没有 `deleted branch` 那一行，也没有任何解释**，退出码 0。分支原地不动。发生在 v0.48.1 上，
也就是 PR #746（"a merged branch stops being reported as unmerged"）已经在生效路径上之后。
**Root cause —— 三个缺陷叠在一起，都在 gate 的下游：**
1. `planAndReap` 先 `removeWorktree`，再调 `deleteBranch(t.Worktree, …)`；`DeleteBranch`
   要用 `mainRepo(wt)` 找主仓库，而它是 **从 wt 目录里问 git** 的 —— 目录刚被删掉，于是
   `git -C <已删目录> branch -d` → `fatal: cannot change to '<path>'` → exit 128。
   （`spawn` 的 `rollbackWorktree` 早就写对了：`MainRepo` 就是**为这个顺序**导出的，注释里
   写着 "resolve BEFORE removal"。reap 从来没采用。）
2. 就算路径对了，`git branch -d` 会跑**它自己的**合并判据，而那个判据只认「是 HEAD/upstream
   的祖先」—— 它结构上看不见 **squash 合并**，也就是本仓库（和 GitHub 默认）每天产出的形态。
   于是 gate 判 merged，git 仍然拒绝。
3. 三步执行全是 `if op(…) == nil { 记一条 action }` —— **只记成功，对失败绝对沉默**。所以
   失败的唯一痕迹是「少了一行」，还照样 `✓` + exit 0。
**为什么 #746 没发现**（这条比 bug 本身更值钱）：#746 修的是**判断**，验证的也是**判断函数**
（`BranchMerged`，拿真分支跑，绿）。而 `internal/app/reap_test.go` 里所有 reap 测试都通过注入
的 ops 驱动，`deleteBranch` 被 stub 成 `func(...) error { return nil }` —— **构造上不可能失败**，
断言只能到「这一步被调用了」。两个缺陷正好落在**注入缝以下、`git_test.go` 被测函数以上**的夹层里，
两边的绿灯都照不到。
**Rule:** 一个命令的修复，验证必须落在**命令级**（真仓库 + 真 git），不能只到函数级。判断对了
不等于动作做了。凡是「gate 通过 → 执行副作用」的结构，测试要断言**世界变了**（分支真没了），
不是「stub 被调用了」。见 `internal/app/reap_live_test.go`。
**Fix (PR #748):** ① 在删 worktree **之前**解析 repo（新增 `reapOps.mainRepo`）；② gate 确认
合并后 `deleteBranch` 用 `-D`（gate 严格强于 `-d`；没跑 gate 的路径仍用 `-d`）；③ 新增
`reapResult.Failed` + `⚠ but these steps failed` 区块 + 有失败则 exit 非零，且 `gitRunLoud`
把 git 的 stderr 折进 error（`exit status 1` 什么也没告诉用户）。

## Driving a pane (dispatch / `gtmux send`)

### An "is there a draft?" check MUST read the COLOR capture (2026-08-09)
**Symptom:** `gtmux send` refuses with "that pane has unsent text in its input box" against
a pane whose box is visibly EMPTY. Shipped in v0.46.2, fixed in v0.46.3.
**Root cause:** Claude Code renders its suggested-next-command as FAINT (SGR 2) ghost text
inside the input box. `tmux capture-pane` WITHOUT `-e` strips the SGR markers, so the ghost
comes back as ordinary text and every "is the box empty?" caller reads it as a half-typed
draft. Measured live on pane `%7`: the plain read returned `把评论里 273 改成 265`, the color
read correctly returned nothing.
**Rule:** a caller asking *"is there an unsubmitted draft?"* uses `dispatch.DraftOfColored`
on a `tmux.CaptureFullColor` capture — never plain `SplitInputRegion`. This is written at
`internal/dispatch/region.go`'s `DraftOfColored` doc comment, which names the exact failure
("a stuck `waiting`, a suppressed `done`, a held HQ nudge") — the draft guard became its
fourth instance. Callers matching a SPECIFIC payload (Deliver's verify, the wake-ack `#id`)
are exempt: a ghost can never equal their target.
**Also:** refuse only on TWO agreeing frames. One frame mid-repaint can show a phantom, and
a refusal — unlike the wake channel's queue-and-retry — cannot be taken back.
**Must-check when adding any such gate:** run it against a LIVE Claude pane that is showing
its ghost suggestion, not just against test fixtures. Fixtures carry no SGR, so
`DraftOfColored` and `SplitInputRegion` agree on them and the bug is invisible in CI.
**Second false positive, same gate (fixed in the same round):** a pane running neither a
known agent nor a bare shell (vim, ssh, a TUI — routing fails SAFE to the agent pipeline for
those) has NO composer, so `SplitInputRegion`'s no-box degrade path returns the TRANSCRIPT as
a draft — measured live at 347 characters of log output on a pane running `diting`. Any
"is there a draft?" gate must therefore ALSO be scoped to panes a known agent drives
(`Opts.HasComposer`, from `radar.AgentDriverKey`), not merely to "not a shell".
**And the shape of the gate matters as much as its inputs:** it must fail OPEN on every
condition it cannot judge (unreadable capture, copy-mode, no input region) and be bounded —
a pre-check on the send path may cost a known amount, it may never loop or hang. The live
probe to reproduce all of this: build `DeliverOpts` for a pane, call the guard read-only,
and print `driverKey / hasComposer / refused` across an agent pane, a TUI pane and a shell.

### A screen check matched what the pane was QUOTING, not what it was drawing (2026-08-10)
**Symptom:** HQ is knocked `» ◆ gtmux·waiting … stuck before running — startup` for a pane
that is working normally, over and over. Measured: 36 deliveries for one pane (`%74`) across
13.6 hours, all false.
**Root cause:** `prompt.IsStartupGate` matched its signatures with `strings.Contains` over
the WHOLE capture — and a capture is `capture-pane -S -200`, i.e. 200 lines of scrollback.
That pane's own Edit diff had rendered this repo's gate table, `"": {"Do you trust the
files"}`, onto its screen at 16:51:23; the first false knock landed at 16:56:25 and they ran
until 06:34. gtmux was reading the pane's CONTENT as the pane's CHROME.
**This is a repeat.** #652 fixed exactly this for boot banners ("a pane whose scrollback
merely MENTIONED those words") by anchoring them to the bottom region — and left the sibling
function unanchored. When you narrow one screen predicate, check every predicate reading the
same capture.
**It costs twice, and the second cost is silent.** The same `IsStartupGate` call suppresses
the `done` wake (`internal/hook/nudge.go`), so a pane that merely quoted the phrase would
also have had a real completion withheld from HQ — a false positive here manufactures noise
in one direction and swallows signal in the other.
**Rule:** a predicate that asks *"is the agent DRAWING this right now?"* reads only the
bottom region (`bottomLines`) of a faint-stripped capture. Whole-capture `Contains` answers
a different question — *"does this text appear anywhere in 200 lines of history?"* — and
that question is never the one being asked.
**Second fix in the same call:** a "stuck BEFORE running" claim is about the DISPATCH, not
the screen, so the ledger decides it — `Task.Undelivered()`. A dispatch whose goal landed was
accepted by the agent and is past both a launch gate and an unsent goal; it is not
screen-classified at all. Screen evidence is the fallback for a question the records can't
answer, never the first source when they can.
**Third, found while fixing:** the per-agent gate map is keyed by registry KEY (`codex`) but
the radar and the HQ slow tick pass the display LABEL (`Codex`), so codex's gates — added the
day before precisely to see a stuck codex worker — resolved to nothing on exactly the paths
that watch a live fleet. A per-agent table lookup must accept whichever identity its callers
actually carry; `agents.KeyForLabel` is the bridge.
**Live repro (a real frame, not a fixture):** print the quoted phrase into a scratch tmux
pane, then push it up past the bottom region with ~30 more lines. `grep` still finds it in
`capture-pane -S -200` (what the old code saw) while the predicate now answers false.

### One instruction pasted 2–3× and submitted in pieces
**Symptom:** a dispatched message appears in the agent's box twice or three times, is
submitted line by line (the tail lines land as "queued messages"), the Enter looks
swallowed and needs a manual re-press — and `gtmux send` still reports `NOT delivered`.
**Root cause:** two, and they compound.
1. `paste-buffer` was **not bracketed** (`-p`), so the payload went in raw and every
   `\n` reached the TUI as a bare Return — submitting each line as its own message.
2. The fragment retry called `ClearDraft` (C-u) and re-pasted **without checking the
   clear worked**. C-u kills only the line the cursor is on; against a multi-line draft
   a second C-u (and Escape) do nothing at all. So the retry pasted onto the leftover
   and concatenated a copy — `PasteRetries: 2` → up to three copies.
**Rules:**
- Any tmux paste into an agent TUI is `paste-buffer -p`. Test with a **multi-line**
  payload — single-line text hides both bugs completely.
- Never re-paste into a box you have not SEEN go empty. Clearing a draft is not
  reliable; failing loudly with evidence beats duplicating an instruction.
- The frame right after a paste is not evidence — the TUI redraws on its own schedule.
  Let a paste settle before judging it a fragment (a stale frame read as a fragment is
  what triggered the destructive retry).
**Must-check:** reproduce against a real agent pane, not a fake — `tmux new-session -d
-s lab; tmux send-keys -t lab claude Enter`, then send a 3-line instruction and read
the box. A unit test with single-line fixtures passes either way.

### 派活时反引号被 shell 执行,spawn 直接挂掉(2026-08-01)

**症状** —— HQ 执行 `gtmux spawn … "<一大段中文 goal,里面有反引号包着的代码标识符>"`,
shell 报 `command substitution: syntax error near unexpected token 'done'`,spawn 根本没跑起来。
**副作用更麻烦**:worktree 和分支已经建了、session 没起;重试时 `git worktree add` 报
`exit status 128`;两次尝试留下两个空 session(goal 一个都没投递)。

**根因** —— goal 走的是 **argv**,那就必然先过调用方的 shell:双引号内反引号会被**执行**、
`$x` 会被展开、换行会截断命令。够长的自然语言指令迟早会含这些字符,所以「每次小心引号」
不是一个系统能持有的性质 —— 事实佐证:这条坑在 HQ 知识库里已经记过**两次**,当天上午还刚被
推广成通则,几小时后照样踩。这是接口问题,不是记性问题。

**修复(本次)**

- `gtmux spawn --goal-file <path|->` / `gtmux send --message-file <path|->`:调用方写文件,
  gtmux 读字节,路径上没有 shell。只做一个明说的归一化:最多去掉一个结尾换行(heredoc 都会带)。
- `gtmux spawn --oneshot` 的 goal 也改走暂存文件(原先 shell-quote + 折叠空白,多行会被压成一行)。
- **失败可重入**:worktree 已存在则复用(不再 128)、上次没投递成功的 session 会被接管、
  这次建了没用上的 worktree/分支会回滚。**重跑同一条命令即可收敛**,不要手工清理。

**规则(与 base64 那条同源)** —— 只要一段文本要穿过 shell,就问它里面有没有反引号/换行/引号/`$`;
有就别走 argv。给工具一个**文件通道**,比给调用方一条纪律更可靠。测试要断言 **round trip 逐字节
一致**,不要只断言「命令没报错」——引号写法「看起来对」是骗得过人的。

---

## Disk / storage

### gtmux state dir balloons to GB (disk red line)
**Symptom:** `~/.local/share/gtmux` grows to hundreds of MB or GB; a disk-space alarm
fires. `gtmux doctor`'s `Storage` row shows red (`✗ very large`).
**Root cause:** it is almost never the event log — `events.jsonl` (20 MB) and the HQ
spool (8 MB) already self-rotate. The culprit is an **unrotated launchd log**:
`serve.log` / `tunnel.log` / `selftunnel.log` / `restore.log` are plain
`StandardOutPath`/`StandardErrorPath` redirects launchd never rotates, and the gtmux
process can't `SetOutput` a redirect it doesn't own. A chatty daemon — classically
`cloudflared` retrying forever against a **QUIC-blocked** corp network — writes with no
ceiling. Secondary: the `uploads/` dir (phone images) and the per-pane churn markers
(`frame/`, `cpu/`, `goalchanged/`, `sends/`) that never cleaned up a dead pane's leftover.
**Fix / must-check:**
- `du -ah ~/.local/share/gtmux | sort -rh | head` — find the big file. A multi-hundred-MB
  `tunnel.log` confirms cloudflared churn (check the tunnel is actually up; see the
  QUIC-blocked entry).
- The slow-tick hygiene sweep (`internal/hq/diskhygiene.go` `diskHygieneSweep`) caps each
  log to its recent tail (8 MB → last 2 MB), age-prunes + LRU-trims `uploads/`, and ages
  out dead-pane churn markers, every 30 min while `gtmux serve` runs. If serve isn't
  running, nothing trims — start it, or manually `: > ~/.local/share/gtmux/tunnel.log`.
- `events.seq` is a single monotonic integer — never delete it to reclaim space; a reset
  would break every consumer's durable cursor.

---

## Radar / process sampling

### Ghost native rows: "two codex sessions running" that nobody started (agent-internal helper calls)
**Symptom:** `gtmux digest --json` / `agents --json` grow `source:"native"` rows —
seen twice as a working+idle Codex pair, ~1 min apart — for sessions nobody started.
The `working` one never resolves; "kill it" has no target. Fingerprint of a fake: no
pane/session/window, no PID, zero tokens in `gtmux usage`, only 5 digest fields
(agent/source/status/since/sense — no goal/last/cwd), record `cwd:"/"`, and the
session id is absent from the agent's own session index (`~/.codex/session_index.jsonl`).
**Root cause:** an agent's INTERNAL helper calls — Codex's ambient-suggestions
generator (`~/.codex/ambient-suggestions`, prompt head `# Overview Generate 0 to 3
hyperpersonal…`) and its auto-mode safety classifier (`You are an expert at upholding
safety…`) — run as full agent sessions: they read `~/.codex/hooks.json` and fire
pane-less SessionStart/UserPromptSubmit/Stop through `gtmux hook --agent codex`, which
sensed them as native sessions. The "Codex" attribution is thus FACT, not a fallback
bug — the helpers really are codex processes. They surfaced only after #703 made codex
hooks fire at all. `working` sticks because the helper's Stop carries no session id to
pair back; no PID because the codex hook path re-execs detached (`setsid`), severing
the ancestry walk.
**Fix (fix/codex-ghost-rows):** two layers, both on positive evidence — never "pane is
empty" alone (real native sessions are pane-less too). ① `internal/hook/helper.go`: a
pane-less UserPromptSubmit whose prompt head matches the known-helper list erases the
session (record dropped, id marked, later events swallowed; the streamed SessionStart
gets its pairing SessionEnd so the unread-debt blink exclusion covers it). ② radar
`nativePanes`: a record with no live PID AND no on-disk conversation is withheld from
every surface.
**Must-check when it recurs:** pull the pane-less UserPromptSubmit's summary from
`gtmux events` — a new helper prompt head means the list in
`internal/hook/helper.go` needs that fingerprint (copy it verbatim from the event
summary; same normalization).
**Symptom:** right after changing networks (office ↔ home, VPN up/down) the menu bar
stops updating and the phone shows the server connected but **0 agents**. `ps aux` in a
shell ALSO hangs. `pgrep -f "gtmux agents"` shows dozens/hundreds piled up.
**Root cause:** a background system process — classically a **corporate VPN/EDR agent**
(seen: `medrAgent`) — wedges in **uninterruptible kernel sleep (`U`/"stuck")** during the
network transition. The radar samples processes with a full-table `ps -axo …command=`,
which reads every process's argv (`KERN_PROCARGS2`); reading the wedged process's args
blocks **forever**, and a `ps` stuck in `U` can't be killed (SIGKILL/SIGALRM stay pending)
nor reaped. The menu bar shells out to `gtmux agents` every poll, so each poll spawns
another undying `ps` — observed **137+ stuck `ps`**, a frozen bar, and the serve (which
samples through the same call) returning an empty agent list to the phone.
**Diagnose (do NOT use full `ps` — it wedges too; each call adds another stuck `ps`):**
- `top -l 1 -stats pid,state,command -n 800 | awk '$2=="stuck"'` — `top` uses libproc and
  does NOT block. The **only non-`ps` "stuck" row is the culprit** (`ps` reading its argv
  is why everything else hangs). A **targeted** `ps -p <pid>` still works — it's only the
  full-table `-axo …command=` walk that blocks.
- The wedged process usually self-heals when its stuck kernel op (network) completes; it
  may flap stuck↔running as the network settles. You can't `kill -9` it while it's `U`.
**Fix (shipped v0.43.11, `internal/radar/agents.go` `boundedOutput`):** the full-table
`ps` runs with a hard timeout that **truly abandons** a hung child — read+`Wait` in a
goroutine, `select` against a timer, on timeout best-effort `Kill` and **return** a
degraded (empty) snapshot. A degraded snapshot loses only the CPU "working" signal +
bare-`node` (idle Codex) detection; **title-identified agents still show**, so the radar
keeps working through a wedge. ⚠️ **`exec.CommandContext` + `WaitDelay` is NOT sufficient**
— `WaitDelay` closes the I/O pipes but `Cmd.Wait` still blocks in `wait4()` on the
unreapable child (v0.43.10 shipped this and still hung). You must abandon the `Wait`.
**Recover a live wedge on an OLD build:** `gtmux update` restarts the serve+menu bar with
the fixed binary — `gtmux agents` then returns in ~4s even while the process is still
wedged. The leftover stuck `ps` drain when the wedged agent finally unblocks.

---

## Clicking a waiting session in the menu bar does nothing (v0.50.0–v0.51.0, `tab-alert` on)

**Symptom.** The menu-bar row for a session that needs you does not jump to its terminal
tab. Every OTHER row jumps fine. `gtmux focus <pane>` prints *"No Ghostty tab is showing
session 'X' (it may be detached)"* while the tab is right there.

**Root cause.** `tab-alert` (v0.50.0, #764) PREPENDS a `● ` to the tab title, and the jump
matched the raw title: the AppleScript asked `tn starts with "<session> — "`, which
`"● MP — multipilot"` never satisfies. Because tab-alert marks **only waiting sessions**,
the failure landed exactly on the rows a user clicks — the ones that need them.

**Must-check when a jump misses.** Read the tab's REAL title before theorising:

```sh
osascript -e 'tell application "Ghostty" to return name of tab 3 of window 1'
# and to see which tab is actually selected — NOT `front tab`, which is always tab 1:
osascript -e 'tell application "Ghostty" to return name of selected tab of front window'
```

**Fix (v0.51.1).** Matching tolerates leading decoration — `ghostty.TitleMatchesSession` /
`StripDecoration`, used by Ghostty + Warp, with iTerm2's script accepting the marked form.
The general point, and why the fix is not about `●`: **the terminal decorates titles too**
(Ghostty prefixes a background tab that rang the bell), so a tab-to-session match must be
decoration-tolerant by construction. `SessionsFromTitles` already did this for the bell
glyph; the focus path did not.

**Two AppleScript traps found while fixing it.**
- `select tab 8 of window 1` is INVALID in Ghostty's dictionary — *"Can't get 8 of window
  1. Access not allowed. (-1723)"*. The only reference `select` accepts is the loop
  variable from `repeat with t in tabs of w`. So match in Go, then select by the tab's
  exact raw title through that loop.
- `name of front tab of front window` returns **tab 1**, not the selected tab. Verifying a
  jump with it reports failure on a jump that worked. Use `selected tab`.

## App Store upload "failed" when it actually landed (2026-08-18, 0.66.1)

**Symptom.** `bundle exec fastlane release` ends with a Ruby crash —
`uninitialized constant Gem::Resolver::APISet::GemParser (NameError)` followed by
*"RubyGems is not listed as your Gem source"* — and an immediate App Store Connect query
still shows the PREVIOUS build. Everything points at a failed upload.

**It had already uploaded.** Re-running the `upload` lane on the same ipa is what proved it:
altool answered *"Redundant Binary Upload. You've already uploaded a build with build number
'12' for version number '0.66.1'"*. Two independent things had lined up to look like failure:

- **The crash is teardown noise.** Homebrew ruby is at 4.0.6; the pinned fastlane (2.237.0)
  predates its Ruby 4 support (`[core] Support for Ruby 4` ships in a later release). A
  subprocess spawned after the transfer dies on it — after the binary is on Apple's side.
- **A fresh build is not in the builds list yet.** ASC takes minutes to surface an upload,
  so "the list still shows build 11" is not evidence of anything in the first few minutes.

**Must check.**
- Never judge a lane by an immediate ASC query. Ask again a few minutes later, or re-run
  `upload` — a *Redundant Binary Upload* error is positive proof the first one landed.
- Never run a lane through `| tail -N`: the pipe hands you `tail`'s exit code (always 0) and
  hides every line above the window. Redirect to a log file and grep it.
- The fix for the noise itself is `bundle update fastlane` (or a Ruby 3.x for this
  toolchain); until then, expect the crash and read past it.

## A pane goes blind: the agent moved, the hook lost its pane (2026-08-18, %13)

**Symptom.** The phone shows a calm, complete conversation whose newest turn is hours
old. The radar shows the pane `idle`. HQ's picture of it is frozen at the same moment.
Nothing errors anywhere — and the pane is working the whole time.

**What happened.** Claude Code 2.1.234 moved the session into a background host:

    34175  the pane's interactive client   TMUX=…  TMUX_PANE=%13
    77257    claude daemon run --origin transient --spawned-by {pid:34175}
    77301      --bg-pty-host
    78152        the conversation process  (no tty, no TMUX_PANE)

The hooks kept firing — `hookErrors` was empty and `stop_hook_summary` listed `gtmux
hook` every time — but from a process with no `$TMUX_PANE`. gtmux identified panes by
that one env var, so every event was filed as a NATIVE (non-tmux) session under
`native/<sessionId>.json`. The pane's resume binding never updated, so `/api/transcript`
kept serving a log whose last message was 17:47 while the conversation ran on until
22:58 in a session gtmux had never heard of.

**Diagnosing it (in this order — three of these contradict each other).**
- `~/.local/share/gtmux/resume/<base64 of session:window.pane>.json` — what the chat is
  being served FROM. Compare its `sessionId` against what is actually growing.
- `ls -lt ~/.claude/projects/<cwd-slug>/` — **mtime lies here.** The dead log had the
  NEWER mtime (Claude appends `permission-mode` records to it long after the
  conversation left). Read the last `"timestamp"` in the file, not the mtime.
- `ps -o ppid=,tty=,command= -p <pid>` up the chain from the agent process, and
  `ps eww -p <pid> | tr ' ' '\n' | grep TMUX` — the decisive evidence: which process
  actually runs the conversation, and whether it can see the pane.
- `~/.local/share/gtmux/native/` — a record here whose cwd matches a tmux pane is the
  fingerprint of this failure.
- `grep '"%13"' ~/.local/share/gtmux/events.jsonl | tail` — the moment the events stop
  is the moment the session moved.

**Fixed (hook-pane-identity).** A hook with no `$TMUX_PANE` now walks its process
ancestry and matches it against tmux's pane table (`pane_pid`, then `pane_tty`) before
concluding it is native. `gtmux doctor`'s `chat binding` row reports a pane whose
directory holds a newer, live, unclaimed session. The mobile chat polls while open
(conditional on an ETag) instead of refreshing only when the status flips — a pane whose
status is stuck cannot flip, which is why the reader saw nothing new for five hours.

**Must-check when something like this recurs:** the events stopping is not evidence the
hook stopped running. Check where the hook RAN before assuming it failed — and never
judge which log is live by its mtime.

### `gtmux hq` moved my terminal and said nothing (2026-08-20)

**Symptom:** you run `gtmux hq` expecting a supervisor to start. Instead the terminal
jumps to some other window, and as far as you can tell nothing was explained.

**What was actually happening:** a supervisor was already running, so the command
focused it rather than starting a second one — which is right, since two supervisors
would both be driving the same panes. It did print a line. But it printed it AFTER the
jump, so the words stayed in the window you were just taken out of, and the wording
("Focused the running supervisor") confirmed an action nobody had asked for instead of
answering the question you actually had.

**Fix:** the line now comes BEFORE the jump, names where the supervisor is
(`HQ:0.0 · %6`), and says plainly that nothing new was started — and the same fact is
put on tmux's status bar in the window you land in (`noteAtPane`), because that is where
you are looking once the jump has happened.

**The general rule this is an instance of:** say something when what happened differs
from what was asked. A jump you asked for needs no words; a jump you did not ask for
needs them, and they have to arrive where your eyes are. `gtmux focus` already worked
this way (it speaks only when the pane you land on is no longer running an agent).

## A permission prompt with the wrong app's name on it (2026-08-23)

**Symptom:** macOS asks *"Gtmux.app would like to access data from other apps"*, now and
then, while gtmux is doing nothing that would need it.

**It is not gtmux.** It is one of your own agents — clearing disk space, reading a cache,
walking `~/Library` — and the prompt is putting gtmux's name on it.

**Root cause:** macOS attributes a file access to the **responsible process**, the
application at the root of the process tree, and fixes that when a process is created. It
survives being re-parented to launchd, so nothing in `ps` reveals it — the tmux server
shows `ppid=1` and looks unowned. `gtmux restore` run from the menu-bar app creates the
tmux server as the app's child, and from then on every pane in that server, and every
command every agent runs in it, answers as Gtmux.app for as long as the server lives.

**How to check it yourself:**

```bash
log show --last 1h --predicate 'process == "tccd" AND eventMessage CONTAINS "AUTHREQ_PROMPTING"' \
  --style compact | grep -oE "responsible_path=[^,]*|binary_path=[^}]*"
```

`responsible_path` is the name on the dialog; `binary_path` is what actually asked. Seeing
`responsible_path=…/GtmuxBar` beside `binary_path=/usr/bin/du` is this exact case.

**Fixed (tmux-server-own-identity):** a restore started by the menu-bar app now creates
the server through launchd, which hands it an identity of its own — measured, same read
both ways: started directly → `GtmuxBar`, started by launchd → `tmux`. A restore from a
terminal is unchanged: it is already attributed to that terminal, which IS the app the
user launched.

**Two things worth knowing:**
- **It cannot fix a server already running.** Identity is set at creation, so an existing
  session keeps answering as Gtmux.app until tmux is next restarted.
- `setsid` does nothing here. Responsibility is neither the process group nor the parent,
  and survives both — tried first, measured, discarded.

## Reinstalling an agent's hooks silences the sessions already running (2026-08-29)

**Symptom:** an agent stops being seen entirely — no waiting, no done, no digest — while
its pane visibly works. `gtmux doctor` says its hooks are installed. They are.

**What happened:** `gtmux install hooks --agent codex` rewrote `~/.codex/hooks.json`
under a Codex that was already running. That session kept using the hooks it started
with for another two hours, then went silent across EVERY Codex pane at once, because
Codex re-read the changed file and stopped trusting the entries. Nothing said so: no
prompt on screen, nothing in 3000 lines of scrollback.

Measured: reinstall at 23:57 → last Codex event at 02:13 → zero Codex events for the next
six hours, during which an approval sat unanswered and the radar showed the session
`working`.

**Why the existing checks could not see it:** they ask about the FILE — is it installed,
is it complete. Both were true the whole time. The channel was dead anyway.

**Fixed (this change):** `gtmux doctor` gains a `hook traffic` row that compares two
clocks gtmux already keeps — the pane is painting, and no event has arrived:

```
⚠  hook traffic   %60 (6h)   these panes are busy but their agent has sent nothing
```

**Must-check when installing hooks into a live fleet:** the sessions already running are
NOT left alone. Restart them (Codex asks you to trust the hooks again after a change —
press `t`), and check the `hook traffic` row afterwards rather than assuming.

## A session came back as a bare shell after a reboot (2026-08-29, Codex + `opencrab`)

**Symptom.** After a machine restart, `gtmux restore` brought the tmux layout back but
one Codex pane was an empty shell — no conversation, no error, nothing in the plan
marked `×`.

**Two different things look identical here, and only one is a bug.**

1. **The save says the pane was a shell.** Then restore is CORRECT: the agent had
   already exited when the layout was saved, and injecting a resume into a pane that
   never ran an agent is exactly the phantom #688 removed. Check it before anything
   else:

   ```sh
   # what the save recorded for each pane — locator, command, full command line
   awk -F'\t' '/^pane/{print $2":"$3"."$6, $10, $11}' ~/.local/share/tmux/resurrect/last
   gtmux restore --plan          # ↻ = will come back · × = transcript gone
   ```

   A pane whose command is a shell and whose full command line is empty (a bare `:`)
   was a shell in the save, and no record anywhere overrides that.

2. **The agent was started through a wrapper.** `opencrab`, an alias, an `npx`-style
   launcher — anything that is not the agent's own binary name. Until the launcher was recorded, the
   resume record held `{agent, sessionId, cwd}` and nothing about the launch, so the
   conversation was resumed as `codex resume <id>`: no wrapper, and none of the
   configuration, credentials or network the wrapper exists to provide. Claude hides
   this class of bug, because tmux-resurrect saves its whole command line and replays
   it verbatim; gtmux's own resume path is the one that assumed the launcher was the
   agent's name.

**Must-check when a wrapper is involved.** The launcher is read from the pane at hook
time, so a session running since before the upgrade has no launcher recorded:

```sh
# what gtmux will type to bring this pane's conversation back
cat "$HOME/.local/share/gtmux/resume/$(printf %s 'session:0.0' | base64 | tr '+/' '-_' | tr -d '=').json"
# → {"agent":"codex", …, "launcher":"opencrab"}   ← present only after a prompt/session-start
tmux display -p -t %NN '#{pane_current_command}'  # the wrapper must be visible here
```

If `pane_current_command` shows the wrapper but the record has no `launcher`, the
session simply has not submitted a prompt since the upgrade — one more turn records it.
