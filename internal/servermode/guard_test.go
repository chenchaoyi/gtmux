package servermode

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The guard is a security artifact: it runs as root, forever, on the user's machine.
// These tests are the reason it can be trusted, so they check properties rather than
// spot-checking strings.

// THE invariant. If this ever fails, the guard has gained the power to escalate and
// the entire safety model of server mode is void.
func TestGuardHasNoPathThatDisablesSleep(t *testing.T) {
	script := GuardScript("/tmp/state.json", "/tmp/revoke")
	if !GuardCanOnlyRestore(script) {
		t.Fatal("the guard must contain NO instruction that disables sleep")
	}
	// Belt and braces: every pmset invocation in the script must be the restore.
	for _, ln := range strings.Split(script, "\n") {
		code, _, _ := strings.Cut(ln, "#")
		if !strings.Contains(code, "PMSET") && !strings.Contains(code, "pmset") {
			continue
		}
		if strings.Contains(code, "disablesleep") && !strings.Contains(code, "disablesleep 0") {
			t.Errorf("pmset line touches disablesleep without restoring it: %q", strings.TrimSpace(ln))
		}
	}
}

// The guard runs as root, so anything it executes must live somewhere a non-root
// user cannot rewrite. A user-writable path here would be a local privilege
// escalation: replace the file, wait 30s, get root.
func TestGuardOnlyRunsSystemBinaries(t *testing.T) {
	script := GuardScript("/tmp/state.json", "/tmp/revoke")
	for _, ln := range strings.Split(script, "\n") {
		code, _, _ := strings.Cut(ln, "#")
		for _, f := range strings.Fields(code) {
			if !strings.HasPrefix(f, "/") || !strings.Contains(f, "/bin/") {
				continue
			}
			f = strings.Trim(f, "\"'")
			if !strings.HasPrefix(f, "/bin/") && !strings.HasPrefix(f, "/usr/bin/") &&
				!strings.HasPrefix(f, "/usr/sbin/") && !strings.HasPrefix(f, "/sbin/") {
				t.Errorf("guard executes something outside the system paths: %q", f)
			}
		}
	}
}

// It must be valid shell. A syntax error would mean the guard silently never runs
// and a Mac quietly stays unable to sleep — a failure with no symptom until the
// battery is flat.
func TestGuardScriptIsValidShell(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-n")
	cmd.Stdin = strings.NewReader(GuardScript("/tmp/state.json", "/tmp/revoke"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("guard script is not valid shell: %v\n%s", err, out)
	}
}

// Every reason the guard can record must be one the rest of the system knows how to
// explain; an unrecognized reason would surface to the user as a bare token.
func TestGuardReasonsAreKnown(t *testing.T) {
	known := map[string]bool{
		ReasonRevoked: true, ReasonBatteryLow: true,
		ReasonStaleHeartbeat: true, ReasonBootReconcile: true,
	}
	script := GuardScript("/tmp/state.json", "/tmp/revoke")
	for _, ln := range strings.Split(script, "\n") {
		_, after, ok := strings.Cut(strings.TrimSpace(ln), "restore ")
		if !ok || strings.HasPrefix(strings.TrimSpace(ln), "#") {
			continue
		}
		reason := strings.Fields(after)
		if len(reason) == 0 || strings.HasPrefix(reason[0], "<") {
			continue // the function definition / doc line
		}
		if !known[reason[0]] {
			t.Errorf("guard records an unknown exit reason %q", reason[0])
		}
	}
}

// The paths are baked in at install time because the guard has no user session to
// resolve $HOME from. A relative or empty path would make it silently do nothing.
func TestGuardScriptBakesAbsolutePaths(t *testing.T) {
	script := GuardScript("/Users/x/.local/share/gtmux/server-mode/state.json",
		"/Users/x/.local/share/gtmux/server-mode/revoke")
	for _, want := range []string{
		"/Users/x/.local/share/gtmux/server-mode/state.json",
		"/Users/x/.local/share/gtmux/server-mode/revoke",
		GuardPlistPath, GuardScriptPath,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("guard script missing baked path %q", want)
		}
	}
	if strings.Contains(script, "$HOME") {
		t.Error("the guard has no user session — $HOME must never appear in it")
	}
}

