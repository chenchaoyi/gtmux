package app

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/relay"
)

// The source takes its row snapshot before checking the sequence counter, just
// like ReadSince. afterSnapshot deterministically inserts a concurrent append
// into that interval; no scheduling assumptions or sleep are needed.
type phase1CursorSource struct {
	rows          []events.Record
	latest        int64
	afterSnapshot func()
}

func (s *phase1CursorSource) read(cursor int64) ([]events.Record, bool) {
	rows := []events.Record{}
	for _, r := range s.rows {
		if r.Seq > cursor {
			rows = append(rows, r)
		}
	}
	if s.afterSnapshot != nil {
		f := s.afterSnapshot
		s.afterSnapshot = nil
		f()
	}
	gap := len(rows) == 0 && cursor > 0 && cursor < s.latest
	for i, r := range rows {
		if i == 0 {
			gap = cursor > 0 && r.Seq != cursor+1
		} else if r.Seq != rows[i-1].Seq+1 {
			gap = true
		}
	}
	return rows, gap
}

func phase1CursorStore(t *testing.T, source *phase1CursorSource, initialized bool) (*hqworkspace.Store, string, hqworkspace.Conversation) {
	t.Helper()
	oldRead, oldSeq, oldRelays, oldObserve := workspaceReadEvents, workspaceEventSequence, workspaceRelayRecords, workspaceObserveEvent
	t.Cleanup(func() {
		workspaceReadEvents, workspaceEventSequence, workspaceRelayRecords, workspaceObserveEvent = oldRead, oldSeq, oldRelays, oldObserve
	})
	workspaceReadEvents = source.read
	workspaceEventSequence = func() int64 { return source.latest }
	workspaceRelayRecords = func() []relay.Request { return nil }
	workspaceObserveEvent = func(_ string, _ string, sid string, _ string, _ int64, _ ...string) bool {
		return sid != "denied-thread"
	}
	home := t.TempDir()
	s := hqworkspace.New(home)
	if _, err := s.Create("cursor-goal", "Synthetic cursor goal", 100); err != nil {
		t.Fatal(err)
	}
	c := hqworkspace.Conversation{Agent: "codex", HomeID: "synthetic-cursor-home", SessionID: "cursor-thread"}
	if err := s.Associate("cursor-goal", hqworkspace.Association{ID: "cursor-association", Conversation: c, Role: "execution", From: 100}, 0, 100); err != nil {
		t.Fatal(err)
	}
	if initialized {
		if err := s.ActivateObservations(source.latest, 100); err != nil {
			t.Fatal(err)
		}
	}
	return s, home, c
}

func phase1CursorEvent(c hqworkspace.Conversation, seq int64) events.Record {
	return events.Record{Seq: seq, Event: "Stop", Pane: "%synthetic-cursor", Agent: "Codex", HomeID: c.HomeID, AgentSession: c.SessionID, Summary: "Synthetic retained result", Ts: 101}
}

