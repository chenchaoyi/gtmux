package servermode

import "testing"

func TestSMC001VerifiedServerModeOSPreservesUnknownVersionCaution(t *testing.T) {
	for _, tc := range []struct {
		version string
		want    bool
	}{
		{"26", true}, {"26.5.2", true}, {"27.2", true}, {"27.2.0", true}, {"27.2.1", true},
		{"27.1", false}, {"27.3", false}, {"28.0", false}, {"15.4", false}, {"", false},
		{"27", false}, {"27.2beta", false}, {"27.2.-1", false}, {"27.2.1.4", false}, {"unknown", false},
	} {
		t.Run(tc.version, func(t *testing.T) {
			if got := verifiedServerModeOS(tc.version); got != tc.want {
				t.Fatalf("verifiedServerModeOS(%q)=%v, want %v", tc.version, got, tc.want)
			}
		})
	}
}
