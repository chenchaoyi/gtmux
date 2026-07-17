package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func post(t *testing.T, h http.Handler, target, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// shareServer wires a Server with enroll + share + a recording Send, and returns a
// master/device/guest token to exercise the scope gate.
func shareServer(t *testing.T) (h http.Handler, share *ShareManager, sent *[]string, device, guest string) {
	t.Helper()
	old := sendSettle
	sendSettle = 0
	t.Cleanup(func() { sendSettle = old })

	var calls []string
	enroll := NewEnrollManager(nil, nil)
	share = NewShareManager(ShareState{}, nil)
	share.OnBroadcast(enroll.BroadcastGuestScopes)
	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{
		Enroll: enroll,
		Share:  share,
		Send:   func(id, text, key string, enter bool) error { calls = append(calls, id); return nil },
	})
	dev, _ := enroll.Redeem(enroll.Mint(), "phone")
	return s.Handler(), share, &calls, dev.Token, enroll.MintGuest("guest", nil, nil, 0).Token
}

func TestGuestSend_BlockedWhenOff(t *testing.T) {
	h, _, sent, _, guest := shareServer(t)
	if rr := post(t, h, "/api/send", guest, `{"id":"%1","text":"ls"}`); rr.Code != http.StatusForbidden {
		t.Fatalf("guest send with sharing off = %d, want 403 (%s)", rr.Code, rr.Body.String())
	}
	if len(*sent) != 0 {
		t.Fatalf("a blocked guest must not reach Send: %v", *sent)
	}
}

func TestGuestSend_BlockedWhenNotAllowlisted(t *testing.T) {
	h, share, sent, _, guest := shareServer(t)
	on := true
	share.SetConfig(&on, &[]string{"%2"}, nil) // consent on, but only %2 allowed
	if rr := post(t, h, "/api/send", guest, `{"id":"%1","text":"ls"}`); rr.Code != http.StatusForbidden {
		t.Fatalf("guest send to a non-allowlisted pane = %d, want 403", rr.Code)
	}
	if len(*sent) != 0 {
		t.Fatalf("must not reach Send: %v", *sent)
	}
}

func TestGuestSend_AllowedWhenConsentedAndAllowlisted(t *testing.T) {
	h, share, sent, _, guest := shareServer(t)
	on := true
	share.SetConfig(&on, &[]string{"%1"}, nil)
	if rr := post(t, h, "/api/send", guest, `{"id":"%1","text":"ls"}`); rr.Code != http.StatusOK {
		t.Fatalf("consented+allowlisted guest send = %d, want 200 (%s)", rr.Code, rr.Body.String())
	}
	if len(*sent) != 1 || (*sent)[0] != "%1" {
		t.Fatalf("Send should have run for %%1: %v", *sent)
	}
}

func TestOwnerSend_Unrestricted(t *testing.T) {
	h, _, sent, device, _ := shareServer(t)
	// Master and device (the owner's own) type anywhere regardless of the share policy.
	if rr := post(t, h, "/api/send", testToken, `{"id":"%1","text":"ls"}`); rr.Code != http.StatusOK {
		t.Fatalf("master send = %d, want 200", rr.Code)
	}
	if rr := post(t, h, "/api/send", device, `{"id":"%9","text":"ls"}`); rr.Code != http.StatusOK {
		t.Fatalf("device send = %d, want 200", rr.Code)
	}
	if len(*sent) != 2 {
		t.Fatalf("both owner sends should run: %v", *sent)
	}
}

func TestShareCapability_ByScope(t *testing.T) {
	h, share, _, _, guest := shareServer(t)
	// Guest, sharing off → input false, no panes.
	var gcap shareCapability
	json.Unmarshal(do(t, h, http.MethodGet, "/api/share", guest).Body.Bytes(), &gcap)
	if gcap.Input || len(gcap.Panes) != 0 {
		t.Fatalf("guest with sharing off: %+v, want input=false no panes", gcap)
	}
	// Guest, sharing on with %1 → input true, panes [%1].
	on := true
	share.SetConfig(&on, &[]string{"%1"}, nil)
	json.Unmarshal(do(t, h, http.MethodGet, "/api/share", guest).Body.Bytes(), &gcap)
	if !gcap.Input || len(gcap.Panes) != 1 || gcap.Panes[0] != "%1" {
		t.Fatalf("guest with sharing on: %+v, want input=true panes=[%%1]", gcap)
	}
	// Master capability.
	var mcap shareCapability
	json.Unmarshal(do(t, h, http.MethodGet, "/api/share", testToken).Body.Bytes(), &mcap)
	if !mcap.Input || !mcap.All {
		t.Fatalf("master capability: %+v, want input=true all=true", mcap)
	}
}

// SHARE management is FULL-only (owner-remote-admin, decision B): a guest is
// refused; an owner device is allowed (like the master); the master is allowed.
func TestShareAdmin_FullOnly(t *testing.T) {
	h, _, _, device, guest := shareServer(t)
	// A guest may NOT configure or mint.
	if rr := post(t, h, "/api/share/config", guest, `{"enabled":true}`); rr.Code != http.StatusForbidden {
		t.Errorf("guest config = %d, want 403", rr.Code)
	}
	if rr := post(t, h, "/api/share/new", guest, `{"label":"x"}`); rr.Code != http.StatusForbidden {
		t.Errorf("guest new = %d, want 403", rr.Code)
	}
	// An owner device MAY, exactly like the master.
	for _, tok := range []string{testToken, device} {
		if rr := post(t, h, "/api/share/config", tok, `{"enabled":true,"panes":["%1"]}`); rr.Code != http.StatusOK {
			t.Errorf("full config = %d, want 200 (%s)", rr.Code, rr.Body.String())
		}
		if rr := post(t, h, "/api/share/new", tok, `{"label":"g"}`); rr.Code != http.StatusOK {
			t.Errorf("full new = %d, want 200", rr.Code)
		}
	}
}

// GET /api/share/config returns the full policy to any FULL caller (master or
// owner device — for `gtmux share status` / the phone's manage screen), and 403s a
// guest.
func TestShareConfigGet_FullOnly(t *testing.T) {
	h, share, _, device, guest := shareServer(t)
	on := true
	share.SetConfig(&on, &[]string{"%1", "%2"}, nil)
	for _, tok := range []string{testToken, device} {
		var st ShareState
		json.Unmarshal(do(t, h, http.MethodGet, "/api/share/config", tok).Body.Bytes(), &st)
		if !st.Enabled || len(st.Panes) != 2 {
			t.Fatalf("full GET config = %+v, want enabled + 2 panes", st)
		}
	}
	if rr := do(t, h, http.MethodGet, "/api/share/config", guest); rr.Code != http.StatusForbidden {
		t.Errorf("guest GET config = %d, want 403", rr.Code)
	}
}

func TestShareManager_Allowed(t *testing.T) {
	m := NewShareManager(ShareState{}, nil)
	if m.Allowed("%1") {
		t.Error("off → not allowed")
	}
	on := true
	m.SetConfig(&on, &[]string{"%1"}, nil)
	if !m.Allowed("%1") {
		t.Error("on + allowlisted → allowed")
	}
	if m.Allowed("%2") {
		t.Error("on + not allowlisted → not allowed")
	}
	off := false
	m.SetConfig(&off, nil, nil)
	if m.Allowed("%1") {
		t.Error("consent off → not allowed even if still listed")
	}
}
