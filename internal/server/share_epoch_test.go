package server

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
)

// Pane grants are only meaningful inside ONE tmux server lifetime: after a restart the
// ids are reassigned, so a stored "%17" can address a completely different pane. These
// pin the fail-closed contract (share-grant-epoch).

func TestGrantsStaleAfterTmuxRestart(t *testing.T) {
	saved := ShareState{}
	m := NewShareManager(ShareState{Enabled: true, Panes: []string{"%17"}, ViewPanes: []string{"%17"}},
		func(s ShareState) { saved = s })

	epoch := "111@Mon Jul 21 10:00:00 2026"
	m.SetEpochSource(func() string { return epoch })

	// Grants predate any stamp → they cannot be proven to belong to this server.
	if !m.GrantsStale() {
		t.Fatal("unstamped grants must be stale while a tmux server is running")
	}

	// The owner (re)grants against the running server.
	m.StampEpoch()
	if m.GrantsStale() {
		t.Error("freshly stamped grants must not be stale")
	}
	if saved.PaneEpoch != epoch {
		t.Errorf("persisted PaneEpoch = %q; want %q (must survive a serve restart)", saved.PaneEpoch, epoch)
	}

	// tmux restarts → a new identity → every stored pane id is suspect again.
	epoch = "222@Tue Jul 22 09:00:00 2026"
	if !m.GrantsStale() {
		t.Error("after the tmux server changed, grants MUST be stale (ids were reassigned)")
	}
}

func TestGrantsNotStaleWhenPersistedEpochMatches(t *testing.T) {
	const epoch = "333@Wed Jul 22 08:00:00 2026"
	// A serve restart reloads the stamp from disk; the tmux server is the same one.
	m := NewShareManager(ShareState{Panes: []string{"%17"}, PaneEpoch: epoch}, func(ShareState) {})
	m.SetEpochSource(func() string { return epoch })
	if m.GrantsStale() {
		t.Error("grants stamped against the SAME running tmux server must remain valid across a serve restart")
	}
}

// Never lock out a setup we can't reason about: no tmux server (or an unreadable
// identity) means there are no panes to serve anyway.
func TestGrantsNotStaleWithoutATmuxIdentity(t *testing.T) {
	m := NewShareManager(ShareState{Panes: []string{"%17"}, PaneEpoch: "old"}, func(ShareState) {})
	if m.GrantsStale() {
		t.Error("with no epoch source at all, nothing should be considered stale")
	}
	m.SetEpochSource(func() string { return "" })
	if m.GrantsStale() {
		t.Error("an unknown tmux identity must not invalidate grants")
	}
}

// A guest whose grants are stale must see NOTHING — the radar can't leak a pane the
// owner never shared.
func TestGuestRadarIsEmptyWhenGrantsAreStale(t *testing.T) {
	raw := []byte(`[{"pane_id":"%17","task":"secret"},{"pane_id":"%20","task":"other"}]`)
	dev := EnrolledDevice{ViewPanes: []string{"%17"}}
	if got := string(filterAgentsForGuest(raw, dev, true)); got != "[]" {
		t.Errorf("stale guest radar = %s; want [] (fail closed)", got)
	}
	// and it still works normally when the grants are current
	if got := string(filterAgentsForGuest(raw, dev, false)); got == "[]" {
		t.Error("a current grant must still show its allowed pane")
	}
}

// Exercise the HTTP contract with synthetic epochs, never a real tmux restart.
func TestShareCapabilityStaleAndRecovery(t *testing.T) {
	h, _, _, share := scopeServer(t)
	epoch := "first-tmux"
	share.SetEpochSource(func() string { return epoch })
	share.StampEpoch()
	minted := post(t, h, "/api/share/new", testToken, `{"label":"test guest","view":["%1"],"input":["%1"]}`)
	var link struct{ ID, Token string }
	if minted.Code != http.StatusOK || json.Unmarshal(minted.Body.Bytes(), &link) != nil || link.ID == "" {
		t.Fatalf("mint guest = %d %s", minted.Code, minted.Body.String())
	}
	guest := link.Token
	for _, stale := range []bool{false, true, false} {
		if stale {
			epoch = "next-tmux"
		} else {
			// Exercise the existing owner re-grant endpoint without widening the link.
			rr := post(t, h, "/api/share/set", testToken, `{"id":"`+link.ID+`","view":["%1"],"input":["%1"]}`)
			if rr.Code != http.StatusOK {
				t.Fatalf("owner re-grant = %d %s", rr.Code, rr.Body.String())
			}
		}
		rr := do(t, h, http.MethodGet, "/api/share", guest)
		var cap shareCapability
		if rr.Code != http.StatusOK || json.Unmarshal(rr.Body.Bytes(), &cap) != nil {
			t.Fatalf("guest capability = %d %s", rr.Code, rr.Body.String())
		}
		if cap.Stale != stale || cap.All || !reflect.DeepEqual(cap.ViewPanes, []string{"%1"}) {
			t.Fatalf("stale=%v capability=%+v", stale, cap)
		}
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(rr.Body.Bytes(), &fields)
		for key := range fields {
			if key != "input" && key != "panes" && key != "view_panes" && key != "stale" {
				t.Fatalf("unexpected guest field %q", key)
			}
		}
		radar := do(t, h, http.MethodGet, "/api/agents", guest)
		pane := do(t, h, http.MethodGet, "/api/pane?id=%251", guest)
		if stale {
			if radar.Code != http.StatusOK || len(paneIDs(t, radar.Body.Bytes())) != 0 || pane.Code != http.StatusForbidden {
				t.Fatalf("stale guest radar=%s pane status=%d", radar.Body.String(), pane.Code)
			}
		} else if len(paneIDs(t, radar.Body.Bytes())) != 1 || pane.Code != http.StatusOK {
			t.Fatalf("current guest radar=%s pane status=%d", radar.Body.String(), pane.Code)
		}
		owner := do(t, h, http.MethodGet, "/api/share", testToken)
		var ownerCap shareCapability
		_ = json.Unmarshal(owner.Body.Bytes(), &ownerCap)
		if !ownerCap.All || ownerCap.Stale {
			t.Fatalf("owner capability=%+v", ownerCap)
		}
	}
}
