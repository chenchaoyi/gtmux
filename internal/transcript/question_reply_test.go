package transcript

import (
	"encoding/json"
	"strings"
	"testing"
)

func questionReplyEnvelope(answers ...string) string {
	var replies []map[string]string
	for i, answer := range answers {
		id, _ := json.Marshal([]any{"request_user_input_async", "call_example", i})
		replies = append(replies, map[string]string{
			"answer": answer, "question": "Question " + string(rune('A'+i)), "questionItemId": string(id),
		})
	}
	body, _ := json.Marshal(replies)
	return "<send_user_message_question_reply>\n" + string(body) + "\n</send_user_message_question_reply>"
}

func TestQuestionRepliesShowAnswersWithoutTransportMetadata(t *testing.T) {
	one := questionReplyEnvelope("Use the remote link")
	for _, tc := range []struct{ name, raw, want string }{
		{"one answer", one, "Use the remote link"},
		{"unicode and markdown", questionReplyEnvelope("同一个 Wi-Fi\n\n**继续**"), "同一个 Wi-Fi\n\n**继续**"},
		{"delimiter in answer", questionReplyEnvelope("Keep </send_user_message_question_reply> literally"), "Keep </send_user_message_question_reply> literally"},
		{"multiple answers retain associations", questionReplyEnvelope("Yes", "No"), "Question A\nYes\n\nQuestion B\nNo"},
		{"following text", one + "\nKeep this follow-up", "Use the remote link\n\nKeep this follow-up"},
		{"adjacent replies", one + "\n" + questionReplyEnvelope("Then install"), "Use the remote link\n\nThen install"},
		{"empty answer", questionReplyEnvelope("  "), ""},
		{"empty answer with follow-up", questionReplyEnvelope("") + "\nContinue", "Continue"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := CleanUserPrompt(tc.raw)
			if got != tc.want || ok != (tc.want != "") {
				t.Fatalf("got %q / %t, want %q", got, ok, tc.want)
			}
			text, kind := ClassifyUserPrompt(tc.raw)
			wantKind := PromptUser
			if tc.want == "" {
				wantKind = PromptDrop
			}
			if text != tc.want || kind != wantKind {
				t.Fatalf("classification got %q / %q", text, kind)
			}
		})
	}
}

func TestQuestionReplyExamplesAndUnknownShapesRemainVisible(t *testing.T) {
	envelope := questionReplyEnvelope("Yes")
	for _, raw := range []string{
		"Explain this envelope: " + envelope,
		"```xml\n" + envelope + "\n```",
		strings.TrimSuffix(envelope, "</send_user_message_question_reply>"),
		strings.Replace(envelope, `"answer":"Yes"`, `"answer":true`, 1),
		strings.Replace(envelope, `"answer":"Yes"`, `"answer":"Yes","extra":"keep me"`, 1),
		strings.Replace(envelope, "request_user_input_async", "unknown_tool", 1),
		strings.Replace(envelope, "call_example", "", 1),
		strings.Replace(envelope, ",0]", ",-1]", 1),
		strings.Replace(envelope, ",0]", ",null]", 1),
		strings.Replace(envelope, "\n</send_user_message_question_reply>", " extra\n</send_user_message_question_reply>", 1),
		"<send_user_message_question_reply>not JSON</send_user_message_question_reply>",
		"<send_user_message_question_reply>[]</send_user_message_question_reply>",
	} {
		if got, ok := CleanUserPrompt(raw); !ok || got != strings.TrimSpace(raw) {
			t.Fatalf("unknown or quoted content changed: %q => %q / %t", raw, got, ok)
		}
	}
}

func TestCodexQuestionReplyWireFormsProduceOneReadableTurn(t *testing.T) {
	raw := questionReplyEnvelope("Proceed")
	st := &parseState{}
	for _, row := range []any{
		map[string]any{"type": "response_item", "payload": map[string]any{"type": "message", "role": "user", "content": []any{map[string]string{"type": "input_text", "text": raw}}}},
		map[string]any{"type": "event_msg", "payload": map[string]string{"type": "user_message", "message": raw}},
		map[string]any{"type": "event_msg", "payload": map[string]string{"type": "agent_message", "message": "Continuing"}},
	} {
		b, _ := json.Marshal(row)
		codexStep(string(b), st)
	}
	st.flush()
	if len(st.turns) != 1 || st.turns[0].Prompt != "Proceed" || st.turns[0].Response != "Continuing" {
		t.Fatalf("transport metadata or duplicate turn survived: %#v", st.turns)
	}
}
