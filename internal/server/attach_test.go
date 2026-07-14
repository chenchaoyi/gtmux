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
