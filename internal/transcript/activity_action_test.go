package transcript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// JSON assertions deliberately compile against the old Step too, so the
// baseline control fails on missing facts, never merely on new Go fields.
func actionFacts(t *testing.T, s Step) (string, int) {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var facts struct {
		Action string `json:"action"`
		Count  int    `json:"subject_count"`
	}
	if err := json.Unmarshal(b, &facts); err != nil {
		t.Fatal(err)
	}
	return facts.Action, facts.Count
}

func providerAction(t *testing.T, provider, name string, args json.RawMessage) Step {
	t.Helper()
	st := &parseState{}
	var record any
	switch provider {
	case "claude":
		record = map[string]any{"type": "assistant", "message": map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "tool_use", "name": name, "input": args}}}}
	case "codex":
		record = map[string]any{"type": "response_item", "payload": map[string]any{"type": "function_call", "name": name, "call_id": "fixture-call", "arguments": string(args)}}
	case "kimi":
		record = map[string]any{"type": "context.append_message", "agentId": "main", "message": map[string]any{"role": "assistant", "toolCalls": []any{map[string]string{"name": name, "arguments": string(args)}}}}
	case "dsh":
		record = map[string]any{"seq": 1, "type": "tool/call", "surfaceOp": "append", "data": map[string]string{"name": name, "arguments": string(args)}}
	case "opencode":
		t.Setenv("HOME", t.TempDir())
		t.Setenv("XDG_DATA_HOME", "")
		event, _ := json.Marshal(map[string]any{"kind": "tool", "name": name, "id": "fixture-call", "input": args})
		AppendOpencodeEvent("action-fixture", event)
		body, err := os.ReadFile(opencodeSessionFile("action-fixture"))
		if err != nil {
			t.Fatal(err)
		}
		opencodeStep(strings.TrimSpace(string(body)), st)
	default:
		t.Fatal(provider)
	}
	if record != nil {
		b, _ := json.Marshal(record)
		switch provider {
		case "claude":
			claudeStep(string(b), st)
		case "codex":
			codexStep(string(b), st)
		case "kimi":
			kimiStep(string(b), st)
		case "dsh":
			if err := dshStep(b, st); err != nil {
				t.Fatal(err)
			}
		}
	}
	st.flush()
	steps := nativeTestSteps(st)
	if len(steps) != 1 {
		t.Fatalf("%s invented/lost calls: %d", provider, len(steps))
	}
	return steps[0]
}

func TestASA001RecordedSkillIdentityAcrossFiveAdapters(t *testing.T) {
	for _, provider := range []string{"claude", "codex", "kimi", "dsh", "opencode"} {
		t.Run(provider, func(t *testing.T) {
			path := "/synthetic/skills/design-kpis/SKILL.md"
			args, _ := json.Marshal(map[string]string{"file_path": path})
			s := providerAction(t, provider, "Read", args)
			a, n := actionFacts(t, s)
			if s.Operation != "read" || a != "skill" || n != 1 || s.Detail != "design-kpis" || s.Input != path {
				t.Fatalf("recorded skill identity lost: %#v", s)
			}
		})
	}
	s := toolStep("Skill", "Skill", json.RawMessage(`{"skill":"design-kpis"}`), "")
	if a, _ := actionFacts(t, s); a != "skill" || s.Detail != "design-kpis" {
		t.Fatalf("explicit skill lost: %#v", s)
	}
}

func TestASA001ShellTextAndBareNamesDoNotInventSkillReads(t *testing.T) {
	for _, tc := range []struct{ name, args string }{
		{"exec_command", `{"cmd":"cat /skills/design-kpis/SKILL.md"}`},
		{"functions.exec", `{"code":"read('skills/design-kpis/SKILL.md')"}`},
		{"Read", `{"path":"SKILL.md"}`},
		{"Read", `{"path":"/unrelated/design-kpis/SKILL.md"}`},
		{"Read", `{"path":"/skills/else/../design-kpis/SKILL.md"}`},
	} {
		s := toolStep(tc.name, tc.name, json.RawMessage(tc.args), "")
		if a, _ := actionFacts(t, s); a != "" {
			t.Fatal("guessed skill from text/basename", s)
		}
	}
}

func TestASA002WebIntentUsesExplicitMethodsOrCompleteActions(t *testing.T) {
	for _, tc := range []struct{ name, args, action, detail string }{
		{"WebSearch", `{"query":"official metrics"}`, "web_search", "official metrics"},
		{"WebFetch", `{"url":"https://example.org/guide"}`, "web_open", "https://example.org/guide"},
		{"web.run", `{"search_query":[{"q":"delivery metrics"}],"response_length":"short"}`, "web_search", "delivery metrics"},
		{"web.run", `{"open":[{"ref_id":"https://example.org"}]}`, "web_open", "https://example.org"},
		{"web.run", `{"find":[{"ref_id":"page","pattern":"outcome"}]}`, "web_find", "outcome"},
		{"web__run", `{"click":[{"ref_id":"page","id":3}]}`, "web_open", "page"},
		{"web.run", `{"click":[{"ref_id":"page","id":0.5}]}`, "", ""},
		{"web.run", `{"open":[{"ref_id":"page"},42]}`, "", ""},
		{"web.run", `{"find":[{"ref_id":"page","pattern":1}]}`, "", ""},
		{"web.run", `{"search_query":[{"q":"q"}],"open":[{"ref_id":"page"}]}`, "", ""},
		{"web.run", `{"search_query":[{"q":42}]}`, "", ""},
		{"web.run", `{"search_query":[{"q":"q"}],"weather":[{"location":"x"}]}`, "", ""},
		{"web.run", `not-json`, "", ""},
	} {
		t.Run(tc.name+tc.args, func(t *testing.T) {
			s := toolStep(tc.name, tc.name, json.RawMessage(tc.args), "")
			a, _ := actionFacts(t, s)
			if s.Operation != "web" || a != tc.action || tc.detail != "" && s.Detail != tc.detail {
				t.Fatalf("web intent lost or guessed: %#v", s)
			}
		})
	}
	for _, provider := range []string{"claude", "codex", "kimi", "dsh", "opencode"} {
		t.Run(provider, func(t *testing.T) {
			s := providerAction(t, provider, "WebSearch", json.RawMessage(`{"query":"provider query"}`))
			if a, _ := actionFacts(t, s); a != "web_search" {
				t.Fatal(s)
			}
		})
	}
}