func TestGuardPlistShape(t *testing.T) {
	p := GuardPlist("/tmp/watch")
	for _, want := range []string{GuardLabel, GuardScriptPath, "RunAtLoad", "StartInterval"} {
		if !strings.Contains(p, want) {
			t.Errorf("guard plist missing %q", want)
		}
	}
	// RunAtLoad is what handles the reboot path; without it a machine could come
	// back up unable to sleep with nothing scheduled to notice. Compare with
	// whitespace collapsed so a formatting change can't fail a semantic check.
	flat := strings.Join(strings.Fields(p), "")
	if !strings.Contains(flat, "<key>RunAtLoad</key><true/>") {
		t.Error("RunAtLoad must be true — it is the reboot safety net")
	}
}

// The charge floor is the guardrail that saves a laptop forgotten in a bag, and it is
// the one branch that never runs during normal use — by the time it matters, nobody
// is watching. So it gets exercised here against fabricated `pmset` output, once per
// case that decides whether a machine is rescued or drained.
//
// The guard's binary paths are absolute BY DESIGN (a root daemon must never resolve
// through PATH), so the test rewrites them to point at a fake — in the copy under
// test only. Production keeps its hardcoded /usr/bin/pmset.
func TestGuardChargeFloor(t *testing.T) {
	for _, tc := range []struct {
		name        string
		pmsetOutput string
		wantRestore bool
	}{
		{"on mains, full", "Now drawing from 'AC Power'\n -InternalBattery-0\t100%; charged; 0:00 remaining present: true\n", false},
		{"on mains, low battery — plugged in, so no reason to act",
			"Now drawing from 'AC Power'\n -InternalBattery-0\t9%; charging; 1:20 remaining present: true\n", false},
		{"on battery, plenty left — carrying it between rooms is the point",
			"Now drawing from 'Battery Power'\n -InternalBattery-0\t80%; discharging; 4:10 remaining present: true\n", false},
		{"on battery, just above the floor", "Now drawing from 'Battery Power'\n -InternalBattery-0\t21%; discharging; 0:50 remaining present: true\n", false},
		{"on battery, AT the floor — must rescue", "Now drawing from 'Battery Power'\n -InternalBattery-0\t20%; discharging; 0:45 remaining present: true\n", true},
		{"on battery, nearly flat — must rescue", "Now drawing from 'Battery Power'\n -InternalBattery-0\t4%; discharging; 0:08 remaining present: true\n", true},
		{"desktop: no battery line at all", "Now drawing from 'AC Power'\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			// A fake pmset that reports our scenario and LOGS any write it is asked
			// to make, so we can tell rescue from no-op.
			fake := dir + "/pmset"
			os.WriteFile(fake, []byte("#!/bin/sh\n"+
				"if [ \"$1\" = \"-g\" ]; then cat <<'OUT'\n"+tc.pmsetOutput+"OUT\n"+
				"  exit 0\nfi\n"+
				"echo \"$@\" >> "+dir+"/calls\n"), 0o755)

			// State fresh enough that only the charge branch can fire.
			os.WriteFile(dir+"/state.json", []byte(`{"tier":"clamshell"}`), 0o644)

			script := GuardScript(dir+"/state.json", dir+"/revoke")
			script = strings.ReplaceAll(script, "/usr/bin/pmset", fake)
			// Neutralise the self-removal + launchctl so the test only measures the
			// decision, not the teardown.
			script = strings.ReplaceAll(script, "/bin/launchctl", "/usr/bin/true")
			script = strings.ReplaceAll(script, "/bin/rm -f", "/usr/bin/true")
			script = strings.ReplaceAll(script, GuardDir+"/last-exit.json", dir+"/last-exit.json")
			sp := dir + "/guard.sh"
			os.WriteFile(sp, []byte(script), 0o755)

			out, _ := exec.Command("/bin/sh", sp).CombinedOutput()
			calls, _ := os.ReadFile(dir + "/calls")
			restored := strings.Contains(string(calls), "disablesleep 0")

			if restored != tc.wantRestore {
				t.Errorf("restore=%v, want %v\ncalls: %q\noutput: %s",
					restored, tc.wantRestore, calls, out)
			}
			// Whatever happens, the guard must never be able to disable sleep.
			if strings.Contains(string(calls), "disablesleep 1") {
				t.Fatal("the guard disabled sleep — it must only ever restore it")
			}
		})
	}
}

