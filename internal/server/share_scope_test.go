package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// scopeServer builds a serve with TWO guest links carrying DIFFERENT per-link
// scopes (pair-share-model): alice may view+type %1, bob may only view %2.
func scopeServer(t *testing.T) (h http.Handler, alice, bob string, share *ShareManager) {
	t.Helper()
	old := sendSettle
	sendSettle = 0
	t.Cleanup(func() { sendSettle = old })

	enroll := NewEnrollManager(nil, nil)
	share = NewShareManager(ShareState{Enabled: true}, nil)
	agents := `[{"pane_id":"%1","agent":"Claude Code"},{"pane_id":"%2","agent":"Codex"}]`
	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{
		Enroll:     enroll,
		Share:      share,
		AgentsJSON: func() ([]byte, error) { return []byte(agents), nil },
		PaneText:   func(id string) (string, bool) { return "screen of " + id, true },
		Send:       func(id, text, key string, enter bool) error { return nil },
	})
	a := enroll.MintGuest("alice", []string{"%1"}, []string{"%1"}, 0)
	b := enroll.MintGuest("bob", []string{"%2"}, nil, 0)
	return s.Handler(), a.Token, b.Token, share
}

// Per-link isolation: each guest sees and types ONLY its own grant — never
// another link's.
func TestPerLinkScopes_Isolation(t *testing.T) {
	h, alice, bob, _ := scopeServer(t)

	if ids := paneIDs(t, do(t, h, http.MethodGet, "/api/agents", alice).Body.Bytes()); len(ids) != 1 || ids[0] != "%1" {
		t.Fatalf("alice radar = %v, want [%%1]", ids)
	}
	if ids := paneIDs(t, do(t, h, http.MethodGet, "/api/agents", bob).Body.Bytes()); len(ids) != 1 || ids[0] != "%2" {
		t.Fatalf("bob radar = %v, want [%%2]", ids)
	}
	// Cross-pane reads refuse.
	if rr := do(t, h, http.MethodGet, "/api/pane?id=%252", alice); rr.Code != http.StatusForbidden {
		t.Fatalf("alice reading bob's pane = %d, want 403", rr.Code)
	}
	if rr := do(t, h, http.MethodGet, "/api/pane?id=%251", bob); rr.Code != http.StatusForbidden {
		t.Fatalf("bob reading alice's pane = %d, want 403", rr.Code)
	}
	// Typing: alice may type %1; bob may type nowhere (view-only link).
	if rr := post(t, h, "/api/send", alice, `{"id":"%1","text":"ls"}`); rr.Code != http.StatusOK {
		t.Fatalf("alice send %%1 = %d, want 200", rr.Code)
	}
	if rr := post(t, h, "/api/send", bob, `{"id":"%2","text":"ls"}`); rr.Code != http.StatusForbidden {
		t.Fatalf("bob send (view-only) = %d, want 403", rr.Code)
	}
	if rr := post(t, h, "/api/send", alice, `{"id":"%2","text":"ls"}`); rr.Code != http.StatusForbidden {
		t.Fatalf("alice send to bob's pane = %d, want 403", rr.Code)
	}
}

// GET /api/share resolves each guest's OWN capability.
func TestShareCapability_PerLink(t *testing.T) {
	h, alice, bob, _ := scopeServer(t)
	var capA, capB struct {
		Input     bool     `json:"input"`
		Panes     []string `json:"panes"`
		ViewPanes []string `json:"view_panes"`
	}
	_ = json.Unmarshal(do(t, h, http.MethodGet, "/api/share", alice).Body.Bytes(), &capA)
	_ = json.Unmarshal(do(t, h, http.MethodGet, "/api/share", bob).Body.Bytes(), &capB)
	if !capA.Input || len(capA.Panes) != 1 || capA.Panes[0] != "%1" || len(capA.ViewPanes) != 1 {
		t.Fatalf("alice capability = %+v", capA)
	}
	if capB.Input || len(capB.Panes) != 0 || len(capB.ViewPanes) != 1 || capB.ViewPanes[0] != "%2" {
		t.Fatalf("bob capability = %+v", capB)
	}
}

// An expired link stops authenticating exactly like a revoked one.
func TestGuestExpiry(t *testing.T) {
	enroll := NewEnrollManager(nil, nil)
	now := time.Now()
	enroll.now = func() time.Time { return now }
	d := enroll.MintGuest("temp", []string{"%1"}, nil, now.Unix()+3600)

	if sc, ok := enroll.TokenScope(d.Token); !ok || sc != "guest" {
		t.Fatalf("fresh link scope = %q %v, want guest true", sc, ok)
	}
	enroll.now = func() time.Time { return now.Add(2 * time.Hour) }
	if _, ok := enroll.TokenScope(d.Token); ok {
		t.Fatal("expired link must not authenticate")
	}
	// No expiry (0) never expires.
	d2 := enroll.MintGuest("forever", nil, nil, 0)
	enroll.now = func() time.Time { return now.Add(24 * 365 * time.Hour) }
	if _, ok := enroll.TokenScope(d2.Token); !ok {
		t.Fatal("a no-expiry link must keep authenticating")
	}
}

