package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/terminalkeys"
)

func TestTerminalKeyInputContract(t *testing.T) {
	var writes atomic.Int32
	binding := "one"
	s := New(Config{Token: testToken}, Deps{
		InputTarget: func(string) (terminalkeys.Target, error) { return terminalkeys.Target{Binding: binding}, nil },
		InputKey: func(_, _ string, _ terminalkeys.Event, _ *terminalkeys.ActionRequest) error {
			writes.Add(1)
			time.Sleep(time.Millisecond)
			return nil
		},
	})
	h := s.Handler()
	rr := do(t, h, "GET", "/api/input-capabilities?id=%251", testToken)
	var cap keyCapabilities
	if rr.Code != 200 || json.Unmarshal(rr.Body.Bytes(), &cap) != nil || len(cap.Keys) == 0 {
		t.Fatalf("capabilities %d %s", rr.Code, rr.Body.String())
	}
	request := map[string]any{"id": "%1", "key_event": terminalkeys.Event{Key: "F2"}, "input_context": cap.Context, "send_id": "same-event"}
	body, _ := json.Marshal(request)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := post(t, h, "/api/send", testToken, string(body))
			if r.Code != 200 {
				t.Errorf("duplicate %d %s", r.Code, r.Body.String())
			}
		}()
	}
	wg.Wait()
	if writes.Load() != 1 {
		t.Fatalf("duplicate injected %d times", writes.Load())
	}
	request["key_event"] = terminalkeys.Event{Key: "F3"}
	b, _ := json.Marshal(request)
	if r := post(t, h, "/api/send", testToken, string(b)); r.Code != 409 {
		t.Fatalf("collision %d", r.Code)
	}
	request["send_id"] = "new-event"
	request["key_event"] = terminalkeys.Event{Key: "NotAKey"}
	b, _ = json.Marshal(request)
	if r := post(t, h, "/api/send", testToken, string(b)); r.Code != 400 {
		t.Fatalf("invalid key %d", r.Code)
	}
	request["key_event"] = terminalkeys.Event{Key: "F2"}
	request["text"] = "must not type"
	b, _ = json.Marshal(request)
	if r := post(t, h, "/api/send", testToken, string(b)); r.Code != 400 {
		t.Fatalf("mixed input %d", r.Code)
	}
	delete(request, "text")
	binding = "two"
	b, _ = json.Marshal(request)
	if r := post(t, h, "/api/send", testToken, string(b)); r.Code != 409 {
		t.Fatalf("stale pane %d", r.Code)
	}
	binding = "one"
	cap.Context.ExpiresAt = time.Now().Unix() - 1
	request["input_context"] = cap.Context
	b, _ = json.Marshal(request)
	if r := post(t, h, "/api/send", testToken, string(b)); r.Code != 409 {
		t.Fatalf("expired context %d", r.Code)
	}
	if writes.Load() != 1 {
		t.Fatal("rejected request reached terminal")
	}
	if r := do(t, h, "GET", "/api/input-capabilities?id=%251", ""); r.Code != 401 {
		t.Fatal("capabilities require auth")
	}
}

func TestTerminalKeyInputGuestAndAction(t *testing.T) {
	enroll := NewEnrollManager(nil, nil)
	share := NewShareManager(ShareState{}, nil)
	share.OnBroadcast(enroll.BroadcastGuestScopes)
	on := true
	share.SetConfig(&on, nil, nil)
	guest := enroll.MintGuest("guest", []string{"%1"}, []string{"%1"}, 0)
	other := enroll.MintGuest("other", []string{"%1"}, nil, 0)
	var writes int
	actions := []terminalkeys.Action{{ID: "codex.view-question", Revision: "a", Event: terminalkeys.Event{Key: "ArrowLeft", Modifiers: []string{"shift"}}}}
	s := New(Config{Token: testToken}, Deps{Enroll: enroll, Share: share,
		InputTarget: func(string) (terminalkeys.Target, error) {
			return terminalkeys.Target{Binding: "one", Actions: actions}, nil
		},
		InputKey: func(_, _ string, _ terminalkeys.Event, _ *terminalkeys.ActionRequest) error { writes++; return nil },
	})
	h := s.Handler()
	r := do(t, h, "GET", "/api/input-capabilities?id=%251", guest.Token)
	var cap keyCapabilities
	if r.Code != 200 {
		t.Fatalf("guest caps %d %s", r.Code, r.Body.String())
	}
	json.Unmarshal(r.Body.Bytes(), &cap)
	req := map[string]any{"id": "%1", "input_context": cap.Context, "send_id": "action", "action": terminalkeys.ActionRequest{ID: actions[0].ID, Revision: "a"}}
	body, _ := json.Marshal(req)
	if r := post(t, h, "/api/send", other.Token, string(body)); r.Code != 403 {
		t.Fatalf("wrong guest input %d", r.Code)
	}
	if r := do(t, h, "GET", "/api/input-capabilities?id=%252", guest.Token); r.Code != 403 {
		t.Fatal("ungranted pane metadata exposed")
	}
	actions = nil
	if r := post(t, h, "/api/send", guest.Token, string(body)); r.Code != 409 {
		t.Fatalf("stale action %d", r.Code)
	}
	if writes != 0 {
		t.Fatal("stale action injected")
	}
	req["key_event"] = terminalkeys.Event{Key: "F2"}
	delete(req, "action")
	req["send_id"] = "key"
	body, _ = json.Marshal(req)
	if r := post(t, h, "/api/send", guest.Token, string(body)); r.Code != http.StatusOK {
		t.Fatalf("guest key %d %s", r.Code, r.Body.String())
	}
	on = false
	share.SetConfig(&on, nil, nil)
	if r := post(t, h, "/api/send", guest.Token, string(body)); r.Code != 403 {
		t.Fatalf("revoked duplicate %d", r.Code)
	}
	if writes != 1 {
		t.Fatalf("writes %d", writes)
	}
}

