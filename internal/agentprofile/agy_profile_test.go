package agentprofile

import "testing"

func TestAgyProfileHookAndTranscript(t *testing.T) {
	if p := New("agy", "c1", false); p.Partial || p.Name != "Antigravity" {
		t.Fatalf("New(agy) = %+v, want non-partial Antigravity profile", p)
	}
	hookRaw := []byte(`{"conversationId":"c1","modelName":"gemini-4-neon-high","hookEventName":"PreInvocation"}`)
	facts := HookFacts("agy", hookRaw, 1764211800000)
	if len(facts) != 1 || facts[0].Key != "model" || facts[0].Value != "gemini-4-neon-high" {
		t.Fatalf("HookFacts(agy) = %+v, want model=gemini-4-neon-high", facts)
	}

	logLine := []byte(`{"step_index":2,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-11-27T02:50:20Z","model":"gemini-4-neon-high","content":"done"}`)
	pt := decode("agy", logLine)
	if !pt.End || pt.At == 0 || len(pt.Facts) != 1 || pt.Facts[0].Key != "model" || pt.Facts[0].Value != "gemini-4-neon-high" {
		t.Fatalf("decode(agy) = %+v, want End=true and model=gemini-4-neon-high", pt)
	}
}
