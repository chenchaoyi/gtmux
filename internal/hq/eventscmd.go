// `gtmux events` — read or --follow the session event stream (session-events):
// the hook appends every lifecycle event of every session to a rotated log, and
// this is the terminal-native SUBSCRIPTION to it — gtmux HQ tails it to stay
// aware of any session's execution, the equivalent of the apps' SSE stream.
package hq

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// parseSince turns "10m"/"2h"/"90s"/"45" (bare = seconds) into seconds, 0 on error.
func parseSince(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	mult := int64(1)
	switch s[len(s)-1] {
	case 's':
		s = s[:len(s)-1]
	case 'm':
		mult, s = 60, s[:len(s)-1]
	case 'h':
		mult, s = 3600, s[:len(s)-1]
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 0 {
		return 0
	}
	return int64(n) * mult
}

// validSeverity reports whether a --severity value is one of the known tiers.
func validSeverity(level string) bool {
	switch level {
	case events.SevRoutine, events.SevNotable, events.SevImportant:
		return true
	}
	return false
}

// CmdEvents implements `gtmux events [--follow] [--json] [--since <dur>]
// [--since-seq <n>] [--severity <level>]`.
func CmdEvents(args []string) int {
	follow, jsonOut, all := false, false, false
	since := int64(0)
	sinceSeq := int64(-1) // -1 = not given (0 is a valid cursor: "everything retained")
	ackSeq := int64(-1)   // -1 = not given (0 is a valid ack: "back to the start")
	minSeverity := ""     // "" = no filter
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--follow" || a == "-f":
			follow = true
		case a == "--json":
			jsonOut = true
		case a == "--all":
			all = true
		case a == "--since":
			if i+1 >= len(args) {
				return eventsUsage()
			}
			i++
			since = parseSince(args[i])
		case strings.HasPrefix(a, "--since="):
			since = parseSince(strings.TrimPrefix(a, "--since="))
		case a == "--since-seq":
			if i+1 >= len(args) {
				return eventsUsage()
			}
			i++
			n, err := strconv.ParseInt(strings.TrimSpace(args[i]), 10, 64)
			if err != nil || n < 0 {
				return eventsUsage()
			}
			sinceSeq = n
		case strings.HasPrefix(a, "--since-seq="):
			n, err := strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(a, "--since-seq=")), 10, 64)
			if err != nil || n < 0 {
				return eventsUsage()
			}
			sinceSeq = n
		case a == "--ack":
			if i+1 >= len(args) {
				return eventsUsage()
			}
			i++
			n, err := strconv.ParseInt(strings.TrimSpace(args[i]), 10, 64)
			if err != nil || n < 0 {
				return eventsUsage()
			}
			ackSeq = n
		case strings.HasPrefix(a, "--ack="):
			n, err := strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(a, "--ack=")), 10, 64)
			if err != nil || n < 0 {
				return eventsUsage()
			}
			ackSeq = n
		case a == "--severity":
			if i+1 >= len(args) {
				return eventsUsage()
			}
			i++
			if !validSeverity(args[i]) {
				return eventsUsage()
			}
			minSeverity = args[i]
		case strings.HasPrefix(a, "--severity="):
			v := strings.TrimPrefix(a, "--severity=")
			if !validSeverity(v) {
				return eventsUsage()
			}
			minSeverity = v
		case a == "-h" || a == "--help":
			return eventsUsage()
		default:
			i18n.Sae("gtmux events: unknown option '"+a+"'", "gtmux events: 未知选项 '"+a+"'")
			return 2
		}
	}

	if ackSeq >= 0 {
		return cmdEventsAck(ackSeq) // explicit writeback; reads nothing
	}

	// Filter to "this level and above" so a supervisor can triage without reading every
	// raw line (an absent severity on a legacy record ranks as routine). A filtered read
	// is a SHORTCUT, not a complete picture: `important` is the escalation subset, and
	// reconciling is the unfiltered `--since-seq` delta's job.
	minRank := events.SeverityRank(minSeverity)
	print := func(r events.Record) {
		if minSeverity != "" && events.SeverityRank(r.Severity) < minRank {
			return
		}
		if jsonOut {
			b, _ := json.Marshal(r)
			fmt.Println(string(b))
		} else {
			fmt.Println(events.Format(r))
		}
	}

	if sinceSeq >= 0 {
		// Sequence-filtered delta read (hq-perception-v2): everything retained with
		// seq strictly greater than the cursor, oldest first — the pull-on-wake
		// primitive. Combinable with --severity/--json; --follow is ignored (one-shot).
		var delta []events.Record
		maxSeq := sinceSeq
		for _, r := range events.Read(0, time.Now().Unix()) {
			if r.Seq > sinceSeq {
				if r.Seq > maxSeq {
					maxSeq = r.Seq
				}
				delta = append(delta, r)
			}
		}
		shown, hidden := pullView(delta, minSeverity == "" && !all)
		for _, r := range shown {
			print(r)
		}
		noteHiddenEcho(hidden)
		stampHQPull()
		// This delta read IS HQ's consumption writeback (hq-watermark-wakes): the everyday
		// path needs no new discipline, because pulling on a wake is what HQ already does.
		//
		// A FILTERED read deliberately does not count. `--severity important` shows the
		// escalation subset, so treating it as consumption would let HQ mark a range read
		// while never seeing most of it — the playbook's "a filter is a triage shortcut,
		// never your model of the world" turned from advice into mechanism. Reading
		// filtered simply leaves the debt standing, and the next knock names it again.
		if minSeverity == "" {
			consumeHQRead(sinceSeq, maxSeq)
		}
		return 0
	}

	if !follow {
		// A bare `gtmux events` shows a recent window by default so it's useful
		// without a flag; --since overrides.
		if since == 0 {
			since = 3600 // last hour
		}
		for _, r := range events.Read(since, time.Now().Unix()) {
			print(r)
		}
		stampHQPull()
		return 0
	}

	// --follow: replay the requested window (default: none — just new events),
	// then stream. Ctrl-C stops.
	stop := make(chan struct{})
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() { <-sig; close(stop) }()
	events.Follow(since, time.Now().Unix(), print, stop)
	return 0
}

