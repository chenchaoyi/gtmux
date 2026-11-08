package app

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqnudge"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/relay"
	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
	"github.com/chenchaoyi/gtmux/internal/state"
)

var workspaceAssessmentOwner = func() string {
	c, ok := currentCaller()
	if !ok {
		return ""
	}
	sid := c.ThreadID
	if sid == "" && c.Pane != "" {
		sid = state.ReadMarker(state.ActivePath(c.Pane))
	}
	if sid == "" {
		return ""
	}
	return c.Agent + ":" + c.HomeID + ":" + sid
}

// These adapters are replaceable by synthetic sources in component tests.
var workspaceReadEvents = events.ReadSince
var workspaceEventSequence = events.CurrentSeq
var workspaceRelayRecords = relay.List
var workspaceObserveEvent = sessionpolicy.ObserveEvent
var workspaceReconcileWake = hqnudge.Enqueue

func ingestWorkspaceObservations(s *hqworkspace.Store, now int64) error {
	d, err := s.Read()
	if err != nil {
		return err
	}
	cursor, initialized := d.ObservationCursors["events"]
	if !initialized {
		// Prospective activation: no historical bulk reconciliation on upgrade.
		return s.ActivateObservations(workspaceEventSequence(), now)
	}
	rows, gap := workspaceReadEvents(cursor)
	last := cursor
	observations := []hqworkspace.Observation{}
	for i, r := range rows {
		if i >= 500 {
			break
		}
		if r.Seq > last {
			last = r.Seq
		}
		if r.Event != "Stop" && r.Event != "StopFailure" && r.Event != "UserPromptSubmit" {
			continue
		}
		if r.Event == "UserPromptSubmit" && r.Origin != events.OriginInstruction {
			continue
		}
		if !workspaceObserveEvent(r.Pane, r.Agent, r.AgentSession, r.Client, r.Ts, r.HomeID) {
			continue
		}
		agent := agents.KeyForLabel(r.Agent)
		if agent == "" {
			agent = r.Agent
		}
		c := hqworkspace.Conversation{Agent: agent, HomeID: r.HomeID, SessionID: r.AgentSession}
		matter, _ := hqworkspace.MatchObservation(d, c, r.Ts)
		// Missing/mismatched home or interval stays unassigned for a known session.
		// Source logs do not carry a goal version: never stamp the current one.
		// Only explicit workspace members feed this queue. An unmatched event still
		// belongs to the existing HQ event stream; it must not enroll new work.
		if matter == "" && !workspaceHasConversation(d, c) {
			continue
		}
		observations = append(observations, hqworkspace.Observation{ID: "event-" + strconv.FormatInt(r.Seq, 10), Kind: r.Event, MatterID: matter, MatterRevision: 0, Body: r.Summary, Ref: fmt.Sprintf("events:%d", r.Seq), Conversation: c, OccurredAt: r.Ts})
	}
	if gap || workspaceEventSequence() < cursor {
		observations = append(observations, hqworkspace.Observation{ID: fmt.Sprintf("event-gap-%d-%d", cursor, last), Kind: "source-gap", Ref: fmt.Sprintf("events after %d through %d", cursor, last)})
		if len(rows) == 0 && workspaceEventSequence() > last {
			last = workspaceEventSequence()
		}
	}
	if err = s.Ingest("events", cursor, last, observations, now); err != nil {
		return err
	}
	// Relay is itself a durable source. Its receipt may be read/closed separately;
	// workspace assessment never grants authority to reply or execute its request.
	start := d.ObservationCursors["relay-start"]
	if start == 0 {
		return s.Ingest("relay-start", 0, now, nil, now)
	}
	for _, r := range workspaceRelayRecords() {
		if r.CreatedAt < start {
			continue
		}
		id := "relay-" + r.ID
		if _, ok := d.Reconciliations[id]; ok {
			continue
		}
		c := hqworkspace.Conversation{Agent: r.Agent, HomeID: r.HomeID, SessionID: r.ThreadID}
		matter, _ := hqworkspace.MatchObservation(d, c, r.CreatedAt)
		// Explicit relay reports are already addressed to HQ; unresolved identity
		// remains unassigned instead of borrowing a task's recycled pane.
		o := hqworkspace.Observation{ID: id, Kind: "relay-" + r.Kind, MatterID: matter, MatterRevision: 0, Body: r.Body, Ref: "relay:" + r.ID, Conversation: c, OccurredAt: r.CreatedAt}
		if err = s.Ingest("relay-start", start, start, []hqworkspace.Observation{o}, now); err != nil {
			return err
		}
	}
	return nil
}

func workspaceHasConversation(d hqworkspace.Database, c hqworkspace.Conversation) bool {
	if c.Agent == "" || c.SessionID == "" {
		return false
	}
	for _, m := range d.Matters {
		for _, a := range m.Participants {
			if a.Conversation.Agent == c.Agent && a.Conversation.SessionID == c.SessionID {
				return true
			}
		}
	}
	return false
}

// Called by serve's periodic loop, separately from instruction delivery. Restart
// resumes the durable cursor and expired leases; a wake is not an assessment.
var workspaceReconciliationRunner workspaceRunner
var workspaceReconciliationNext int64
var workspaceReconciliationWakeAt int64

func scheduleWorkspaceReconciliation() {
	now := time.Now().Unix()
	if now < workspaceReconciliationNext {
		return
	}
	workspaceReconciliationNext = now + 30
	workspaceReconciliationRunner.start(func() {
		s := workspaceStore()
		if err := ingestWorkspaceObservations(s, now); err != nil {
			diag.ReportStoreFailure("hq-reconciliation", err)
			return
		}
		next, err := notifyWorkspaceReconciliation(s, now, workspaceReconciliationWakeAt)
		if err != nil {
			diag.ReportStoreFailure("hq-reconciliation", err)
			return
		}
		workspaceReconciliationWakeAt = next
	})
}

func workspaceReconciliationJSON(s *hqworkspace.Store, now int64) (json.RawMessage, error) {
	d, err := s.Read()
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"revision": d.Revision, "pending": hqworkspace.ReconciliationQueue(d, now), "assessments": d.Assessments})
}

func notifyWorkspaceReconciliation(s *hqworkspace.Store, now, next int64) (int64, error) {
	d, err := s.Read()
	if err != nil {
		return next, err
	}
	rows := hqworkspace.ReconciliationQueue(d, now)
	if len(rows) > 0 && now >= next {
		workspaceReconcileWake(hqwake.Line(hqwake.ClassTick, "workspace reconciliation", fmt.Sprintf("pending:%d", len(rows)), "pull: gtmux hq workspace reconcile --json"))
		return now + 300, nil
	}
	return next, nil
}
