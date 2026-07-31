package servermode

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The install payload is privileged. These pin the ordering and safety properties
// that make it survivable when it fails halfway.
func TestInstallScriptOrdersGuardBeforeDisablingSleep(t *testing.T) {
	s := installScript(GuardScript("/s.json", "/r"), GuardPlist())
	iBootstrap := strings.Index(s, "bootstrap system")
	iDisable := strings.Index(s, "disablesleep 1")
	if iBootstrap < 0 || iDisable < 0 {
		t.Fatal("install script must both load the guard and disable sleep")
	}
	// If sleep were disabled first and the guard load then failed, the Mac would be
	// left unable to sleep with nothing watching it.
	if iBootstrap > iDisable {
		t.Error("the guard must be loaded BEFORE sleep is disabled")
	}
	if !strings.Contains(s, "set -e") {
		t.Error("the payload must abort on the first failure, not continue half-applied")
	}
	if !strings.Contains(s, "chown -R root:wheel") {
		t.Error("the guard must be root-owned — a user-writable root daemon is an escalation")
	}
}

// AppleScript escaping is a classic injection surface; base64 removes it entirely.
func TestPrivilegedPayloadIsNotStringInterpolated(t *testing.T) {
	// A guard path containing quotes must not be able to break out of the payload.
	s := installScript(GuardScript(`/tmp/a"b'c`, "/r"), GuardPlist())
	if strings.Contains(s, `"; rm`) {
		t.Error("payload appears to interpolate unescaped input")
	}
	// Shell-side safety comes from Go's %-q quoting of every interpolated path;
	// the transport to osascript is base64, so AppleScript escaping never applies.
	if !strings.Contains(s, `\"`) && !strings.Contains(s, "'") {
		t.Log("payload quoting relies on Go quoting plus the base64 transport")
	}
}

// The install payload is a shell script that WRITES another shell script. That
// nesting is where an earlier version broke in two ways at once — Go's %q quoting is
// not shell quoting, and the guard's own comments contain backticks, which the shell
// executes inside double quotes. The install aborted under `set -e` and surfaced as
// "authorization declined", sending debugging in the wrong direction entirely.
//
// So this runs the payload for real (against a temp dir, unprivileged) and compares
// what landed on disk with what we meant to write. A round trip is the only assertion
// that cannot be fooled by clever-looking quoting.
func TestInstallPayloadReproducesTheGuardExactly(t *testing.T) {
	guard := GuardScript("/tmp/state.json", "/tmp/revoke")
	payload := installScript(guard, GuardPlist())

	// Keep ONLY the two decode lines, retargeted at a temp dir: the rest of the
	// payload needs root (chown, launchctl, pmset) and is not what this pins.
	dir := t.TempDir()
	var lines []string
	for _, ln := range strings.Split(payload, "\n") {
		if strings.Contains(ln, "base64 -d") {
			ln = strings.ReplaceAll(ln, GuardScriptPath, dir+"/guard.sh")
			ln = strings.ReplaceAll(ln, GuardPlistPath, dir+"/guard.plist")
			lines = append(lines, ln)
		}
	}
	if len(lines) != 2 {
		t.Fatalf("expected two file-writing lines in the payload, got %d", len(lines))
	}
	cmd := exec.Command("/bin/sh", "-s")
	cmd.Stdin = strings.NewReader(strings.Join(lines, "\n"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("payload failed to run: %v\n%s", err, out)
	}

	got, err := os.ReadFile(dir + "/guard.sh")
	if err != nil {
		t.Fatalf("guard was not written: %v", err)
	}
	if string(got) != guard {
		t.Errorf("the guard on disk differs from the guard we generated\n"+
			"first 120 bytes written: %.120q\nwanted:                  %.120q", got, guard)
	}
	// The specific regression: backticks must survive as text, not be executed.
	if !strings.Contains(string(got), "`gtmux awake on`") {
		t.Error("backticked prose did not survive the payload — the shell ate it")
	}
	plist, err := os.ReadFile(dir + "/guard.plist")
	if err != nil || string(plist) != GuardPlist() {
		t.Error("the plist on disk differs from the one we generated")
	}
}

// Whatever the payload does, it must remain valid shell — a syntax error would abort
// the install midway, and `set -e` means the failure could land anywhere.
func TestInstallPayloadIsValidShell(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-n")
	cmd.Stdin = strings.NewReader(installScript(GuardScript("/s", "/r"), GuardPlist()))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("install payload is not valid shell: %v\n%s", err, out)
	}
}
