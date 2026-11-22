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

func TestHSW001HSW002ObserveWorkspaceSuccessorReceipt(t *testing.T) {
	for _, tc := range []string{
		"exact", "no rotation", "stale rotation", "wrong successor", "wrong rotation role", "foreign home",
		"wrong receipt session", "wrong receipt turn", "unbound receipt", "nonworkspace tick", "mixed",
		"old turn", "input before start", "different first input", "message without role", "assistant before echo",
		"unrelated turn context", "wrong batch", "over budget", "continuation", "expired", "unknown",
	} {
		t.Run(tc, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("CODEX_HOME", t.TempDir())
			const session = "01a20000-0000-7000-8000-000000000041"
			const turn = "01a20000-0000-7000-8000-000000000042"
			const oldSession = "01a20000-0000-7000-8000-000000000043"
			const cwd = "/synthetic/hq"
			since := time.Date(2026, 11, 22, 2, 0, 0, 321000000, time.UTC)
			started := since.Add(1480 * time.Millisecond)
			inputAt := started.Add(5411 * time.Millisecond)
			growthAt := inputAt.Add(3904 * time.Millisecond)
			now := since
			f := &fake{}
			x := f.io()
			x.nowNano = func() int64 { return now.UnixNano() }
			receipt := dispatch.WakeReceipt{Request: "synthetic-receiver", Session: session, Turn: turn, Home: transcript.CodexHomeID()}
			switch tc {
			case "foreign home":
				receipt.Home = "foreign-codex-home"
			case "wrong receipt session":
				receipt.Session = oldSession
			case "wrong receipt turn":
				receipt.Turn = "foreign-turn"
			case "unbound receipt":
				receipt.Request = ""
			}
			var payload string
			calls := 0
			x.nativeSubmit = func(_, text, _ string) (dispatch.Result, bool) {
				calls++
				payload = text
				if tc == "unknown" {
					return dispatch.Result{State: dispatch.StateFailed, Submission: dispatch.SubmissionUnknown}, true
				}
				return dispatch.Result{State: dispatch.StateQueued, WakeReceipt: &receipt}, true
			}
			head := "workspace reconciliation"
			if tc == "nonworkspace tick" {
				head = "status summary"
			}
			line := hqwake.Line(hqwake.ClassTick, head, "pending:4", "HQ coordination remains due; pull: gtmux hq workspace reconcile --json")
			enqueue(x, line)
			if tc == "mixed" {
				now = now.Add(time.Nanosecond)
				enqueue(x, hqwake.Line(hqwake.ClassTick, "other maintenance"))
			}
			drain(x, "%hq")
			if calls != 1 || !strings.HasPrefix(payload, line) {
				t.Fatal("fixture did not submit the intended batch")
			}
			if tc == "mixed" && !strings.Contains(payload, line+" · "+hqwake.Line(hqwake.ClassTick, "other maintenance")) {
				t.Fatal("fixture did not coalesce the ordinary entry")
			}

			rotationSession, rotationAgent, rotationAt := session, "codex", started.Add(time.Second)
			switch tc {
			case "wrong successor":
				rotationSession = "unrelated-successor"
			case "wrong rotation role":
				rotationAgent = "claude"
			case "stale rotation":
				rotationAt = since.Add(-10 * time.Second)
			}
			if tc != "no rotation" {
				events.AuditRotateConfirmed(rotationAgent, oldSession, rotationSession, "/new", rotationAt.Unix())
			}

			dir := filepath.Join(transcript.CodexHome(), "sessions", "2026", "11", "22")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			name := "rollout-2026-11-22T02-00-00-" + session + ".jsonl"
			if tc == "continuation" {
				name = strings.TrimSuffix(name, ".jsonl") + "_continued.jsonl"
			}
			path := filepath.Join(dir, name)
			row := func(at time.Time, kind string, fields map[string]any) []byte {
				t.Helper()
				data, err := json.Marshal(map[string]any{"timestamp": at, "type": kind, "payload": fields})
				if err != nil {
					t.Fatal(err)
				}
				return append(data, '\n')
			}
			sized := func(size int, at time.Time, kind string, fields map[string]any) []byte {
				t.Helper()
				fields["fixture_padding"] = ""
				empty := row(at, kind, fields)
				if size < len(empty) {
					t.Fatal("fixture exceeds its requested row size")
				}
				fields["fixture_padding"] = strings.Repeat("p", size-len(empty))
				return row(at, kind, fields)
			}
			boundary := func(at time.Time, kind, id string) []byte {
				return row(at, "event_msg", map[string]any{"type": kind, "turn_id": id})
			}
			text := func(value string) map[string]string { return map[string]string{"type": "input_text", "text": value} }
			message := func(role string, blocks ...map[string]string) map[string]any {
				p := map[string]any{"type": "message", "content": blocks}
				if role != "" {
					p["role"] = role
				}
				return p
			}
			var log bytes.Buffer
			log.Write(sized(31493, since, "session_meta", map[string]any{"id": session, "cwd": cwd}))
			switch tc {
			case "old turn":
				log.Write(boundary(since, "task_started", "older-turn"))
				log.Write(boundary(since, "task_complete", "older-turn"))
			case "input before start":
				log.Write(row(since, "response_item", message("user", text("Earlier synthetic input."))))
			}
			log.Write(sized(494, started, "event_msg", map[string]any{"type": "task_started", "turn_id": turn}))
			log.Write(sized(48370, started, "response_item", map[string]any{"type": "additional_tools", "role": "developer"}))
			log.Write(sized(78226, started, "response_item", message("developer", text("Synthetic developer context."))))
			log.Write(row(started, "response_item", message("user",
				text("# AGENTS.md instructions for "+cwd+"\nSynthetic instructions."),
				text("<environment_context>\n<cwd>"+cwd+"</cwd>\n</environment_context>"))))
			switch tc {
			case "different first input":
				log.Write(row(started, "response_item", message("user", text("Earlier synthetic input."))))
			case "message without role":
				log.Write(row(started, "response_item", message("", text("Unclassifiable synthetic input."))))
			case "assistant before echo":
				log.Write(row(started, "response_item", message("assistant", map[string]string{"type": "output_text", "text": "Earlier synthetic execution."})))
			}
			contextTurn := turn
			if tc == "unrelated turn context" {
				contextTurn = "unrelated-turn"
			}
			log.Write(row(started, "turn_context", map[string]any{"turn_id": contextTurn}))
			log.Write(row(started, "event_msg", map[string]any{"type": "thread_settings_applied"}))
			inputOffset := 275382
			if tc == "over budget" {
				inputOffset = 31493 + (256 << 10) - 100
			}
			log.Write(sized(inputOffset-log.Len(), started, "world_state", map[string]any{}))
			input := payload
			if tc == "wrong batch" {
				input = payload[:strings.LastIndex(payload, " · #")] + " · #other0"
			}
			log.Write(sized(970, inputAt, "response_item", message("user", text(input))))
			if err := os.WriteFile(path, log.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			if tc == "exact" {
				got := transcript.CodexWakeWitness(session, turn, payload, since)
				if !got.Started.Equal(started) || !got.Delivered.Equal(inputAt) {
					t.Fatal("the short tail did not prove the exact first input", got)
				}
			}
			log.Write(sized(70617, growthAt, "world_state", map[string]any{}))
			if err := os.WriteFile(path, log.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			if got := transcript.CodexWakeWitness(session, turn, payload, since); !got.Delivered.IsZero() {
				t.Fatal("the fixture did not move the start out of the normal tail")
			}
			if tc == "exact" {
				t.Logf("synthetic header_end=31493 input_end=%d grown_size=%d tail_offset=%d append_delay=%s",
					inputOffset+970, log.Len(), log.Len()-(256<<10), growthAt.Sub(inputAt))
			}

			files, err := os.ReadDir(acceptedWakeDir())
			var receiptPath string
			var original []byte
			switch tc {
			case "unknown", "unbound receipt":
				if !os.IsNotExist(err) && (err != nil || len(files) != 0) {
					t.Fatal("unproved receiver created an observation receipt")
				}
			default:
				if err != nil || len(files) != 1 {
					t.Fatal("fixture lost the active accepted receipt")
				}
				receiptPath = filepath.Join(acceptedWakeDir(), files[0].Name())
				original, err = os.ReadFile(receiptPath)
				if err != nil {
					t.Fatal(err)
				}
			}
			now = growthAt.Add(time.Second)
			if tc == "expired" {
				now = since.Add(acceptedProofLifetime)
			}
			ObserveWakeEvidence(now)
			var archive []byte
			switch tc {
			case "exact":
				if _, err := os.Stat(receiptPath); !os.IsNotExist(err) {
					t.Fatal("the accepted workspace tick remained unverified after the proven first input")
				}
				if err := os.WriteFile(receiptPath, original, 0600); err != nil {
					t.Fatal(err)
				}
			case "expired":
				archive, err = os.ReadFile(acceptedArchivePath())
				if err != nil {
					t.Fatal(err)
				}
				var record acceptedArchiveRecord
				if err := json.Unmarshal(archive, &record); err != nil {
					t.Fatal(err)
				}
				accepted, err := json.Marshal(record.Accepted)
				if err != nil || !bytes.Equal(accepted, original) || record.Key != files[0].Name() || record.Reason != "proof-window-expired" {
					t.Fatal("expiry lost the original receipt")
				}
			case "unknown":
				files := readQueue()
				if len(files) != 1 || !strings.HasSuffix(files[0].Name(), nativeUncertainSuffix) {
					t.Fatal("unknown input was not protected")
				}
			case "unbound receipt":
			default:
				if current, err := os.ReadFile(receiptPath); err != nil || !bytes.Equal(current, original) {
					t.Fatal("observation changed unverified evidence")
				}
			}
			ObserveWakeEvidence(now.Add(3 * time.Second))
			drain(x, "%hq")
			delivered := auditRecords(t, events.AuditEventWakeDelivered)
			if tc == "exact" {
				if len(delivered) != 1 || delivered[0].OpID != strings.TrimSuffix(files[0].Name(), ".json") || delivered[0].Ts != inputAt.Unix() {
					t.Fatal("delivery did not produce one audit bound to the receipt and native time")
				}
			} else if len(delivered) != 0 {
				t.Fatal("ambiguous evidence produced a delivery audit")
			}
			if tc == "expired" {
				current, err := os.ReadFile(acceptedArchivePath())
				if err != nil || !bytes.Equal(current, archive) {
					t.Fatal("observation of an expired receipt changed its archive")
				}
			}
			if calls != 1 || f.pastes != 0 || f.enters != 0 {
				t.Fatal("evidence observation attempted input again")
			}
		})
	}
}
