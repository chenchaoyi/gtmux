# 夜间自测记录 · 2026-09-09 夜 → 2026-09-10 晨

一个 worker 在 `gtmux-wt/test-nightly-selftest` worktree 里跑了一整夜。这份是给司令早上读的:
查了什么、发现了什么、修了什么、还剩什么没查。

---

## 一句话结论

今晚翻出来的六个缺陷里,五个是同一个病:**一个只会说「对」的检查**。守卫存在、测试存在、
门禁存在,但它们守的不是自己声称守的东西。全量测试从头到尾是绿的,这些缺陷一个都没让它变红过。

---

## 这个仓库怎么测(实测跑法)

| 层 | 跑法 | 规模 | 在 CI 里吗 |
|---|---|---|---|
| Go 核心 | `make check` = gofmt + vet + staticcheck(锁 v0.7.0)+ `go test -race ./...` | 241 个测试文件 / 38 个包 / 约 40 秒 | 是 |
| 设计与架构一致性 | `./scripts/check-design.sh` | 状态色、import 无环、图标尺寸、spec 校验、命令文档漂移、唤醒词表、pane 写入者声明、`$HOME` 收口、移动端发版说明 | 是 |
| 菜单栏 app | `cd macapp && swift build -c release && swift test` | 171 个 Swift 测试 | 是 |
| 手机 app(JS 层) | `cd mobileapp && npm run check` = tsc + eslint + jest | 80 个 suite / 877 个用例 | 是 |
| 推送中继 Worker | `cd relay-worker && npm run typecheck && npm test` | 7 个 golden payload 用例 | 是 |
| 隧道 Worker | `cd tunnel-worker && npm run typecheck && npm test` | 4 个用例 | 是 |
| **模拟器自动化** | `cd mobileapp && npm run e2e:build && npm run test:e2e` | 27 个 Appium suite + 5 个 fake-serve 契约测试 | **否** |
| **Go 集成测试** | `GTMUX_RESTORE_E2E=1` / `GTMUX_IT=1 go test ./internal/app/` | restore 契约 7 个子用例等 | **否** |

开工时先跑了一遍全部不吃磁盘的部分:**全绿**,零失败。所有缺陷都是在这个绿的基础上找出来的。

关于「模拟器自动化测试可能根本不存在」的猜测:**它存在,而且相当完整**。27 个 Appium 测试
覆盖雷达、终端渲染、SSE 重连、发送回执、HQ 页、知识库、截图,另有一份带编号的手工回归
`TEST-PLAN.md`,以及一层 `GTMUX_DEBUG_*` 启动参数调试通道。缺口不在「有没有」,在「跑不跑得到」。

---

## 已修并合入的六条

### #992 · 守卫误伤了隔离做对了的测试

9 月 9 日新加的 home 守卫,判断「HOME 是不是临时目录」时只认 `os.TempDir()`。macOS 上那是
`/var/folders/…`,于是位于 `/tmp` 下的 HOME 被判成真实目录,直接 panic。

仓库里十几个 tmux 测试**不能**用 `t.TempDir()`:unix socket 路径上限约 104 字节,
`/var/folders/…` 前缀就吃掉大半,所以它们用短的 `os.MkdirTemp("/tmp", "gtx")`。今天有七个测试
这么写。它们隔离做得是对的,而 panic 打印的「先重定向」正是它们已经做过的事。

Linux 上 `os.TempDir()` 就是 `/tmp`,两个根重合,所以 CI 永远照不出这个,只在司令这台 Mac 上出现。

### #993 · 守卫自称「唯一收口」,实际有 23 个旁路

`state.go` 的注释把这个包描述成「every gtmux path 都要经过的唯一收口」,并以此解释守卫为什么
放在这里而不是每个写入口。**这句话没有任何东西在核。**

实际有 23 个文件自己读 `$HOME`,其中包含写入方:装 hook 时改 `~/.claude/settings.json`、
serve 的推送 token / 设备名册 / 分享配置、`remotes.json` 里的配对凭据、用户配置、手机上传目录、
图标缓存、opencode 转写、server-mode 状态。守卫想拦的事故,从这里面任何一条都还走得通。

