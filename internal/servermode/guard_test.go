package servermode

import (
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
	p := GuardPlist()
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
