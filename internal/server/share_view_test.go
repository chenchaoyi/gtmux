package server

import (
	"encoding/json"
	"net/http"
	"testing"
)

// viewServer wires a Server with a multi-pane radar + pane text + usage/digest, plus
// enroll + share, returning a guest token to exercise the VIEW scope.
func viewServer(t *testing.T) (h http.Handler, share *ShareManager, guest string) {
	t.Helper()
	enroll := NewEnrollManager(nil, nil)
	share = NewShareManager(ShareState{}, nil)
	agents := `[{"pane_id":"%1","agent":"Claude Code"},{"pane_id":"%2","agent":"Codex"},{"source":"native"}]`
	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{
		Enroll:     enroll,
		Share:      share,
		AgentsJSON: func() ([]byte, error) { return []byte(agents), nil },
		PaneText:   func(id string) (string, bool) { return "screen of " + id, true },
		UsageJSON:  func() ([]byte, error) { return []byte(`{"sessions":[]}`), nil },
		DigestJSON: func() ([]byte, error) { return []byte(`[]`), nil },
	})
	return s.Handler(), share, enroll.MintGuest("g").Token
}

func paneIDs(t *testing.T, body []byte) []string {
	t.Helper()
	var rows []struct {
		PaneID string `json:"pane_id"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		t.Fatalf("agents body: %v (%s)", err, body)
	}
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.PaneID)
	}
	return ids
}

// A guest's radar is filtered to its view allowlist; a full caller is unfiltered.
func TestGuestAgents_FilteredToViewAllowlist(t *testing.T) {
	h, share, guest := viewServer(t)
	// Default: empty view allowlist → guest sees NOTHING.
	if ids := paneIDs(t, do(t, h, http.MethodGet, "/api/agents", guest).Body.Bytes()); len(ids) != 0 {
		t.Fatalf("fresh guest radar = %v, want empty", ids)
	}
	// Master sees the full radar unfiltered (both panes + the native row).
	if ids := paneIDs(t, do(t, h, http.MethodGet, "/api/agents", testToken).Body.Bytes()); len(ids) != 3 {
		t.Fatalf("master radar rows = %v, want all 3 unfiltered", ids)
	}
	// Allow %1 for viewing → guest sees only %1 (the native row is never a target).
	share.SetConfig(nil, nil, &[]string{"%1"})
	if ids := paneIDs(t, do(t, h, http.MethodGet, "/api/agents", guest).Body.Bytes()); len(ids) != 1 || ids[0] != "%1" {
		t.Fatalf("guest radar = %v, want [%%1]", ids)
	}
}

// A guest may read a pane's screen only if it's on the view allowlist.
func TestGuestPane_GatedByView(t *testing.T) {
	h, share, guest := viewServer(t)
	if rr := do(t, h, http.MethodGet, "/api/pane?id=%251", guest); rr.Code != http.StatusForbidden {
		t.Fatalf("guest pane before view = %d, want 403", rr.Code)
	}
	share.SetConfig(nil, nil, &[]string{"%1"})
	if rr := do(t, h, http.MethodGet, "/api/pane?id=%251", guest); rr.Code != http.StatusOK {
		t.Fatalf("guest pane after view add = %d, want 200 (%s)", rr.Code, rr.Body.String())
	}
	if rr := do(t, h, http.MethodGet, "/api/pane?id=%252", guest); rr.Code != http.StatusForbidden {
		t.Fatalf("guest pane %%2 (not viewable) = %d, want 403", rr.Code)
	}
	if rr := do(t, h, http.MethodGet, "/api/pane?id=%252", testToken); rr.Code != http.StatusOK {
		t.Fatalf("master pane = %d, want 200", rr.Code)
	}
}

// Usage + digest are owner/HQ surfaces — refused for a guest, served to the owner.
func TestGuestUsageDigest_Refused(t *testing.T) {
	h, _, guest := viewServer(t)
	for _, p := range []string{"/api/usage", "/api/digest"} {
		if rr := do(t, h, http.MethodGet, p, guest); rr.Code != http.StatusForbidden {
			t.Errorf("guest %s = %d, want 403", p, rr.Code)
		}
		if rr := do(t, h, http.MethodGet, p, testToken); rr.Code != http.StatusOK {
			t.Errorf("master %s = %d, want 200 (%s)", p, rr.Code, rr.Body.String())
		}
	}
}

// GET /api/share carries the guest's view allowlist so a surface renders only
// viewable panes; a view-only pane reports input=false.
func TestShareCapability_CarriesViewPanes(t *testing.T) {
	h, share, guest := viewServer(t)
	share.SetConfig(nil, nil, &[]string{"%1"})
	var cap shareCapability
	json.Unmarshal(do(t, h, http.MethodGet, "/api/share", guest).Body.Bytes(), &cap)
	if len(cap.ViewPanes) != 1 || cap.ViewPanes[0] != "%1" {
		t.Fatalf("guest capability view_panes = %v, want [%%1]", cap.ViewPanes)
	}
	if cap.Input {
		t.Error("view-only guest should have input=false")
	}
}

// input ⊆ view: an input pane is always viewable; a view-only pane is not typable;
// an old share.json with only `panes` migrates those panes to viewable.
func TestShareManager_ViewInvariant(t *testing.T) {
	m := NewShareManager(ShareState{}, nil)
	on := true
	m.SetConfig(&on, &[]string{"%1"}, nil)
	if !m.CanView("%1") {
		t.Error("input pane %1 should be viewable (input ⊆ view)")
	}
	m.SetConfig(nil, nil, &[]string{"%1", "%2"})
	if !m.CanView("%2") || m.Allowed("%2") {
		t.Error("%2 view-only: want CanView true, Allowed false")
	}
	if !m.Allowed("%1") || !m.CanView("%1") {
		t.Error("%1 should still be typable + viewable")
	}
	// Migration: old state with only `panes` keeps those viewable, hides the rest.
	old := NewShareManager(ShareState{Enabled: true, Panes: []string{"%7"}}, nil)
	if !old.CanView("%7") {
		t.Error("migrated input pane %7 should be viewable")
	}
	if old.CanView("%8") {
		t.Error("a pane never shared should not be viewable")
	}
}