全部收口后 panic 开始响,抓到 **8 个测试一直在读操作者本机的真实配置**:

- `driver/registry_wiring_test.go` 断言「每个装了 hook 的 agent 都有 Receipt 和 Ready」。
  它调用的 `driver.For()` 会读真实的 `~/.config/gtmux/config.json` 并应用其中的 kill-switch,
  而 kill-switch 正是唯一能让这条断言变假的东西。它在谁的机器上都绿,但绿的原因跟它声称要钉住的
  接线无关:它绿是因为这台机器的人没关那个开关。
- `terminal/appearance_test.go` 重定向了 `XDG_CONFIG_HOME` 并写了一份 Ghostty 配置,但
  `ghosttyConfigPaths()` 还会读 `$HOME/Library/Application Support/com.mitchellh.ghostty/config`。
  这跟九月那次事故是同一个形状:看起来对的那个覆盖,旁边站着一个读别处的解析器。
- 其余几个读的是 Warp 启动配置、tmux 是否用 XDG 布局、`agents.json`、`usage.json`。

同时给 `check-design.sh` 加了一条:`internal/state` 之外再出现 `$HOME` 解析就红。

### #994 · 防踩坑的测试,查的方向是反的

`qr.go` 顶上有条全大写警告:别用 quadrant blocks 缩小终端 QR,会把码拉成 2:1 竖长条,
PR #179 就是因此被回滚的。以这条警告命名的测试,查的是**渲染宽度有没有超过 60 列**。

而 quadrant 渲染会把列数**减半**。它禁止的那种变形渲染出来是 19 列(正确 38 列),稳稳小于 60。
我把 quadrant 渲染器原样放回去验证过:三个 QR 测试**全绿**,#179 那个 bug 今天可以原封不动再发一次。

现在改成对着真正渲染出的网格钉宽高比:每模块列一个字符、每两个模块行一行。

### #995 · restore 契约测试信任隔离,但没有证明它

`restorecontract_test.go` 搭沙箱(临时 HOME、`TMUX_TMPDIR` 指新 socket、清 `$TMUX`),然后跑
tmux-resurrect **真正的** `restore.sh`。那个脚本会建 session、重新拉起存档里的命令,包括
`claude --resume` —— 正是当年往没跑过 agent 的 pane 里注入幽灵 agent 的那条路径。

三层重定向此前全是假设。任何一层没生效,restore 就落到司令正在用的 fleet 上,而文件里没有一条
断言会发现:它断言的是「恢复回来的东西对不对」,一台真实服务器能给它一大堆看着很对的东西。
同目录的 `paneids_test.go` 动手前就先证明隔离,而它的破坏面只是改个窗口名格式。

### #996 · e2e 跳过三分之二,输出跟全绿一样

实测一次默认跑法:**27 个 suite 跳了 19 个,35 个用例跳了 21 个,退出码 0**。

跳过本身是对的(提交进仓库的测试不该带地址和 token),问题是 jest 对这件事的总结
(`19 skipped, 8 passed`)跟一次全跑完几乎长得一样。现在运行结束会说清跑了哪些、没跑哪些、
设什么能跑起来,名单从 suite 文件自己读出来所以不会漂移;另加一句列名单代替不了的话:
没有活 serve 的一次运行一个真实 pane 都碰不到,对雷达、终端、HQ、知识库、用量在真 serve 下
是否正常,它一个字都没说。`GTMUX_E2E_REQUIRE_LIVE=1` 可以把「没跑全」变成失败。

### #997 · 章程改了不提版本号,没人拦

CLAUDE.md 粗体写着:改 `hqInstructions` 必须提 `hqPlaybookVersion`。因为 `gtmux hq` 只在
「发布版本 > 已安装版本」时重新播种,不提版本号,新章程就永远到不了任何已经有 `AGENTS.md`
的家目录 —— 正是这条改动要写给的那批人。

