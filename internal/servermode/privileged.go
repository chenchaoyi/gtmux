package servermode

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// The privileged half. Everything here is written so that the FAILURE modes are
// safe: a refused authorization changes nothing, a partial install is undone, and
// nothing is ever reported as on until the kernel has been asked and agreed.

// EnableThresholdPct is the charge below which enabling is refused: a session that
// would end almost immediately is not worth an authorization prompt.
const EnableThresholdPct = 30

var (
	ErrUnsupported  = errors.New("server mode is not supported on this platform")
	ErrLowBattery   = errors.New("battery too low to start server mode")
	ErrNotVerified  = errors.New("the sleep setting did not take effect")
	ErrGuardMissing = errors.New("the de-escalation guard could not be installed")
	ErrNoAuth       = errors.New("administrator authorization was declined or unavailable")
	// ErrPrivilegedFailed: the user DID authorize, but the privileged script failed.
	// A separate error because the remedy is completely different.
	ErrPrivilegedFailed = errors.New("the privileged step failed after you authorized it")
)

// isUserCancelled recognises the dialog being dismissed. osascript reports it as
// error -128 ("User canceled"); a headless session (no GUI to show the dialog) also
// lands here, which is the correct outcome — enabling needs a human at the machine.
func isUserCancelled(out string) bool {
	return strings.Contains(out, "-128") ||
		strings.Contains(strings.ToLower(out), "user canceled") ||
		strings.Contains(strings.ToLower(out), "user cancelled")
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	if s == "" {
		return "no output"
	}
	return s
}

// runPrivileged executes a shell script as root via one macOS authorization prompt.
//
// The script is passed base64-encoded rather than interpolated: AppleScript string
// escaping is a well-known source of injection and of silently-mangled commands, and
// base64 has no characters that need escaping at all.
//
// It returns the combined output. Note that output — not the exit status — is what
// callers must judge: `pmset` exits 0 even when it refuses for lack of privilege.
func runPrivileged(script, prompt string) (string, error) {
	enc := base64.StdEncoding.EncodeToString([]byte(script))
	osa := fmt.Sprintf(
		`do shell script "echo %s | /usr/bin/base64 -d | /bin/sh" with prompt "%s" with administrator privileges`,
		enc, strings.ReplaceAll(prompt, `"`, `'`))
	out, err := exec.Command("osascript", "-e", osa).CombinedOutput()
	if err == nil {
		return string(out), nil
	}
	// Distinguish "the user said no" from "the script blew up". Reporting a failed
	// install as a declined authorization sends the user looking in exactly the wrong
	// place — it cost a debugging session to learn that.
	if isUserCancelled(string(out)) {
		return string(out), fmt.Errorf("%w: %v", ErrNoAuth, err)
	}
	return string(out), fmt.Errorf("%w: %s", ErrPrivilegedFailed, firstLine(string(out)))
}

// Enable turns on the clamshell tier: it disables sleep AND installs the guard in
// one authorization, then VERIFIES the effect before claiming success.
//
// The verification is not defensive padding. `pmset` reports success when it has
// done nothing, and the setting is invisible to that tool's own reporting, so an
// unverified enable is how gtmux would end up telling someone their Mac is being
// kept awake when it is not — and later that sleep was restored when it was not.
func Enable() error {
	sup := Supported()
	if !sup.OK {
		return fmt.Errorf("%w (%s)", ErrUnsupported, sup.Reason)
	}
	if src, pct, hasBattery := Power(); hasBattery && src == PowerBattery && pct < EnableThresholdPct {
		return fmt.Errorf("%w: %d%% left, floor is %d%%", ErrLowBattery, pct, EnableThresholdPct)
	}

	// A stale stand-down marker would make the guard undo this a moment after it
	// lands, which would look like "it just doesn't work".
	ClearRevoke()
	if err := os.MkdirAll(StateDir(), 0o755); err != nil {
		return err
	}

	install := installScript(GuardScript(StatePath(), RevokePath()), GuardPlist())
	if _, err := runPrivileged(install,
		"gtmux needs your administrator password to keep this Mac awake with the lid closed."); err != nil {
		return err
	}

	// Ask the kernel, not the command. Give the write a moment to land first.
	if !awaitSleepDisabled(true, 3*time.Second) {
		_ = disablesleepOffBestEffort()
		return ErrNotVerified
	}
	// A Mac that cannot sleep with nothing installed to restore it is strictly worse
	// than one that never enabled. Undo rather than leave that behind.
	if !GuardInstalled() {
		_ = disablesleepOffBestEffort()
		return ErrGuardMissing
	}
	return writeStamp(TierClamshell)
}

