package agentprofile

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestAgyAutoModelRouterGuardAndHomeIsolation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AGY_CONFIG_DIR", "")

	// "auto" in hook modelName is a router placeholder and must not emit a concrete model fact
	// or clobber the concrete PLANNER_RESPONSE.model recorded in transcript.jsonl.
	if facts := HookFacts("agy", []byte(`{"conversationId":"c-auto","modelName":"auto"}`), 1764211900000); len(facts) != 0 {
		t.Fatalf("HookFacts(modelName=auto) = %+v, want empty", facts)
	}

	homeA := t.TempDir()
	homeB := t.TempDir()
	logPath := filepath.Join(homeA, "transcript.jsonl")
	logLine := `{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-11-27T02:50:20Z","model":"gemini-4-carbon-medium","content":"ok"}` + "\n"
	if err := os.WriteFile(logPath, []byte(logLine), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AGY_HOME", homeA)
	Observe("agy", "c-auto", "", "Stop", []byte(`{"conversationId":"c-auto","modelName":"auto"}`), false)
	pA := New("agy", "c-auto", false)
	ReadLogs(&pA, []string{logPath}, 0, 0)
	Current(&pA, "")
	if got := observationFact(pA.Facts, "model").Value; got != "gemini-4-carbon-medium" {
		t.Fatalf("model after auto hook = %q, want gemini-4-carbon-medium", got)
	}

	// Concrete model observation in homeA must not leak into homeB for the same session ID.
	Observe("agy", "shared-id", "", "Stop", []byte(`{"conversationId":"shared-id","modelName":"gemini-4-neon-high"}`), false)
	t.Setenv("AGY_HOME", homeB)
	pB := New("agy", "shared-id", false)
	Current(&pB, "")
	if len(pB.Facts) != 0 {
		t.Fatalf("AGY_HOME isolation leaked facts across homes: %+v", pB.Facts)
	}
}
