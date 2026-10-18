package app

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/chenchaoyi/gtmux/internal/userservice"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestInstalledAppCandidatesIncludeSystemInstall(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	system := filepath.Join(dir, "Applications", "Gtmux.app")
	makeBundle := func(path string) {
		t.Helper()
		contents := filepath.Join(path, "Contents")
		if err := os.MkdirAll(contents, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(contents, "Info.plist"), []byte("plist"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	makeBundle(system)
	if got := installedAppCandidates(home, system); !reflect.DeepEqual(got, []string{system}) {
		t.Fatalf("system-level app went undetected: %v", got)
	}
	userApp := filepath.Join(home, "Applications", "Gtmux.app")
	makeBundle(userApp)
	if got := installedAppCandidates(home, system); !reflect.DeepEqual(got, []string{userApp, system}) {
		t.Fatalf("unexpected install precedence: %v", got)
	}
	if err := os.Remove(filepath.Join(userApp, "Contents", "Info.plist")); err != nil {
		t.Fatal(err)
	}
	if got := installedAppCandidates(home, system); !reflect.DeepEqual(got, []string{system}) {
		t.Fatalf("incomplete home bundle hid valid system app: %v", got)
	}
}

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
// true when a real newer tag exists than EITHER the CLI or the installed app; an
// unreachable API reports an error and never claims an update (so the app can't
// prompt on a failed check).
func TestUpdateCheckPayload(t *testing.T) {
	cases := []struct {
		cur, app, latest string
		wantUpdate       bool
		wantErr          bool
	}{
		{"0.12.1", "0.12.1", "0.12.2", true, false},  // both behind → update
		{"0.12.1", "0.12.1", "0.12.1", false, false}, // both current → no update
		{"0.12.1", "0.12.1", "", false, true},        // API unreachable → no update, error set
		{"0.12.2", "0.12.1", "0.12.2", true, false},  // CLI current, APP behind → update (the fix)
		{"0.12.2", "", "0.12.2", false, false},       // CLI current, no app installed → no update
	}
	for _, c := range cases {
		got := updateCheckPayload(c.cur, c.app, c.latest)
		if got.Current != c.cur || got.Latest != c.latest {
			t.Errorf("payload(%q,%q,%q) echoed wrong versions: %+v", c.cur, c.app, c.latest, got)
		}
		if got.Update != c.wantUpdate {
			t.Errorf("payload(%q,%q,%q).Update = %v, want %v", c.cur, c.app, c.latest, got.Update, c.wantUpdate)
		}
		if (got.Error != "") != c.wantErr {
			t.Errorf("payload(%q,%q,%q).Error = %q, wantErr %v", c.cur, c.app, c.latest, got.Error, c.wantErr)
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

// An update restarts every loaded agent that runs the gtmux binary. v1.0.34 restarted
// only serve, so the Direct tunnel client kept the old binary and the tunnel's new status
// file never appeared until the next login.
func TestUpdateRestartsTheDirectTunnelClientToo(t *testing.T) {
	for _, backend := range []string{"launchd", "systemd"} {
		t.Run(backend, func(t *testing.T) {
			loaded := map[string]bool{serveAgentLabel: true, selfTunnelAgentLabel: true}
			var kicked []string
			runner := func(_ context.Context, name string, args ...string) ([]byte, error) {
				label := args[len(args)-1]
				label = label[strings.LastIndex(label, "/")+1:]
				label = strings.TrimSuffix(label, ".service")
				if !loaded[label] {
					return nil, errors.New("not active")
				}
				if args[0] == "kickstart" || name == "systemctl" && args[1] == "restart" {
					kicked = append(kicked, label)
				}
				return nil, nil
			}
			old := serviceManager
			serviceManager = func() userservice.Manager {
				if backend == "launchd" {
					return userservice.Launchd{Home: t.TempDir(), Run: runner}
				}
				return userservice.Systemd{Home: t.TempDir(), Run: runner}
			}
			t.Cleanup(func() { serviceManager = old })
			restartServeAgents()
			if !reflect.DeepEqual(kicked, []string{serveAgentLabel, selfTunnelAgentLabel}) {
				t.Fatalf("restarted %v", kicked)
			}
			delete(loaded, selfTunnelAgentLabel)
			kicked = nil
			restartServeAgents()
			if !reflect.DeepEqual(kicked, []string{serveAgentLabel}) {
				t.Fatalf("restarted inactive service: %v", kicked)
			}
		})
	}
}
