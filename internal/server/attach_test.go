package server

import (
	"net/http"
	"testing"
)

// The /api/attach scope gate runs BEFORE any PTY is spawned (before the WS upgrade),
// so we can assert it with plain GETs (the real bridge is verified manually on a
// terminal). viewServer wires Share + a guest token but no AttachCommand.

func TestAttach_GuestRefusedNonViewable(t *testing.T) {
	h, _, guest := viewServer(t)
	// Empty view allowlist → a guest cannot attach %1 → refused before upgrade.
	if rr := do(t, h, http.MethodGet, "/api/attach?id=%251", guest); rr.Code != http.StatusForbidden {
		t.Fatalf("guest attach to non-viewable = %d, want 403 (%s)", rr.Code, rr.Body.String())
	}
}

func TestAttach_GuestViewablePassesGate(t *testing.T) {
	h, share, guest := viewServer(t)
	share.SetConfig(nil, nil, &[]string{"%1"}) // allow %1 for viewing
	// Passes the scope gate; with no AttachCommand wired it then 503s — proving the
	// gate ALLOWED the viewable guest (did not 403).
	if rr := do(t, h, http.MethodGet, "/api/attach?id=%251", guest); rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("viewable guest attach = %d, want 503 (past the gate) (%s)", rr.Code, rr.Body.String())
	}
}

func TestAttach_OwnerNoCommand503(t *testing.T) {
	h, _, _ := viewServer(t)
	if rr := do(t, h, http.MethodGet, "/api/attach?id=%251", testToken); rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("owner attach, no AttachCommand = %d, want 503", rr.Code)
	}
}

func TestAttach_MissingID(t *testing.T) {
	h, _, _ := viewServer(t)
	if rr := do(t, h, http.MethodGet, "/api/attach", testToken); rr.Code != http.StatusBadRequest {
		t.Fatalf("attach missing id = %d, want 400", rr.Code)
	}
}

// The tmux client spawned in the PTY MUST get a valid TERM (else "does not support
// clear") AND a UTF-8 locale (else CJK renders as placeholder dashes). Regression guard.
func TestAttachEnv_SetsTermAndUTF8(t *testing.T) {
	env := attachEnv("xterm-256color")
	has := func(want string) bool {
		for _, e := range env {
			if e == want {
				return true
			}
		}
		return false
	}
	if !has("TERM=xterm-256color") {
		t.Errorf("attachEnv must set TERM")
	}
	if !has("LC_CTYPE=en_US.UTF-8") {
		t.Errorf("attachEnv must set a UTF-8 LC_CTYPE (else CJK → dashes)")
	}
}

// sanitizeTerm rejects anything that isn't a terminfo-safe name (a client-supplied
// TERM becomes a spawned process's env var).
func TestSanitizeTerm(t *testing.T) {
	for _, ok := range []string{"xterm-256color", "xterm-ghostty", "screen.xterm-256color", "tmux-256color"} {
		if sanitizeTerm(ok) != ok {
			t.Errorf("sanitizeTerm(%q) should pass", ok)
		}
	}
	for _, bad := range []string{"", "xterm; rm -rf", "a b", "évil", "x\n"} {
		if sanitizeTerm(bad) != "" {
			t.Errorf("sanitizeTerm(%q) should be rejected", bad)
		}
	}
}

// resolveTerm falls back to a safe terminfo when the client TERM is empty or unknown.
func TestResolveTerm_Fallback(t *testing.T) {
	if got := resolveTerm(""); got != "xterm-256color" {
		t.Errorf("empty client term → %q, want xterm-256color", got)
	}
	if got := resolveTerm("definitely-not-a-real-terminfo-xyz"); got != "xterm-256color" {
		t.Errorf("unknown client term → %q, want xterm-256color fallback", got)
	}
}
