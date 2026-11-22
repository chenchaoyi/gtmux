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
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestHSP003StartupReceiptObservationDoesNotReplayInput(t *testing.T) {
	const session = "01a20000-0000-7000-8000-000000000001"
	const turn = "01a20000-0000-7000-8000-000000000002"
	const startup = "» gtmux·startup │ synthetic briefing"
	const ordinary = "» ▸ gtmux·done │ synthetic status"
	for _, name := range []string{"startup", "ordinary", "mixed", "foreign home", "expired", "unknown"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("CODEX_HOME", t.TempDir())
			now := time.Date(2026, 11, 22, 2, 0, 0, 0, time.UTC)
			since := now
			f := &fake{}
			x := f.io()
			x.nowNano = func() int64 { return now.UnixNano() }
			var payload, batch string
			calls := 0
			x.nativeSubmit = func(_, text, id string) (dispatch.Result, bool) {
				calls++
				payload, batch = text, id
				if name == "unknown" {
					return dispatch.Result{State: dispatch.StateFailed, Submission: dispatch.SubmissionUnknown}, true
				}
				home := transcript.CodexHomeID()
				if name == "foreign home" {
					home = "synthetic-foreign-home"
				}
				return dispatch.Result{State: dispatch.StateQueued, WakeReceipt: &dispatch.WakeReceipt{
					Request: "synthetic-request", Session: session, Home: home, Turn: turn,
				}}, true
			}
			if name == "ordinary" {
				enqueue(x, ordinary)
			} else {
				enqueue(x, startup)
			}
			if name == "mixed" {
				now = now.Add(time.Nanosecond)
				enqueue(x, ordinary)
			}
			drain(x, "%hq")
			if calls != 1 || !strings.HasSuffix(payload, " · "+batch) {
				t.Fatal("fixture did not submit one bound batch")
			}
			if name == "mixed" && !strings.Contains(payload, startup+" · "+ordinary) {
				t.Fatal("fixture did not coalesce an ordinary wake behind the startup")
			}
			deliveryAt := since.Add(2 * time.Second)
			path := filepath.Join(transcript.CodexHome(), "sessions", "2026", "11", "22",
				"rollout-2026-11-22T02-00-00-"+session+".jsonl")
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			var log bytes.Buffer
			row := func(at time.Time, kind string, fields map[string]string) {
				t.Helper()
				if err := json.NewEncoder(&log).Encode(map[string]any{"timestamp": at, "type": kind, "payload": fields}); err != nil {
					t.Fatal(err)
				}
			}
			row(since, "session_meta", map[string]string{"id": session, "base_instructions": strings.Repeat("m", 16<<10)})
			row(since.Add(time.Second), "event_msg", map[string]string{"type": "task_started", "turn_id": turn})
			row(since.Add(time.Second), "world_state", map[string]string{"context": strings.Repeat("p", 230<<10)})
			row(deliveryAt, "event_msg", map[string]string{"type": "user_message", "message": payload})
			row(deliveryAt, "world_state", map[string]string{"context": strings.Repeat("p", 48<<10)})
			if err := os.WriteFile(path, log.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			if got := transcript.CodexWakeWitness(session, turn, payload, since); !got.Delivered.IsZero() {
				t.Fatal("fixture left the turn start in the tail")
			}

			now = since.Add(10 * time.Second)
			if name == "expired" {
				now = since.Add(acceptedProofLifetime)
			}
			files, err := os.ReadDir(acceptedWakeDir())
			if name == "unknown" {
				if !os.IsNotExist(err) {
					t.Fatal("an uncertain admission produced a receiver receipt")
				}
				unknown := readQueue()
				if len(unknown) != 1 || !strings.HasSuffix(unknown[0].Name(), nativeUncertainSuffix) {
					t.Fatal("unknown original did not remain protected")
				}
			} else if err != nil || len(files) != 1 || len(auditRecords(t, events.AuditEventWakeAccepted)) != 1 {
				t.Fatal("fixture did not retain the accepted receiver receipt")
			}
			var receiptPath string
			var original []byte
			if name != "unknown" {
				receiptPath = filepath.Join(acceptedWakeDir(), files[0].Name())
				original, err = os.ReadFile(receiptPath)
				if err != nil {
					t.Fatal(err)
				}
			}
			ObserveWakeEvidence(now)
			switch name {
			case "startup":
				if _, err := os.Stat(receiptPath); !os.IsNotExist(err) {
					t.Fatal("startup receipt remained unverified after observing its exact first input")
				}
				// Model an exit after syncing the audit, before unlinking the receipt.
				if err := os.WriteFile(receiptPath, original, 0600); err != nil {
					t.Fatal(err)
				}
			case "expired":
				body, err := os.ReadFile(acceptedArchivePath())
				if err != nil {
					t.Fatal(err)
				}
				var archived acceptedArchiveRecord
				if err := json.Unmarshal(body, &archived); err != nil ||
					archived.Reason != "proof-window-expired" || archived.Key != files[0].Name() ||
					archived.Accepted.Payload != payload || archived.Accepted.Receipt.Turn != turn {
					t.Fatal("the original expired receiver evidence was not archived")
				}
			case "unknown":
			default:
				if current, err := os.ReadFile(receiptPath); err != nil || !bytes.Equal(current, original) {
					t.Fatal("observation unexpectedly changed accepted evidence")
				}
			}
			now = now.Add(3 * time.Second)
			ObserveWakeEvidence(now)
			drain(x, "%hq")
			rows := auditRecords(t, events.AuditEventWakeDelivered)
			if name == "startup" {
				if len(rows) != 1 || rows[0].OpID != strings.TrimSuffix(files[0].Name(), ".json") || rows[0].Ts != deliveryAt.Unix() {
					t.Fatal("startup proof did not produce one audit at the proven time")
				}
			} else if len(rows) != 0 {
				t.Fatal("ineligible evidence was marked delivered")
			}
			if calls != 1 || f.pastes != 0 || f.enters != 0 {
				t.Fatal("observation or drain retried input")
			}
		})
	}
}
