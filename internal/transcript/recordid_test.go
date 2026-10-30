package transcript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecordIdentitySurvivesTailReadsAndStreaming(t *testing.T) {
	cases := []struct {
		name          string
		step          stepFn
		prompt, reply string
	}{
		{"claude", claudeStep, `{"type":"user","message":{"role":"user","content":"Continue"}}` + "\n", `{"type":"assistant","message":{"content":[{"type":"text","text":"Result"}]}}` + "\n"},
		{"kimi", kimiStep, kimiLine(t, "context.append_message", kimiMsg("user", "Continue", nil), 0) + "\n", kimiLine(t, "context.append_message", kimiMsg("assistant", "Result", nil), 0) + "\n"},
		{"codex", codexStep, chatRecord("user_message", "Continue"), chatRecord("agent_message", "Result")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "session.jsonl")
			text := tc.prompt + tc.reply + tc.prompt
			if err := os.WriteFile(file, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			l := &Loader{}
			load := func(n int) []Turn {
				var turns []Turn
				var err error
				if tc.name == "codex" {
					turns, err = l.loadCodexFile(file, n)
				} else {
					turns, err = l.loadFile(file, tc.step, n)
				}
				if err != nil {
					t.Fatal(err)
				}
				return turns
			}
			tail := load(1)
			if len(tail) != 1 || tail[0].RecordID == "" {
				t.Fatal(tail)
			}
			id := tail[0].RecordID
			full := load(20)
			if len(full) != 2 || full[0].RecordID == id || full[1].RecordID != id {
				t.Fatal(full)
			}
			f, _ := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0600)
			_, _ = f.WriteString(tc.reply + tc.prompt)
			_ = f.Close()
			after := load(20)
			if len(after) != 3 || after[1].RecordID != id || after[1].Response != "Result" {
				t.Fatal(after)
			}
			if strings.Contains(id, file) {
				t.Fatal("source path exposed")
			}
			other := filepath.Join(t.TempDir(), "other.jsonl")
			_ = os.WriteFile(other, []byte(text), 0600)
			got, _, _, err := parseTurns(other, 0, tc.step)
			if err != nil || got[1].RecordID == id {
				t.Fatal("different sources reused identity", err)
			}
		})
	}
}
