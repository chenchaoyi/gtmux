package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// A paired device must be able to find this Mac after it moves to another Direct server,
// and the only thing it can ask is the Mac itself (openspec/changes/direct-server-choice).

func writeAddresses(t *testing.T, addrs []string) {
	t.Helper()
	writeAddressFile(t, map[string]any{"addresses": addrs})
}

func writeAddressFile(t *testing.T, body any) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(state.Dir(), 0o700); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(body)
	if err := os.WriteFile(state.TunnelAddressesPath(), b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAddressesAreHandedToAPairedDeviceCurrentFirst(t *testing.T) {
	writeAddresses(t, []string{"https://sh.example.test/p35047", "https://la.example.test/p35047"})
	var got addressesReply
	if err := json.Unmarshal([]byte(callAddresses(t)), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Addresses) != 2 || got.Addresses[0] != "https://sh.example.test/p35047" {
		t.Fatalf("addresses = %+v, want the current one first", got.Addresses)
	}
	if got.Current != "https://sh.example.test/p35047" {
		t.Fatalf("current = %q", got.Current)
	}
}

func TestOnlyRealHTTPSAddressesAreHandedOut(t *testing.T) {
	// A client sends its token to whatever comes back, so anything that is not an https
	// URL must never become an address it tries. Duplicates go too: they are just retries.
	writeAddresses(t, []string{
		"https://sh.example.test/p35047",
		"http://sh.example.test/p35047", // plain HTTP
		"://nonsense",
		"https://sh.example.test/p35047", // the same one again
		"https://la.example.test/p35047",
	})
	var got addressesReply
	if err := json.Unmarshal([]byte(callAddresses(t)), &got); err != nil {
		t.Fatal(err)
	}
	want := []string{"https://sh.example.test/p35047", "https://la.example.test/p35047"}
	if len(got.Addresses) != len(want) {
		t.Fatalf("addresses = %+v, want %+v", got.Addresses, want)
	}
	for i := range want {
		if got.Addresses[i] != want[i] {
			t.Fatalf("addresses = %+v, want %+v", got.Addresses, want)
		}
	}
}

func TestNoTunnelMeansAnEmptyListNotAnError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var got addressesReply
	body := callAddresses(t)
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Addresses) != 0 || got.Current != "" {
		t.Fatalf("with no tunnel, got %+v", got)
	}
}

func TestAddressesNeedAToken(t *testing.T) {
	writeAddresses(t, []string{"https://sh.example.test/p35047"})
	s := New(Config{Addr: "x", Token: "tok"}, Deps{})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/addresses", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("an unauthenticated caller got %d, want 401", rec.Code)
	}
}

func callAddresses(t *testing.T) string {
	t.Helper()
	s := New(Config{Addr: "x", Token: "tok"}, Deps{})
	req := httptest.NewRequest(http.MethodGet, "/api/addresses", nil)
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/addresses = %d: %s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// A phone away from the Mac needs to know WHERE its connection goes, so the answer names
// the server as a place rather than as the id gtmux uses internally.
func TestTheAnswerNamesTheServerInBothLanguages(t *testing.T) {
	writeAddressFile(t, map[string]any{
		"addresses": []string{"https://sh.example.test/p35047"},
		"server":    map[string]string{"id": "sh", "en": "Shanghai", "zh": "上海"},
	})
	var got addressesReply
	if err := json.Unmarshal([]byte(callAddresses(t)), &got); err != nil {
		t.Fatal(err)
	}
	if got.Server == nil || got.Server.EN != "Shanghai" || got.Server.ZH != "上海" {
		t.Fatalf("server = %+v, want the place in both languages", got.Server)
	}
}

func TestNothingToNameIsNoServerRatherThanAnEmptyOne(t *testing.T) {
	// A LAN address, the standard tunnel, or a Mac on a provisioner with no server list.
	writeAddressFile(t, map[string]any{"addresses": []string{"https://x.example.test/p1"}})
	var got addressesReply
	if err := json.Unmarshal([]byte(callAddresses(t)), &got); err != nil {
		t.Fatal(err)
	}
	if got.Server != nil {
		t.Fatalf("server = %+v, want none", got.Server)
	}
}

// A tunnel that was already running when gtmux was updated keeps writing the older shape,
// a bare array. Its addresses must still be handed out.
func TestTheOlderFileShapeStillAnswers(t *testing.T) {
	writeAddressFile(t, []string{"https://sh.example.test/p35047", "https://la.example.test/p35047"})
	var got addressesReply
	if err := json.Unmarshal([]byte(callAddresses(t)), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Addresses) != 2 || got.Server != nil {
		t.Fatalf("addresses = %+v server = %+v", got.Addresses, got.Server)
	}
}
