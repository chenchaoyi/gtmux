// `gtmux hq-feed` — the perception feed (hq-attention-system): a gtmux-managed,
// LLM-free daemon that tails the session-event journal from a persisted cursor and
// spools every event to a rotated file the supervisor (HQ) subscribes to in the
// background. This is the SILENT channel that feeds HQ everything, so gtmux no
// longer types low-value nudge lines into the HQ pane. The gtmux-side watchdog
// (serve slow-tick) keeps the daemon alive; HQ reads the spool via `--tail`.
package hq

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqfeed"
	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// CmdHQFeed implements `gtmux hq-feed [--daemon|--tail|--status]`.
func CmdHQFeed(args []string) int {
	mode := ""
	for _, a := range args {
		switch a {
		case "--daemon":
			mode = "daemon"
		case "--tail", "-f":
			mode = "tail"
		case "--status":
			mode = "status"
		case "-h", "--help":
			return hqFeedUsage()
		default:
			i18n.Sae("gtmux hq-feed: unknown option '"+a+"'", "gtmux hq-feed: 未知选项 '"+a+"'")
			return 2
		}
	}
	switch mode {
	case "daemon":
		return hqFeedDaemon()
	case "tail":
		return hqFeedTail()
	case "status":
		return hqFeedStatus()
	default:
		return hqFeedUsage()
	}
}

// hqFeedDaemon runs the perception daemon in the foreground of THIS process
// (the watchdog starts it detached). Singleton-guarded: a second daemon is a no-op.
func hqFeedDaemon() int {
	if !hqfeed.AcquireSingleton() {
		i18n.Say("gtmux hq-feed: a daemon is already running.", "gtmux hq-feed: 已有 daemon 在运行。")
		return 0
	}
	defer hqfeed.ReleaseSingleton()
	stop := make(chan struct{})
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() { <-sig; close(stop) }()
	hqfeed.Run(nil, stop)
	return 0
}

// hqFeedTail streams the spool — the supervisor backgrounds this to receive every
// event silently. It replays a short recent window, then follows new records
// (rotation-aware), until Ctrl-C.
func hqFeedTail() int {
	// Ensure the daemon is up even when `gtmux serve` (the mechanical watchdog's host)
	// is not running — HQ attaching its tail is enough to bring the feed alive. The
	// singleton guard makes a redundant spawn safe.
	if !hqfeed.Running() {
		_ = spawnFeedDaemon()
	}
	stop := make(chan struct{})
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() { <-sig; close(stop) }()
	// Replay the last few minutes so a freshly-(re)attached tail has context, then
	// stream. The daemon's startup reconcile record tells HQ to snapshot on (re)start.
	hqfeed.FollowSpool(300, time.Now().Unix(), func(r events.Record) {
		fmt.Println(formatSpoolLine(r))
	}, stop)
	return 0
}

// formatSpoolLine renders one spool record for HQ. Control records (reconcile /
// feed-degraded / self-check) are called out; ordinary events reuse the shared
// event formatter plus a severity tag and any summary tail.
func formatSpoolLine(r events.Record) string {
	switch r.Event {
	case hqfeed.ControlReconcile, hqfeed.ControlFeedDegraded, hqfeed.ControlSelfCheck, hqfeed.ControlDistill:
		tag := "CONTROL"
		if r.Event == hqfeed.ControlFeedDegraded {
			tag = "CRITICAL"
		}
		return fmt.Sprintf("%s  [%s %s] %s", clockLocal(r.Ts), tag, r.Event, r.Summary)
	default:
		line := events.Format(r)
		if r.Severity != "" && r.Severity != events.SevRoutine {
			line += "  <" + r.Severity + ">"
		}
		if r.Summary != "" {
			line += "  — " + r.Summary
		}
		return line
	}
}

// clockLocal renders a unix ts as HH:MM:SS local ("--:--:--" for 0).
func clockLocal(ts int64) string {
	if ts <= 0 {
		return "--:--:--"
	}
	return time.Unix(ts, 0).Format("15:04:05")
}

// hqFeedStatus prints the feed's health for humans / the doctor.
func hqFeedStatus() int {
	now := time.Now().Unix()
	running := hqfeed.Running()
	hb := hqfeed.HeartbeatAt() // read ONCE and derive both age + staleness consistently
	age := int64(-1)
	stale := true
	if hb > 0 {
		age = now - hb
		stale = now-hb > int64(hqfeed.StaleAfter/time.Second)
	}
	cursor := hqfeed.ReadCursor()
	latest := events.LatestSeq()
	i18n.Say(
		fmt.Sprintf("hq-feed: running=%v pid=%d heartbeat_age=%ds stale=%v cursor=%d journal_latest=%d behind=%d",
			running, hqfeed.ReadPid(), age, stale, cursor, latest, maxZero(latest-cursor)),
		fmt.Sprintf("hq-feed: 运行=%v pid=%d 心跳龄=%ds 陈旧=%v 游标=%d 日志最新=%d 落后=%d",
			running, hqfeed.ReadPid(), age, stale, cursor, latest, maxZero(latest-cursor)),
	)
	if running && !stale {
		return 0
	}
	return 1
}

func maxZero(n int64) int64 {
	if n < 0 {
		return 0
	}
	return n
}

// spawnFeedDaemon starts the perception daemon DETACHED (its own session, stdio to
// /dev/null) so it survives the spawning process — used by the watchdog and by
// `gtmux hq` startup. Best-effort; the singleton guard makes a redundant spawn safe.
func spawnFeedDaemon() error {
	cmd := exec.Command(selfPath(), "hq-feed", "--daemon")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if devnull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0); err == nil {
		cmd.Stdin, cmd.Stdout, cmd.Stderr = devnull, devnull, devnull
	}
	return cmd.Start()
}

func hqFeedUsage() int {
	i18n.Say("usage: gtmux hq-feed [--daemon|--tail|--status]",
		"用法：gtmux hq-feed [--daemon|--tail|--status]")
	i18n.Say("  The perception feed: gtmux tails the event journal and spools it for HQ.",
		"  感知 feed：gtmux 跟随事件日志并投递给中控（HQ）。")
	i18n.Say("  --daemon  run the feed daemon (the watchdog starts it; singleton).",
		"  --daemon  运行 feed daemon（看门狗会自动拉起；单例）。")
	i18n.Say("  --tail    stream the spool — HQ backgrounds this as its silent feed.",
		"  --tail    跟随 spool —— HQ 把它挂后台当静默 feed。")
	i18n.Say("  --status  print feed health (running / heartbeat age / cursor lag).",
		"  --status  打印 feed 健康（运行/心跳龄/游标落后）。")
	return 0
}
