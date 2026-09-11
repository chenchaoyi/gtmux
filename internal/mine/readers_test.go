package mine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The three other agents' envelopes, each exercised through the same convo: a
// correction after a reply is a lead, an injected user-role record is not, and (where
// the agent journals tool output) a shell failure tallies. All logs are synthetic.

func writeLines(t *testing.T, p string, recs ...any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	for _, r := range recs {
		b, _ := json.Marshal(r)
		sb.Write(b)
		sb.WriteByte('\n')
	}
	if err := os.WriteFile(p, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runRoot(t *testing.T, root Root, led string) Report {
	t.Helper()
	rep, err := Run(led, Options{Roots: []Root{root}, Now: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

// ---- Codex ---------------------------------------------------------------------------

func cx(ord int64, ts, typ string, payload any) rec {
	return rec{"timestamp": ts, "ordinal": ord, "type": typ, "payload": payload}
}

func cxMsg(role, text string) rec {
	return rec{"type": "message", "role": role, "content": []any{rec{"type": "input_text", "text": text}}}
}

func TestCodexReader(t *testing.T) {
	root, led := t.TempDir(), t.TempDir()
	boom := "bash: wrangler: command not found"
	day := filepath.Join(root, "2026", "09", "01")
	writeLines(t, filepath.Join(day, "rollout-a.jsonl"),
		cx(0, t1, "session_meta", rec{"session_id": "cx-1", "cwd": "/w/demo-repo"}),
		cx(1, t1, "response_item", cxMsg("user", "<environment_context>\n  <cwd>/w/demo-repo</cwd>\n</environment_context>")),
		cx(2, t1, "response_item", cxMsg("user", "<recommended_plugins> 不对 </recommended_plugins>")),
		cx(3, t1, "response_item", cxMsg("developer", "injected: 不对")),
		cx(4, t1, "response_item", cxMsg("user", "deploy the worker")),
		cx(5, t2, "response_item", rec{"type": "function_call", "name": "exec_command", "call_id": "c1", "arguments": "{}"}),
		cx(6, t2, "response_item", rec{"type": "function_call_output", "call_id": "c1", "output": `{"output":"` + boom + `\n","metadata":{"exit_code":127}}`}),
		cx(7, t2, "response_item", cxMsg("assistant", "Deployed the worker.")),
		cx(8, t3, "response_item", cxMsg("user", "不对，你没部署，那是 dry run")),
	)
	writeLines(t, filepath.Join(day, "rollout-b.jsonl"),
		cx(0, t1, "session_meta", rec{"session_id": "cx-2", "cwd": "/w/other"}),
		cx(1, t2, "response_item", rec{"type": "custom_tool_call", "name": "exec", "call_id": "c9", "input": "wrangler deploy"}),
		cx(2, t2, "response_item", rec{"type": "custom_tool_call_output", "call_id": "c9", "output": boom}),
	)
	rep := runRoot(t, Root{Agent: "codex", Dir: root, Glob: filepath.Join("*", "*", "*", "*.jsonl")}, led)
	corr, errs := kinds(rep)
	if corr != 1 || errs != 1 {
		t.Fatalf("want 1 correction + 1 recurring error, got %d/%d: %+v", corr, errs, rep.Candidates)
	}
	if rep.Lines != 2 {
		t.Fatalf("two typed lines (the task and the correction), got %d", rep.Lines)
	}
	for _, c := range rep.Candidates {
		switch c.Kind {
		case KindCorrection:
			if c.Session != "cx-1" || c.Project != "demo-repo" || c.Agent != "codex" || !strings.Contains(c.Context, "Deployed") {
				t.Fatalf("codex correction lost its shape: %+v", c)
			}
		case KindError:
			if c.Sessions != 2 || c.Count != 2 || c.Line != boom {
				t.Fatalf("codex error tally: %+v (the JSON-wrapped output must unwrap)", c)
			}
		}
	}
}

// ---- opencode -----------------------------------------------------------------------

func TestOpencodeReader(t *testing.T) {
	root, led := t.TempDir(), t.TempDir()
	writeLines(t, filepath.Join(root, "ses_abc.jsonl"),
		rec{"timestamp": t1, "role": "user", "text": "rename the flag"},
		rec{"timestamp": t2, "role": "assistant", "text": "Renamed it to --quiet."},
		rec{"timestamp": t3, "role": "user", "text": "不对，我说的是 --silent"},
	)
	rep := runRoot(t, Root{Agent: "opencode", Dir: root, Glob: "*.jsonl"}, led)
	if corr, errs := kinds(rep); corr != 1 || errs != 0 {
		t.Fatalf("opencode: %+v", rep.Candidates)
	}
	c := rep.Candidates[0]
	if c.Session != "ses_abc" || c.Project != "" || !strings.Contains(c.Context, "--quiet") {
		t.Fatalf("opencode lead: %+v", c)
	}
	// Idempotent without a record id: the timestamp+hash stands in.
	if again := runRoot(t, Root{Agent: "opencode", Dir: root, Glob: "*.jsonl"}, led); len(again.Candidates) != 0 {
		t.Fatalf("re-emitted: %+v", again.Candidates)
	}
}

// ---- Kimi -----------------------------------------------------------------------------

func km(typ string, ms int64, extra rec) rec {
	r := rec{"type": typ, "agentId": "main", "time": ms}
	for k, v := range extra {
		r[k] = v
	}
	return r
}

func kmUser(ms int64, text string, origin any) rec {
	m := rec{"role": "user", "content": []any{rec{"type": "text", "text": text}}}
	if origin != nil {
		m["origin"] = origin
	}
	return km("context.append_message", ms, rec{"message": m})
}

func TestKimiReader(t *testing.T) {
	root, led := t.TempDir(), t.TempDir()
	p := filepath.Join(root, "wdk", "sess-9", "agents", "main", "wire.jsonl")
	base := int64(1756720800000)
	writeLines(t, p,
		rec{"type": "metadata", "protocol_version": "1.5"},
		kmUser(base, "fix the flaky test", rec{"kind": "user"}),
		kmUser(base+1, "<system-reminder>不对 date is …</system-reminder>", rec{"kind": "injection"}),
		km("context.append_loop_event", base+2, rec{"event": rec{"type": "content.part", "part": rec{"type": "text", "text": "I deleted the test."}}}),
		// a streaming partial is re-appended in final form
		km("context.append_message", base+3, rec{"message": rec{"role": "user", "partial": true, "content": []any{rec{"type": "text", "text": "不对"}}}}),
		kmUser(base+4, "不对！！删掉不叫修好", "user"),
	)
	rep := runRoot(t, Root{Agent: "kimi", Dir: root, Glob: filepath.Join("*", "*", "agents", "main", "wire.jsonl")}, led)
	if corr, errs := kinds(rep); corr != 1 || errs != 0 {
		t.Fatalf("kimi: %+v", rep.Candidates)
	}
	c := rep.Candidates[0]
	if c.Session != "sess-9" || c.Agent != "kimi" || !strings.Contains(c.Context, "deleted") || c.At != (base+4)/1000 {
		t.Fatalf("kimi lead: %+v", c)
	}
	if rep.Lines != 2 {
		t.Fatalf("the injection and the partial are not typed lines; got %d", rep.Lines)
	}
}

func TestUnknownAgentRootIsSkipped(t *testing.T) {
	root, led := t.TempDir(), t.TempDir()
	writeLines(t, filepath.Join(root, "x.jsonl"), rec{"role": "user", "text": "不对"})
	rep := runRoot(t, Root{Agent: "aider", Dir: root, Glob: "*.jsonl"}, led)
	if rep.Files != 0 {
		t.Fatal("a root with no reader must not be opened")
	}
}
