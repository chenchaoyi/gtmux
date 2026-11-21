package hqnudge

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestHWW003RealWakePromptGetsExactIndependentAudit(t *testing.T) {
	const session = "12345678-1234-4321-8123-123456789abc"
	const turn = "aaaaaaaa-1234-4321-8123-123456789abc"
	wait := hqwake.Line(hqwake.ClassWaiting, "synthetic-worker:0.0 (%66)", "stuck before running — draft")
	relay := hqwake.Line(hqwake.ClassTick, "workspace relay follow-up", "relay:synthetic-proof", "pull: gtmux relay show synthetic-proof")
	expected := strings.Join([]string{wait, wait, relay, "+173 more queued", "#bfd465"}, " · ")
	since := time.Unix(1795257538, 864801000).UTC()
	delivery := since.Add(2172 * time.Millisecond)
	for _, name := range []string{
		"exact", "legacy event", "foreign home", "wrong session", "missing session metadata", "wrong turn", "old turn",
		"old turn repeated after submission", "future input",
		"same batch tag wrong text", "different wake opened turn", "ordinary prompt opened turn", "extra content",
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("CODEX_HOME", t.TempDir())
			accepted := nativeIntent{Pane: "%13", ID: "#bfd465", Payload: expected, SubmittedAt: since.UnixNano(),
				WakeReceipt: &dispatch.WakeReceipt{Request: "synthetic-request", Session: session, Turn: turn, Home: transcript.CodexHomeID()}}
			if name == "foreign home" {
				accepted.WakeReceipt.Home = "foreign-home"
			}
			if err := retainAcceptedWake(accepted); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(transcript.CodexHome(), "sessions", "2026", "11", "21", "rollout-2026-11-21T10-38-00-"+session+".jsonl")
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			var b bytes.Buffer
			row := func(typ string, at time.Time, payload map[string]any) {
				t.Helper()
				if err := json.NewEncoder(&b).Encode(map[string]any{"type": typ, "timestamp": at, "payload": payload}); err != nil {
					t.Fatal(err)
				}
			}
			meta := session
			if name == "wrong session" {
				meta = "foreign-session"
			}
			if name != "missing session metadata" {
				row("session_meta", since.Add(-time.Minute), map[string]any{"id": meta, "originator": "codex-tui"})
			}
			started, turnID := since.Add(time.Second), turn
			switch name {
			case "old turn":
				started = since.Add(-time.Nanosecond)
			case "wrong turn":
				turnID = "another-turn"
			case "old turn repeated after submission":
				row("event_msg", since.Add(-time.Second), map[string]any{"type": "task_started", "turn_id": turnID})
			}
			row("event_msg", started, map[string]any{"type": "task_started", "turn_id": turnID, "root_turn_id": turnID})
			modern := func(text string, at time.Time, extra bool) {
				content := []map[string]string{{"type": "input_text", "text": text}}
				if extra {
					content = append(content, map[string]string{"type": "input_image", "image_url": "synthetic"})
				}
				row("response_item", at, map[string]any{"type": "message", "role": "user", "content": content})
			}
			modern("# AGENTS.md instructions\nsynthetic context", since.Add(time.Second), false)
			input, inputAt := expected, delivery
			switch name {
			case "same batch tag wrong text":
				input = strings.Replace(expected, "synthetic-proof", "other-reference", 1)
			case "ordinary prompt opened turn":
				modern("Please continue the original work.", since.Add(1500*time.Millisecond), false)
			case "different wake opened turn":
				modern(wait+" · #000abc", since.Add(1500*time.Millisecond), false)
			case "future input":
				inputAt = since.Add(time.Minute)
			}
			if name == "legacy event" {
				row("event_msg", inputAt, map[string]any{"type": "user_message", "message": input, "images": []string{}})
			} else {
				modern(input, inputAt, name == "extra content")
			}
			if err := os.WriteFile(path, b.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			ObserveWakeEvidence(since.Add(4 * time.Second))
			ObserveWakeEvidence(since.Add(5 * time.Second))
			rows := auditRecords(t, events.AuditEventWakeDelivered)
			files, err := os.ReadDir(acceptedWakeDir())
			if err != nil {
				t.Fatal(err)
			}
			if name == "exact" || name == "legacy event" {
				auditText := strings.Join(strings.Fields(expected), " ")
				if len(rows) != 1 || rows[0].Pane != "%13" || rows[0].Summary != auditText || rows[0].Ts != delivery.Unix() || len(files) != 0 {
					t.Fatalf("the exact first input must produce one independent audit and settle its receipt: %#v %v", rows, files)
				}
			} else if len(rows) != 0 || len(files) != 1 {
				t.Fatalf("mismatched input produced a delivery or removed its evidence: %#v %v", rows, files)
			}
		})
	}
	if _, kind := transcript.ClassifyUserPrompt(expected); kind != transcript.PromptDrop {
		t.Fatal("a wake echo entered the ordinary user prompt stream")
	}
	const ordinary = "Please continue the original work."
	if got, kind := transcript.ClassifyUserPrompt(ordinary); got != ordinary || kind != transcript.PromptUser {
		t.Fatal("ordinary user prompt classification changed")
	}
}