func TestTerminalKeyInputStrictAndUnconfirmed(t *testing.T) {
	var writes int
	s := New(Config{Token: testToken}, Deps{
		InputTarget: func(string) (terminalkeys.Target, error) { return terminalkeys.Target{Binding: "pane"}, nil },
		InputKey: func(_, _ string, _ terminalkeys.Event, _ *terminalkeys.ActionRequest) error {
			writes++
			return fmt.Errorf("write outcome unknown")
		},
	})
	h := s.Handler()
	var cap keyCapabilities
	json.Unmarshal(do(t, h, "GET", "/api/input-capabilities?id=%251", testToken).Body.Bytes(), &cap)
	base := map[string]any{"id": "%1", "send_id": "one", "input_context": cap.Context, "key_event": map[string]any{"key": "F2", "modifiers": []string{}}}
	send := func(v map[string]any) int {
		t.Helper()
		b, _ := json.Marshal(v)
		return post(t, h, "/api/send", testToken, string(b)).Code
	}
	for _, extra := range []string{"text", "key", "enter"} {
		base[extra] = nil
		if send(base) != 400 {
			t.Fatalf("mixed field %s accepted", extra)
		}
		delete(base, extra)
	}
	base["key_event"] = map[string]any{"key": "F2", "modifier": []string{"control"}}
	if send(base) != 400 {
		t.Fatal("misspelled modifier accepted")
	}
	base["key_event"] = terminalkeys.Event{Key: "F2"}
	tampered := cap.Context
	// The public boot ID must not double as the signing secret.
	forged := hmac.New(sha256.New, []byte(cap.Context.Boot))
	caller := sha256.Sum256([]byte("Bearer " + testToken))
	for _, part := range []string{hex.EncodeToString(caller[:]), "%1", cap.Context.Boot, cap.Context.PaneBinding, strconv.FormatInt(cap.Context.ExpiresAt, 10)} {
		forged.Write([]byte(part))
		forged.Write([]byte{0})
	}
	tampered.Proof = hex.EncodeToString(forged.Sum(nil))
	base["input_context"] = tampered
	if send(base) != 409 {
		t.Fatal("unsigned context accepted")
	}
	base["input_context"] = cap.Context
	if writes != 0 {
		t.Fatal("invalid input reached injection")
	}
	first, repeated := send(base), send(base)
	if first != 409 || repeated != 409 || writes != 1 {
		t.Fatalf("unconfirmed injection replayed: %d", writes)
	}
	// Never evict an active receipt to admit a new ID.
	for i := len(s.keyInputs.receipts); i < maxKeyReceipts; i++ {
		s.keyInputs.receipts[fmt.Sprint(i)] = keyReceipt{expires: time.Now().Unix() + 300}
	}
	base["send_id"] = "overflow"
	if send(base) != 503 || writes != 1 {
		t.Fatal("full receipt cache allowed another injection")
	}
	base["send_id"] = "one"
	if send(base) != 409 || writes != 1 {
		t.Fatal("full cache lost the original receipt")
	}
	s.keyInputs.boot = newKeyBoot()
	base["send_id"] = "restart"
	if send(base) != 409 || writes != 1 {
		t.Fatal("old boot replayed")
	}
}

func TestTerminalKeyCatalogCacheRequiresCurrentBoot(t *testing.T) {
	s := New(Config{Token: testToken}, Deps{InputTarget: func(string) (terminalkeys.Target, error) { return terminalkeys.Target{Binding: "pane"}, nil }, InputKey: func(string, string, terminalkeys.Event, *terminalkeys.ActionRequest) error { return nil }})
	read := func(query string) keyCapabilities {
		t.Helper()
		r := do(t, s.Handler(), "GET", "/api/input-capabilities?id=%251"+query, testToken)
		var c keyCapabilities
		if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &c) != nil {
			t.Fatalf("catalog %d %s", r.Code, r.Body)
		}
		return c
	}
	first := read("")
	for _, query := range []string{"&catalog_version=1", "&catalog_version=1&catalog_boot=previous-server"} {
		if len(read(query).Keys) == 0 {
			t.Fatal("same schema version hid the catalog from an older server boot")
		}
	}
	cached := read("&catalog_version=1&catalog_boot=" + first.Context.Boot)
	if len(cached.Keys) != 0 || cached.Context.Boot != first.Context.Boot {
		t.Fatal("current boot did not reuse its catalog")
	}
}
