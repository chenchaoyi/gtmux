package transcript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHWA013WakeWitnessRequiresExpectedNewTurnAndExactFirstPrompt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	since := time.Unix(1700000000, 0)
	const prompt = "synthetic wake · #8d2f03"
	record := func(kind, turn, message string, t time.Time) string {
		b, _ := json.Marshal(map[string]any{"type": "event_msg", "timestamp": t.Format(time.RFC3339Nano),
			"payload": map[string]string{"type": kind, "turn_id": turn, "message": message}})
		return string(b) + "\n"
	}
	wakeStarted := record("task_started", "wanted", "", since.Add(time.Second))
	wakeMessage := record("user_message", "", prompt, since.Add(2*time.Second))
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	for name, fixture := range map[string]string{
		"matched":       wakeStarted + wakeMessage,
		"existing turn": record("task_started", "wanted", "", since.Add(-time.Second)) + wakeMessage,
		"different":     record("task_started", "foreign", "", since.Add(time.Second)) + wakeMessage,
		"user turn":     wakeStarted + record("user_message", "", "unrelated input", since.Add(time.Second)) + wakeMessage,
		"partial":       wakeStarted + wakeMessage[:len(wakeMessage)-1],
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(fixture), 0600); err != nil {
				t.Fatal(err)
			}
			got := wakeWitnessIn(path, "wanted", prompt, since)
			if name == "matched" {
				if !got.Started.Equal(since.Add(time.Second)) || !got.Delivered.Equal(since.Add(2*time.Second)) {
					t.Fatal("positive start and prompt evidence did not match", got)
				}
			} else if !got.Started.IsZero() || !got.Delivered.IsZero() {
				t.Fatal("unrelated, old or partial evidence was accepted", got)
			}
		})
	}
}
