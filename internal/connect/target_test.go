package connect

import "testing"

func TestParseTargetGuestLink(t *testing.T) {
	// `#g=` is what `gtmux share new` mints; `#t=` is the legacy form old links carry.
	for _, arg := range []string{
		"https://gtmux-7a3f.ccy.dev/#g=SECRET",
		"https://gtmux-7a3f.ccy.dev/#t=SECRET",
	} {
		tg, err := ParseTarget(arg, "")
		if err != nil {
			t.Fatalf("share link %s: %v", arg, err)
		}
		if tg.Scope != ScopeGuest || tg.URL != "https://gtmux-7a3f.ccy.dev" || tg.Token != "SECRET" {
			t.Fatalf("guest target for %s = %+v", arg, tg)
		}
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
	t.Setenv("HOME", t.TempDir()) // isolate from a real remotes.json (owner fallback)
	if _, err := ParseTarget("", ""); err == nil {
		t.Error("empty target should error")
	}
	if _, err := ParseTarget("some-host", ""); err == nil {
		t.Error("owner host with no credential should error")
	}
}

// A pair link (#c=<code>) parses to an OWNER target carrying the enroll code.
func TestParseTarget_PairLink(t *testing.T) {
	tgt, err := ParseTarget("https://gtmux-abc.ccy.dev/#c=deadbeef01234567", "")
	if err != nil {
		t.Fatal(err)
	}
	if tgt.Scope != ScopeOwner || tgt.EnrollCode != "deadbeef01234567" || tgt.Token != "" {
		t.Fatalf("pair target = %+v", tgt)
	}
	if tgt.URL != "https://gtmux-abc.ccy.dev" {
		t.Fatalf("pair URL = %q", tgt.URL)
	}
}

// A bare host with a persisted remote token resolves as owner without --token.
func TestParseTarget_RemotesFallback(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, err := ParseTarget("gtmux-abc.ccy.dev:443", ""); err == nil {
		t.Fatal("no credential anywhere must error")
	}
	if err := SaveRemoteToken("http://gtmux-abc.ccy.dev:443", "tok-123"); err != nil {
		t.Fatal(err)
	}
	tgt, err := ParseTarget("gtmux-abc.ccy.dev:443", "")
	if err != nil {
		t.Fatal(err)
	}
	if tgt.Scope != ScopeOwner || tgt.Token != "tok-123" {
		t.Fatalf("remotes fallback target = %+v", tgt)
	}
	// An explicit --token beats the persisted one.
	tgt, _ = ParseTarget("gtmux-abc.ccy.dev:443", "explicit")
	if tgt.Token != "explicit" {
		t.Fatalf("explicit token must win: %+v", tgt)
	}
}