// Turning server mode off must never require a password — the whole safety model
// rests on de-escalation being free. WatchPaths is what makes that possible without
// also making the user wait for the next tick: writing the unprivileged marker wakes
// the daemon immediately.
func TestGuardWatchesTheStandDownMarker(t *testing.T) {
	plist := GuardPlist("/Users/x/.local/share/gtmux/server-mode")
	flat := strings.Join(strings.Fields(plist), "")
	if !strings.Contains(flat, "<key>WatchPaths</key>") {
		t.Fatal("without WatchPaths, turning it off waits for the next tick — which is " +
			"exactly the pressure that led to prompting for a password")
	}
	if !strings.Contains(plist, "/Users/x/.local/share/gtmux/server-mode") {
		t.Error("the watched path must be where the stand-down marker is written")
	}
	// The interval stays as the backstop for what a path watch cannot observe:
	// charge falling, a stale heartbeat, an abandoned boot.
	if !strings.Contains(flat, "<key>StartInterval</key>") {
		t.Error("StartInterval must remain — WatchPaths cannot see the battery draining")
	}
}

// After the guard restores sleep, gtmux's ownership stamp must be gone too. If the
// stamp outlives the state, the next status read sees "our record says on, the kernel
// says off" and reports a normal shutdown as a LAPSE — alarming the user about a
// failure that did not happen.
func TestGuardClearsTheOwnershipStampWhenItRestores(t *testing.T) {
	dir := t.TempDir()
	fake := dir + "/pmset"
	os.WriteFile(fake, []byte("#!/bin/sh\n"+
		"if [ \"$1\" = \"-g\" ]; then echo \"Now drawing from 'AC Power'\"; exit 0; fi\n"+
		"echo \"$@\" >> "+dir+"/calls\n"), 0o755)
	state := dir + "/state.json"
	revoke := dir + "/revoke"
	os.WriteFile(state, []byte(`{"tier":"clamshell"}`), 0o644)
	os.WriteFile(revoke, []byte("1\n"), 0o644) // stand-down requested

	script := GuardScript(state, revoke)
	script = strings.ReplaceAll(script, "/usr/bin/pmset", fake)
	script = strings.ReplaceAll(script, "/bin/launchctl", "/usr/bin/true")
	script = strings.ReplaceAll(script, GuardDir+"/last-exit.json", dir+"/last-exit.json")
	sp := dir + "/guard.sh"
	os.WriteFile(sp, []byte(script), 0o755)
	if out, err := exec.Command("/bin/sh", sp).CombinedOutput(); err != nil {
		t.Fatalf("guard failed: %v\n%s", err, out)
	}

	if _, err := os.Stat(state); err == nil {
		t.Error("the ownership stamp must be removed — otherwise a clean shutdown reads as a lapse")
	}
	if _, err := os.Stat(revoke); err == nil {
		t.Error("the stand-down marker must be cleared, or the next enable is undone instantly")
	}
	calls, _ := os.ReadFile(dir + "/calls")
	if !strings.Contains(string(calls), "disablesleep 0") {
		t.Errorf("sleep was not restored: %q", calls)
	}
}
