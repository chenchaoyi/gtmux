package transcript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The record shapes here are Kimi's own, taken from agent-core-v2's generated wire
// manifest (protocol 1.5, shipped in kimi 0.41.0): a `metadata` opening line, then
// {"type", …payload spread at top level…, "time": epoch ms}.

func kimiLine(t *testing.T, typ string, payload map[string]any, ms int64) string {
	t.Helper()
	payload["type"] = typ
	if ms > 0 {
		payload["time"] = ms
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func kimiMsg(role, text string, extra map[string]any) map[string]any {
	m := map[string]any{
		"role":      role,
		"content":   []any{map[string]any{"type": "text", "text": text}},
		"toolCalls": []any{},
	}
	for k, v := range extra {
		m[k] = v
	}
	return map[string]any{"agentId": "main", "message": m}
}

func runKimi(t *testing.T, lines []string) []Turn {
	t.Helper()
	st := &parseState{}
	for _, ln := range lines {
		kimiStep(ln, st)
	}
	st.flush()
	return st.turns
}

func TestKimiReadsAConversation(t *testing.T) {
	turns := runKimi(t, []string{
		`{"type":"metadata","protocol_version":"1.5","created_at":1788700000000}`,
		kimiLine(t, "context.append_message", kimiMsg("user", "fix the flaky test", nil), 1788700000123),
		kimiLine(t, "context.append_message", kimiMsg("assistant", "Looking at it now.", nil), 1788700001000),
		kimiLine(t, "context.append_message", kimiMsg("assistant", "Fixed — it was a race.", nil), 1788700002000),
		kimiLine(t, "context.append_message", kimiMsg("user", "ship it", nil), 1788700003000),
		kimiLine(t, "context.append_message", kimiMsg("assistant", "Tagged v1.2.0.", nil), 1788700004000),
	})
	if len(turns) != 2 {
		t.Fatalf("got %d turns, want 2: %+v", len(turns), turns)
	}
	if turns[0].Prompt != "fix the flaky test" {
		t.Errorf("prompt = %q", turns[0].Prompt)
	}
	if !strings.Contains(turns[0].Response, "Fixed — it was a race.") {
		t.Errorf("response = %q", turns[0].Response)
	}
	if turns[1].Prompt != "ship it" || turns[1].Response != "Tagged v1.2.0." {
		t.Errorf("second turn = %+v", turns[1])
	}
	// The journal stamps epoch ms; every other parser hands up RFC3339.
	if !strings.HasPrefix(turns[0].Time, "2026-") {
		t.Errorf("time = %q, want RFC3339", turns[0].Time)
	}
}

func TestKimiOnlyARealInstructionOpensATurn(t *testing.T) {
	// Eleven origins put a user-role message into the context, and only one of them
	// is the user. `hook_result` in particular is gtmux's OWN hook output coming
	// back — reading it as the goal would have gtmux reporting itself to itself.
	for _, origin := range []string{
		"skill_activation", "injection", "system_trigger", "hook_result",
		"compaction_summary", "task", "cron_job", "retry", "shell_command", "plugin_command",
	} {
		turns := runKimi(t, []string{
			kimiLine(t, "context.append_message", kimiMsg("user", "the real goal", nil), 1788700000000),
			kimiLine(t, "context.append_message", kimiMsg("user", "NOT a goal", map[string]any{"origin": origin}), 1788700001000),
			kimiLine(t, "context.append_message", kimiMsg("assistant", "done", nil), 1788700002000),
		})
		if len(turns) != 1 {
			t.Fatalf("origin %q opened a turn: %d turns", origin, len(turns))
		}
		if turns[0].Prompt != "the real goal" {
			t.Errorf("origin %q: prompt = %q", origin, turns[0].Prompt)
		}
	}
}

func TestKimiExplicitUserOriginIsAUser(t *testing.T) {
	turns := runKimi(t, []string{
		kimiLine(t, "context.append_message", kimiMsg("user", "typed by hand", map[string]any{"origin": "user"}), 1788700000000),
	})
	if len(turns) != 1 || turns[0].Prompt != "typed by hand" {
		t.Fatalf("an explicit origin:user prompt was dropped: %+v", turns)
	}
}

func TestKimiSkipsAPartialMessage(t *testing.T) {
	// A streaming message is appended again when it finishes; counting both prints
	// the reply twice, the first copy truncated mid-sentence.
	turns := runKimi(t, []string{
		kimiLine(t, "context.append_message", kimiMsg("user", "explain", nil), 1788700000000),
		kimiLine(t, "context.append_message", kimiMsg("assistant", "The answer is fo", map[string]any{"partial": true}), 1788700001000),
		kimiLine(t, "context.append_message", kimiMsg("assistant", "The answer is forty-two.", nil), 1788700002000),
	})
	if len(turns) != 1 {
		t.Fatalf("got %d turns", len(turns))
	}
	if turns[0].Response != "The answer is forty-two." {
		t.Errorf("response = %q — a partial was counted", turns[0].Response)
	}
}

func TestKimiLeavesReasoningOutOfTheAnswer(t *testing.T) {
	// A `think` part is the model reasoning, not what it told you. Reported as the
	// reply it is worse than no reply.
	line := kimiLine(t, "context.append_message", map[string]any{
		"agentId": "main",
		"message": map[string]any{
			"role": "assistant",
			"content": []any{
				map[string]any{"type": "think", "think": "maybe it is the mutex"},
				map[string]any{"type": "text", "text": "It was the mutex."},
			},
			"toolCalls": []any{},
		},
	}, 1788700001000)
	turns := runKimi(t, []string{
		kimiLine(t, "context.append_message", kimiMsg("user", "why", nil), 1788700000000),
		line,
	})
	if got := turns[0].Response; got != "It was the mutex." {
		t.Errorf("response = %q — reasoning leaked into the answer", got)
	}
}

func TestKimiRecordsToolSteps(t *testing.T) {
	line := kimiLine(t, "context.append_message", map[string]any{
		"agentId": "main",
		"message": map[string]any{
			"role":    "assistant",
			"content": []any{map[string]any{"type": "text", "text": "Checking."}},
			"toolCalls": []any{
				map[string]any{"type": "function", "id": "1", "name": "Bash", "arguments": `{"command":"go test ./...\nwith a second line"}`},
				map[string]any{"type": "function", "id": "2", "name": "mcp.github.create_issue", "arguments": `{"description":"flaky test"}`},
				map[string]any{"type": "function", "id": "3", "name": "Read", "arguments": `not json`},
			},
		},
	}, 1788700001000)
	turns := runKimi(t, []string{
		kimiLine(t, "context.append_message", kimiMsg("user", "check", nil), 1788700000000),
		line,
	})
	steps := turns[0].Segments[0].Steps
	if len(steps) != 3 {
		t.Fatalf("got %d steps, want 3: %+v", len(steps), steps)
	}
	if steps[0].Title != "Bash" || steps[0].Detail != "go test ./..." {
		t.Errorf("step 0 = %+v — want the command's first line only", steps[0])
	}
	if steps[1].Title != "create_issue" {
		t.Errorf("an MCP tool kept its dotted path: %q", steps[1].Title)
	}
	// Arguments are a JSON *string* and may be malformed or null; that is normal.
	if steps[2].Title != "Read" || steps[2].Detail != "" {
		t.Errorf("step 2 = %+v — unparseable arguments should yield no detail", steps[2])
	}
}

func TestKimiIgnoresTheOtherFiftyNineRecordTypes(t *testing.T) {
	// The journal is event-sourced: token counts, permission decisions, plan
	// revisions, cron. Only context.append_message carries the conversation.
	turns := runKimi(t, []string{
		`{"type":"metadata","protocol_version":"1.5","created_at":1}`,
		`{"type":"token_counting.turn_recorded","agentId":"main","length":40,"tokens":81234,"turnId":3,"time":1788700000000}`,
		`{"type":"permission.set_mode","agentId":"main","time":1788700000001}`,
		`{"type":"llm.request","agentId":"main","model":"k2","time":1788700000002}`,
		`not json at all`,
		``,
		kimiLine(t, "context.append_message", kimiMsg("user", "hello", nil), 1788700000003),
	})
	if len(turns) != 1 || turns[0].Prompt != "hello" {
		t.Fatalf("noise records disturbed the parse: %+v", turns)
	}
}

func TestKimiHandlesAToolMessageWithoutOpeningATurn(t *testing.T) {
	turns := runKimi(t, []string{
		kimiLine(t, "context.append_message", kimiMsg("user", "run it", nil), 1788700000000),
		kimiLine(t, "context.append_message", kimiMsg("tool", "exit 0", nil), 1788700001000),
		kimiLine(t, "context.append_message", kimiMsg("system", "reminder", nil), 1788700002000),
		kimiLine(t, "context.append_message", kimiMsg("assistant", "It passed.", nil), 1788700003000),
	})
	if len(turns) != 1 {
		t.Fatalf("got %d turns, want 1", len(turns))
	}
	if turns[0].Response != "It passed." {
		t.Errorf("response = %q — a tool or system message leaked in", turns[0].Response)
	}
}

// --- session resolution ---------------------------------------------------

func writeKimiSession(t *testing.T, home, workKey, sessionID string, indexed bool) string {
	t.Helper()
	dir := filepath.Join(home, "sessions", workKey, sessionID)
	agentDir := filepath.Join(dir, "agents", "main")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wire := filepath.Join(agentDir, "wire.jsonl")
	if err := os.WriteFile(wire, []byte(`{"type":"metadata","protocol_version":"1.5","created_at":1}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if indexed {
		line, _ := json.Marshal(kimiIndexLine{SessionID: sessionID, SessionDir: dir, WorkDir: "/proj"})
		f, err := os.OpenFile(filepath.Join(home, "session_index.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if _, err := f.Write(append(line, '\n')); err != nil {
			t.Fatal(err)
		}
	}
	return wire
}

func TestKimiResolvesASessionThroughTheIndex(t *testing.T) {
	home := t.TempDir()
	t.Setenv("KIMI_CODE_HOME", home)
	// The directory is keyed by a HASH of the working directory, which is exactly why
	// the index is read rather than the path re-derived.
	want := writeKimiSession(t, home, "a1b2c3d4e5", "sess-1", true)
	if got := kimiLogPath("sess-1"); got != want {
		t.Errorf("kimiLogPath = %q, want %q", got, want)
	}
}

func TestKimiFallsBackToAGlobWhenTheIndexIsMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("KIMI_CODE_HOME", home)
	want := writeKimiSession(t, home, "deadbeef", "sess-2", false)
	if got := kimiLogPath("sess-2"); got != want {
		t.Errorf("with no index, kimiLogPath = %q, want %q", got, want)
	}
}

func TestKimiHonoursATombstone(t *testing.T) {
	// The index is append-only. A deleted id whose directory was reused would
	// otherwise resolve to a stale path.
	home := t.TempDir()
	t.Setenv("KIMI_CODE_HOME", home)
	writeKimiSession(t, home, "k1", "sess-3", true)
	line, _ := json.Marshal(map[string]any{"sessionId": "sess-3", "deleted": true})
	f, _ := os.OpenFile(filepath.Join(home, "session_index.jsonl"), os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = f.Write(append(line, '\n'))
	_ = f.Close()

	// The tombstone clears the index entry; the glob still finds the files on disk,
	// which is the truthful answer — the journal is right there.
	if got := kimiLogPath("sess-3"); got == "" {
		t.Error("a tombstoned session whose files remain should still be readable")
	}
	if got := kimiSessionDir("sess-3"); got != "" {
		t.Errorf("kimiSessionDir ignored the tombstone: %q", got)
	}
}

func TestKimiUnknownSessionResolvesToNothing(t *testing.T) {
	t.Setenv("KIMI_CODE_HOME", t.TempDir())
	if got := kimiLogPath("nope"); got != "" {
		t.Errorf("kimiLogPath(unknown) = %q, want empty", got)
	}
	if got := kimiLogPath(""); got != "" {
		t.Errorf("kimiLogPath(\"\") = %q, want empty", got)
	}
}

func TestKimiIsWiredIntoTheDispatch(t *testing.T) {
	// The parser existing is not the same as it being reachable. resolveLog is what
	// the digest calls.
	home := t.TempDir()
	t.Setenv("KIMI_CODE_HOME", home)
	writeKimiSession(t, home, "w", "sess-4", true)
	for _, agent := range []string{"kimi", "Kimi Code", "KIMI"} {
		path, step := resolveLog(agent, "sess-4")
		if path == "" || step == nil {
			t.Errorf("resolveLog(%q) = (%q, %v) — the agent does not reach its parser", agent, path, step)
		}
	}
}

// --- message times ----------------------------------------------------------

func TestKimiMessageTimesAreReadable(t *testing.T) {
	// Found by probing rather than assuming: Kimi stamps `"time": <epoch ms>` — a
	// NUMBER, on a differently-named field — so the shared `"timestamp":"<RFC3339>"`
	// matcher found nothing and both readers returned 0. HQ self-rotation's `age`
	// criterion would have been blind on a Kimi-hosted supervisor while the docs
	// claimed Tier 2 gave it, and a criterion with no data is omitted, so nothing
	// would have said so.
	home := t.TempDir()
	t.Setenv("KIMI_CODE_HOME", home)
	dir := filepath.Join(home, "sessions", "k", "s")
	if err := os.MkdirAll(filepath.Join(dir, "agents", "main"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Written in Kimi's OWN key order — type first, time last — not Go's, which sorts
	// keys alphabetically and so puts `time` BEFORE `type`. That difference is why this
	// is spelled out by hand: a fixture built with json.Marshal tests a shape Kimi does
	// not write, and the third line below covers the sorted order as well, so neither
	// can be assumed.
	lines := []string{
		`{"type":"metadata","protocol_version":"1.5","created_at":1788700000000}`,
		`{"type":"context.append_message","agentId":"main","message":{"role":"user","content":[{"type":"text","text":"first"}],"toolCalls":[]},"time":1788700010000}`,
		`{"agentId":"main","message":{"content":[{"text":"reply","type":"text"}],"role":"assistant","toolCalls":[]},"time":1788700020000,"type":"context.append_message"}`,
		// Written AFTER the last message, and carrying a `time` of its own: an
		// unscoped match would report this as the last thing the agent said.
		`{"type":"token_counting.turn_recorded","agentId":"main","tokens":900,"time":1788700099000}`,
	}
	if err := os.WriteFile(filepath.Join(dir, "agents", "main", "wire.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	line, _ := json.Marshal(kimiIndexLine{SessionID: "s", SessionDir: dir, WorkDir: "/p"})
	if err := os.WriteFile(filepath.Join(home, "session_index.jsonl"), append(line, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := FirstMessageTime("kimi", "s"); got != 1788700010 {
		t.Errorf("FirstMessageTime = %d, want 1788700010 (the first message, in seconds)", got)
	}
	if got := LastMessageTime("kimi", "s"); got != 1788700020 {
		t.Errorf("LastMessageTime = %d, want 1788700020 — a token-count record is not something the agent said", got)
	}
}

func TestKimiTimesDoNotDisturbTheOtherAgents(t *testing.T) {
	// The reader is chosen per agent; Claude and Codex must keep reading RFC3339.
	buf := "{\"timestamp\":\"2026-09-06T12:00:00Z\"}\n{\"timestamp\":\"2026-09-06T13:00:00Z\"}\n"
	for _, agent := range []string{"claude", "codex", "opencode"} {
		if got := messageTimeIn(agent, buf, true); got != 1788699600 {
			t.Errorf("%s last = %d, want 1788699600", agent, got)
		}
		if got := messageTimeIn(agent, buf, false); got != 1788696000 {
			t.Errorf("%s first = %d, want 1788696000", agent, got)
		}
	}
}

func TestKimiTimesSurviveATruncatedWindow(t *testing.T) {
	// LastMessageTime reads a 64KB TAIL, so the first line in the window is usually
	// cut in half. A partial line must simply not match, never yield a wrong stamp.
	buf := `sage","agentId":"main","time":1788700005000}` + "\n" +
		`{"type":"context.append_message","agentId":"main","time":1788700010000}` + "\n"
	if got := messageTimeIn("kimi", buf, true); got != 1788700010 {
		t.Errorf("got %d, want 1788700010 — a half-line at the window edge was read", got)
	}
}

// --- against a REAL journal --------------------------------------------------
//
// testdata/kimi-wire.jsonl is a genuine wire.jsonl, produced by running kimi 0.41.0
// against a local stand-in provider (no account needed) and scrubbed of local paths.
// It is here because the fixtures above — every one of them built from Kimi's own
// generated wire manifest, and all thirteen passing — described a shape the runtime
// does not write. Against the real bytes the parser returned ZERO turns.
//
// Three things the manifest did not say and this file does:
//
//  1. `origin` is an OBJECT (`{"kind":"user"}`), not the documented string. Decoding
//     it into a string failed the whole unmarshal and dropped every message.
//  2. The assistant's reply is NOT a `context.append_message`. A completed turn
//     writes none at all; the reply is a `context.append_loop_event` carrying a
//     `content.part`.
//  3. Because of (2), the last thing SAID in a session is a loop event, and the
//     message-time readers matched only append_message — so on a real session
//     LastMessageTime returned 0. (The 69,521-byte `llm.tools_snapshot` in this file
//     looks like it should also overflow the 64KB window; measured, it does not — it
//     is written once per session, AFTER the first messages, and a second turn does
//     not repeat it. The window is fine; the record type was the bug.)
func realKimiSession(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("KIMI_CODE_HOME", home)
	id := "session_e3ac5267-8f9a-4941-9dae-13687a969056"
	dir := filepath.Join(home, "sessions", "wd_proj_fbe8ae6f0ff8", id)
	if err := os.MkdirAll(filepath.Join(dir, "agents", "main"), 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join("testdata", "kimi-wire.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "agents", "main", "wire.jsonl"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	line, _ := json.Marshal(kimiIndexLine{SessionID: id, SessionDir: dir, WorkDir: "/proj"})
	if err := os.WriteFile(filepath.Join(home, "session_index.jsonl"), append(line, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestKimiReadsARealSession(t *testing.T) {
	id := realKimiSession(t)
	turns, err := Load("kimi", id, 10)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(turns) != 1 {
		t.Fatalf("got %d turns from a real one-turn session, want 1", len(turns))
	}
	if turns[0].Prompt != "add kimi support" {
		t.Errorf("prompt = %q", turns[0].Prompt)
	}
	// The half that lives in a loop event, not in a message record.
	if turns[0].Response != "Wired the manifest and the installer." {
		t.Errorf("response = %q — the assistant's reply is a content.part, not a message", turns[0].Response)
	}
	if turns[0].Time == "" {
		t.Error("no timestamp")
	}
}

func TestKimiRealOriginFiltersTheInjections(t *testing.T) {
	// The real session carries two injected user-role messages beside the prompt — a
	// date reminder and a permission-mode notice, both `{"kind":"injection"}`. Neither
	// is the goal, and the digest would have reported one as it.
	id := realKimiSession(t)
	turns, _ := Load("kimi", id, 10)
	for _, x := range turns {
		if strings.Contains(x.Prompt, "system-reminder") {
			t.Errorf("an injected reminder opened a turn: %q", x.Prompt)
		}
	}
}

func TestKimiRealMessageTimesAreFound(t *testing.T) {
	// The last thing said in this session is the assistant's reply, which is a loop
	// event; matching only append_message made LastMessageTime return 0 on a real
	// session — and a criterion with no data is omitted from the wake line, so
	// nothing would have said so.
	id := realKimiSession(t)
	first, last := FirstMessageTime("kimi", id), LastMessageTime("kimi", id)
	if first == 0 {
		t.Error("FirstMessageTime = 0 on a real session — HQ self-rotation's age criterion would be blind")
	}
	if last == 0 {
		t.Error("LastMessageTime = 0 on a real session — the assistant's reply is a loop event, not a message record")
	}
	if last < first {
		t.Errorf("last (%d) is before first (%d)", last, first)
	}
}

func TestKimiRealOriginIsAnObjectNotAString(t *testing.T) {
	// Pinned on its own because it is the one that silently emptied everything: the
	// generated manifest declares a string, the runtime writes {"kind":"user"}, and a
	// type mismatch fails the WHOLE record rather than one field.
	if got := originKind(json.RawMessage(`{"kind":"user"}`)); got != "user" {
		t.Errorf("object origin = %q, want user", got)
	}
	if got := originKind(json.RawMessage(`{"kind":"injection","variant":"date_change"}`)); got != "injection" {
		t.Errorf("injection origin = %q", got)
	}
	// The documented shape still works, in case a later version writes it.
	if got := originKind(json.RawMessage(`"user"`)); got != "user" {
		t.Errorf("string origin = %q, want user", got)
	}
	// Anything unrecognised reads as absent, which lets the message THROUGH: the
	// filter drops known non-user origins, it does not guess at unknown ones.
	if got := originKind(json.RawMessage(`12`)); got != "" {
		t.Errorf("unknown origin = %q, want empty", got)
	}
	if got := originKind(nil); got != "" {
		t.Errorf("absent origin = %q, want empty", got)
	}
}
