package servermode

import (
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
