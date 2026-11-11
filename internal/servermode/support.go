package servermode

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// Reason codes for an unsupported or unverified platform. The CLI/app localize
// these; the package stays language-free.
const (
	ReasonSupported     = ""
	ReasonNotMacOS      = "not-macos"
	ReasonNoSetting     = "setting-unrecognized"
	ReasonNoReadback    = "no-readback"
	ReasonUnverifiedOS  = "unverified-os"
	verifiedMajorMacOS  = 26 // original measured baseline; the user also verified 27.2
	verifiedOSForHumans = "macOS 26 and 27.2"
)

// Support is the verdict on whether server mode can work here at all.
//
// Two distinct answers, deliberately not merged: OK=false means the mechanism is
// absent and no authorization should ever be requested; OK=true with Verified=false
// means it looks present but this project has never measured this configuration —
// the honest word is "unverified", not "unsupported". A hard version allowlist would
// break the feature on every future macOS until someone shipped a release, which is
// a worse failure than an honest warning.
type Support struct {
	OK        bool   `json:"ok"`
	Verified  bool   `json:"verified"`
	Reason    string `json:"reason,omitempty"`
	OSVersion string `json:"os_version,omitempty"`
}

// Supported preflights the platform WITHOUT privilege, so a refusal costs the user
// nothing: no password prompt, no system change. The whole point is to fail before
// asking for authorization rather than after silently doing nothing.
func Supported() Support {
	s := Support{OSVersion: osVersion()}
	if runtime.GOOS != "darwin" {
		s.Reason = ReasonNotMacOS
		return s
	}
	if !settingRecognized() {
		s.Reason = ReasonNoSetting
		return s
	}
	if !readbackAvailable() {
		s.Reason = ReasonNoReadback
		return s
	}
	s.OK = true
	if !verifiedServerModeOS(s.OSVersion) {
		s.Reason = ReasonUnverifiedOS
		return s
	}
	s.Verified = true
	return s
}

// settingRecognized probes whether the power tool still accepts the setting name,
// using the same discrimination that verified it by hand: a LIVE name gets as far as
// the privilege check ("must be run as root"), while a bogus name is rejected with a
// usage message. Argument parsing happens before the privilege check, so this is a
// read-only question answered by a command that cannot succeed.
//
// SAFETY: it can only stay read-only while we are NOT root. Running this as root
// would actually enable server mode as a side effect of asking whether it exists.
// So as root we skip the probe and assume present — a root gtmux has bigger levers
// anyway, and 2.0b's readback verification is the real guarantee either way.
func settingRecognized() bool {
	if os.Geteuid() == 0 {
		return true
	}
	out, _ := exec.Command("pmset", "-a", "disablesleep", "1").CombinedOutput()
	return WriteRefused(string(out))
}

// readbackAvailable checks that the kernel power node can be read at all. It
// deliberately does NOT require the SleepDisabled key to be present: a machine that
// has never had it set may not expose it, and absence correctly reads as "off".
// Whether the key APPEARS once set is proven by the post-enable readback, which is
// the check that actually protects the user.
func readbackAvailable() bool {
	_, known := ReadSleepDisabled()
	return known
}

func osVersion() string {
	out, err := exec.Command("sw_vers", "-productVersion").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// osMajor extracts the leading major version ("26.5.2" → 26); -1 when unparseable,
// which counts as unverified rather than unsupported.
func osMajor(v string) int {
	major, _, _ := strings.Cut(v, ".")
	n, err := strconv.Atoi(strings.TrimSpace(major))
	if err != nil {
		return -1
	}
	return n
}

// Keep version presentation separate from live platform and readback probes.
func verifiedServerModeOS(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) > 3 {
		return false
	}
	values := make([]int, len(parts))
	for i, part := range parts {
		if part == "" {
			return false
		}
		for _, char := range part {
			if char < '0' || char > '9' {
				return false
			}
		}
		value, err := strconv.Atoi(part)
		if err != nil {
			return false
		}
		values[i] = value
	}
	// The 27.2 baseline comes from the user's real-system confirmation, not an
	// additional privileged probe. Unknown future versions retain their caution.
	return values[0] == verifiedMajorMacOS || len(values) >= 2 && values[0] == 27 && values[1] == 2
}
