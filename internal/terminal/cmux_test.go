package terminal

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestCmuxFocusMatchesDecoratedPanelAndReportsMissing(t *testing.T) {
	old := osa
	t.Cleanup(func() { osa = old })
	var scripts []string
	osa = func(script string) (string, error) {
		scripts = append(scripts, script)
		if len(scripts) == 1 {
			return "other-id\tother — shell\nmatch-id\t✦ work — editor\n", nil
		}
		return "ok", nil
	}
	got, err := (cmux{}).FocusTab("work")
	if err != nil || got != "ok" || len(scripts) != 2 || !strings.Contains(scripts[1], `id of terminalItem is "match-id"`) || !strings.Contains(scripts[1], "focus terminalItem") {
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

func TestCmuxViewingAndOrder(t *testing.T) {
	old := osa
	t.Cleanup(func() { osa = old })
	osa = func(string) (string, error) { return "✦ work — editor", nil }
	if !(cmux{}).IsViewing("work") || (cmux{}).IsViewing("other") {
		t.Fatal("focused cmux panel title was not matched")
	}
	osa = func(string) (string, error) { return "", errors.New("cmux unavailable") }
	if (cmux{}).IsViewing("work") || (cmux{}).TabOrder() != nil {
		t.Fatal("unavailable cmux must not suppress notification or invent order")
	}
	osa = func(string) (string, error) { return "one — shell\n✦ two — editor\none — shell", nil }
	if got := (cmux{}).TabOrder(); !reflect.DeepEqual(got, []string{"one", "two"}) {
		t.Fatalf("TabOrder = %q", got)
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
