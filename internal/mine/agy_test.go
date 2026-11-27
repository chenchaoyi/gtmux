package mine

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestAgyReader(t *testing.T) {
	root, led := t.TempDir(), t.TempDir()
	p := filepath.Join(root, "conv-agy-1", ".system_generated", "logs", "transcript.jsonl")
	writeLines(t, p,
		rec{"step_index": 0, "source": "USER_EXPLICIT", "type": "USER_INPUT", "status": "DONE", "created_at": t1, "content": "<CONTEXT_SUMMARY>\n不对 compaction summary\n</CONTEXT_SUMMARY>"},
		rec{"step_index": 1, "source": "USER_EXPLICIT", "type": "USER_INPUT", "status": "DONE", "created_at": t1, "content": "<USER_REQUEST>\nfix the failing test\n</USER_REQUEST>\n<ADDITIONAL_METADATA>\nOS: mac\n</ADDITIONAL_METADATA>"},
		rec{"step_index": 2, "source": "SYSTEM", "type": "SYSTEM_MESSAGE", "status": "DONE", "created_at": t2, "content": "不对 system reminder"},
		rec{"step_index": 3, "source": "MODEL", "type": "PLANNER_RESPONSE", "status": "ERROR", "created_at": t2, "content": "Partial error draft"},
		rec{"step_index": 4, "source": "MODEL", "type": "PLANNER_RESPONSE", "status": "DONE", "created_at": t2, "content": "I skipped the failing test."},
		rec{"step_index": 5, "source": "USER_EXPLICIT", "type": "USER_INPUT", "status": "DONE", "created_at": t3, "content": "<USER_REQUEST>\n不对，不要跳过测试，要修好它\n</USER_REQUEST>"},
	)
	rep := runRoot(t, Root{Agent: "agy", Dir: root, Glob: filepath.Join("*", ".system_generated", "logs", "transcript.jsonl")}, led)
	if corr, errs := kinds(rep); corr != 1 || errs != 0 {
		t.Fatalf("agy: want 1 correction, got %d/%d: %+v", corr, errs, rep.Candidates)
	}
	c := rep.Candidates[0]
	if c.Session != "conv-agy-1" || c.Agent != "agy" || !strings.Contains(c.Context, "skipped") || strings.Contains(c.Context, "Partial error") {
		t.Fatalf("agy lead: %+v", c)
	}
	if rep.Lines != 2 {
		t.Fatalf("compaction summary and system message are not typed lines; got %d", rep.Lines)
	}
}
