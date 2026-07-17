package app

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// parseTagName pulls the tag out of GitHub's releases/latest JSON; parseJsdelivrLatest
// out of jsdelivr's newest-first versions list (the CN-reachable fallback). Both
// normalize to a "vX.Y.Z" tag (jsdelivr adds the missing "v").
func TestParseTagName(t *testing.T) {
	if got := parseTagName(`{"url":"x","tag_name":"v0.12.40","draft":false}`); got != "v0.12.40" {
		t.Errorf("parseTagName = %q, want v0.12.40", got)
	}
	if got := parseTagName(`{"message":"Not Found"}`); got != "" {
		t.Errorf("parseTagName(no tag) = %q, want empty", got)
	}
}

func TestParseJsdelivrLatest(t *testing.T) {
	body := `{"type":"github","name":"gtmux","versions":[{"version":"0.12.40"},{"version":"0.12.39"}]}`
	if got := parseJsdelivrLatest(body); got != "v0.12.40" {
		t.Errorf("parseJsdelivrLatest = %q, want v0.12.40 (v prepended)", got)
	}
	// Already tag-shaped → unchanged; empty list → "".
	if got := parseJsdelivrLatest(`{"versions":[{"version":"v1.2.3"}]}`); got != "v1.2.3" {
		t.Errorf("parseJsdelivrLatest(v-prefixed) = %q, want v1.2.3", got)
	}
	if got := parseJsdelivrLatest(`{"versions":[]}`); got != "" {
		t.Errorf("parseJsdelivrLatest(empty) = %q, want empty", got)
	}
	if got := parseJsdelivrLatest(`not json`); got != "" {
		t.Errorf("parseJsdelivrLatest(garbage) = %q, want empty", got)
	}
}

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

// The menu-bar app self-update wedged forever on "Updating…" because install.sh
// relaunched the swapped app with a bare `open`, which re-activates a not-yet-exited
// old instance instead of launching the new binary. The relaunch MUST use `open -n`
// (force a new instance); the app's single-instance guard terminates the old one.
// This pins that so a future edit can't regress to the stuck-spinner behavior.
func TestInstallScriptRelaunchesAppWithOpenN(t *testing.T) {
	b, err := os.ReadFile("../../install.sh")
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `open -n "${APP_DIR}/Gtmux.app"`) {
		t.Errorf("install.sh must relaunch the menu-bar app with `open -n` (force a new instance), " +
			"else a lingering old instance is re-activated and the app hangs on \"Updating…\"")
	}
	// And it must NOT relaunch with a bare `open "${APP_DIR}/Gtmux.app"` (the bug).
	if strings.Contains(s, `open "${APP_DIR}/Gtmux.app"`) {
		t.Errorf("install.sh still relaunches the app with a bare `open` — must be `open -n`")
	}
}

// fetchLatestTag is intentionally NOT unit-tested: it does a live HTTP GET to
// api.github.com and inlines its tag_name parse (no extractable pure helper).
// Exercising it would require network, which is out of scope for these tests.

// TestRunBounded pins that a wedged external command can't hang the caller — the
// timeout kills it (this backstops the `launchctl kickstart -k` that froze a user's
// `gtmux doctor --fix`). A quick command returns cleanly.
func TestRunBounded(t *testing.T) {
	// A command that outruns the deadline is killed → non-nil error, and it returns
	// close to the timeout (not the full sleep).
	start := time.Now()
	if err := runBounded(150*time.Millisecond, "sleep", "10"); err == nil {
		t.Fatal("runBounded should error when the command exceeds the timeout")
	}
	if el := time.Since(start); el > 3*time.Second {
		t.Fatalf("runBounded didn't bound the runtime: took %v", el)
	}
	// A fast command completes cleanly within the timeout.
	if err := runBounded(5*time.Second, "true"); err != nil {
		t.Fatalf("runBounded(true) = %v, want nil", err)
	}
}
