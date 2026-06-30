package app

import (
	"encoding/json"
	"strings"
	"testing"
)

// updateCheckPayload feeds the menu-bar app's "check for updates": `update` is
// true ONLY when a real newer tag exists; an unreachable API reports an error and
// never claims an update (so the app can't prompt on a failed check).
func TestUpdateCheckPayload(t *testing.T) {
	cases := []struct {
		cur, latest string
		wantUpdate  bool
		wantErr     bool
	}{
		{"0.12.1", "0.12.2", true, false},  // newer available
		{"0.12.1", "0.12.1", false, false}, // already current
		{"0.12.1", "", false, true},        // API unreachable → no update, error set
	}
	for _, c := range cases {
		got := updateCheckPayload(c.cur, c.latest)
		if got.Current != c.cur || got.Latest != c.latest {
			t.Errorf("payload(%q,%q) echoed wrong versions: %+v", c.cur, c.latest, got)
		}
		if got.Update != c.wantUpdate {
			t.Errorf("payload(%q,%q).Update = %v, want %v", c.cur, c.latest, got.Update, c.wantUpdate)
		}
		if (got.Error != "") != c.wantErr {
			t.Errorf("payload(%q,%q).Error = %q, wantErr %v", c.cur, c.latest, got.Error, c.wantErr)
		}
		// must round-trip to the {current,latest,update} contract the app parses
		b, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		for _, k := range []string{`"current"`, `"latest"`, `"update"`} {
			if !strings.Contains(string(b), k) {
				t.Errorf("json %s missing key %s", b, k)
			}
		}
	}
}

// installScriptMirrors must lead with the raw GitHub URL, then list the CN
// mirrors (jsdelivr + gh-proxy family). Every entry must be https and reference
// the repo path so each one fetches the same install.sh.
func TestInstallScriptMirrorsShape(t *testing.T) {
	if len(installScriptMirrors) < 3 {
		t.Fatalf("expected GitHub + several mirrors, got %d", len(installScriptMirrors))
	}

	first := installScriptMirrors[0]
	if !strings.HasPrefix(first, "https://raw.githubusercontent.com/") {
		t.Errorf("first mirror must be the raw GitHub URL, got %q", first)
	}

	rest := strings.Join(installScriptMirrors, "\n")
	if !strings.Contains(rest, "cdn.jsdelivr.net") {
		t.Errorf("mirrors must include the jsdelivr CDN")
	}
	if !strings.Contains(rest, "gh-proxy.com") {
		t.Errorf("mirrors must include the gh-proxy mirror")
	}

	for _, u := range installScriptMirrors {
		if !strings.HasPrefix(u, "https://") {
			t.Errorf("mirror not https: %q", u)
		}
		if !strings.Contains(u, "chenchaoyi/gtmux") {
			t.Errorf("mirror missing repo path chenchaoyi/gtmux: %q", u)
		}
		if !strings.HasSuffix(u, "install.sh") {
			t.Errorf("mirror must fetch install.sh: %q", u)
		}
	}
}

// fetchLatestTag is intentionally NOT unit-tested: it does a live HTTP GET to
// api.github.com and inlines its tag_name parse (no extractable pure helper).
// Exercising it would require network, which is out of scope for these tests.
