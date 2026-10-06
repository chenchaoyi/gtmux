package app

import (
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/notify"
	"github.com/chenchaoyi/gtmux/internal/servermode"
)

// The guard records why it gave sleep back; nothing told the user (%12, 2026-10-06).
// exitNotice decides, once per exit, whether and what to say on this Mac.
func TestExitNoticeDecides(t *testing.T) {
	t.Setenv("GTMUX_LANG", "en")
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.Local)
	at := func(ago time.Duration) int64 { return now.Add(-ago).Unix() }
	exit := func(reason string, ago time.Duration) *servermode.Exit {
		return &servermode.Exit{At: at(ago), Reason: reason}
	}
	for _, tc := range []struct {
		name       string
		e          *servermode.Exit
		cursor     int64
		haveCursor bool
		localOff   int64
		want       string // a fragment of the message; "" = nothing said
	}{
		{"no record", nil, 0, true, 0, ""},
		{"battery floor", exit(servermode.ReasonBatteryLow, time.Minute), 0, true, 0, "battery reached 20%"},
		{"already said", exit(servermode.ReasonBatteryLow, time.Minute), at(time.Minute), true, 0, ""},
		{"gtmux gone for hours, seen late", exit(servermode.ReasonStaleHeartbeat, 3*time.Hour), 0, true, 0, "stopped checking in, so sleep was restored at 09:00."},
		{"after a restart", exit(servermode.ReasonBootReconcile, time.Minute), 0, true, 0, "no server-mode record"},
		{"the user's own awake off here", exit(servermode.ReasonRevoked, time.Minute), 0, true, at(2 * time.Minute), ""},
		{"a stand-down from elsewhere", exit(servermode.ReasonRevoked, time.Minute), 0, true, 0, "request to turn it off"},
		{"a local off long before", exit(servermode.ReasonRevoked, time.Minute), 0, true, at(time.Hour), "request to turn it off"},
		{"an unknown reason", exit("thermal", time.Minute), 0, true, 0, "(thermal)"},
		{"no cursor, recent", exit(servermode.ReasonBatteryLow, time.Minute), 0, false, 0, "battery reached 20%"},
		{"no cursor, old record from before the update", exit(servermode.ReasonBatteryLow, 2*time.Hour), 0, false, 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			title, msg, next := exitNotice(tc.e, tc.cursor, tc.haveCursor, tc.localOff, now)
			if tc.want == "" {
				if title != "" {
					t.Fatalf("said %q / %q, want nothing", title, msg)
				}
			} else if title != "Server mode ended" || !strings.Contains(msg, tc.want) {
				t.Fatalf("said %q / %q, want a message with %q", title, msg, tc.want)
			}
			if tc.e != nil && next < tc.e.At {
				t.Fatalf("cursor %d left behind the exit at %d", next, tc.e.At)
			}
		})
	}
}

// Once per exit, through the files the tick keeps: a second tick on the same record says
// nothing, and the user's own `awake off` is spent by the exit it caused.
func TestAnnounceServerModeExitOnce(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GTMUX_LANG", "en")
	var sent []notify.Options
	saved := sendExitNotice
	sendExitNotice = func(o notify.Options) { sent = append(sent, o) }
	t.Cleanup(func() { sendExitNotice = saved })

	now := time.Now()
	writeUnixFile(exitNoticeCursor, 0) // as `awake on` leaves it
	battery := &servermode.Exit{At: now.Add(-time.Minute).Unix(), Reason: servermode.ReasonBatteryLow}
	announceServerModeExit(battery, now)
	announceServerModeExit(battery, now.Add(30*time.Second))
	if len(sent) != 1 || sent[0].Kind != "done" {
		t.Fatalf("sent %+v, want one notice", sent)
	}

	markLocalServerModeOff(now)
	off := &servermode.Exit{At: now.Add(5 * time.Second).Unix(), Reason: servermode.ReasonRevoked}
	announceServerModeExit(off, now.Add(10*time.Second))
	if len(sent) != 1 {
		t.Fatalf("the user's own off was announced: %+v", sent[1:])
	}
	if _, ok := readUnixFile(localOffMarker); ok {
		t.Error("the local-off mark should be spent on the exit it explains")
	}
	later := &servermode.Exit{At: now.Add(time.Minute).Unix(), Reason: servermode.ReasonRevoked}
	announceServerModeExit(later, now.Add(70*time.Second))
	if len(sent) != 2 {
		t.Fatalf("a later stand-down from elsewhere was not announced: %+v", sent)
	}
}

// standIn replaces the machine-facing calls of `awake off` and the guard's record.
func standIn(t *testing.T, on, known, guard bool, disable error) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GTMUX_LANG", "en")
	r, g, d, l := smReadSleep, smGuardInstalled, smDisable, loadLastExit
	smReadSleep = func() (bool, bool) { return on, known }
	smGuardInstalled = func() bool { return guard }
	smDisable = func() error { return disable }
	loadLastExit = func() (servermode.Exit, bool) { return servermode.Exit{}, false }
	t.Cleanup(func() { smReadSleep, smGuardInstalled, smDisable, loadLastExit = r, g, d, l })
}

// The local-off mark goes down only when a stand-down really starts (%12's review of
// 0caa17f1): left behind by "already off" or by a failed off, it would hide a stand-down
// from elsewhere for ten minutes.
func TestLocalOffMarksOnlyARealStandDown(t *testing.T) {
	for _, tc := range []struct {
		name      string
		on, known bool
		guard     bool
		disable   error
		wantMark  bool
	}{
		{"already off: nothing started", false, true, false, nil, false},
		{"the off failed: nothing started", true, true, false, servermode.ErrNotVerified, false},
		{"a stand-down started", true, true, true, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			standIn(t, tc.on, tc.known, tc.guard, tc.disable)
			_ = serverModeOff()
			if _, marked := readUnixFile(localOffMarker); marked != tc.wantMark {
				t.Fatalf("local-off mark present = %v, want %v", marked, tc.wantMark)
			}
		})
	}
}

// Turning it on again starts a new session: an old local-off mark is over.
func TestTurningOnClearsAnOldLocalOff(t *testing.T) {
	standIn(t, false, true, false, nil)
	markLocalServerModeOff(time.Now())
	markServerModeExitsSeen()
	if _, marked := readUnixFile(localOffMarker); marked {
		t.Fatal("an old local-off mark survived turning server mode on")
	}
	if c, ok := readUnixFile(exitNoticeCursor); !ok || c != 0 {
		t.Fatalf("cursor = %d, %v; want 0 with no exit on record", c, ok)
	}
}
