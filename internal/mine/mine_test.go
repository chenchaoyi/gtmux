package mine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Every log in these tests is synthetic and written into a temp dir. Nothing here reads
// the machine's own transcripts — the privacy boundary the change proposal states.

type rec map[string]any

func user(session, uuid, ts, text string) rec {
	return rec{"type": "user", "uuid": uuid, "sessionId": session, "cwd": "/w/demo-repo", "timestamp": ts,
		"message": rec{"role": "user", "content": text}}
}

func assistant(session, uuid, ts, text string, tools ...string) rec {
	blocks := []any{}
	if text != "" {
		blocks = append(blocks, rec{"type": "text", "text": text})
	}
	for i := 0; i+1 < len(tools); i += 2 {
		blocks = append(blocks, rec{"type": "tool_use", "id": tools[i], "name": tools[i+1]})
	}
	return rec{"type": "assistant", "uuid": uuid, "sessionId": session, "cwd": "/w/demo-repo", "timestamp": ts,
		"message": rec{"role": "assistant", "content": blocks}}
}

func toolResult(session, uuid, ts, toolID, out string) rec {
	return rec{"type": "user", "uuid": uuid, "sessionId": session, "cwd": "/w/demo-repo", "timestamp": ts,
		"message": rec{"role": "user", "content": []any{rec{"type": "tool_result", "tool_use_id": toolID, "content": out}}}}
}

