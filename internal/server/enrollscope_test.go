package server

import (
	"encoding/json"
	"net/http"
	"testing"
)

// A guest link must not be tradeable for a device token.
//
// The property, not the mechanism: whatever a guest can reach, it must not come back
// holding a credential that reaches more. Minting a pairing code was the route — a guest
// called POST /api/enroll/mint, redeemed the code at the unauthenticated /api/enroll, and
// held a DEVICE token, which is what the owner's own phone holds.
//
// Written as the whole chain rather than as "mint returns 403", because the 403 is the
// mechanism and the escalation is the thing that must stay impossible. A second way to
// obtain a code would pass a test that only watched this endpoint.
func TestAGuestCannotTradeItsLinkForADeviceToken(t *testing.T) {
	h, _, _, _, guestTok := bindingServer(t)

	// Routes a guest is refused today. If a guest ever holds a token that reaches these,
	// the link stopped being a restricted view.
	ownerOnly := []string{"/api/hq/board", "/api/devices"}
	for _, p := range ownerOnly {
		if rr := do(t, h, http.MethodGet, p, guestTok); rr.Code != http.StatusForbidden {
			t.Fatalf("precondition: guest GET %s = %d, want 403", p, rr.Code)
		}
	}

	rr := post(t, h, "/api/enroll/mint", guestTok, `{}`)
	if rr.Code != http.StatusForbidden {
		t.Errorf("guest POST /api/enroll/mint = %d, want 403", rr.Code)
	}

	// Follow it through anyway: a code minted here, redeemed, must not outrank the guest.
	var body struct {
		EnrollCode string `json:"enrollCode"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body.EnrollCode == "" {
		return // nothing to redeem, which is the point
	}
	var got struct {
		Token string `json:"token"`
	}
	red := post(t, h, "/api/enroll", "", `{"enrollCode":"`+body.EnrollCode+`","name":"x"}`)
	_ = json.Unmarshal(red.Body.Bytes(), &got)
	if got.Token == "" {
		return
	}
	for _, p := range ownerOnly {
		if rr := do(t, h, http.MethodGet, p, got.Token); rr.Code == http.StatusOK {
			t.Errorf("a token obtained from a guest link reaches %s — the link was traded up", p)
		}
	}
}

// The legitimate callers must keep working: the Mac's own master token, and a paired
// device showing a pairing QR for a new surface (mobileapp's enrollMint, macapp's
// Pairing.swift). A gate that closed those would be a different outage.
func TestMasterAndDeviceStillMint(t *testing.T) {
	h, _, devTok, _, _ := bindingServer(t)
	for name, tok := range map[string]string{"master": testToken, "device": devTok} {
		rr := post(t, h, "/api/enroll/mint", tok, `{}`)
		if rr.Code != http.StatusOK {
			t.Errorf("%s POST /api/enroll/mint = %d, want 200 — pairing a new surface broke", name, rr.Code)
		}
		var body struct {
			EnrollCode string `json:"enrollCode"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil || body.EnrollCode == "" {
			t.Errorf("%s mint returned no code: %s", name, rr.Body.String())
		}
	}
}