func eventsUsage() int {
	i18n.Say("usage: gtmux events [--follow|-f] [--json] [--all] [--since 10m|2h|90s] [--since-seq N] [--severity routine|notable|important] [--ack N]",
		"用法：gtmux events [--follow|-f] [--json] [--all] [--since 10m|2h|90s] [--since-seq N] [--severity routine|notable|important] [--ack N]")
	i18n.Say("  The live stream of every session's lifecycle events — the subscription",
		"  每个 session 生命周期事件的实时流 —— gtmux HQ 及脚本的订阅入口。")
	i18n.Say("  gtmux HQ and scripts tail it. Bare form shows the last hour.",
		"  裸命令显示最近一小时;--follow 持续跟随(跨 rotation)。")
	i18n.Say("  --severity filters to that tier and above: `important` = the escalation",
		"  --severity 过滤到该等级及以上：important = 升级流(阻塞/提问/崩溃),")
	i18n.Say("  subset (blocked/asking/crashed), `notable` = fleet changes too. A filter",
		"  notable = 连同变化流(指令、回合结束、生命周期)。过滤是分诊捷径,")
	i18n.Say("  is a triage shortcut — reconcile with the unfiltered --since-seq delta.",
		"  不是全貌 —— 对账请用不过滤的 --since-seq 增量。")
	i18n.Say("  --since-seq N: one-shot delta read of everything after sequence N",
		"  --since-seq N：一次性读取序号 N 之后的全部事件(唤醒后拉增量用)。")
	i18n.Say("  (the pull-on-wake primitive — HQ reads exactly the delta a wake covered).",
		"  (即唤醒线覆盖区间的增量拉取原语)。")
	i18n.Say("  An UNFILTERED --since-seq read from the HQ home advances HQ's consumption",
		"  从中控目录运行的不过滤 --since-seq 读取会推进中控的消费水位;")
	i18n.Say("  watermark; anything past it re-knocks as `unread` until consumed.",
		"  水位之后仍未消费的事件会以 `unread` 反复敲门,直到被消费。")
	i18n.Say("  That pull shows exactly the DEBT: HQ's own records and pane-less blinks are",
		"  该增量只显示「债务」本身：你自己的记录与无 pane 闪断会被隐藏(它们本就不计数),")
	i18n.Say("  hidden (they never counted) — `--all` includes them, and still consumes.",
		"  需要全量加 `--all`(同样计入消费)。")
	i18n.Say("  --ack N: write the watermark back explicitly (HQ home only), for when the",
		"  --ack N：显式回写水位(仅中控目录),用于以别的方式(如 digest 全量对账)")
	i18n.Say("  stream was reconciled another way (a full `gtmux digest`).",
		"  完成消费的场合。")
	return 0
}

// stampHQPull records HQ's pull freshness when THIS invocation ran from the HQ
// home (cwd-keyed — the same role rule the radar uses). Any other caller is not
// the supervisor and must not refresh its consumer stamp.
func stampHQPull() {
	if fromHQHome() {
		hqwake.StampPull()
	}
}

// fromHQHome reports whether this invocation is the supervisor speaking — the cwd-keyed
// role rule the radar and the pull stamp already use. HQ's watermark is HQ's alone: a
// worker running `gtmux events` in some repo must never be able to declare the
// supervisor caught up.
func fromHQHome() bool {
	cwd, err := os.Getwd()
	return err == nil && cwd == state.HQHome()
}

// insideHQHome reports the cd-DRIFT shape: a cwd strictly inside the HQ home (`notes/`,
// `knowledge/`, …) rather than at it. Nobody but HQ works in there, so a read from there
// is HQ's read — made from the wrong directory.
//
// This is deliberately the ONLY widening of the role rule. Keying on `$TMUX_PANE == the HQ
// pane` as well would catch a drift to an unrelated cwd, but it would put tmux resolution
// on a path that today touches no tmux at all — and a wedged tmux hanging the pull HQ makes
// on every wake is the exact failure mode that froze the radar once already. The measured
// B9 evidence is 5 for 5 inside the home, so the cheap rule covers every observed case; a
// cwd fully outside the home is indistinguishable from a bystander's read, which must stay
// silent.
func insideHQHome() bool {
	cwd, err := os.Getwd()
	if err != nil {
		return false
	}
	home := state.HQHome()
	return home != "" && cwd != home && strings.HasPrefix(cwd, home+string(os.PathSeparator))
}

