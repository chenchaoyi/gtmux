package app

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
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
var workspaceReconcileWake = func(line string) { hqnudge.EnqueueKeyed("workspace-reconciliation", line) }

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
		// A sequence sampled after an empty scan may include a concurrent
		// append that this read never returned. Preserve the cursor until an
		// actual processed row supplies a safe checkpoint, including on gaps.
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
		if r.Kind == "report" {
			o.DriveProof = workspaceRelayDriveProof(r.Body, matter)
		}
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
var workspaceGoalCheckStamp int64

func scheduleWorkspaceReconciliation() {
	now := time.Now().Unix()
	stamp := hqworkspace.GoalCheckStamp(state.HQHome())
	if now < workspaceReconciliationNext && stamp <= workspaceGoalCheckStamp {
		return
	}
	workspaceReconciliationNext = now + 30
	workspaceGoalCheckStamp = stamp
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
	return json.Marshal(map[string]any{"revision": d.Revision, "pending": hqworkspace.ReconciliationQueue(d, now), "assessments": d.Assessments, "goals": workspaceGoalStatuses(d, now), "intent_attempts": d.IntentAttempts})
}

func notifyWorkspaceReconciliation(s *hqworkspace.Store, now, next int64) (int64, error) {
	if err := s.ScanGoals(now, 64); err != nil {
		return next, err
	}
	d, err := s.Read()
	if err != nil {
		return next, err
	}
	rows := hqworkspace.ReconciliationQueue(d, now)
	if len(rows) > 0 && now >= next {
		goals, repeated, err := s.RemindGoals(now)
		if err != nil {
			return next, err
		}
		ordinary := 0
		for _, q := range rows {
			if q.Observation.Kind != "goal-followup" {
				ordinary++
			}
		}
		if goals == 0 && ordinary == 0 {
			return next, nil
		}
		tail := "HQ coordination remains due; pull: gtmux hq workspace reconcile --json"
		if repeated {
			tail = "HQ has not completed its coordination after repeated reminders; pull: gtmux hq workspace reconcile --json"
		}
		workspaceReconcileWake(hqwake.Line(hqwake.ClassTick, "workspace reconciliation", fmt.Sprintf("pending:%d", len(rows)), tail))
		return now + 300, nil
	}
	return next, nil
}

func workspaceGoalStatuses(d hqworkspace.Database, now int64) map[string]hqworkspace.GoalStatus {
	out := map[string]hqworkspace.GoalStatus{}
	for _, m := range hqworkspace.Matters(d) {
		out[m.ID] = hqworkspace.GoalResponsibility(d, m, now)
	}
	return out
}

// Workers may cite a native/background receipt in this opt-in wire envelope.
// The trusted relay identity and source time remain outside the supplied body.
func workspaceRelayDriveProof(body, matter string) *hqworkspace.DriveProof {
	var envelope struct {
		Schema string                 `json:"schema"`
		Matter string                 `json:"matter"`
		Drive  hqworkspace.DriveProof `json:"drive"`
	}
	decoder := json.NewDecoder(strings.NewReader(body))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&envelope) != nil || envelope.Schema != "gtmux.goal-drive/v1" || envelope.Matter != matter || matter == "" {
		return nil
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil
	}
	switch envelope.Drive.State {
	case "received", "progress", "background", "native":
		return &envelope.Drive
	}
	return nil
}

func workspaceReconciliationProbe(line string) (bool, string) {
	if !strings.Contains(line, "workspace reconciliation") {
		return true, line
	}
	d, err := workspaceStore().Read()
	if err != nil {
		return true, line
	}
	return revalidateWorkspaceReconciliation(d, time.Now().Unix(), line)
}

func revalidateWorkspaceReconciliation(d hqworkspace.Database, now int64, line string) (bool, string) {
	pending := 0
	repeated := false
	for _, q := range hqworkspace.ReconciliationQueue(d, now) {
		if q.Observation.Kind == "goal-followup" {
			m, ok := d.Matters[q.Observation.MatterID]
			if ok {
				repeated = repeated || q.ReminderCount >= 3
				g := hqworkspace.GoalResponsibility(d, m, now)
				if g.State != "verify" && g.State != "owes-hq" {
					continue
				}
			}
		}
		pending++
	}
	if pending == 0 {
		return false, ""
	}
	tail := "HQ coordination remains due; pull: gtmux hq workspace reconcile --json"
	if repeated {
		tail = "HQ has not completed its coordination after repeated reminders; pull: gtmux hq workspace reconcile --json"
	}
	return true, hqwake.Line(hqwake.ClassTick, "workspace reconciliation", fmt.Sprintf("pending:%d", pending), tail)
}
