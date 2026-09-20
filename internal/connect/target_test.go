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

// normalizeHost must put the default port on the HOST, never after a path — a
// path-based tunnel target (https://tunnel.example/pNNNNN) previously became
// "…/pNNNNN:8765", which reaches nothing.
func TestNormalizeHost(t *testing.T) {
	cases := []struct{ in, want string }{
		{"192.168.1.20", "http://192.168.1.20:8765"},                       // bare host → serve default
		{"http://192.168.1.20", "http://192.168.1.20:8765"},                // explicit http, no port
		{"192.168.1.20:9000", "http://192.168.1.20:9000"},                  // explicit port preserved
		{"http://host:9000/", "http://host:9000"},                          // trailing slash trimmed
		{"https://tunnel.ccy.dev/p35047", "https://tunnel.ccy.dev/p35047"}, // tunnel: untouched
		{"https://host.example", "https://host.example"},                   // https → 443, no bogus port
		{"", ""},
	}
	for _, c := range cases {
		if got := normalizeHost(c.in); got != c.want {
			t.Errorf("normalizeHost(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

// A share link is its short code now, and it is the same link whichever way it arrived:
// pasted whole, or typed as the address plus the code someone read out. Both land on a
// guest connection that redeems the code and keeps the token (share-link-is-the-code).
func TestParseTarget_ShortShareLink(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // the bare-host path reads remotes.json
	for _, arg := range []string{
		"https://tunnel.ccy.dev/p35047#code=4F7K-Q9X2",
		"https://tunnel.ccy.dev/p35047/#code=4F7K-Q9X2",
	} {
		got, err := ParseTarget(arg, "")
		if err != nil {
			t.Fatalf("%s: %v", arg, err)
		}
		if got.URL != "https://tunnel.ccy.dev/p35047" {
			t.Errorf("%s: url = %q", arg, got.URL)
		}
		if got.EnrollCode != "4F7K-Q9X2" || got.Scope != ScopeGuest || got.Token != "" {
			t.Errorf("%s: %+v", arg, got)
		}
	}

	// Typing the two lines back gives the same target as pasting the link.
	typed, err := ParseTarget("https://tunnel.ccy.dev/p35047", "", "4f7kq9x2")
	if err != nil {
		t.Fatal(err)
	}
	if typed.URL != "https://tunnel.ccy.dev/p35047" || typed.Scope != ScopeGuest || typed.EnrollCode == "" {
		t.Errorf("the read-out form resolved differently: %+v", typed)
	}

	// The older link, with the raw token in it, still connects.
	old, err := ParseTarget("https://tunnel.ccy.dev/p35047/#g=deadbeef", "")
	if err != nil || old.Token != "deadbeef" || old.Scope != ScopeGuest {
		t.Errorf("an older share link stopped working: %+v %v", old, err)
	}
	// And a pair link is still an owner link: "code" must not swallow "c".
	pair, err := ParseTarget("https://tunnel.ccy.dev/p35047/#c=ABCD1234", "")
	if err != nil || pair.Scope != ScopeOwner || pair.EnrollCode != "ABCD1234" {
		t.Errorf("a pair link was read as a share link: %+v %v", pair, err)
	}
}
