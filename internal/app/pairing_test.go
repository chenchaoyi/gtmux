package app

import (
	"strings"
	"testing"
)

// TestPairingPayload: a minted code yields the secure v2 enrollCode QR (no token);
// an empty code falls back to the legacy v1 token QR so pairing still works.
func TestPairingPayload(t *testing.T) {
	v2 := string(pairingPayload("https://x.dev", "MASTERTOK", "code123", "Mac"))
	if !strings.Contains(v2, `"v":2`) || !strings.Contains(v2, `"enrollCode":"code123"`) {
		t.Errorf("with a code, want v2 enrollCode QR; got %s", v2)
	}
	if strings.Contains(v2, "MASTERTOK") {
		t.Errorf("v2 QR must NOT leak the master token; got %s", v2)
	}

	v1 := string(pairingPayload("https://x.dev", "MASTERTOK", "", "Mac"))
	if !strings.Contains(v1, `"v":1`) || !strings.Contains(v1, `"token":"MASTERTOK"`) {
		t.Errorf("with no code, want legacy v1 token QR; got %s", v1)
	}
}
