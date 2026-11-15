package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// A callback-defined error keeps the old-server negative control compilable.
type sfrCodedRefusal struct{ code string }

func (e sfrCodedRefusal) Error() string           { return "input state unavailable" }
func (e sfrCodedRefusal) SendFailureCode() string { return e.code }

func TestSFR002SendFailureCodeBody(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, preferred := range []bool{false, true} {
		for _, code := range []string{"input-state-unavailable", "input-awaiting-confirmation", "future-code", ""} {
			t.Run(fmt.Sprintf("preferred=%t/%s", preferred, code), func(t *testing.T) {
				calls := 0
				refuse := func(id, text, key string, enter bool, sendID string) error {
					calls++
					if id != "%1" || text != "synthetic" || sendID != "same-id" || !enter {
						t.Fatal("changed request", id, text, key, enter, sendID)
					}
					return fmt.Errorf("wrapped: %w", sfrCodedRefusal{code})
				}
				deps := Deps{Send: refuse}
				if preferred {
					deps.SendWithReceipt = func(id, text, key string, enter bool, sendID string) (InputReceipt, error) {
						return InputReceipt{State: "accepted"}, refuse(id, text, key, enter, sendID)
					}
				}
				s := New(Config{Token: testToken}, deps)
				rr := post(t, s.Handler(), "/api/send", testToken, `{"id":"%1","text":"synthetic","enter":true,"send_id":"same-id"}`)
				var got map[string]any
				if rr.Code != 400 || json.Unmarshal(rr.Body.Bytes(), &got) != nil || calls != 1 {
					t.Fatal(rr.Code, rr.Body.String(), calls)
				}
				if got["error"] != "send failed: wrapped: input state unavailable" || got["delivery"] != nil {
					t.Fatal("legacy error or receipt changed", got)
				}
				if strings.HasPrefix(code, "input-") {
					if got["code"] != code {
						t.Fatalf("known code missing: got %v, want %s", got, code)
					}
				} else if _, exists := got["code"]; exists {
					t.Fatal("unknown code escaped", got)
				}
			})
		}
	}
	s := New(Config{Token: testToken}, Deps{Send: func(string, string, string, bool, string) error { return errors.New("legacy refusal") }})
	rr := post(t, s.Handler(), "/api/send", testToken, `{"id":"%1","text":"synthetic"}`)
	var got map[string]string
	if rr.Code != 400 || json.Unmarshal(rr.Body.Bytes(), &got) != nil || len(got) != 1 || got["error"] != "send failed: legacy refusal" {
		t.Fatal("legacy response changed", rr.Code, rr.Body.String())
	}
}

func TestSFR002SendCodeCannotBypassAuthority(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	enroll := NewEnrollManager(nil, nil)
	share := NewShareManager(ShareState{}, nil)
	calls := 0
	s := New(Config{Token: testToken}, Deps{Enroll: enroll, Share: share, Send: func(string, string, string, bool, string) error {
		calls++
		return sfrCodedRefusal{"input-awaiting-confirmation"}
	}})
	guest := enroll.MintGuest("guest", []string{"%1"}, []string{"%1"}, 0).Token
	for token, status := range map[string]int{"bad-token": 401, guest: 403} {
		rr := post(t, s.Handler(), "/api/send", token, `{"id":"%1","text":"synthetic"}`)
		if rr.Code != status || calls != 0 || strings.Contains(rr.Body.String(), "input-awaiting-confirmation") {
			t.Fatal(rr.Code, rr.Body.String(), calls)
		}
	}
}
