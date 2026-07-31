// Server mode's live half: the heartbeat that tells the guard gtmux is still here,
// plus the two things that must reach a user who is NOT at the machine — a charge
// warning before the floor, and the detection of a session that silently died.
package app

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/notify"
	"github.com/chenchaoyi/gtmux/internal/servermode"
)

// batteryWarnPct is the charge at which the user is told, while there is still time
// to do something about it. The floor (where sleep is restored) is lower and lives
// in the guard, which enforces it whether or not gtmux is running.
const batteryWarnPct = servermode.EnableThresholdPct

// serverModeTick runs on the serve slow tick (~30s) — the same single-writer cadence
// the resource warnings use, so there is exactly one writer and no race.
//
// It is a no-op on machines not running server mode, which is almost all of them.
func serverModeTick() {
	st := servermode.Current()

	// 1. Liveness. This is what stands between an abandoned machine and a battery
	//    drained flat: stop refreshing and the guard restores sleep.
	if st.OwnedByGtmux && st.SystemDisableSleep {
		_ = servermode.Heartbeat()
	}

	// 2. A session that died underneath us. The kernel says sleep is enabled while
	//    our stamp says server mode is on — so the closed-lid session the user is
	//    relying on is already over, and they are the last to know.
	if st.State == servermode.StateLapsed {
		if markServerModeOnce("lapsed") {
			notify.Send(notify.Options{
				Kind:  "done",
				Title: i18n.Tr("Server mode stopped", "服务器模式已停止"),
				Message: i18n.Tr("The sleep setting is no longer in force — closing the lid will sleep this Mac.",
					"睡眠设置已不再生效 —— 现在合盖会让这台 Mac 休眠。"),
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
