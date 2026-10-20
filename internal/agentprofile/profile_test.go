package agentprofile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/klauspost/compress/zstd"
)

func values(p Profile) map[string]string {
	m := map[string]string{}
	for _, f := range p.Facts {
		m[f.Key] = f.Value
	}
	return m
}
func TestNativeContractsRemainAgentSpecific(t *testing.T) {
	tests := []struct{ agent, line, key, want string }{
		{"claude", `{"type":"assistant","timestamp":"2026-10-20T00:00:00Z","version":"2.1.303","message":{"model":"claude-example","stop_reason":"end_turn"}}`, "version", "2.1.303"},
		{"codex", `{"type":"turn_context","timestamp":"2026-10-20T00:00:00Z","payload":{"model":"gpt-example","effort":"high","collaboration_mode":{"mode":"plan"}}}`, "effort", "high"},
		{"dsh", `{"type":"request/header","time":1792454400000,"data":{"header":{"config":{"provider":"custom","model":"other-model","reasoningEffort":"max"}}}}`, "provider", "custom"},
		{"dsh", `{"type":"plan/mode","time":1792454400000,"data":{"active":true}}`, "mode", "plan"},
		{"kimi", `{"type":"llm.request","agentId":"main","kind":"loop","time":1792454400000,"provider":"local","model":"m","modelAlias":"alias","thinkingEffort":"off"}`, "effort", "off"},
	}
	for _, tc := range tests {
		t.Run(tc.agent+tc.key, func(t *testing.T) {
			p := New(tc.agent, "s", true)
			apply(&p, []point{decode(tc.agent, []byte(tc.line))}, 0, 0)
			if values(p)[tc.key] != tc.want {
				t.Fatal(p)
			}
			if tc.agent != "codex" && values(p)["subscription"] != "" {
				t.Fatal("invented tier")
			}
		})
	}
	for _, line := range []string{`{"type":"metadata","protocol_version":"1.5"}`, `{"type":"llm.request","agentId":"child","kind":"loop","model":"secret-child"}`, `{"type":"llm.request","agentId":"main","kind":"compaction","model":"auxiliary"}`} {
		if x := decode("kimi", []byte(line)); len(x.Facts) > 0 {
			t.Fatal("auxiliary or protocol metadata became profile", x)
		}
	}
	if x := decode("dsh", []byte(`{"type":"session","version":4,"id":"s"}`)); len(x.Facts) > 0 {
		t.Fatal("format became runtime version")
	}
	for _, a := range agents.All() {
		p := New(a.Key, "s", false)
		if p.Name != a.Label || len(p.Facts) != 0 {
			t.Fatal("registry identity acquired invented facts", p)
		}
	}
}

func TestHistoryStopsAtTurnEndAndNativeDefaultsClear(t *testing.T) {
	p := New("codex", "s", true)
	apply(&p, []point{
		{At: 10, Facts: []Fact{fact("model", "old", "session_log", 10), fact("effort", "high", "session_log", 10)}},
		{At: 12, End: true},
		{At: 13, Facts: []Fact{fact("model", "later-idle-selection", "session_log", 13)}},
	}, 10, 20)
	if values(p)["model"] != "old" {
		t.Fatal("borrowed later selection", p)
	}
	x := decode("codex", []byte(`{"type":"turn_context","timestamp":"2026-10-20T00:00:00Z","payload":{"model":"other"}}`))
	apply(&p, []point{x}, 0, 0)
	if values(p)["effort"] != "" {
		t.Fatal("carried effort across a complete snapshot", p)
	}
}

func TestLogIndexAppendPartialTailAndReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	write := func(s string) {
		t.Helper()
		if e := os.WriteFile(path, []byte(s), 0600); e != nil {
			t.Fatal(e)
		}
	}
	header := `{"type":"session_meta","timestamp":"2026-10-20T00:00:00Z","payload":{"cli_version":"0.163.0"}}` + "\n"
	line := `{"type":"turn_context","timestamp":"2026-10-20T00:00:01Z","payload":{"model":"one","effort":"high"}}`
	write(header + line)
	p := New("codex", "s", false)
	ReadLogs(&p, []string{path}, 0, 0)
	if values(p)["model"] != "" {
		t.Fatal("parsed torn tail")
	}
	f, e := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		t.Fatal(e)
	}
	_, _ = f.WriteString("\n")
	_ = f.Close()
	p = New("codex", "s", false)
	ReadLogs(&p, []string{path}, 0, 0)
	if values(p)["model"] != "one" {
		t.Fatal(p)
	}
	replacement := filepath.Join(dir, "replacement")
	_ = os.WriteFile(replacement, []byte(strings.ReplaceAll(header+line+"\n", "one", "two")), 0600)
	_ = os.Rename(replacement, path)
	p = New("codex", "s", false)
	ReadLogs(&p, []string{path}, 0, 0)
	if values(p)["model"] != "two" {
		t.Fatal("kept replaced facts", p)
	}
}

