// Server mode's live half: the heartbeat that tells the guard gtmux is still here,
// plus the two things that must reach a user who is NOT at the machine — a charge
// warning before the floor, and the detection of a session that silently died.
package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/notify"
	"github.com/chenchaoyi/gtmux/internal/servermode"
)

// batteryWarnPct is the charge at which the user is told, while there is still time
// to do something about it. The floor (where sleep is restored) is lower and lives
// in the guard, which enforces it whether or not gtmux is running.
const batteryWarnPct = servermode.EnableThresholdPct

// serverModeTick runs on the serve slow tick (~20s) — the same single-writer cadence
// the resource warnings use, so there is exactly one writer and no race.
//
// It is a no-op on machines not running server mode, which is almost all of them.
func serverModeTick() {
	st := servermode.Current()
	lastServerModeSig.Store(serverModeSig(st))

	// 1. Liveness. This is what stands between an abandoned machine and a battery
	//    drained flat: stop refreshing and the guard restores sleep.
	if st.OwnedByGtmux && st.SystemDisableSleep {
		_ = servermode.Heartbeat()
	}

	// An end the guard recorded: say so once, on this Mac, unless it was the user's own
	// `awake off` here.
	announceServerModeExit(st.LastExit, time.Now())

	// 2. A session that died underneath us. The kernel says sleep is enabled while
	//    our stamp says server mode is on — so the closed-lid session the user is
	//    relying on is already over, and they are the last to know.
	if st.State == servermode.StateLapsed {
		if markServerModeOnce("lapsed") {
			notify.Send(notify.Options{
				Kind:  "done",
				Title: i18n.Tr("Server mode stopped", "服务器模式已停止"),
				Message: i18n.Tr("The sleep setting is no longer in force, so closing the lid will sleep this Mac.",
					"睡眠设置已不再生效，现在合盖会让这台 Mac 休眠。"),
			})
		}
		servermode.ClearStampForLapse()
		return
	}
	if !st.SystemDisableSleep || !st.OwnedByGtmux {
		clearServerModeMarks() // back to normal — re-arm the one-shot warnings
		return
	}

	// 3. Charge warning, while the user can still act. The floor itself is the
	//    guard's job precisely because nobody may be here to read this.
	if st.Power == servermode.PowerBattery && st.BatteryPct > 0 && st.BatteryPct <= batteryWarnPct {
		if markServerModeOnce(fmt.Sprintf("batt%d", batteryWarnPct)) {
			notify.Send(notify.Options{
				Kind:  "input",
				Title: i18n.Tr("Server mode: battery low", "服务器模式：电量偏低"),
				Message: i18n.Tr(
					fmt.Sprintf("%d%% left on battery. Sleep is restored automatically at 20%%.", st.BatteryPct),
					fmt.Sprintf("电池还剩 %d%%。掉到 20%% 会自动恢复睡眠。", st.BatteryPct)),
			})
		}
	} else if st.Power == servermode.PowerAC {
		clearServerModeMarks()
	}
}

// lastServerModeSig is the signature of the state serverModeTick last read. serve's hub
// compares it after each slow tick and pushes an `awake` event when it moved, so a remote
// surface hears of a change without a second system read.
var lastServerModeSig atomic.Value // string

func serverModeSignature() string {
	s, _ := lastServerModeSig.Load().(string)
	return s
}

// serverModeSig is what a remote surface shows of server mode, and nothing that moves on
// its own: the heartbeat and `since` stay out, or every tick would be a change. Power and
// charge count only while server mode is, or may be, in force; on an ordinary machine a
// laptop charging would otherwise push an event every few minutes for a feature that is
// off.
func serverModeSig(st servermode.Status) string {
	sig := fmt.Sprintf("%s|%t|%t|%t|%t", st.State, st.SystemDisableSleep, st.OwnedByGtmux, st.Guard.Installed, st.Guard.Healthy)
	if st.LastExit != nil {
		sig += fmt.Sprintf("|exit:%s@%d", st.LastExit.Reason, st.LastExit.At)
	}
	if st.State != servermode.StateOff || st.SystemDisableSleep || st.OwnedByGtmux {
		sig += fmt.Sprintf("|%s|%d", st.Power, st.BatteryPct)
	}
	return sig
}

// One-shot markers so a warning fires once per episode rather than every tick. They
// live next to the rest of server mode's state and are cleared when the condition
// clears, which is what re-arms them.
func serverModeMarkPath(name string) string {
	return filepath.Join(servermode.StateDir(), "warned-"+name)
}

func markServerModeOnce(name string) bool {
	p := serverModeMarkPath(name)
	if _, err := os.Stat(p); err == nil {
		return false
	}
	_ = os.MkdirAll(servermode.StateDir(), 0o755)
	return os.WriteFile(p, []byte(time.Now().Format(time.RFC3339)), 0o644) == nil
}

func clearServerModeMarks() {
	matches, _ := filepath.Glob(filepath.Join(servermode.StateDir(), "warned-*"))
	for _, m := range matches {
		_ = os.Remove(m)
	}
}

