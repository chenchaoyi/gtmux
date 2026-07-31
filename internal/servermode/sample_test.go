package servermode

import (
	"runtime"
	"testing"
	"time"
)

// Fixtures are REAL output captured on the dev Mac (M4 Pro, macOS 26.5.2) while
// verifying this change — not invented. Each case below corresponds to a bug that
// actually happened during that verification.

const ioregOn = `
    +-o AppleARMPE  <class IOPMrootDomain, id 0x100000278, registered, matched>
        "IOPMSystemSleepType" = 3
        "SleepDisabled" = Yes
        "IOSleepSupported" = Yes
`

const ioregOff = `
    +-o AppleARMPE  <class IOPMrootDomain, id 0x100000278, registered, matched>
        "IOPMSystemSleepType" = 3
        "SleepDisabled" = No
        "IOSleepSupported" = Yes
`

// A machine that has never had this set: the key simply is not there.
const ioregAbsent = `
    +-o AppleARMPE  <class IOPMrootDomain, id 0x100000278, registered, matched>
        "IOSleepSupported" = Yes
`

func TestParseSleepDisabled(t *testing.T) {
	for _, tc := range []struct {
		name string
		out  string
		want bool
	}{
		{"on", ioregOn, true},
		{"off", ioregOff, false},
		{"key absent means off", ioregAbsent, false},
		{"empty output means off", "", false},
	} {
		if got := parseSleepDisabled(tc.out); got != tc.want {
			t.Errorf("%s: parseSleepDisabled = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// `pmset -g` never prints `disablesleep`, in EITHER state. This test exists because
// reading state from pmset made the verification harness report "sleep restored,
// the Mac will sleep normally" on a Mac that could not sleep — the worst failure
// this feature has. If anyone ever points the readback at pmset output again, this
// pins what they would get: a confident, permanent "off".
func TestPmsetOutputCannotAnswerTheQuestion(t *testing.T) {
	const pmsetG = `System-wide power settings:
Currently in use:
 standby              1
 hibernatefile        /var/vm/sleepimage
 powernap             1
 sleep                0 (sleep prevented by caffeinate, powerd)
 displaysleep         10
 tcpkeepalive         1
`
	if parseSleepDisabled(pmsetG) {
		t.Fatal("fixture sanity: pmset output has no SleepDisabled line")
	}
	// The point: this "false" is indistinguishable from a genuinely-off machine,
	// which is exactly why pmset must never be the source. Sensing uses ioreg.
}

func TestParsePersisted(t *testing.T) {
	for _, tc := range []struct {
		name      string
		out       string
		wantOn    bool
		wantKnown bool
	}{
		{"true", "true\n", true, true},
		// A RESTORED machine keeps the key with value false — so "key present" is
		// not the test, the value is.
		{"false after a restore", "false\n", false, true},
		{"numeric true", "1", true, true},
		{"numeric false", "0", false, true},
		// plutil errors when the key was never written (a machine that never had
		// server mode on). Distinct from "written and off".
		{"never set", "", false, false},
	} {
		on, known := parsePersisted(tc.out)
		if on != tc.wantOn || known != tc.wantKnown {
			t.Errorf("%s: parsePersisted = (%v,%v), want (%v,%v)",
				tc.name, on, known, tc.wantOn, tc.wantKnown)
		}
	}
}

func TestParsePowerSource(t *testing.T) {
	const onAC = `Now drawing from 'AC Power'
 -InternalBattery-0 (id=7995491)	100%; charged; 0:00 remaining present: true
`
	const onBattery = `Now drawing from 'Battery Power'
 -InternalBattery-0 (id=7995491)	95%; discharging; 3:20 remaining present: true
`
	// A desktop (Mac mini/Studio): power line only, no battery row. The charge
	// guardrails must not apply there at all.
	const desktop = "Now drawing from 'AC Power'\n"

	for _, tc := range []struct {
		name    string
		out     string
		wantSrc string
		wantPct int
		wantBat bool
	}{
		{"ac with battery", onAC, PowerAC, 100, true},
		{"on battery", onBattery, PowerBattery, 95, true},
		{"desktop has no battery", desktop, PowerAC, 0, false},
		{"unreadable falls back to ac", "", PowerAC, 0, false},
	} {
		src, pct, bat := parsePowerSource(tc.out)
		if src != tc.wantSrc || pct != tc.wantPct || bat != tc.wantBat {
			t.Errorf("%s: parsePowerSource = (%q,%d,%v), want (%q,%d,%v)",
				tc.name, src, pct, bat, tc.wantSrc, tc.wantPct, tc.wantBat)
		}
	}
}

// `pmset` exits 0 when it refuses for lack of privilege, printing the error to
// output. A caller that trusts the exit code will believe it disabled (or restored)
// sleep when it did nothing. Measured on macOS 26.5.2.
func TestWriteRefusedDespiteExitZero(t *testing.T) {
	for _, tc := range []struct {
		name string
		out  string
		want bool
	}{
		{"refused as non-root", "'/usr/bin/pmset' must be run as root...\n", true},
		{"refused, bare name", "'pmset' must be run as root...", true},
		{"success is silent", "", false},
	} {
		if got := WriteRefused(tc.out); got != tc.want {
			t.Errorf("%s: WriteRefused = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestRecordStale(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	fresh := Record{HeartbeatAt: now.Add(-10 * time.Second).Unix()}
	if fresh.Stale(now) {
		t.Error("a 10s-old heartbeat is fresh")
	}
	old := Record{HeartbeatAt: now.Add(-5 * time.Minute).Unix()}
	if !old.Stale(now) {
		t.Error("a 5min-old heartbeat is stale")
	}
	// A record with no heartbeat at all cannot vouch for a running gtmux.
	if !(Record{}).Stale(now) {
		t.Error("a record with no heartbeat must count as stale")
	}
}

func TestOSMajor(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int
	}{
		{"26.5.2", 26},
		{"15.4", 15},
		{"26", 26},
		{"", -1}, // sw_vers unavailable → unverified, not unsupported
		{"weird", -1},
	} {
		if got := osMajor(tc.in); got != tc.want {
			t.Errorf("osMajor(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// The platform gate has to be right in BOTH directions, and CI gives us the second
// one for free: the Go job runs on Linux, where the mechanism genuinely does not
// exist. So this asserts the refusal there and the acceptance on macOS — one test,
// both halves, and the CI run is real evidence that the gate refuses rather than
// limping on somewhere unsupported.
func TestSupportedGateBothDirections(t *testing.T) {
	s := Supported()
	if runtime.GOOS != "darwin" {
		if s.OK {
			t.Fatal("the gate must refuse a non-macOS host — the mechanism is macOS-only")
		}
		if s.Reason != ReasonNotMacOS {
			t.Errorf("reason = %q, want %q so the user is told WHY", s.Reason, ReasonNotMacOS)
		}
		return
	}
	if !s.OK {
		t.Fatalf("preflight refuses the machine this feature was verified on: reason=%q os=%q",
			s.Reason, s.OSVersion)
	}
	if s.OSVersion == "" {
		t.Error("the OS version should be reported so an unverified one can be named")
	}
}
