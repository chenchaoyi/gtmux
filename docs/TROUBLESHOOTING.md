# Troubleshooting & footguns (living checklist)

Pitfalls we've actually hit during **development, debugging, and release** — with
the check that would have caught each one early. This is a **living document**:
when a new footgun costs real time, add an entry here (symptom → root cause → the
must-check / rule), so the next person (or Claude) trips a checklist instead of the
rake. Keep entries short and action-first.

> Related runbooks live next to their subsystem: remote-access / pairing debug in
> `docs/design/remote-access-tunnel.md`; deploy paths in `CLAUDE.md` (Deploy table).

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
- 新增一个 gtmux 要 shell 出去调用的工具时，用 `lookTool()`（`internal/app/toolpath.go`）而不是
  `exec.LookPath`。
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

### `gtmux hq` "Focused the running supervisor" but the HQ session is dead
**Symptom:** you quit the HQ agent but left its tmux window open (a bare shell). Later
`gtmux hq` says "Focused the running supervisor" and jumps to that window — which holds
only a shell prompt, no agent. Confusing.
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

### `feed-degraded` in HQ — the perception feed is down
**Symptom:** HQ surfaces `⚠ perception feed down — on the 5-min polling backstop`, or a
`[CRITICAL gtmux:feed-degraded]` line appears in `gtmux hq-feed --tail`.
**Root cause:** the `gtmux hq-feed` daemon died and mechanical self-heal failed twice
(the no-LLM watchdog lives in the `gtmux serve` slow-tick — if serve is OFF, nothing
restarts it automatically).
**Must-check / fix:** `gtmux hq-feed --status` (running? heartbeat age ≤ 90s? cursor lag?).
If down, `gtmux hq-feed --daemon &` restarts it (singleton-guarded), or just re-attach
HQ's `gtmux hq-feed --tail` — the tail auto-starts the daemon. Confirm `gtmux serve` is
running so the watchdog can supervise it going forward. Files:
`~/.local/share/gtmux/hq-feed/{pid,cursor,heartbeat,spool.jsonl}`.

### HQ went quiet — is it the feed or the surfacing threshold?
**Symptom:** HQ stopped printing routine updates.
**Root cause:** by design. The feed is SILENT (gtmux no longer types low-value receipt
nudges into the pane); HQ only PRINTS CRITICAL/NORMAL and ledger-records QUIET. Quiet
mode raises the bar to CRITICAL-only.
**Must-check:** `gtmux quiet status` (the resolved threshold). QUIET items are in
`gtmux tasks --verbose`, not lost. A `feed-degraded` CRITICAL is never quieted, so
silence there means the feed is healthy, not broken.

### Seed is generated ONCE — a live HQ home won't auto-update
The attention-system behavior lives in the HQ playbook (`hq.go` `hqInstructions` →
`~/.config/gtmux/hq/AGENTS.md`), which is seeded once and never overwritten. A FRESH hq
home gets it automatically; the commander's EXISTING HQ needs a deliberate re-seed
(back up and remove/replace AGENTS.md, then `gtmux hq`) to pick up the feed/threshold/
self-check instructions.

---

## Driving a pane (dispatch / `gtmux send`)

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

### Menu bar frozen + phone shows "0 agents" after a network switch (wedged `ps`)
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