func phase1CursorDatabase(t *testing.T, s *hqworkspace.Store) hqworkspace.Database {
	t.Helper()
	d, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func phase1CursorBytes(t *testing.T, home string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(home, "workspace", "records.json"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func phase1CursorReplayStable(t *testing.T, home string) {
	t.Helper()
	before := phase1CursorBytes(t, home)
	if err := ingestWorkspaceObservations(hqworkspace.New(home), 110); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, phase1CursorBytes(t, home)) {
		t.Fatal("duplicate/reopened ingestion changed persisted database")
	}
}

func TestPhase1CursorEmptyScanConcurrentAppend(t *testing.T) {
	for _, kind := range []string{"Stop", "StopFailure", "UserPromptSubmit"} {
		t.Run(kind, func(t *testing.T) {
			source := &phase1CursorSource{latest: 10}
			s, home, c := phase1CursorStore(t, source, true)
			event := phase1CursorEvent(c, 11)
			event.Event = kind
			event.Origin = events.OriginInstruction
			source.afterSnapshot = func() { source.rows = []events.Record{event}; source.latest = 11 }
			if err := ingestWorkspaceObservations(s, 102); err != nil {
				t.Fatal(err)
			}
			d := phase1CursorDatabase(t, s)
			if d.ObservationCursors["events"] != 10 {
				t.Fatalf("empty scan advanced over unread retained event: cursor=%d", d.ObservationCursors["events"])
			}
			if _, ok := d.Reconciliations["event-gap-10-10"]; !ok {
				t.Fatal("empty scan lost its source-gap notice")
			}
			if err := ingestWorkspaceObservations(hqworkspace.New(home), 103); err != nil {
				t.Fatal(err)
			}
			d = phase1CursorDatabase(t, hqworkspace.New(home))
			queued, ok := d.Reconciliations["event-11"]
			want := hqworkspace.Observation{ID: "event-11", Kind: kind, MatterID: "cursor-goal", Body: event.Summary, Ref: "events:11", Conversation: c, OccurredAt: event.Ts}
			if !ok || !reflect.DeepEqual(queued.Observation, want) || d.ObservationCursors["events"] != 11 {
				t.Fatalf("retained event was not durably queued with its cursor: queue=%+v cursor=%d", queued, d.ObservationCursors["events"])
			}
			phase1CursorReplayStable(t, home)
		})
	}
}

func TestPhase1CursorProspectiveActivation(t *testing.T) {
	for _, boundary := range []int64{0, 10} {
		t.Run(map[int64]string{0: "empty_source", 10: "historical_boundary"}[boundary], func(t *testing.T) {
			source := &phase1CursorSource{latest: boundary}
			s, home, c := phase1CursorStore(t, source, false)
			event := phase1CursorEvent(c, boundary+1)
			workspaceEventSequence = func() int64 {
				observed := source.latest
				source.rows = []events.Record{event}
				source.latest = event.Seq
				return observed
			}
			if err := ingestWorkspaceObservations(s, 101); err != nil {
				t.Fatal(err)
			}
			d := phase1CursorDatabase(t, s)
			cursor, initialized := d.ObservationCursors["events"]
			if !initialized || cursor != boundary || len(d.Reconciliations) != 0 {
				t.Fatalf("prospective activation changed its sampled boundary: initialized=%t cursor=%d queued=%d", initialized, cursor, len(d.Reconciliations))
			}
			workspaceEventSequence = func() int64 { return source.latest }
			if err := ingestWorkspaceObservations(hqworkspace.New(home), 102); err != nil {
				t.Fatal(err)
			}
			d = phase1CursorDatabase(t, hqworkspace.New(home))
			id := "event-1"
			if boundary == 10 {
				id = "event-11"
			}
			if d.ObservationCursors["events"] != event.Seq || len(d.Reconciliations) != 1 || d.Reconciliations[id].Observation.Conversation != c {
				t.Fatalf("activation skipped new event: cursor=%d queue=%v", d.ObservationCursors["events"], d.Reconciliations)
			}
			phase1CursorReplayStable(t, home)
		})
	}
}

func TestPhase1CursorEmptyRetentionGap(t *testing.T) {
	source := &phase1CursorSource{latest: 10}
	s, home, c := phase1CursorStore(t, source, true)
	source.latest = 15
	if err := ingestWorkspaceObservations(s, 102); err != nil {
		t.Fatal(err)
	}
	d := phase1CursorDatabase(t, s)
	if d.ObservationCursors["events"] != 10 || len(d.Reconciliations) != 1 || d.Reconciliations["event-gap-10-10"].Observation.Kind != "source-gap" {
		t.Fatalf("empty gap invented consumption: cursor=%d queue=%v", d.ObservationCursors["events"], d.Reconciliations)
	}
	phase1CursorReplayStable(t, home)
	source.rows = []events.Record{phase1CursorEvent(c, 16)}
	source.latest = 16
	if err := ingestWorkspaceObservations(hqworkspace.New(home), 111); err != nil {
		t.Fatal(err)
	}
	d = phase1CursorDatabase(t, hqworkspace.New(home))
	if d.ObservationCursors["events"] != 16 || d.Reconciliations["event-16"].Observation.Conversation != c || d.Reconciliations["event-gap-10-16"].Observation.Kind != "source-gap" {
		t.Fatalf("retained tail lost after empty gap: cursor=%d queue=%v", d.ObservationCursors["events"], d.Reconciliations)
	}
	phase1CursorReplayStable(t, home)
}

func TestPhase1CursorFilteredPageBoundary(t *testing.T) {
	source := &phase1CursorSource{latest: 10}
	s, home, c := phase1CursorStore(t, source, true)
	for seq := int64(11); seq <= 511; seq++ {
		r := phase1CursorEvent(c, seq)
		r.Event = "PostToolUse"
		source.rows = append(source.rows, r)
	}
	source.rows[0].Event, source.rows[0].AgentSession = "Stop", "denied-thread"
	source.rows[1].Event = "UserPromptSubmit" // no instruction origin
	source.rows[2].Event, source.rows[2].AgentSession = "Stop", "unregistered-thread"
	source.rows[500] = phase1CursorEvent(c, 511)
	source.latest = 511
	if err := ingestWorkspaceObservations(s, 102); err != nil {
		t.Fatal(err)
	}
	d := phase1CursorDatabase(t, s)
	if d.ObservationCursors["events"] != 510 || len(d.Reconciliations) != 0 {
		t.Fatalf("filtered first page enrolled excluded work or skipped the next page: cursor=%d queue=%v", d.ObservationCursors["events"], d.Reconciliations)
	}
	if err := ingestWorkspaceObservations(hqworkspace.New(home), 103); err != nil {
		t.Fatal(err)
	}
	d = phase1CursorDatabase(t, hqworkspace.New(home))
	if d.ObservationCursors["events"] != 511 || len(d.Reconciliations) != 1 || d.Reconciliations["event-511"].Observation.Conversation != c {
		t.Fatalf("next-page event lost: cursor=%d queue=%v", d.ObservationCursors["events"], d.Reconciliations)
	}
	phase1CursorReplayStable(t, home)
}