func TestNativeHookFieldsAndProcessPrivacy(t *testing.T) {
	for _, tc := range []struct{ agent, raw, key, want string }{
		{"claude", `{"effort":{"level":"xhigh"},"permission_mode":"plan"}`, "effort", "xhigh"},
		{"cursor", `{"cursor_version":"3.0","model_id":"m","model_params":[{"id":"context","value":"1m"}]}`, "context", "1m"},
		{"gemini", `{"llm_request":{"model":"g","config":{"thinkingConfig":{"thinkingLevel":"HIGH","thinkingBudget":200}}}}`, "thinking_level", "HIGH"},
		{"opencode", `{"gtmux_profile":{"role":"plan","variant":"custom","model":"m","provider":"p"}}`, "variant", "custom"},
	} {
		p := New(tc.agent, "s", false)
		for _, f := range HookFacts(tc.agent, []byte(tc.raw), 1) {
			p.put(f)
		}
		if values(p)[tc.key] != tc.want {
			t.Fatal(tc, p)
		}
	}
	for _, name := range []string{"codex", "claude", "zsh", "python", "node"} {
		args := sanitizeArgs(name, name+` --prompt "do --model private-value" --api-key secret --model also-private`)
		if len(args) != 0 {
			t.Fatal("reconstructed options from flat ps text", args)
		}
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	Observe("claude", "s", "", "Stop", []byte(`{"model":"first","effort":{"level":"high"}}`), false)
	Observe("claude", "s", "", "PostModelSwitch", []byte(`{"to_model":"second"}`), false)
	p := New("claude", "s", false)
	Current(&p, "")
	if values(p)["model"] != "second" || values(p)["effort"] != "" {
		t.Fatal(p)
	}
	other := New("claude", "s", false)
	Current(&other, "foreign")
	if len(other.Facts) > 0 {
		t.Fatal("borrowed other home's facts")
	}
	b, _ := json.Marshal(loadObservation("claude", "s", ""))
	if strings.Contains(string(b), "api-key") {
		t.Fatal("persisted raw hook")
	}
}

func TestModelHookProjectionDropsContextBeforeBuffering(t *testing.T) {
	raw := `{"session_id":"s","cwd":"/work","llm_request":{"model":"g","messages":[{"content":"` + strings.Repeat("private-context", 200000) + `"}],"config":{"thinkingConfig":{"thinkingLevel":"HIGH","thinkingBudget":200},"headers":{"Authorization":"secret"}}}}`
	b, err := ReadModelHook(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(b) > 512 || strings.Contains(string(b), "private-context") || strings.Contains(string(b), "secret") {
		t.Fatal("buffered context", len(b))
	}
	p := New("gemini", "s", false)
	for _, f := range HookFacts("gemini", b, 1) {
		p.put(f)
	}
	if values(p)["model"] != "g" || values(p)["thinking_budget"] != "200" {
		t.Fatal(p)
	}
	if _, err := ReadModelHook(strings.NewReader(`{"session_id":"s","llm_request":`)); err == nil {
		t.Fatal("accepted incomplete input")
	}
}

func TestProfileCacheBoundsAndReadFailureAreExplicit(t *testing.T) {
	indexes.Lock()
	indexes.entries = map[string]*logIndex{}
	indexes.Unlock()
	dir := t.TempDir()
	for i := 0; i < 20; i++ {
		path := filepath.Join(dir, fmt.Sprintf("%d.jsonl", i))
		_ = os.WriteFile(path, []byte(`{"type":"turn_context","payload":{"model":"m"}}`+"\n"), 0600)
		p := New("codex", "s", false)
		ReadLogs(&p, []string{path}, 0, 0)
	}
	indexes.Lock()
	n := len(indexes.entries)
	indexes.Unlock()
	if n != 16 {
		t.Fatalf("cache entries=%d", n)
	}
	p := New("codex", "s", false)
	ReadLogs(&p, []string{filepath.Join(dir, "missing")}, 0, 0)
	if !p.Unavailable || !p.Partial {
		t.Fatal("read failure represented as unrecorded", p)
	}
	path := filepath.Join(dir, "large.jsonl")
	data := `{"type":"session_meta","payload":{"cli_version":"0.163.0"}}` + "\n" + strings.Repeat("ignored\n", tailBudget/4) + `{"type":"turn_context","payload":{"model":"last"}}` + "\n"
	_ = os.WriteFile(path, []byte(data), 0600)
	p = New("codex", "s", false)
	ReadLogs(&p, []string{path}, 0, 0)
	if !p.Partial || values(p)["model"] != "last" || values(p)["version"] != "0.163.0" {
		t.Fatal("bounded tail lost header/latest or claimed completeness", p)
	}
}

func TestNativeRequestDefaultsDoNotCarryAcrossCalls(t *testing.T) {
	for _, tc := range []struct{ agent, first, second, key string }{
		{"gemini", `{"llm_request":{"model":"m","config":{"thinkingConfig":{"thinkingBudget":200}}}}`, `{"llm_request":{"model":"m"}}`, "thinking_budget"},
		{"opencode", `{"gtmux_profile":{"model":"m","variant":"high","effort":"high"}}`, `{"gtmux_profile":{"model":"m"}}`, "variant"},
	} {
		p := New(tc.agent, "s", false)
		for _, raw := range []string{tc.first, tc.second} {
			for _, f := range HookFacts(tc.agent, []byte(raw), 1) {
				p.put(f)
			}
		}
		if values(p)[tc.key] != "" {
			t.Fatal("stale request options", p)
		}
	}
}

func TestDSHCompressedProfileAppendsAndResets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.v4.jsonl.zstd")
	enc, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer enc.Close()
	record := func(model string) []byte {
		return enc.EncodeAll([]byte(`{"type":"request/header","time":1792454400000,"data":{"header":{"config":{"model":"`+model+`","provider":"p"}}}}`+"\n"), nil)
	}
	_ = os.WriteFile(path, record("one"), 0600)
	p := New("dsh", "s", false)
	ReadLogs(&p, []string{path}, 0, 0)
	if values(p)["model"] != "one" {
		t.Fatal(p)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Write(record("two"))
	_ = f.Close()
	if err != nil {
		t.Fatal(err)
	}
	p = New("dsh", "s", false)
	ReadLogs(&p, []string{path}, 0, 0)
	if values(p)["model"] != "two" {
		t.Fatal("appended frame missing", p)
	}
	_ = os.WriteFile(path, record("new"), 0600)
	p = New("dsh", "s", false)
	ReadLogs(&p, []string{path}, 0, 0)
	if values(p)["model"] != "new" {
		t.Fatal("truncated frame kept old metadata", p)
	}
}

func TestCurrentHookCannotOverrideNewerNativeRequest(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	Observe("claude", "freshness", "", "Stop", []byte(`{"model":"old","effort":{"level":"high"}}`), false)
	p := New("claude", "freshness", false)
	p.put(fact("model", "new", "session_log", time.Now().Add(time.Minute).UnixMilli()))
	Current(&p, "")
	if values(p)["model"] != "new" || values(p)["effort"] != "" {
		t.Fatal("borrowed stale hook model/options", p)
	}
}

func TestCursorUnknownOptionalParametersPreserveIdentityFacts(t *testing.T) {
	p := New("cursor", "s", false)
	for _, f := range HookFacts("cursor", []byte(`{"model_id":"cursor-model","cursor_version":"3.0","model_params":[{"id":"context","value":{"unknown":"shape"}},{"id":"thinking","value":null}]}`), 1) {
		p.put(f)
	}
	if values(p)["model"] != "cursor-model" || values(p)["cursor_version"] != "3.0" || values(p)["thinking"] != "" {
		t.Fatal(p)
	}
}