// pullView applies the SUPERVISOR'S PULL VIEW: the delta HQ reads to clear its debt shows
// exactly the records that made up that debt, and nothing else. It returns what to print
// and how many records it withheld.
//
// This closes an asymmetry that was, measured, the single largest cost in HQ perception
// (hq-unread-noise audit, 2026-08-08): the COUNT excluded HQ's own records and pane-less
// blinks, while the READ that clears it returned everything — so 68.7 % of what a knock
// sent HQ to read was its own echo (75.1 % for the ≤2-event knocks that are 79 % of them),
// and the median knock spent a whole HQ turn reading ~3.4 records to find ONE new fact.
// One set said what HQ owed; a larger, different set was what HQ had to read.
//
// It is NOT a filter in the sense the consumption rule forbids. A `--severity` read shows a
// SUBSET of what HQ owes, which is why it cannot count; this shows precisely what HQ owes,
// so consumption is untouched. `--all` restores the raw view for the times HQ needs to see
// its own trail (and still consumes — it is a superset).
//
// HQ's own records are identified by the CALLER'S OWN `$TMUX_PANE`, never by resolving the
// HQ pane through tmux: an agent running this from its pane already knows which pane it is,
// and reading an env var keeps the read path free of the tmux round-trips whose wedging
// once froze the radar. (That is also why the B9 warning does not key on the pane — see
// insideHQHome: knowing your own pane tells you nothing about whether you are the
// supervisor, which is the question that one has to answer.)
func pullView(delta []events.Record, apply bool) (shown []events.Record, hidden int) {
	if !apply || !fromHQHome() {
		return delta, 0
	}
	own := os.Getenv("TMUX_PANE")
	blink := unreadBlinks(delta)
	for i, r := range delta {
		if (own != "" && r.Pane == own) || blink[i] {
			hidden++
			continue
		}
		shown = append(shown, r)
	}
	return shown, hidden
}

// noteHiddenEcho tells HQ what the pull view withheld. Silent hiding would be the same
// failure as the silent non-consumption B9 fixed: a read that quietly shows less than it
// says is one HQ cannot reason about. stdout stays the read's product; this goes to stderr.
func noteHiddenEcho(hidden int) {
	if hidden <= 0 {
		return
	}
	n := strconv.Itoa(hidden)
	i18n.Sae(n+" of your own records and pane-less blinks hidden (they are not debt) — `--all` to include them",
		n+" 条你自己的记录与无 pane 闪断已隐藏（它们不计入债务）—— 需要全量请加 `--all`")
}

// consumeHQRead advances HQ's consumption watermark for a completed delta read, and — when
// the caller plainly IS the supervisor but stood in the wrong directory — says so instead
// of failing silently.
//
// That asymmetry is the whole point (B9): `--ack` from the wrong cwd refuses LOUDLY, while
// this path just skipped the writeback and returned 0. Two paths with one meaning and
// opposite failure modes, so HQ read the delta, believed it had consumed, and watched the
// same cursor re-knock — reproduced five times, once in the very turn after writing the
// note about it. A discipline that fails silently is not a discipline.
func consumeHQRead(from, to int64) {
	if fromHQHome() {
		hqwake.ConsumeRead(from, to)
		return
	}
	if insideHQHome() {
		i18n.Sae("gtmux events: this read was NOT counted as consumption (run it from "+state.HQHome()+", or `gtmux events --ack <seq>`)",
			"gtmux events：本次读取未计入消费水位（请在 "+state.HQHome()+" 下运行，或用 `gtmux events --ack <seq>` 回写）")
	}
}

// cmdEventsAck implements `gtmux events --ack <seq>`: the EXPLICIT writeback, for when HQ
// consumed the stream some way other than an unfiltered delta read (a full `gtmux digest`
// reconcile, a filtered read it judged complete). It performs no read of its own — it only
// moves the cursor, monotonically, and never past the end of the stream, so a mistyped
// value cannot blind HQ to everything up to it.
func cmdEventsAck(seq int64) int {
	if !fromHQHome() {
		i18n.Sae("gtmux events --ack: only the HQ session can acknowledge its own watermark (run it from "+state.HQHome()+")",
			"gtmux events --ack：只有中控会话能回写自己的消费水位（请在 "+state.HQHome()+" 下运行）")
		return 1
	}
	if latest := events.CurrentSeq(); seq > latest {
		seq = latest
	}
	hqwake.Consume(seq)
	i18n.Say("consumed through seq "+strconv.FormatInt(hqwake.Consumed(), 10),
		"已消费至序号 "+strconv.FormatInt(hqwake.Consumed(), 10))
	return 0
}
