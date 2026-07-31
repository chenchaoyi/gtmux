package servermode

import (
	"os/exec"
	"strconv"
	"strings"
)

// Paths the sensing layer reads. The plist is root:wheel but world-readable, so an
// unprivileged gtmux can answer "would this survive a reboot" with no helper.
//
// Note the path: third-party references (and an earlier draft of this design) give
// .../Preferences/SystemConfiguration/com.apple.PowerManagement.plist, which does
// NOT exist on macOS 26. Measured, not copied.
const (
	pmPlistPath = "/Library/Preferences/com.apple.PowerManagement.plist"
	guardPlist  = "/Library/LaunchDaemons/com.gtmux.sleepguard.plist"
	guardScript = "/Library/Application Support/gtmux/sleepguard.sh"
)

// SleepDisabled reports whether the kernel is currently refusing to sleep — the
// live, authoritative readback. See the package doc for why this is `ioreg` and
// emphatically not `pmset`.
func SleepDisabled() bool {
	out, err := exec.Command("ioreg", "-r", "-c", "IOPMrootDomain", "-d", "1", "-w0").Output()
	if err != nil {
		return false
	}
	return parseSleepDisabled(string(out))
}

// parseSleepDisabled finds `"SleepDisabled" = Yes` in ioreg output. Absent means
// off, which is also what a machine that never had this set looks like.
func parseSleepDisabled(out string) bool {
	for _, ln := range strings.Split(out, "\n") {
		if !strings.Contains(ln, "\"SleepDisabled\"") {
			continue
		}
		_, val, ok := strings.Cut(ln, "=")
		if !ok {
			continue
		}
		return strings.EqualFold(strings.TrimSpace(val), "Yes")
	}
	return false
}

// PersistedSleepDisabled reports the value stored in the power-management
// preferences — i.e. whether sleep would still be disabled after a reboot. The
// second return is false when the setting has never been written on this machine
// (no key at all), which is distinct from "written and turned off".
//
// This is NOT the current state: the file lags a write by a noticeable margin.
// Use SleepDisabled for "is it on now".
func PersistedSleepDisabled() (on bool, known bool) {
	out, err := exec.Command("plutil",
		"-extract", "SystemPowerSettings.SleepDisabled", "raw", "-o", "-", pmPlistPath).Output()
	if err != nil {
		return false, false // key absent → never set on this machine
	}
	return parsePersisted(string(out))
}

// parsePersisted reads plutil's raw output. Off is `false` with the key still
// present — a restored machine keeps the key — so presence is NOT the test.
func parsePersisted(out string) (on, known bool) {
	switch strings.TrimSpace(out) {
	case "true", "1":
		return true, true
	case "false", "0":
		return false, true
	default:
		return false, false
	}
}

// Power reports the current power source and remaining charge. hasBattery is false
// on a desktop, where the charge guardrails do not apply at all.
func Power() (src string, pct int, hasBattery bool) {
	out, err := exec.Command("pmset", "-g", "ps").Output()
	if err != nil {
		return PowerAC, 0, false
	}
	return parsePowerSource(string(out))
}

// parsePowerSource parses `pmset -g ps`, e.g.
//
//	Now drawing from 'AC Power'
//	 -InternalBattery-0 (id=7995491)	100%; charged; 0:00 remaining present: true
//
// A desktop prints the first line and no battery line.
func parsePowerSource(out string) (src string, pct int, hasBattery bool) {
	src = PowerAC
	for _, ln := range strings.Split(out, "\n") {
		if strings.Contains(ln, "Battery Power") {
			src = PowerBattery
		}
		if !strings.Contains(ln, "InternalBattery") {
			continue
		}
		hasBattery = true
		// The percentage is the first field ending in '%' after the id.
		for _, f := range strings.Fields(ln) {
			f = strings.TrimSuffix(f, ";")
			if !strings.HasSuffix(f, "%") {
				continue
			}
			if n, err := strconv.Atoi(strings.TrimSuffix(f, "%")); err == nil {
				pct = n
				break
			}
		}
	}
	return src, pct, hasBattery
}

// GuardStatus reports whether the de-escalation-only guard is installed and looks
// healthy. Both files must be present: a daemon with no script (or the reverse)
// cannot give sleep back, which is the one job it exists for.
func GuardStatus() Guard {
	g := Guard{Installed: fileExists(guardPlist) || fileExists(guardScript)}
	g.Healthy = fileExists(guardPlist) && fileExists(guardScript)
	return g
}

// WriteRefused reports whether a `pmset` invocation was refused for lack of
// privilege DESPITE exiting 0 — measured behaviour on macOS 26.5.2: it prints
// "must be run as root" to output and still returns status 0.
//
// Every privileged write must be checked with this AND confirmed by reading the
// state back. Trusting the exit code makes gtmux claim it restored sleep when it
// did not, which is precisely the failure this feature must never produce.
func WriteRefused(out string) bool {
	return strings.Contains(out, "must be run as root")
}