func writeLog(t *testing.T, root, session string, recs ...rec) string {
	t.Helper()
	dir := filepath.Join(root, "-w-demo-repo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, session+".jsonl")
	var sb strings.Builder
	for _, r := range recs {
		b, _ := json.Marshal(r)
		sb.Write(b)
		sb.WriteByte('\n')
	}
	if err := os.WriteFile(p, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const (
	t1 = "2026-09-01T10:00:00.000Z"
	t2 = "2026-09-01T10:01:00.000Z"
	t3 = "2026-09-01T10:02:00.000Z"
)

func run(t *testing.T, root, led string, o Options) Report {
	t.Helper()
	o.Roots = []Root{{Agent: "claude", Dir: root, Glob: filepath.Join("*", "*.jsonl")}}
	if o.Now.IsZero() {
		o.Now = time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	}
	rep, err := Run(led, o)
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

func kinds(rep Report) (corr, errs int) {
	for _, c := range rep.Candidates {
		if c.Kind == KindCorrection {
			corr++
		} else {
			errs++
		}
	}
	return
}

func TestACorrectionAnswersAReply(t *testing.T) {
	root, led := t.TempDir(), t.TempDir()
	// The same lexicon line as the FIRST prompt opens a task; after a reply it reacts.
	writeLog(t, root, "s1",
		user("s1", "u1", t1, "这个不对，重新做"),
		assistant("s1", "a1", t2, "I moved the toggle to the header and shipped it."),
		user("s1", "u2", t3, "这个不对，重新做"),
	)
	rep := run(t, root, led, Options{})
	corr, _ := kinds(rep)
	if corr != 1 {
		t.Fatalf("want exactly the reacting line, got %d: %+v", corr, rep.Candidates)
	}
	c := rep.Candidates[0]
	if c.Line != "这个不对，重新做" || !strings.Contains(c.Context, "shipped it") || c.Project != "demo-repo" || c.Session != "s1" {
		t.Fatalf("candidate lost its shape: %+v", c)
	}
	if rep.Lines != 2 {
		t.Fatalf("both typed lines are human lines, got %d", rep.Lines)
	}
}

func TestTheMachinesOwnSendIsNotTheHuman(t *testing.T) {
	root, led := t.TempDir(), t.TempDir()
	relay := "司令让我转一句：这个不对，重新做，理由见上。"
	writeLog(t, root, "s1",
		assistant("s1", "a1", t1, "done."),
		user("s1", "u1", t2, relay),
	)
	heads := HeadsFromSendSummaries([]string{"sent: " + relay})
	if corr, _ := kinds(run(t, root, led, Options{MachineHeads: heads, DryRun: true})); corr != 0 {
		t.Fatal("a line the audit journal says gtmux typed must not be a candidate")
	}
	// Remove the guard: the same line IS emitted — the subtraction is what decided.
	if corr, _ := kinds(run(t, root, led, Options{DryRun: true})); corr != 1 {
		t.Fatal("without the audit heads the line reads as a correction; the guard was not what suppressed it")
	}
}

func TestNotTypedByAHuman(t *testing.T) {
	root, led := t.TempDir(), t.TempDir()
	paste := strings.Repeat("不对 ", 800)
	writeLog(t, root, "s1",
		assistant("s1", "a1", t1, "done.", "tu1", "Bash"),
		// a tool result whose text carries lexicon words
		toolResult("s1", "u1", t2, "tu1", "grep: 不对 not found"),
		assistant("s1", "a2", t2, "again."),
		// gtmux's own wake line echoed back
		user("s1", "u2", t3, "» ⚠ gtmux·waiting │ %3 不对"),
		assistant("s1", "a3", t3, "again."),
		// a compaction summary that quotes a correction
		rec{"type": "user", "uuid": "u3", "sessionId": "s1", "timestamp": t3, "isCompactSummary": true,
			"message": rec{"role": "user", "content": "This session is being continued. The user said 这个不对。"}},
		assistant("s1", "a4", t3, "again."),
		// a paste
		user("s1", "u4", t3, paste),
		assistant("s1", "a5", t3, "again."),
		// a slash command wrapper
		user("s1", "u5", t3, "<command-name>/clear</command-name><command-message>不对</command-message>"),
	)
	rep := run(t, root, led, Options{DryRun: true})
	if corr, _ := kinds(rep); corr != 0 {
		t.Fatalf("none of these were typed by a human: %+v", rep.Candidates)
	}
	if rep.Lines != 0 {
		t.Fatalf("no human lines expected, got %d", rep.Lines)
	}
}

func TestARecurringErrorNeedsTwoSessions(t *testing.T) {
	root, led := t.TempDir(), t.TempDir()
	boom := "xcrun: error: unable to find utility \"devicectl\" at /Applications/Xcode-16.2.app (exit 72)"
	writeLog(t, root, "s1",
		assistant("s1", "a1", t1, "installing", "tu1", "Bash"),
		toolResult("s1", "u1", t2, "tu1", boom),
		assistant("s1", "a2", t2, "retry", "tu2", "Bash"),
		toolResult("s1", "u2", t3, "tu2", boom),
	)
	if _, errs := kinds(run(t, root, led, Options{})); errs != 0 {
		t.Fatal("twice in ONE session is not a recurrence across sessions")
	}
	writeLog(t, root, "s2",
		assistant("s2", "a1", t1, "installing", "tu1", "Bash"),
		toolResult("s2", "u1", t2, "tu1", strings.Replace(boom, "16.2", "26.0", 1)),
	)
	rep := run(t, root, led, Options{})
	if _, errs := kinds(rep); errs != 1 {
		t.Fatalf("second session crosses the line: %+v", rep.Candidates)
	}
	c := rep.Candidates[0]
	if c.Count != 3 || c.Sessions != 2 || strings.Contains(c.Line, "16.2") || !strings.Contains(c.Line, "<path>") {
		t.Fatalf("tally or normalization wrong: %+v", c)
	}
	// A later sighting keeps counting but is not emitted again.
	writeLog(t, root, "s3",
		assistant("s3", "a1", t1, "installing", "tu1", "Bash"),
		toolResult("s3", "u1", t2, "tu1", boom),
	)
	if rep := run(t, root, led, Options{}); len(rep.Candidates) != 0 {
		t.Fatalf("already emitted: %+v", rep.Candidates)
	}
	st := ReadStatus(led, 5)
	if len(st.TopErrors) != 1 || st.TopErrors[0].Count != 4 || st.TopErrors[0].Sessions != 3 || !st.TopErrors[0].Emitted {
		t.Fatalf("the counter must keep growing after emission: %+v", st.TopErrors)
	}
}

func TestErrorSignatureIgnoresTestRunners(t *testing.T) {
	for _, out := range []string{
		"Tests: 3 failed, 40 passed, 43 total",
		"--- FAIL: TestX (0.00s)\nFAIL\nFAIL\tgithub.com/x/y\t0.1s",
		"✖ 12 problems (3 errors, 9 warnings)",
		"Exit code 1",
		`{"last": "the serve said error: something", "ok": false}`,
		`"summary": "API Error: connection lost"`,
		"⏺ API Error: Connection lost mid-response. The response above may be incomplete.",
	} {
		if sig, ok := errorSignature(out); ok {
			t.Errorf("%q is the ordinary shape of work, not a footgun: %q", out, sig)
		}
	}
	sig, ok := errorSignature("warning: x\nbash: wrangler: command not found\nExit code 127")
	if !ok || sig != "bash: wrangler: command not found" {
		t.Fatalf("want the hard line, got %q %v", sig, ok)
	}
}

func TestTheLedgerReadsNothingTwice(t *testing.T) {
	root, led := t.TempDir(), t.TempDir()
	p := writeLog(t, root, "s1",
		assistant("s1", "a1", t1, "done."),
		user("s1", "u1", t2, "不对，你没测"),
	)
	first := run(t, root, led, Options{})
	if len(first.Candidates) != 1 || first.Bytes == 0 {
		t.Fatalf("first pass: %+v", first)
	}
	second := run(t, root, led, Options{})
	if second.Bytes != 0 || second.Files != 0 || len(second.Candidates) != 0 {
		t.Fatalf("second pass must read nothing: %+v", second)
	}
	// Append two records: only they are read, and the new reaction is the only candidate.
	f, _ := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o644)
	for _, r := range []rec{assistant("s1", "a2", t3, "fixed."), user("s1", "u2", t3, "还是不行！！")} {
		b, _ := json.Marshal(r)
		f.Write(append(b, '\n'))
	}
	f.Close()
	third := run(t, root, led, Options{})
	if len(third.Candidates) != 1 || third.Candidates[0].Line != "还是不行！！" {
		t.Fatalf("third pass: %+v", third.Candidates)
	}
	// Even if the offsets were lost, an emitted id is never emitted again.
	os.Remove(filepath.Join(led, "sources.json"))
	fourth := run(t, root, led, Options{})
	if len(fourth.Candidates) != 0 || fourth.Skipped != 2 {
		t.Fatalf("emitted ids must hold without offsets: %+v skipped=%d", fourth.Candidates, fourth.Skipped)
	}
	// A rewritten (shrunken) file is read from the start again.
	writeLog(t, root, "s1", assistant("s1", "a9", t3, "new life."), user("s1", "u9", t3, "不对"))
	fifth := run(t, root, led, Options{})
	if len(fifth.Candidates) != 1 || fifth.Candidates[0].Line != "不对" {
		t.Fatalf("shrunken file: %+v", fifth.Candidates)
	}
}

func TestAPartialTailWaitsForTheNextPass(t *testing.T) {
	root, led := t.TempDir(), t.TempDir()
	p := writeLog(t, root, "s1", assistant("s1", "a1", t1, "done."))
	b, _ := json.Marshal(user("s1", "u1", t2, "不对"))
	os.WriteFile(p, append(append([]byte{}, mustRead(t, p)...), b[:len(b)/2]...), 0o644) // half a line, no newline
	rep := run(t, root, led, Options{})
	if len(rep.Candidates) != 0 {
		t.Fatal("a half-written record is not a candidate")
	}
	os.WriteFile(p, append(append([]byte{}, mustRead(t, p)...), append(b[len(b)/2:], '\n')...), 0o644)
	if rep := run(t, root, led, Options{}); len(rep.Candidates) != 1 {
		t.Fatalf("completed line must be read by the next pass: %+v", rep)
	}
}

func TestSinceBoundsCorrectionsOnly(t *testing.T) {
	root, led := t.TempDir(), t.TempDir()
	boom := "fatal: not a git repository"
	writeLog(t, root, "s1", assistant("s1", "a1", t1, "done.", "tu1", "Bash"), toolResult("s1", "u1", t1, "tu1", boom), user("s1", "u2", t2, "不对"))
	writeLog(t, root, "s2", assistant("s2", "a1", t1, "done.", "tu1", "Bash"), toolResult("s2", "u1", t1, "tu1", boom))
	rep := run(t, root, led, Options{Since: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)})
	corr, errs := kinds(rep)
	if corr != 0 || errs != 1 {
		t.Fatalf("since bounds corrections, never the tally: corr=%d errs=%d", corr, errs)
	}
}

func TestDryRunLeavesNoLedger(t *testing.T) {
	root, led := t.TempDir(), t.TempDir()
	writeLog(t, root, "s1", assistant("s1", "a1", t1, "done."), user("s1", "u1", t2, "不对"))
	if rep := run(t, root, led, Options{DryRun: true}); len(rep.Candidates) != 1 {
		t.Fatal("dry run still reports")
	}
	if st := ReadStatus(led, 1); st.LastPass != 0 || st.Emitted != 0 {
		t.Fatalf("dry run wrote a ledger: %+v", st)
	}
	if rep := run(t, root, led, Options{}); len(rep.Candidates) != 1 {
		t.Fatal("a real pass after a dry run still emits")
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