func TestASA003NativeTargetCountEnrichesOneCallAndKeepsScopedInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	prompt := chatRecord("user_message", "q")
	function := `{"type":"response_item","payload":{"type":"function_call","name":"exec_command","call_id":"same-call","arguments":"{\"cmd\":\"cat a.md b.md c.md\"}"}}` + "\n"
	if err := os.WriteFile(path, []byte(prompt+function), 0600); err != nil {
		t.Fatal(err)
	}
	l := &Loader{}
	before, err := l.loadCodexFile(path, 300)
	if err != nil {
		t.Fatal(err)
	}
	ref := before[0].Segments[0].Steps[0].InputRef
	values := map[string]any{"id": "same-call", "command": []string{"bash", "-c", "cat a.md b.md c.md"}, "parsed_cmd": []any{map[string]string{"type": "read", "path": "/a/a.md"}, map[string]string{"type": "read", "path": "/b/b.md"}, map[string]string{"type": "read", "path": "/c/c.md"}}}
	record := nativeTestRecord("CommandExecution", values)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString(record + record)
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		t.Fatal(err, closeErr)
	}
	turns, err := l.loadCodexFile(path, 300)
	if err != nil {
		t.Fatal(err)
	}
	steps := turns[0].Segments[0].Steps
	if len(steps) != 1 {
		t.Fatal("mirrors invented calls", len(steps))
	}
	s := steps[0]
	_, n := actionFacts(t, s)
	if n != 3 || s.Operation != "read" || s.Input != "cat a.md b.md c.md" || s.InputRef != ref || s.Detail != "a.md, b.md, c.md" {
		t.Fatalf("target facts replaced source input: %#v", s)
	}
	full, err := readToolInput(path, codexStep, int64(len(prompt)), ref)
	if err != nil || full.Input != s.Input || full.DetailsTruncated {
		t.Fatal("scoped original input lost", full, err)
	}
}

func TestASA003NativeTargetsStayDistinctWithoutGuessingMixedCommands(t *testing.T) {
	for _, tc := range []struct {
		name, parsed, operation, action string
		count                           int
	}{
		{"four", `[{"type":"read","path":"/a/a.md"},{"type":"read","path":"/b/b.md"},{"type":"read","path":"/c/c.md"},{"type":"read","path":"/d/d.md"}]`, "read", "", 4},
		{"duplicate", `[{"type":"read","path":"/a/a.md"},{"type":"read","path":"/a/a.md"}]`, "read", "", 1},
		{"mixed-skill", `[{"type":"read","path":"/skills/design-kpis/SKILL.md"},{"type":"read","path":"/a/a.md"}]`, "read", "", 2},
		{"one-skill", `[{"type":"read","path":"/skills/design-kpis/SKILL.md"}]`, "read", "skill", 1},
		{"missing", `[{"type":"read","path":"/a/a.md"},{"type":"read"}]`, "read", "", 0},
		{"mixed-command", `[{"type":"read","path":"/a/a.md"},{"type":"unknown"}]`, "command", "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var parsed any
			if err := json.Unmarshal([]byte(tc.parsed), &parsed); err != nil {
				t.Fatal(err)
			}
			st := &parseState{}
			record := nativeTestRecord("CommandExecution", map[string]any{"id": "one-call", "command": []string{"bash", "-c", "recorded original command"}, "parsed_cmd": parsed})
			codexNativeEvent(strings.TrimSpace(record), st)
			st.flush()
			steps := nativeTestSteps(st)
			if len(steps) != 1 {
				t.Fatal("target count became tool calls", len(steps))
			}
			a, n := actionFacts(t, steps[0])
			if a != tc.action || n != tc.count || steps[0].Operation != tc.operation {
				t.Fatalf("incorrect recorded targets: %#v", steps[0])
			}
		})
	}
}

func TestASA003ActionMetadataParticipatesInRetentionBudget(t *testing.T) {
	a := Step{Kind: "tool", Title: "Read"}
	b := a
	if err := json.Unmarshal([]byte(`{"action":"skill","subject_count":1}`), &b); err != nil {
		t.Fatal(err)
	}
	turns := func(s Step) []Turn { return []Turn{{Segments: []Segment{{Steps: []Step{s}}}}} }
	if cachedTurnBytes(turns(b))-cachedTurnBytes(turns(a)) < int64(len("skill")) {
		t.Fatal("new retained metadata omitted from cache budget")
	}
}