// installScript is the one privileged payload: place the guard root-owned, load it,
// then disable sleep. Order matters — the guard is in place BEFORE sleep is
// disabled, so there is never a window where the Mac cannot sleep and nothing is
// watching.
//
// File contents are embedded as BASE64, never as shell-quoted text. An earlier
// version interpolated them with Go's %q, which produced two failures at once: Go
// quoting is not shell quoting (its `\n` stays a literal backslash-n inside shell
// double quotes, so the guard was written as one broken line), and the guard's own
// comments contain BACKTICKS — which the shell executes inside double quotes. With
// `set -e` that aborted the whole install, and the caller reported it as "authorization
// declined". Base64's alphabet has no character the shell treats specially, so this
// class of bug cannot recur. (Same footgun as the `--body "$(…)"` one in CLAUDE.md.)
func installScript(guard, plist string) string {
	return fmt.Sprintf(`set -e
/bin/mkdir -p %[1]q
/bin/echo %[2]q | /usr/bin/base64 -d > %[3]q
/usr/sbin/chown -R root:wheel %[1]q
/bin/chmod 755 %[3]q
/bin/echo %[4]q | /usr/bin/base64 -d > %[5]q
/usr/sbin/chown root:wheel %[5]q
/bin/chmod 644 %[5]q
/bin/launchctl bootout system/%[6]s 2>/dev/null || true
/bin/launchctl bootstrap system %[5]q
/usr/bin/pmset -a disablesleep 1
`, GuardDir, base64.StdEncoding.EncodeToString([]byte(guard)), GuardScriptPath,
		base64.StdEncoding.EncodeToString([]byte(plist)), GuardPlistPath, GuardLabel)
}

// DisableRemote stands server mode down WITHOUT any authorization prompt: it writes
// the unprivileged marker and lets the guard do the rest on its next tick.
//
// This is the only correct behaviour for a request that did not come from someone
// sitting at the Mac. Raising an authorization dialog for a remote caller would put a
// password prompt on an unattended screen with nobody to answer it — and block the
// serve request handler waiting for a reply that never comes. The phone tapping
// "turn off" must not be able to hang the Mac's UI.
//
// It is slower (bounded by the guard's interval) and that is the right trade: the
// marker is already down, so the outcome is guaranteed either way.
func DisableRemote() error { return Revoke() }

// Disable turns server mode off from the machine itself, where a human is present.
//
// The stand-down marker is written FIRST, unprivileged, so that even if the user
// dismisses the password prompt the guard still restores sleep within its next tick.
// The privileged call is only there to make it immediate. Turning it off must never
// be able to fail in a way that leaves the Mac awake.
func Disable() error {
	if err := Revoke(); err != nil {
		return err
	}
	script := `/usr/bin/pmset -a disablesleep 0
/bin/launchctl bootout system/` + GuardLabel + ` 2>/dev/null || true
/bin/rm -f ` + fmt.Sprintf("%q %q", GuardScriptPath, GuardPlistPath)
	out, err := runPrivileged(script, "gtmux needs your administrator password to let this Mac sleep again.")
	if err == nil && !WriteRefused(out) && awaitSleepDisabled(false, 3*time.Second) {
		clearStamp()
		ClearRevoke()
		return nil
	}
	// Authorization declined or ineffective: the marker is already down, so the
	// guard will finish the job unattended. Say so rather than reporting failure.
	if GuardInstalled() {
		return nil
	}
	return err
}

// awaitSleepDisabled polls the live kernel state until it matches want. Polling
// rather than reading once: the write is asynchronous, and reading too early was
// what made an earlier draft report a successful restore as a failure.
func awaitSleepDisabled(want bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if SleepDisabled() == want {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// disablesleepOffBestEffort undoes a partially-applied enable. Best effort: it runs
// inside a failure path, and the guard is the real backstop.
func disablesleepOffBestEffort() error {
	_ = Revoke()
	_, err := runPrivileged(`/usr/bin/pmset -a disablesleep 0
/bin/launchctl bootout system/`+GuardLabel+` 2>/dev/null || true`,
		"gtmux is undoing an incomplete change and needs your administrator password.")
	return err
}

// writeStamp records that gtmux owns this state — the thing that separates "ours to
// clean up" from "someone else's, report only".
func writeStamp(tier string) error {
	now := time.Now().Unix()
	b, err := json.Marshal(Record{Tier: tier, Since: now, HeartbeatAt: now})
	if err != nil {
		return err
	}
	return os.WriteFile(StatePath(), b, 0o644)
}

// Heartbeat refreshes liveness. It is what tells the guard gtmux is still here to
// serve — never an expiry.
func Heartbeat() error {
	rec, ok := LoadState()
	if !ok {
		return nil // not ours / not on
	}
	rec.HeartbeatAt = time.Now().Unix()
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return os.WriteFile(StatePath(), b, 0o644)
}

func clearStamp() { _ = os.Remove(StatePath()) }

// ClearStampForLapse drops our ownership record after the setting stopped taking
// effect on its own. The state is genuinely over — keeping the stamp would make
// every later status report a lapse that already happened, and would make the guard
// look responsible for something it did not do.
func ClearStampForLapse() {
	clearStamp()
	ClearRevoke()
}
