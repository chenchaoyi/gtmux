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
var workspaceReconcileWake = enqueueWorkspaceReconciliationWake

func enqueueWorkspaceReconciliationWake(line string) {
	key := "workspace-reconciliation"
	if strings.Contains(line, workspaceContinuationWake) {
		key = "workspace-continuation"
	}
	hqnudge.EnqueueKeyed(key, line)
}

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
		observations = append(observations, hqworkspace.Observation{RecordingPolicy: r.RecordingPolicy, Continuation: matter != "" && (r.Event == "Stop" || r.Event == "StopFailure"), ID: "event-" + strconv.FormatInt(r.Seq, 10), Kind: r.Event, MatterID: matter, MatterRevision: 0, Body: r.Summary, Ref: fmt.Sprintf("events:%d", r.Seq), Conversation: c, OccurredAt: r.Ts})
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
	var facts []hqworkspace.Observation
	for _, r := range workspaceRelayRecords() {
		inbox := eligibleWorkspaceReply(r)
		if r.CreatedAt < start && !(inbox && (r.ReplyAt >= start || r.FetchedAt >= start)) {
			continue
		}
		c := hqworkspace.Conversation{Agent: r.Agent, HomeID: r.HomeID, SessionID: r.ThreadID}
		request, seen := d.Reconciliations["relay-"+r.ID]
		o := request.Observation
		if !seen {
			matter, _ := hqworkspace.MatchObservation(d, c, r.CreatedAt)
			o = hqworkspace.Observation{ID: "relay-" + r.ID, Kind: "relay-" + r.Kind, MatterID: matter, Body: r.Body, Ref: "relay:" + r.ID, Conversation: c, OccurredAt: r.CreatedAt}
			if r.Kind == "report" && r.For != "user" {
				o.DriveProof = workspaceRelayDriveProof(r.Body, matter)
				o.Continuation = matter != "" && o.DriveProof == nil
			}
			facts = append(facts, o)
		}
		if r.ReplyAt <= 0 || r.ReplyAt > now || !o.Conversation.Same(c) {
			continue
		}
		// Stage IDs cannot collide with request IDs. Never copy the reply body.
		stage := func(prefix, kind, body string, when int64) {
			id := prefix + ":" + r.ID
			if _, exists := d.Reconciliations[id]; !exists {
				facts = append(facts, hqworkspace.Observation{ID: id, Kind: kind, MatterID: o.MatterID, Conversation: c, Ref: o.Ref, Body: body, OccurredAt: when})
			}
		}
		if !inbox {
			if r.State == "replied" && r.Agent == "codex" && (r.ReplyAttempt != "" || r.Delivered) {
				stage("legacy-reply", "relay-legacy-delivery", "Historical reply delivery record. Inspect the original transport receipt; no continuation is eligible.", r.ReplyAt)
			}
			continue
		}
		stage("inbox", "relay-inbox", "Reply entered the source inbox. HQ must check fetch and next responsibility.", r.ReplyAt)
		if n := r.Notice; n != nil && n.At >= r.ReplyAt && n.At <= now {
			switch n.State {
			case "submitting", "accepted", "unconfirmed":
				stage("notice-"+n.State, "relay-notice", "Short active-turn inbox hint: "+n.State, n.At)
			}
		}
		if r.FetchedAt >= r.ReplyAt && r.FetchedAt <= now {
			stage("fetched", "relay-fetched", "Source fetched the reply. HQ must check execution evidence for the authorized step.", r.FetchedAt)
		}
	}
	if len(facts) > 0 {
		return s.Ingest("relay-start", start, start, facts, now)
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
var workspaceReconciliationNow = func() int64 { return time.Now().Unix() }

const workspaceReconciliationInterval = 30

func scheduleWorkspaceReconciliation() {
	if workspaceReconciliationRunner.running.Load() {
		return // Leave a marker received during the current scan for the next tick.
	}
	now := workspaceReconciliationNow()
	stamp := hqworkspace.GoalCheckStamp(state.HQHome())
	if now < workspaceReconciliationNext && stamp <= workspaceGoalCheckStamp {
		return
	}
	workspaceReconciliationNext = now + workspaceReconciliationInterval
	workspaceGoalCheckStamp = stamp
	workspaceReconciliationRunner.start(func() {
		s := workspaceStore()
		if err := ingestWorkspaceObservations(s, now); err != nil {
			diag.ReportStoreFailure("hq-reconciliation", err)
			return
		}
		checkedAt := workspaceReconciliationNow()
		// Ingestion may cross a second boundary. Schedule the next scan on
		// the same clock sample used to record the reminder's retry deadline.
		workspaceReconciliationNext = checkedAt + workspaceReconciliationInterval
		next, err := notifyWorkspaceReconciliation(s, checkedAt, workspaceReconciliationWakeAt)
		if err != nil {
			diag.ReportStoreFailure("hq-reconciliation", err)
			return
		}
		workspaceReconciliationWakeAt = next
		if err := pumpWorkspaceReplies(s, time.Now().Unix()); err != nil {
			diag.ReportStoreFailure("hq-reply-continuation", err)
		}
	})
}

func workspaceReconciliationJSON(s *hqworkspace.Store, now int64) (json.RawMessage, error) {
	d, err := s.Read()
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"revision": d.Revision, "pending": hqworkspace.ReconciliationQueue(d, now), "assessments": d.Assessments, "goals": workspaceGoalStatuses(d, now), "reply_observations": hqworkspace.ReplyObservations(d), "intent_attempts": d.IntentAttempts})
}

func notifyWorkspaceReconciliation(s *hqworkspace.Store, now, next int64) (int64, error) {
	if err := s.ScanGoals(now, 64); err != nil {
		return next, err
	}
	d, err := s.Read()
	if err != nil {
		return next, err
	}
	signals, err := s.RemindContinuations(d, now)
	if err != nil {
		return next, err
	}
	if len(signals) > 0 {
		workspaceReconcileWake(workspaceContinuationLine(signals, strconv.FormatInt(now, 10)))
	}
	rows := hqworkspace.ReconciliationWakeQueue(d, now)
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

const workspaceContinuationWake = "workspace reconciliation follow-up"

func workspaceContinuationLine(rows []hqworkspace.Reconciliation, check string) string {
	latest := rows[len(rows)-1].Observation
	// The queue key coalesces waiting input. The check timestamp gives each
	// periodic attempt its own native delivery ID even if the facts match.
	return hqwake.Line(hqwake.ClassTick, workspaceContinuationWake+" "+check,
		fmt.Sprintf("pending:%d · %s", len(rows), latest.Ref),
		"claim the new evidence: gtmux hq workspace reconcile --json")
}

func revalidateWorkspaceReconciliation(d hqworkspace.Database, now int64, line string) (bool, string) {
	if _, tail, ok := strings.Cut(line, workspaceContinuationWake); ok {
		rows := hqworkspace.ContinuationWakeQueue(d, now)
		if len(rows) == 0 {
			return false, ""
		}
		check, _, _ := strings.Cut(tail, hqwake.FieldSep())
		return true, workspaceContinuationLine(rows, strings.TrimSpace(check))
	}
	pending := 0
	repeated := false
	for _, q := range hqworkspace.ReconciliationWakeQueue(d, now) {
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
