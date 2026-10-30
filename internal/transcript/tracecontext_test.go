package transcript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func contextMessage(text string) string {
	value := map[string]any{"type": "response_item", "timestamp": "2026-10-30T10:00:00Z", "payload": map[string]any{"type": "message", "role": "user", "content": []any{map[string]any{"type": "input_text", "text": text}}}}
	encoded, _ := json.Marshal(value)
	return string(encoded) + "\n"
}
func contexts(turn Turn) []ContextRecord {
	var out []ContextRecord
	for _, segment := range turn.Segments {
		if segment.Context != nil {
			out = append(out, *segment.Context)
		}
	}
	return out
}
func TestContextPrefixSurvivesCodexIncrementalReadWithoutInventedTurn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	if err := os.WriteFile(path, []byte(contextMessage("# AGENTS.md instructions\nUse the local fixture")), 0600); err != nil {
		t.Fatal(err)
	}
	loader := &Loader{}
	before, err := loader.loadCodexFile(path, 20)
	if err != nil || len(before) != 0 {
		t.Fatalf("prefix made a fake turn: %v %v", before, err)
	}
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	_, _ = f.WriteString(contextMessage("Check retries") + chatRecord("agent_message", "Inspecting"))
	_ = f.Close()
	turns, err := loader.loadCodexFile(path, 20)
	if err != nil || len(turns) != 1 || turns[0].Prompt != "Check retries" || turns[0].Response != "Inspecting" {
		t.Fatalf("public conversation changed: %#v %v", turns, err)
	}
	ctx := contexts(turns[0])
	if len(ctx) != 1 || !strings.Contains(ctx[0].Text, "local fixture") || ctx[0].Source != "codex/response_item.message.user" {
		t.Fatalf("context lost: %#v", ctx)
	}
}
func TestContextOrderAndBudgetDoNotChangePromptsOrReplies(t *testing.T) {
	st := &parseState{}
	st.open("Human instruction", "now")
	st.addText("Before")
	st.promptContexts("<task-notification>Background result</task-notification>", "claude/user.content", "later")
	st.addText("After")
	for i := 0; i < 100; i++ {
		st.addContext("Environment", strings.Repeat("文", 10000), "fixture/context", "")
	}
	st.flush()
	turn := st.turns[0]
	if turn.Prompt != "Human instruction" || turn.Response != "Before\n\nAfter" || turn.Segments[1].Context == nil {
		t.Fatalf("ordering/public text: %#v", turn)
	}
	total := 0
	for _, ctx := range contexts(turn) {
		total += len(ctx.Text)
		if len(ctx.Text) > maxContextBytes {
			t.Fatal("per-record budget")
		}
	}
	if total > maxTurnContextBytes || len(contexts(turn)) > maxTurnContexts {
		t.Fatal("turn budget")
	}
}
func TestContextDoesNotExtractToolOutputInlineExamplesOrReasoning(t *testing.T) {
	st := &parseState{}
	st.open("Review", "now")
	for _, text := range []string{"Explain <system-reminder>example</system-reminder>", "```xml\n<system-reminder>example</system-reminder>\n```", "<system-reminder>incomplete"} {
		st.promptContexts(text, "fixture", "")
	}
	codexStep(`{"type":"response_item","payload":{"type":"reasoning","summary":[{"text":"private"}]} }`, st)
	codexStep(`{"type":"response_item","payload":{"type":"function_call_output","output":"<system-reminder>not context</system-reminder>"}}`, st)
	if len(contexts(*st.cur)) != 0 {
		t.Fatal("non-context content was collected")
	}
}

func TestCodexMirroredContextAppearsOnceAndLaterRepeatsRemain(t *testing.T) {
	st := &parseState{}
	text := "<system-reminder>First fixture</system-reminder>\n<task-notification>Second fixture</task-notification>"
	codexStep(strings.TrimSpace(contextMessage(text)), st)
	encoded, _ := json.Marshal(map[string]any{"type": "event_msg", "payload": map[string]any{"type": "user_message", "message": text}})
	codexStep(string(encoded), st)
	codexStep(strings.TrimSpace(contextMessage("Human instruction")), st)
	if got := len(contexts(*st.cur)); got != 2 {
		t.Fatalf("mirrored prefix has %d contexts", got)
	}
	st.addText("Public update")
	st.promptContexts("<system-reminder>First fixture</system-reminder>", "codex/event_msg.user_message", "later")
	if got := len(contexts(*st.cur)); got != 3 {
		t.Fatalf("later genuine repeat was dropped: %d", got)
	}
}

func TestProviderContextKeepsTheHumanTurnAndItsRecordedSource(t *testing.T) {
	for _, provider := range []string{"claude", "kimi", "dsh"} {
		t.Run(provider, func(t *testing.T) {
			st := &parseState{}
			st.open("Human instruction", "2026-10-30T10:00:00Z")
			switch provider {
			case "claude":
				claudeStep(`{"type":"user","timestamp":"2026-10-30T10:01:00Z","message":{"role":"user","content":"<system-reminder>Fixture context</system-reminder>"}}`, st)
			case "kimi":
				kimiStep(`{"type":"context.append_message","time":1793354460000,"message":{"role":"user","origin":{"kind":"injection"},"content":[{"type":"text","text":"Fixture context"}]}}`, st)
			case "dsh":
				if err := dshStep([]byte(`{"type":"user/message","surfaceOp":"append","time":1793354460000,"seq":3,"data":{"role":"user","source":{"kind":"injection"},"content":[{"type":"text","text":"Fixture context"}]}}`), st); err != nil {
					t.Fatal(err)
				}
			}
			st.addText("Public reply")
			st.flush()
			if len(st.turns) != 1 || st.turns[0].Prompt != "Human instruction" || st.turns[0].Response != "Public reply" {
				t.Fatalf("public turn changed: %#v", st.turns)
			}
			ctx := contexts(st.turns[0])
			if len(ctx) != 1 || !strings.HasPrefix(ctx[0].Source, provider+"/") || !strings.Contains(ctx[0].Text, "Fixture context") {
				t.Fatalf("missing provenance: %#v", ctx)
			}
		})
	}
}
