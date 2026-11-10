package server

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestMSD003HTTPPreservesActualReceiptAndLegacySend(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, state := range []string{"queued", "accepted", "submitted"} {
		t.Run(state, func(t *testing.T) {
			calls := 0
			s := New(Config{Token: testToken}, Deps{SendWithReceipt: func(id, text, key string, enter bool, sendID string) (InputReceipt, error) {
				calls++
				if id != "%1" || text != "synthetic" || !enter || sendID != "same-id" {
					t.Fatal(id, text, key, sendID)
				}
				return InputReceipt{State: state}, nil
			}, Send: func(string, string, string, bool, string) error { t.Fatal("legacy called twice"); return nil }})
			rr := post(t, s.Handler(), "/api/send", testToken, `{"id":"%1","text":"synthetic","enter":true,"send_id":"same-id"}`)
			var got sendResponse
			if rr.Code != 200 || json.Unmarshal(rr.Body.Bytes(), &got) != nil || got.Status != "ok" || got.Delivery == nil || got.Delivery.State != state || got.Delivery.SendID != "same-id" || calls != 1 {
				t.Fatal(rr.Code, rr.Body.String(), calls)
			}
			rr = post(t, s.Handler(), "/api/send", "bad-token", `{"id":"%1","text":"synthetic"}`)
			if rr.Code != 401 || calls != 1 {
				t.Fatal(rr.Code, calls)
			}
		})
	}
	calls := 0
	s := New(Config{Token: testToken}, Deps{Send: func(string, string, string, bool, string) error { calls++; return nil }})
	rr := post(t, s.Handler(), "/api/send", testToken, `{"id":"%1","text":"synthetic"}`)
	var got sendResponse
	if rr.Code != 200 || json.Unmarshal(rr.Body.Bytes(), &got) != nil || got.Delivery != nil || calls != 1 {
		t.Fatal(rr.Body.String(), calls)
	}
	s = New(Config{Token: testToken}, Deps{SendWithReceipt: func(string, string, string, bool, string) (InputReceipt, error) {
		return InputReceipt{}, errors.New("approval pending")
	}})
	rr = post(t, s.Handler(), "/api/send", testToken, `{"id":"%1","text":"synthetic"}`)
	if rr.Code != 400 {
		t.Fatal(rr.Code)
	}
}

func TestMSD003PreferredReceiptCannotBypassGuestScope(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	enroll := NewEnrollManager(nil, nil)
	share := NewShareManager(ShareConfig{}, nil)
	calls := 0
	s := New(Config{Token: testToken}, Deps{Enroll: enroll, Share: share, SendWithReceipt: func(string, string, string, bool, string) (InputReceipt, error) {
		calls++
		return InputReceipt{State: "accepted"}, nil
	}})
	guest := enroll.MintGuest("guest", []string{"%1"}, []string{"%1"}, 0).Token
	body := `{"id":"%1","text":"synthetic","enter":true}`
	rr := post(t, s.Handler(), "/api/send", guest, body)
	if rr.Code != 403 || calls != 0 {
		t.Fatal(rr.Code, calls)
	}
	on := true
	share.SetConfig(&on, &[]string{"%1"}, nil)
	rr = post(t, s.Handler(), "/api/send", guest, body)
	if rr.Code != 200 || calls != 1 {
		t.Fatal(rr.Code, calls, rr.Body.String())
	}
	rr = post(t, s.Handler(), "/api/send", guest, `{"id":"%2","text":"synthetic","enter":true}`)
	if rr.Code != 403 || calls != 1 {
		t.Fatal(rr.Code, calls)
	}
}