现有测试断言的是「某几句话在不在正文里」和「版本号 ≥ N」,所以新增一条规则、改写一条、
或删掉一条那些测试没点名的规则,版本号不动也照样绿。实测:往英文章程加一句话,
`go test ./internal/hq/` 绿,`check-design.sh` 绿。

现在把正文做成指纹按版本号归档。边界写在测试注释里:这个守卫强迫不了版本号往上走,
它消掉的只是「不小心漏了」。

---

## 每条修复的注入验证

要求是「把守卫拿掉,测试必须变红」。逐条记录:

| PR | 注入点 | 结果 |
|---|---|---|
| #992 | 去掉 `/tmp` 白名单 | 「短 /tmp 目录算可丢弃」变红 |
| #992 | 把可丢弃根放宽到 `/` | 「操作者真实 HOME 不可丢弃」变红 |
| #993 | 把 `usercfg.Path()` 改回直接读 `$HOME` | 门禁点名该文件,构建失败 |
| #994 | 放回 quadrant 渲染器 | 报「19 glyphs across a 38-module row」变红 |
| #995 | 在隔离 socket 上先起一个占位 session | 报「not isolated … holding [squatter]」变红 |
| #996 | 严格模式下跑一个会被跳过的 suite | 退出码 1 |
| #996 | 严格模式下跑一个**会执行**的 suite | 退出码 0(反向验,否则就是无差别报错) |
| #997 | 改英文章程不动版本号 | 红,并打印该填的新指纹 |
| #997 | 只改中文章程 | 红 |
| #997 | 把指纹改成只哈希英文一半 | 第二个测试红 |

---

## 还没查的(明天可以接着做)

**活 serve 那 16 个 e2e suite 没跑。** 判断是不冒险:仓库里有两条直接先例 ——
一条记着「隔离 HOME 挡不住启动 app、对活 tmux 下命令这类全局副作用」,当时拿 mutating 命令
当探针把菜单栏跑成了僵尸;另一条记着两个 serve 同时跑会让配对报「过期」。`gtmux serve` 是常驻的、
会驱动 tmux、会往 pane 里打字的进程,深夜无人看着时起一个正落在这两条警告的范围内。
**要跑这 16 个,需要司令决定起一个隔离 serve 实例的方式。**

**e2e 和两个 Go 集成测试都不在 CI 里。** 塞进 CI 是有成本的决定(时长、机器、macOS runner),
留给司令拍板。两个 Go 集成测试今晚手动跑过,都是绿的(`GTMUX_RESTORE_E2E=1`、`GTMUX_IT=1`)。

**`internal/app` 拆分后仍有 15822 行非测试代码**,测试 7885 行,比值 0.50,是大包里最低的
(`hq` 0.79,`dispatch` 1.26,`server` 0.88)。最大的三个文件是 `doctor.go`(1741 行、28 个检查项)、
`serve.go`(1046)、`doctorfix.go`(908)。没来得及逐个翻。

**死代码这条线基本干净**,不用再花时间。staticcheck 一直开着;又扫了一遍「包外零引用的导出标识符」,
剩下的全是接口方法和值类型方法,扫描器分不清而已。

**一条低价值观察,列出来不修:** `internal/hq/slowtick_test.go` 的 `TestResourceTierKey` 用
`resource.MachineTier(m).String()` 做断言的标准答案,而被测函数的实现就是这一句。它只能测出
「返回空」和「用了别的机器」,测不出 `MachineTier` 本身算错。改法不明显,拿不准值不值得动。

---

## 环境与清理

- 全程在 `gtmux-wt/test-nightly-selftest` worktree 里做,没碰主仓工作树。
- `node_modules` 用软链复用主仓,省 660MB;移动端构建复用主仓已有的 Pods 和热缓存,省约 3GB。
- 自建模拟器 `gtmux-selftest`(UDID `BA72804E-E4C0-4549-AEE7-01ABB465072C`),**没有碰**司令那台
  启动着的 iPhone 17 Pro,也没动关机那台 17 Pro Max。
- Swift 构建产物(243MB)跑完即删。收工时会删掉自建模拟器和 `.e2e-artifacts`。
