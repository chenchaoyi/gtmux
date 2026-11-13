package transcript

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestASA006NativeSkillSchema(t *testing.T) {
	for _, tc := range []struct{ provider, args string }{
		{"dsh", `{"name":"design-kpis"}`},
		{"opencode", `{"name":"design-kpis"}`},
		{"opencode", `{"id":"design-kpis"}`},
		{"claude", `{"skill":"design-kpis"}`},
	} {
		t.Run(tc.provider+tc.args, func(t *testing.T) {
			step := providerAction(t, tc.provider, "skill", json.RawMessage(tc.args))
			if step.Action != "skill" || step.Detail != "design-kpis" || step.SubjectCount != 1 {
				t.Fatalf("native skill identity missing: %#v", step)
			}
			encoded, _ := json.Marshal(step)
			t.Logf("ASA_NATIVE_FACTS=%s", encoded)
		})
	}
	for _, provider := range []string{"dsh", "opencode"} {
		for _, args := range []string{`{"name":42}`, `{"id":true}`, `{"name":" "}`, `{"name":"design-kpis","id":"other"}`, `{"name":"design-kpis","skill":"other"}`, `{"name":"design-kpis","id":null}`} {
			step := providerAction(t, provider, "skill", json.RawMessage(args))
			if step.Action != "" || step.SubjectCount != 0 {
				t.Fatalf("ambiguous skill identity guessed: %s %s %#v", provider, args, step)
			}
		}
	}
	for _, tc := range []struct{ provider, name string }{{"claude", "skill"}, {"kimi", "skill"}, {"codex", "skill"}, {"opencode", "mcp__foreign__skill"}, {"dsh", "some.skill"}} {
		step := providerAction(t, tc.provider, tc.name, json.RawMessage(`{"name":"do-not-guess","id":"do-not-guess"}`))
		if step.Action != "" || step.SubjectCount != 0 {
			t.Fatalf("foreign name/id became a skill: %#v", step)
		}
	}
}

func TestASA007NativeQueriesSchema(t *testing.T) {
	step := providerAction(t, "dsh", "web_search", json.RawMessage(`{"queries":["official metrics","delivery metrics"]}`))
	if step.Action != "web_search" || step.Detail != "official metrics, delivery metrics" || step.SubjectCount != 0 {
		t.Fatalf("native query preview missing or call count invented: %#v", step)
	}
	encoded, _ := json.Marshal(step)
	t.Logf("ASA_NATIVE_FACTS=%s", encoded)
	for _, args := range []string{`{"queries":[]}`, `{"queries":"not-array"}`, `{"queries":["valid",42]}`, `{"queries":["valid",""]}`, `{"queries":[" "]}`} {
		step := providerAction(t, "dsh", "web_search", json.RawMessage(args))
		if step.Action != "web_search" || step.Detail != "" {
			t.Fatalf("invalid queries guessed: %s %#v", args, step)
		}
	}
	values := make([]string, 65)
	for i := range values {
		values[i] = "query"
	}
	args, _ := json.Marshal(map[string]any{"queries": values})
	if got := providerAction(t, "dsh", "web_search", args); got.Detail != "" {
		t.Fatal("oversized query list accepted", got)
	}
	args, _ = json.Marshal(map[string]any{"queries": []string{strings.Repeat("长", 1000), "second", "third", "fourth"}})
	if got := providerAction(t, "dsh", "web_search", args); got.Action != "web_search" || len([]rune(got.Detail)) > 81 || strings.Contains(got.Detail, "fourth") {
		t.Fatal("native queries unbounded", got)
	}
	for _, provider := range []string{"claude", "codex", "kimi", "opencode"} {
		if got := providerAction(t, provider, "web_search", json.RawMessage(`{"queries":["not-this-provider"]}`)); got.Detail != "" {
			t.Fatal("foreign queries schema guessed", provider, got)
		}
	}
}
