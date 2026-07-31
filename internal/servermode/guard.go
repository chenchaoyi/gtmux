package servermode

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The de-escalation-only guard. This is the load-bearing safety component of server
// mode: a small root-owned LaunchDaemon whose ONLY power is to give sleep back.
//
// The asymmetry is the whole design. Disabling sleep requires a human at the machine
// answering an authorization prompt. Restoring it must work with nobody there at all
// — because the moments that most need sleep back (charge nearly empty, gtmux
// crashed, gtmux uninstalled, the machine rebooted into a login screen) are exactly
// the moments when no one can type a password.
//
// Therefore, by construction:
//   - the script contains NO code path that disables sleep (asserted by test);
//   - it removes itself once it has acted, so it never outlives its purpose;
//   - its removal is triggerable by an UNPRIVILEGED marker, so `off` and uninstall
//     never need a second password;
//   - it only ever runs SIP-protected binaries, and lives in a root-owned directory,
//     so a user-writable path can never become root execution.
const (
	GuardLabel      = "com.gtmux.sleepguard"
	GuardDir        = "/Library/Application Support/gtmux"
	GuardScriptPath = GuardDir + "/sleepguard.sh"
	GuardPlistPath  = "/Library/LaunchDaemons/" + GuardLabel + ".plist"

	// guardInterval is how often the guard re-checks. Short enough that an
	// abandoned machine returns to normal quickly; long enough to be free.
	guardInterval = 30
	// staleSeconds: no heartbeat for this long means gtmux is gone.
	staleSeconds = 120
	// bootGraceSeconds: after a reboot, give launchd time to bring `gtmux serve`
	// back before concluding the state was abandoned. Without this, every reboot of
	// a remote machine would kill server mode at exactly the moment the user is
	// least able to fix it.
	bootGraceSeconds = 300
	// batteryFloorPct: restore sleep at this charge. Running on battery is a
	// supported use case (carrying a closed laptop between rooms); running it to
	// empty is not.
	batteryFloorPct = 20
)

// RevokePath is the unprivileged marker that asks the guard to stand down. Writing
// it needs no privilege, which is what makes "turn it off" free.
func RevokePath() string { return filepath.Join(StateDir(), "revoke") }

// GuardScript renders the guard. Pure, so its exact content is pinned by a golden
// test — this is a security artifact and must never drift silently.
func GuardScript(statePath, revokePath string) string {
	return fmt.Sprintf(`#!/bin/sh
# gtmux sleep guard — DE-ESCALATION ONLY.
#
# Installed by `+"`gtmux server-mode on`"+` in the same administrator authorization that
# disabled sleep. Its only power is to give sleep back and then delete itself. There
# is deliberately no code path here that disables sleep: the worst thing this file
# can be tricked into doing is letting the Mac sleep.
#
# Generated — do not edit. See openspec/changes/server-mode.
set -u

PMSET=/usr/bin/pmset
STATE='%s'
REVOKE='%s'
EXITLOG='%s/last-exit.json'
SELF_PLIST='%s'
SELF_SCRIPT='%s'

now=$(/bin/date +%%s)

# restore <reason>: give sleep back, record why, then remove ourselves. Order
# matters — sleep is restored FIRST, so a failure in the bookkeeping can never
# leave a Mac unable to sleep.
restore() {
  "$PMSET" -a disablesleep 0
  /bin/mkdir -p "$(/usr/bin/dirname "$EXITLOG")" 2>/dev/null
  /bin/echo "{\"at\":$now,\"reason\":\"$1\"}" > "$EXITLOG" 2>/dev/null
  /bin/rm -f "$SELF_SCRIPT" "$SELF_PLIST"
  /bin/launchctl bootout system/%s 2>/dev/null
  exit 0
}

# 1. Explicit stand-down. An unprivileged marker — this is why turning server mode
#    off, or uninstalling gtmux, never asks for a password.
[ -f "$REVOKE" ] && restore revoked

# 2. Charge floor. Being on battery is fine and expected; running it flat is not.
batt=$("$PMSET" -g ps 2>/dev/null | /usr/bin/grep -o '[0-9]*%%' | /usr/bin/head -1 | /usr/bin/tr -d '%%')
onbatt=$("$PMSET" -g ps 2>/dev/null | /usr/bin/grep -c "Battery Power")
if [ "$onbatt" -gt 0 ] && [ -n "$batt" ] && [ "$batt" -le %d ]; then
  restore battery-low
fi

# 3. Liveness. No gtmux keeping the heartbeat means nobody is serving anything, so
#    there is nothing left to stay awake for.
#    After a reboot, allow a grace window for launchd to start gtmux again —
#    otherwise a remote machine's restart would end the session it is meant to keep.
boot=$(/usr/sbin/sysctl -n kern.boottime 2>/dev/null | /usr/bin/sed 's/^{ sec = \([0-9]*\).*/\1/')
uptime_s=$((now - ${boot:-0}))
if [ ! -f "$STATE" ]; then
  [ "$uptime_s" -gt %d ] && restore boot-reconcile
  exit 0
fi
beat=$(/usr/bin/stat -f %%m "$STATE" 2>/dev/null || /bin/echo 0)
age=$((now - beat))
if [ "$age" -gt %d ]; then
  [ "$uptime_s" -gt %d ] && restore stale-heartbeat
fi
exit 0
`, statePath, revokePath, GuardDir, GuardPlistPath, GuardScriptPath, GuardLabel,
		batteryFloorPct, bootGraceSeconds, staleSeconds, bootGraceSeconds)
}

// GuardPlist renders the LaunchDaemon. RunAtLoad covers the reboot path;
// StartInterval covers everything else.
func GuardPlist() string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key><string>%s</string>
	<key>ProgramArguments</key>
	<array><string>/bin/sh</string><string>%s</string></array>
	<key>RunAtLoad</key><true/>
	<key>StartInterval</key><integer>%d</integer>
</dict>
</plist>
`, GuardLabel, GuardScriptPath, guardInterval)
}

// GuardInstalled reports whether both halves are present. One without the other
// cannot restore sleep, which is the only job it has.
func GuardInstalled() bool {
	return fileExists(GuardPlistPath) && fileExists(GuardScriptPath)
}

// Revoke asks the guard to stand down. Unprivileged by design.
func Revoke() error {
	if err := os.MkdirAll(StateDir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(RevokePath(), []byte("1\n"), 0o644)
}

// ClearRevoke removes a stale stand-down marker so a later enable is not undone by
// it a second after it lands.
func ClearRevoke() { _ = os.Remove(RevokePath()) }

// GuardCanOnlyRestore is the invariant, checked as data rather than trusted as
// prose: the rendered script must contain no instruction that disables sleep.
func GuardCanOnlyRestore(script string) bool {
	for _, ln := range strings.Split(script, "\n") {
		code, _, _ := strings.Cut(ln, "#") // ignore comments
		if strings.Contains(code, "disablesleep 1") {
			return false
		}
	}
	return true
}
