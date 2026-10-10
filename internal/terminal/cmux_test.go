package terminal

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestCmuxFocusByClientTTY(t *testing.T) {
	oldCLI, oldTTY, oldOSA := cmuxCLI, clientTTYsFor, osa
	t.Cleanup(func() { cmuxCLI, clientTTYsFor, osa = oldCLI, oldTTY, oldOSA })

	cmuxCLI = func(args ...string) ([]byte, error) {
		if !reflect.DeepEqual(args, []string{"--json", "--id-format", "uuids", "tree", "--all"}) {
			t.Fatalf("cmux args = %v", args)
		}
		return []byte(`{
		  "active": {"surface_id": "other"},
		  "windows": [{"workspaces": [{"panes": [{"surfaces": [
		    {"id": "other", "title": "plain shell", "tty": "ttys000", "type": "terminal"},
		    {"id": "match-id", "title": "zhangyong@host:~", "tty": "ttys019", "type": "terminal", "focused": true}
		  ]}]}]}]
		}`), nil
	}
	clientTTYsFor = func(session string) []string {
		if session == "work" {
			return []string{"ttys019"}
		}
		return nil
	}
	var scripts []string
	osa = func(script string) (string, error) {
		scripts = append(scripts, script)
		return "ok", nil
	}
	got, err := (cmux{}).FocusTab("work")
	if err != nil || got != "ok" || len(scripts) != 1 {
		t.Fatalf("focus = %q, %v; scripts=%d", got, err, len(scripts))
	}
	if !strings.Contains(scripts[0], `id of terminalItem is "match-id"`) || strings.Contains(scripts[0], "name of terminalItem") {
		t.Fatalf("expected focus-by-id only, got %s", scripts[0])
	}
}

func TestCmuxFocusMatchesTreeTitleWhenTTYMissing(t *testing.T) {
	oldCLI, oldTTY, oldOSA := cmuxCLI, clientTTYsFor, osa
	t.Cleanup(func() { cmuxCLI, clientTTYsFor, osa = oldCLI, oldTTY, oldOSA })
	cmuxCLI = func(args ...string) ([]byte, error) {
		return []byte(`{"windows":[{"workspaces":[{"panes":[{"surfaces":[
		  {"id":"match-id","title":"✦ work — editor","tty":"","type":"terminal"}
		]}]}]}]}`), nil
	}
	clientTTYsFor = func(string) []string { return nil }
	var scripts []string
	osa = func(script string) (string, error) {
		scripts = append(scripts, script)
		return "ok", nil
	}
	got, err := (cmux{}).FocusTab("work")
	if err != nil || got != "ok" || len(scripts) != 1 || !strings.Contains(scripts[0], `id of terminalItem is "match-id"`) {
		t.Fatalf("title focus = %q, %v; scripts=%q", got, err, scripts)
	}
}

func TestCmuxFocusFallsBackToAppleScriptTitle(t *testing.T) {
	oldCLI, oldTTY, oldOSA := cmuxCLI, clientTTYsFor, osa
	t.Cleanup(func() { cmuxCLI, clientTTYsFor, osa = oldCLI, oldTTY, oldOSA })
	cmuxCLI = func(args ...string) ([]byte, error) { return nil, errNoCmuxCLI }
	clientTTYsFor = func(string) []string { return nil }

	var scripts []string
	osa = func(script string) (string, error) {
		scripts = append(scripts, script)
		if len(scripts) == 1 {
			return "other-id\tother — shell\nmatch-id\t✦ work — editor\n", nil
		}
		return "ok", nil
	}
	got, err := (cmux{}).FocusTab("work")
	if err != nil || got != "ok" || len(scripts) != 2 || !strings.Contains(scripts[1], `id of terminalItem is "match-id"`) {
		t.Fatalf("focus = %q, %v; scripts=%q", got, err, scripts)
	}
	scripts = nil
	osa = func(script string) (string, error) {
		scripts = append(scripts, script)
		return "other-id\tother — shell", nil
	}
	got, err = (cmux{}).FocusTab("work")
	if err != nil || got != "notfound" || len(scripts) != 1 {
		t.Fatalf("missing focus = %q, %v; scripts=%q", got, err, scripts)
	}
}

func TestCmuxFocusSinglePanelWhenTitlesAreLiteralTerminal(t *testing.T) {
	oldCLI, oldTTY, oldOSA := cmuxCLI, clientTTYsFor, osa
	t.Cleanup(func() { cmuxCLI, clientTTYsFor, osa = oldCLI, oldTTY, oldOSA })
	cmuxCLI = func(args ...string) ([]byte, error) { return nil, errNoCmuxCLI }
	clientTTYsFor = func(string) []string { return []string{"ttys019"} }
	var scripts []string
	osa = func(script string) (string, error) {
		scripts = append(scripts, script)
		if len(scripts) == 1 {
			return "only-id\tTerminal\n", nil
		}
		return "ok", nil
	}
	got, err := (cmux{}).FocusTab("work")
	if err != nil || got != "ok" || len(scripts) != 2 || !strings.Contains(scripts[1], `id of terminalItem is "only-id"`) {
		t.Fatalf("single-panel focus = %q, %v; scripts=%q", got, err, scripts)
	}
}

