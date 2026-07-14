package connect

import "testing"

func TestParseTargetGuestLink(t *testing.T) {
	tg, err := ParseTarget("https://gtmux-7a3f.ccy.dev/#t=SECRET", "")
	if err != nil {
		t.Fatalf("share link: %v", err)
	}
	if tg.Scope != ScopeGuest || tg.URL != "https://gtmux-7a3f.ccy.dev" || tg.Token != "SECRET" {
		t.Fatalf("guest target = %+v", tg)
	}
}

func TestParseTargetOwnerHostToken(t *testing.T) {
	tg, err := ParseTarget("192.168.1.20", "TOK")
	if err != nil {
		t.Fatalf("host+token: %v", err)
	}
	if tg.Scope != ScopeOwner || tg.URL != "http://192.168.1.20:8765" || tg.Token != "TOK" {
		t.Fatalf("owner target = %+v (want default scheme+port)", tg)
	}
}

func TestParseTargetOwnerKeepsSchemeAndPort(t *testing.T) {
	tg, _ := ParseTarget("https://host:9000", "T")
	if tg.URL != "https://host:9000" {
		t.Fatalf("url = %q, want kept as-is", tg.URL)
	}
}

func TestParseTargetErrors(t *testing.T) {
	if _, err := ParseTarget("", ""); err == nil {
		t.Error("empty target should error")
	}
	if _, err := ParseTarget("some-host", ""); err == nil {
		t.Error("owner host with no --token should error")
	}
}
