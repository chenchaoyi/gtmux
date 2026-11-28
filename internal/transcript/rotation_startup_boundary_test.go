package transcript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCodexFirstTurnStartRequiresUniqueCompleteOriginal(t *testing.T) {
	const sid = "successor"
	const meta = `{"type":"session_meta","payload":{"id":"successor"}}` + "\n"
	const start = `{"timestamp":"2026-11-28T09:26:16.577123456Z","type":"event_msg","payload":{"type":"task_started","turn_id":"first"}}` + "\n"
	const complete = `{"timestamp":"2026-11-28T09:26:15Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"older"}}` + "\n"
	const basename = "rollout-time-" + sid + ".jsonl"
	at := time.Date(2026, 11, 28, 9, 26, 16, 577123456, time.UTC)
	for _, tc := range []struct {
		name, body string
		other      string
		want       bool
	}{
		{name: "unique", body: meta + start, want: true},
		{name: "same-file-alias", body: meta + start, other: "alias", want: true},
		{name: "conflicting-copy", body: meta + start, other: "copy"},
		{name: "continuation", body: meta + start, other: "continuation"},
		{name: "conflicting-ids", body: strings.Replace(meta, `"id":"successor"`, `"id":"successor","session_id":"different"`, 1) + start},
		{name: "wrong-sid", body: strings.Replace(meta, sid, "old", 1) + start},
		{name: "malformed-row", body: meta + "{\n" + start},
		{name: "no-turn-id", body: meta + strings.Replace(start, `,"turn_id":"first"`, "", 1)},
		{name: "past-budget", body: meta + `{"type":"world_state","payload":{"value":"` + strings.Repeat("x", 256<<10) + `"}}` + "\n" + start},
		{name: "partial-append", body: meta + strings.TrimSuffix(start, "\n")},
		{name: "completion-before-start", body: meta + complete + start},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("CODEX_HOME", home)
			active := filepath.Join(home, "sessions", "2026", "11", "28")
			archive := filepath.Join(home, "archived_sessions")
			for _, dir := range []string{active, archive} {
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
			}
			original := filepath.Join(active, basename)
			if err := os.WriteFile(original, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			var err error
			switch tc.other {
			case "alias":
				err = os.Link(original, filepath.Join(archive, basename))
			case "copy":
				err = os.WriteFile(filepath.Join(archive, basename), []byte(tc.body), 0600)
			case "continuation":
				err = os.WriteFile(filepath.Join(active, "rollout-time-"+sid+"_continued.jsonl"), []byte(meta), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			want := time.Time{}
			if tc.want {
				want = at
			}
			if got := CodexFirstTurnStart(sid); !got.Equal(want) {
				t.Fatalf("first start = %v, want %v", got, want)
			}
		})
	}
}
