package servermode

import (
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// A kernel reading that cannot be taken is unknown, never "sleep is back" (%12,
// 2026-10-06). Every test here answers for the kernel, the authorization dialog and the
// root-owned guard files: nothing reads or changes the real machine.

// A reading: one of the captured ioreg fixtures, or a failure.
type reading struct {
	out string
	err error
}

var (
	readOn      = reading{out: ioregOn}
	readOff     = reading{out: ioregOff}
	readFails   = reading{err: errors.New("ioreg: exit status 1")}
	readNoNode  = reading{out: "\n"} // ran, but printed no IOPMrootDomain
	errNotAsked = errors.New("the authorization dialog must not be raised here")
)

// rig stands in for the machine. kernel answers each read in turn and repeats its last
// answer; privileged is what the authorization dialog and the script return, and is
// called at most once unless a test says otherwise.
type rig struct {
	mu         sync.Mutex
	kernel     []reading
	reads      int
	privileged func(script string) (string, error)
	asked      []string
}

func newRig(t *testing.T, guard bool, kernel ...reading) *rig {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	r := &rig{kernel: kernel, privileged: func(string) (string, error) {
		t.Error(errNotAsked)
		return "", errNotAsked
	}}
	saved := []any{ioregRoot, runPrivileged, guardInstalled, guardRestoreWait, writeRestoreWait}
	ioregRoot = func() ([]byte, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		k := r.kernel[min(r.reads, len(r.kernel)-1)]
		r.reads++
		return []byte(k.out), k.err
	}
	runPrivileged = func(script, _ string) (string, error) {
		r.mu.Lock()
		r.asked = append(r.asked, script)
		r.mu.Unlock()
		return r.privileged(script)
	}
	guardInstalled = func() bool { return guard }
	guardRestoreWait, writeRestoreWait = 600*time.Millisecond, 600*time.Millisecond
	t.Cleanup(func() {
		ioregRoot = saved[0].(func() ([]byte, error))
		runPrivileged = saved[1].(func(string, string) (string, error))
		guardInstalled = saved[2].(func() bool)
		guardRestoreWait, writeRestoreWait = saved[3].(time.Duration), saved[4].(time.Duration)
	})
	if err := os.MkdirAll(StateDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeStamp(TierClamshell); err != nil {
		t.Fatal(err)
	}
	return r
}

func (r *rig) stampKept() bool  { return fileExists(StatePath()) }
func (r *rig) markerKept() bool { return fileExists(RevokePath()) }

func TestReadSleepDisabledHasAThirdState(t *testing.T) {
	for _, tc := range []struct {
		name      string
		k         reading
		on, known bool
	}{
		{"on", readOn, true, true},
		{"off", readOff, false, true},
		{"never set: no key, node present", reading{out: ioregAbsent}, false, true},
		{"ioreg fails", readFails, false, false},
		{"ioreg prints no power node", readNoNode, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			newRig(t, true, tc.k)
			on, known := ReadSleepDisabled()
			if on != tc.on || known != tc.known {
				t.Fatalf("ReadSleepDisabled() = (%v, %v), want (%v, %v)", on, known, tc.on, tc.known)
			}
			if got := readbackAvailable(); got != tc.known {
				t.Fatalf("readbackAvailable() = %v, want %v: the platform gate and the reading must agree", got, tc.known)
			}
		})
	}
}

// With the guard installed, turning off writes the marker and waits for the kernel. A
// kernel that cannot be read must leave the marker for the guard and keep our record:
// clearing them on a failed read was the bug.
func TestDisableKeepsTheMarkerWhileTheKernelCannotBeRead(t *testing.T) {
	for _, k := range []reading{readFails, readNoNode} {
		r := newRig(t, true, k)
		if err := Disable(); err != nil {
			t.Fatalf("Disable: %v (a stand-down the guard has yet to act on is not a failure)", err)
		}
		if !r.markerKept() {
			t.Fatal("the stand-down marker was removed on an unreadable kernel; the guard has nothing to act on")
		}
		if !r.stampKept() {
			t.Fatal("the ownership record was cleared on an unreadable kernel")
		}
		if len(r.asked) != 0 {
			t.Fatalf("with the guard installed, turning off asked for a password: %q", r.asked)
		}
	}
}

func TestDisableClearsOnlyOnceTheKernelSaysSleepIsBack(t *testing.T) {
	r := newRig(t, true, readOn, readFails, readOn, readOff)
	if err := Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if r.markerKept() || r.stampKept() {
		t.Fatalf("a confirmed reading should clear both: marker=%v stamp=%v", r.markerKept(), r.stampKept())
	}
}

// With no guard, turning off is a privileged write, and it is done only when the kernel
// confirms it. The write used to be reported as done whenever the dialog succeeded.
func TestDisableWithoutTheGuardNeedsTheKernelToConfirm(t *testing.T) {
	for _, tc := range []struct {
		name        string
		kernel      []reading
		out         string
		err         error
		want        error
		wantCleared bool
	}{
		{"confirmed", []reading{readOff}, "", nil, nil, true},
		{"the kernel still reads disabled", []reading{readOn}, "", nil, ErrNotVerified, false},
		{"the kernel cannot be read", []reading{readFails}, "", nil, ErrNotVerified, false},
		{"pmset refused, exit 0", []reading{readOff}, "pmset: must be run as root\n", nil, ErrPrivilegedFailed, false},
		{"authorization declined", []reading{readOn}, "", ErrNoAuth, ErrNoAuth, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newRig(t, false, tc.kernel...)
			r.privileged = func(string) (string, error) { return tc.out, tc.err }
			err := Disable()
			if tc.want == nil && err != nil {
				t.Fatalf("Disable: %v, want success", err)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("Disable = %v, want %v", err, tc.want)
			}
			if len(r.asked) != 1 || !strings.Contains(r.asked[0], "pmset -a disablesleep 0") {
				t.Fatalf("expected one privileged restore, got %q", r.asked)
			}
			if cleared := !r.stampKept() && !r.markerKept(); cleared != tc.wantCleared {
				t.Fatalf("record cleared = %v, want %v (stamp=%v marker=%v)",
					cleared, tc.wantCleared, r.stampKept(), r.markerKept())
			}
		})
	}
}

// Lapsed means the kernel says sleep is back while our record says on. A failed read
// says nothing, so it is unknown; concluding a lapse cleared the record and told the
// user server mode had stopped.
func TestStateForAnUnreadableKernelIsUnknownNotLapsed(t *testing.T) {
	for _, tc := range []struct {
		live, known, rec bool
		want             string
	}{
		{false, false, true, StateUnknown},
		{false, false, false, StateUnknown},
		{true, true, true, StateOn},
		{true, true, false, StateOn},
		{false, true, true, StateLapsed},
		{false, true, false, StateOff},
	} {
		if got := stateFor(tc.live, tc.known, tc.rec); got != tc.want {
			t.Errorf("stateFor(live=%v, known=%v, rec=%v) = %q, want %q", tc.live, tc.known, tc.rec, got, tc.want)
		}
	}
}