// Migration copies the global lists into LEGACY entries exactly once; an
// explicitly-scoped link is never touched.
func TestMigrateGuestScopes_OneTime(t *testing.T) {
	// A legacy roster entry: guest scope but no per-link fields (pre-upgrade).
	legacy := EnrolledDevice{ID: "legacy1", Name: "old", Token: "tok-legacy", Scope: "guest"}
	enroll := NewEnrollManager([]EnrolledDevice{legacy}, nil)
	explicit := enroll.MintGuest("tailored", []string{"%9"}, nil, 0)

	enroll.MigrateGuestScopes([]string{"%1", "%2"}, []string{"%1"})

	d, _ := enroll.DeviceByToken("tok-legacy")
	if len(d.ViewPanes) != 2 || len(d.InputPanes) != 1 || !d.ScopeSet {
		t.Fatalf("legacy entry not migrated: %+v", d)
	}
	e, _ := enroll.DeviceByToken(explicit.Token)
	if len(e.ViewPanes) != 1 || e.ViewPanes[0] != "%9" {
		t.Fatalf("explicit scope must never be touched by migration: %+v", e)
	}
	// Second migration (e.g. next serve start with different globals) is a no-op
	// for the already-migrated entry.
	enroll.MigrateGuestScopes([]string{"%5"}, nil)
	d2, _ := enroll.DeviceByToken("tok-legacy")
	if len(d2.ViewPanes) != 2 {
		t.Fatalf("migration must be one-time: %+v", d2)
	}
}

// The legacy broadcast REPLACES every link's lists (old global semantics).
func TestBroadcastGuestScopes(t *testing.T) {
	enroll := NewEnrollManager(nil, nil)
	a := enroll.MintGuest("a", []string{"%1"}, []string{"%1"}, 0)
	b := enroll.MintGuest("b", []string{"%2"}, nil, 0)
	enroll.BroadcastGuestScopes([]string{"%3"}, []string{"%3"})
	for _, tok := range []string{a.Token, b.Token} {
		d, _ := enroll.DeviceByToken(tok)
		if len(d.ViewPanes) != 1 || d.ViewPanes[0] != "%3" || len(d.InputPanes) != 1 {
			t.Fatalf("broadcast must replace every link's lists: %+v", d)
		}
	}
}

// POST /api/share/new without explicit scope copies the global TEMPLATE; with
// explicit scope it uses exactly that. POST /api/share/set edits one link.
func TestShareNewTemplateAndSet(t *testing.T) {
	h, _, _, share := scopeServer(t)
	share.SetConfig(nil, &[]string{"%1"}, &[]string{"%1", "%2"}) // template: view %1,%2 · type %1

	// Template default.
	rr := post(t, h, "/api/share/new", testToken, `{"label":"tpl"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("share/new = %d", rr.Code)
	}
	var minted struct{ Token, ID string }
	_ = json.Unmarshal(rr.Body.Bytes(), &minted)
	var cap struct {
		ViewPanes []string `json:"view_panes"`
		Panes     []string `json:"panes"`
	}
	_ = json.Unmarshal(do(t, h, http.MethodGet, "/api/share", minted.Token).Body.Bytes(), &cap)
	if len(cap.ViewPanes) != 2 || len(cap.Panes) != 1 {
		t.Fatalf("template-minted capability = %+v, want view 2 / type 1", cap)
	}

	// Explicit scope beats the template.
	rr = post(t, h, "/api/share/new", testToken, `{"label":"explicit","view":["%2"],"input":[]}`)
	var minted2 struct{ Token string }
	_ = json.Unmarshal(rr.Body.Bytes(), &minted2)
	_ = json.Unmarshal(do(t, h, http.MethodGet, "/api/share", minted2.Token).Body.Bytes(), &cap)
	if len(cap.ViewPanes) != 1 || cap.ViewPanes[0] != "%2" || len(cap.Panes) != 0 {
		t.Fatalf("explicit-minted capability = %+v", cap)
	}

	// share/set edits exactly one link (per-flag replace).
	rr = post(t, h, "/api/share/set", testToken, `{"id":"`+minted.ID+`","input":["%2"]}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("share/set = %d (%s)", rr.Code, rr.Body.String())
	}
	_ = json.Unmarshal(do(t, h, http.MethodGet, "/api/share", minted.Token).Body.Bytes(), &cap)
	if len(cap.Panes) != 1 || cap.Panes[0] != "%2" {
		t.Fatalf("after set, capability = %+v, want type [%%2]", cap)
	}
	if !strings.Contains(strings.Join(cap.ViewPanes, ","), "%2") {
		t.Fatalf("set input must imply view: %+v", cap)
	}
	// Unknown id → 404.
	if rr := post(t, h, "/api/share/set", testToken, `{"id":"nope","input":[]}`); rr.Code != http.StatusNotFound {
		t.Fatalf("share/set unknown id = %d, want 404", rr.Code)
	}
	// Guests cannot call the master-only endpoints.
	if rr := post(t, h, "/api/share/set", minted.Token, `{"id":"x"}`); rr.Code != http.StatusForbidden {
		t.Fatalf("guest share/set = %d, want 403", rr.Code)
	}
}
