package app

import (
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/servermode"
)

// The sleep-disable setting is invisible by default (it survives reboot and the
// `pmset` reporting commands never mention it), so doctor is the only place a user
// would ever learn their Mac cannot sleep. These pin each branch — especially the
// two that must NOT act: an unstamped setting is somebody else's deliberate choice.
func TestSleepChecksFor(t *testing.T) {
	const (
		on  = true
		off = false
	)
	for _, tc := range []struct {
		name      string
		st        servermode.Status
		stale     bool
		wantRows  int
		wantState int
		// wantManual: the row must show the manual undo command, because gtmux
		// will not run it for the user.
		wantManual bool
	}{
		{
			name:     "never used here — stay silent",
			st:       servermode.Status{State: servermode.StateOff},
			stale:    true,
			wantRows: 0,
		},
		{
			name: "ours and in use — healthy, not a finding",
			st: servermode.Status{State: servermode.StateOn, SystemDisableSleep: on,
				OwnedByGtmux: true},
			stale:     false,
			wantRows:  1,
			wantState: stOK,
		},
		{
			name: "ours but abandoned — blocking: the Mac cannot sleep",
			st: servermode.Status{State: servermode.StateOn, SystemDisableSleep: on,
				OwnedByGtmux: true},
			stale:      true,
			wantRows:   1,
			wantState:  stMiss,
			wantManual: true,
		},
		{
			name: "someone else's setting — report only, never change it",
			st: servermode.Status{State: servermode.StateOn, SystemDisableSleep: on,
				OwnedByGtmux: false},
			stale:      true,
			wantRows:   1,
			wantState:  stRec,
			wantManual: true,
		},
		{
			name: "lapsed — we think it's on, the kernel says otherwise",
			st: servermode.Status{State: servermode.StateLapsed, SystemDisableSleep: off,
				OwnedByGtmux: true},
			stale:     false,
			wantRows:  1,
			wantState: stRec,
		},
		{
			name: "sleeps now, but would be disabled again after a reboot",
			st: servermode.Status{State: servermode.StateOff, SystemDisableSleep: off,
				PersistedDisableSleep: on},
			stale:      true,
			wantRows:   1,
			wantState:  stRec,
			wantManual: true,
		},
	} {
		rows := sleepChecksFor(tc.st, tc.stale)
		if len(rows) != tc.wantRows {
			t.Errorf("%s: got %d rows, want %d", tc.name, len(rows), tc.wantRows)
			continue
		}
		if tc.wantRows == 0 {
			continue
		}
		if rows[0].status != tc.wantState {
			t.Errorf("%s: status = %d, want %d", tc.name, rows[0].status, tc.wantState)
		}
		hasManual := strings.Contains(rows[0].note, "pmset -a disablesleep 0")
		if hasManual != tc.wantManual {
			t.Errorf("%s: manual-command shown = %v, want %v (note: %q)",
				tc.name, hasManual, tc.wantManual, rows[0].note)
		}
	}
}

// gtmux reverts only what gtmux owns — the same report-only discipline `gtmux reap`
// applies to an unclean worktree. A regression here would mean silently undoing a
// setting a user (or their MDM) chose on purpose.
func TestUnownedSleepSettingIsNeverActedOn(t *testing.T) {
	rows := sleepChecksFor(servermode.Status{
		State: servermode.StateOn, SystemDisableSleep: true, OwnedByGtmux: false,
	}, true)
	if len(rows) != 1 {
		t.Fatalf("expected one row, got %d", len(rows))
	}
	if rows[0].status == stMiss {
		t.Error("an unowned setting must not be reported as blocking — it is not ours to fix")
	}
	if !strings.Contains(rows[0].note, "pmset") {
		t.Error("report-only findings must show the manual command instead of acting")
	}
}