func TestCmuxViewingByTTYAndOrder(t *testing.T) {
	oldCLI, oldTTY, oldAll, oldOSA := cmuxCLI, clientTTYsFor, allClientTTYSessions, osa
	t.Cleanup(func() {
		cmuxCLI, clientTTYsFor, allClientTTYSessions, osa = oldCLI, oldTTY, oldAll, oldOSA
	})
	cmuxCLI = func(args ...string) ([]byte, error) {
		return []byte(`{
		  "active": {"surface_id": "surf-work"},
		  "windows": [{"workspaces": [{"panes": [{"surfaces": [
		    {"id": "surf-work", "title": "host:~", "tty": "ttys019", "type": "terminal", "focused": true},
		    {"id": "surf-other", "title": "one — shell", "tty": "ttys020", "type": "terminal"}
		  ]}]}]}]
		}`), nil
	}
	clientTTYsFor = func(session string) []string {
		if session == "work" {
			return []string{"ttys019"}
		}
		return nil
	}
	allClientTTYSessions = func() map[string]string {
		return map[string]string{"ttys019": "work", "ttys020": "one"}
	}
	if !(cmux{}).IsViewing("work") || (cmux{}).IsViewing("other") {
		t.Fatal("focused cmux surface was not matched by tty")
	}
	if got := (cmux{}).TabOrder(); !reflect.DeepEqual(got, []string{"work", "one"}) {
		t.Fatalf("TabOrder = %q", got)
	}

	cmuxCLI = func(args ...string) ([]byte, error) { return nil, errors.New("cmux unavailable") }
	osa = func(string) (string, error) { return "", errors.New("cmux unavailable") }
	if (cmux{}).IsViewing("work") || (cmux{}).TabOrder() != nil {
		t.Fatal("unavailable cmux must not suppress notification or invent order")
	}
}

func TestCmuxSpawnTabsPlanAndFailure(t *testing.T) {
	old := osa
	t.Cleanup(func() { osa = old })
	var calls []string
	osa = func(script string) (string, error) { calls = append(calls, script); return "", nil }
	plan, err := (cmux{}).SpawnTabs([]string{"work one", "other"}, true)
	if err != nil || len(calls) != 0 || !strings.Contains(plan, "attach -t") {
		t.Fatalf("dry-run = %q, %v, calls=%v", plan, err, calls)
	}
	_, err = (cmux{}).SpawnTabs([]string{"work one", "other"}, false)
	if err != nil || len(calls) != 1 || calls[0] != plan {
		t.Fatalf("spawn = %v, calls=%v", err, calls)
	}
	for _, part := range []string{"new window", "new tab in w", "focus terminalItem", "perform action \"send_key:enter\"", " attach -t 'work one'", " attach -t 'other'"} {
		if !strings.Contains(plan, part) {
			t.Errorf("spawn script missing %q: %s", part, plan)
		}
	}
	osa = func(string) (string, error) { return "", errors.New("AppleScript denied") }
	_, err = (cmux{}).SpawnTabs([]string{"work one"}, false)
	if err == nil || !strings.Contains(err.Error(), "AppleScript denied") {
		t.Fatalf("spawn failure should be returned: %v", err)
	}
}

func TestCmuxOpenWindowTargetsOwnApp(t *testing.T) {
	old := osa
	t.Cleanup(func() { osa = old })
	osa = func(script string) (string, error) {
		for _, part := range []string{`application id "com.cmuxterm.app"`, "new window", `input text "echo test"`, `perform action "send_key:enter"`} {
			if !strings.Contains(script, part) {
				t.Errorf("missing %q in %s", part, script)
			}
		}
		return "", nil
	}
	_, _ = (cmux{}).OpenWindow("echo test")
}

func TestParseCmuxTreeSkipsNonTerminals(t *testing.T) {
	surfaces, active, err := parseCmuxTree([]byte(`{
	  "active": {"surface_id": "term-1"},
	  "windows": [{"workspaces": [{"panes": [{"surfaces": [
	    {"id": "term-1", "title": "a — b", "tty": "/dev/ttys001", "type": "terminal"},
	    {"id": "browser-1", "title": "docs", "tty": "", "type": "browser"}
	  ]}]}]}]
	}`))
	if err != nil || active != "term-1" || len(surfaces) != 1 || surfaces[0].TTY != "ttys001" {
		t.Fatalf("parse = %+v active=%q err=%v", surfaces, active, err)
	}
}