// Exit notices. The guard records why it gave sleep back (last-exit.json, written as
// root) and leaves gtmux to tell the user. Nothing read it for that, so a session that
// ended on battery, on a stale heartbeat or after a restart ended silently on the Mac
// too (%12, 2026-10-06). The paired-phone half of the same requirement is not wired.
const (
	// exitNoticeCursor holds the time of the last exit already handled.
	exitNoticeCursor = "exit-announced"
	// localOffMarker holds when this Mac's `gtmux awake off` (the menu bar's too) last
	// ran: the stand-down that follows it is the user's own and is not announced.
	localOffMarker = "local-off"
	localOffWindow = 10 * time.Minute
	// With no cursor at all (gtmux updated while an old record sits there), an exit
	// older than this is history, not news. `awake on` sets the cursor, so after that
	// every exit is announced however late gtmux sees it.
	uncursoredExitAge = time.Hour
)

// sendExitNotice is the notification sink and loadLastExit the guard's record; tests
// replace both.
var (
	sendExitNotice = notify.Send
	loadLastExit   = servermode.LoadLastExit
)

func serverModeStatePath(name string) string {
	return filepath.Join(servermode.StateDir(), name)
}

func readUnixFile(name string) (int64, bool) {
	b, err := os.ReadFile(serverModeStatePath(name))
	if err != nil {
		return 0, false
	}
	v, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	return v, err == nil
}

func writeUnixFile(name string, v int64) {
	_ = os.MkdirAll(servermode.StateDir(), 0o755)
	_ = os.WriteFile(serverModeStatePath(name), []byte(strconv.FormatInt(v, 10)+"\n"), 0o644)
}

// markLocalServerModeOff records that the user turned server mode off at this Mac.
func markLocalServerModeOff(now time.Time) { writeUnixFile(localOffMarker, now.Unix()) }

func clearLocalServerModeOff() { _ = os.Remove(serverModeStatePath(localOffMarker)) }

// markServerModeExitsSeen sets the cursor when server mode is turned on: every exit
// after this one is news. An earlier local-off mark is over too: it explained an exit
// that has passed, and kept, it would hide a stand-down of this new session.
func markServerModeExitsSeen() {
	var at int64
	if e, ok := loadLastExit(); ok {
		at = e.At
	}
	writeUnixFile(exitNoticeCursor, at)
	clearLocalServerModeOff()
}

// exitNotice decides what to say about the guard's last exit and which cursor to keep.
// An empty title says nothing.
func exitNotice(e *servermode.Exit, cursor int64, haveCursor bool, localOffAt int64, now time.Time) (title, msg string, next int64) {
	if e == nil || (haveCursor && e.At <= cursor) {
		return "", "", cursor
	}
	next = e.At
	if !haveCursor && now.Unix()-e.At > int64(uncursoredExitAge/time.Second) {
		return "", "", next
	}
	if e.Reason == servermode.ReasonRevoked && localOffAt > 0 && e.At >= localOffAt &&
		e.At-localOffAt <= int64(localOffWindow/time.Second) {
		return "", "", next // the user's own `awake off` here
	}
	title = i18n.Tr("Server mode ended", "服务器模式已结束")
	switch e.Reason {
	case servermode.ReasonBatteryLow:
		msg = i18n.Tr("The battery reached 20%, so sleep was restored", "电量降到 20%，已恢复睡眠")
	case servermode.ReasonStaleHeartbeat:
		msg = i18n.Tr("gtmux stopped checking in, so sleep was restored", "gtmux 没有按时报到，已恢复睡眠")
	case servermode.ReasonBootReconcile:
		// The guard's evidence: after a restart and its grace, no server-mode record.
		msg = i18n.Tr("After a restart there was no server-mode record, so sleep was restored",
			"重启后找不到服务器模式的记录，已恢复睡眠")
	case servermode.ReasonRevoked:
		msg = i18n.Tr("A request to turn it off arrived, so sleep was restored", "收到了关闭请求，已恢复睡眠")
	default:
		msg = fmt.Sprintf(i18n.Tr("Sleep was restored (%s)", "已恢复睡眠（%s）"), e.Reason)
	}
	// Seen late (gtmux was down, which is how a heartbeat goes stale): say when.
	if now.Unix()-e.At > 5*60 {
		at := time.Unix(e.At, 0).Format("15:04")
		if time.Unix(e.At, 0).Format("2006-01-02") != now.Format("2006-01-02") {
			at = time.Unix(e.At, 0).Format("01-02 15:04")
		}
		msg += fmt.Sprintf(i18n.Tr(" at %s", "（%s）"), at)
	}
	return title, msg + i18n.Tr(".", "。"), next
}

// announceServerModeExit tells the user, once, about an exit the guard recorded.
func announceServerModeExit(e *servermode.Exit, now time.Time) {
	if e == nil {
		return
	}
	cursor, haveCursor := readUnixFile(exitNoticeCursor)
	localOff, _ := readUnixFile(localOffMarker)
	title, msg, next := exitNotice(e, cursor, haveCursor, localOff, now)
	if next == cursor && haveCursor {
		return
	}
	writeUnixFile(exitNoticeCursor, next)
	if localOff > 0 && e.At >= localOff {
		clearLocalServerModeOff() // spent on this exit
	}
	if title != "" {
		sendExitNotice(notify.Options{Kind: "done", Title: title, Message: msg})
	}
}
