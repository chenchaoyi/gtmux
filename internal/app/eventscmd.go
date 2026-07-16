// `gtmux events` — read or --follow the session event stream (session-events):
// the hook appends every lifecycle event of every session to a rotated log, and
// this is the terminal-native SUBSCRIPTION to it — gtmux HQ tails it to stay
// aware of any session's execution, the equivalent of the apps' SSE stream.
package app

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

// cmdEvents implements `gtmux events [--follow] [--json] [--since <dur>]
// [--since-seq <n>] [--severity <level>]`.
func cmdEvents(args []string) int {
	follow, jsonOut := false, false
	since := int64(0)
	sinceSeq := int64(-1) // -1 = not given (0 is a valid cursor: "everything retained")
	minSeverity := ""     // "" = no filter
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--follow" || a == "-f":
			follow = true
		case a == "--json":
			jsonOut = true
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

	// Filter to "this level and above" so a supervisor reads the attention stream,
	// not every raw line (an absent severity on a legacy record ranks as routine).
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
		for _, r := range events.Read(0, time.Now().Unix()) {
			if r.Seq > sinceSeq {
				print(r)
			}
		}
		stampHQPull()
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
	i18n.Say("usage: gtmux events [--follow|-f] [--json] [--since 10m|2h|90s] [--since-seq N] [--severity routine|notable|important]",
		"用法：gtmux events [--follow|-f] [--json] [--since 10m|2h|90s] [--since-seq N] [--severity routine|notable|important]")
	i18n.Say("  The live stream of every session's lifecycle events — the subscription",
		"  每个 session 生命周期事件的实时流 —— gtmux HQ 及脚本的订阅入口。")
	i18n.Say("  gtmux HQ and scripts tail it. Bare form shows the last hour.",
		"  裸命令显示最近一小时;--follow 持续跟随(跨 rotation)。")
	i18n.Say("  --severity filters to that tier and above (the attention stream).",
		"  --severity 过滤到该等级及以上(只看需要关注的事件)。")
	i18n.Say("  --since-seq N: one-shot delta read of everything after sequence N",
		"  --since-seq N：一次性读取序号 N 之后的全部事件(唤醒后拉增量用)。")
	i18n.Say("  (the pull-on-wake primitive — HQ reads exactly the delta a wake covered).",
		"  (即唤醒线覆盖区间的增量拉取原语)。")
	return 0
}

// stampHQPull records HQ's pull freshness when THIS invocation ran from the HQ
// home (cwd-keyed — the same role rule the radar uses). Any other caller is not
// the supervisor and must not refresh its consumer stamp.
func stampHQPull() {
	if cwd, err := os.Getwd(); err == nil && cwd == state.HQHome() {
		hqwake.StampPull()
	}
}
