// The journal's AUDIT sub-namespace (hq-action-journal): gtmux's own acts —
// a wake delivered or dropped, a supervisor-driven send, a reap, a rotation —
// recorded in the same stream that already journals everyone else.
//
// Audit records are TRAIL, not debt. They document acts their actor already
// knows about, so the unread tally and the supervisor's default pull both
// exclude them (a delivered wake that minted fresh debt would knock about
// itself forever — the self-feeding loop the watermark work killed). They nest
// inside the `gtmux:` control namespace on purpose: every sensor that excludes
// control records from its measured deltas covers them with no further case
// analysis.
package events

import "strings"

// AuditPrefix namespaces the audit trail inside the control namespace, so
// IsControl(r) is true for every audit record by construction.
const AuditPrefix = ControlPrefix + "audit:"

// IsAudit reports whether a record is gtmux's own audit trail — an act already
// known to its actor, excluded from the consumption debt and the supervisor's
// default pull, never from the log.
func IsAudit(r Record) bool { return strings.HasPrefix(r.Event, AuditPrefix) }

// Audit event names.
const (
	// AuditEventWakeDelivered records one CONFIRMED wake batch: the id and the
	// full coalesced payload, one record per batch.
	AuditEventWakeDelivered = AuditPrefix + "wake-delivered"
	// AuditEventWakeDropped records one dropped wake line and why — the control
	// trace "never silently dropped" needs to be a journal property.
	AuditEventWakeDropped = AuditPrefix + "wake-dropped"
	// AuditEventSend records a `gtmux send` text delivery's settlement.
	AuditEventSend = AuditPrefix + "send"
	// AuditEventReap records a reap that actually reclaimed a dispatch, written
	// before the ledger entry it describes is removed.
	AuditEventReap = AuditPrefix + "reap"
	// AuditEventRotate records the `gtmux hq --rotate` act with the retiring
	// session id — the pointer the state files overwrite.
	AuditEventRotate = AuditPrefix + "rotate"
	// AuditEventHQSession records the health sensor observing the HQ session id
	// change: the successor chain that survives every handoff.
	AuditEventHQSession = AuditPrefix + "hq-session"
)

// Wake-drop reasons (the wake-dropped record's vocabulary).
const (
	// DropEvicted — the queue cap evicted the lowest-priority oldest entry.
	DropEvicted = "evicted"
	// DropUnconfirmed — the ack budget was exhausted (drain path or an abandoned
	// Enter repair); the degradation counter has already announced the doubt.
	DropUnconfirmed = "unconfirmed"
	// DropSuperseded — the delivery-side revalidation probe found the premise
	// gone, so the line was dropped instead of delivered stale.
	DropSuperseded = "superseded"
)

// Audit summary budgets (bytes of rendered text, truncated rune-safe). A wake
// batch is already capped near 800 chars by the delivery layer; the send head
// mirrors the 200-char reply-tail convention.
const (
	auditWakeMax = 1000
	auditSendMax = 200
	auditReapMax = 300
)

// auditLine renders arbitrary payload text as one bounded journal-safe line:
// newlines collapse to spaces and over-budget text truncates at a rune
// boundary with an ellipsis.
func auditLine(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= max {
		return s
	}
	cut := 0
	for i := range s { // ranges by rune start, so the cut never splits a rune
		if i > max {
			break
		}
		cut = i
	}
	return s[:cut] + "…"
}

// auditAppend stamps the invariant parts of every audit record: routine
// severity (a trail never asks for attention) and the caller's clock.
func auditAppend(r Record, now int64) {
	r.Ts = now
	r.Severity = SevRoutine
	Append(r)
}

// AuditWakeDelivered journals one confirmed wake batch. pane is the HQ pane it
// landed in; payload is the full coalesced line, id included.
func AuditWakeDelivered(pane, payload string, now int64) {
	auditAppend(Record{
		Event: AuditEventWakeDelivered, Pane: pane,
		Summary: auditLine(payload, auditWakeMax),
	}, now)
}

// AuditWakeDropped journals one dropped wake line with its reason
// (DropEvicted / DropUnconfirmed / DropSuperseded).
func AuditWakeDropped(reason, line string, now int64) {
	auditAppend(Record{
		Event:   AuditEventWakeDropped,
		Summary: reason + ": " + auditLine(line, auditWakeMax),
	}, now)
}

// AuditSend journals a `gtmux send` text delivery's settlement: the target
// pane, the outcome state, and a bounded head of the payload.
func AuditSend(pane, state, payload string, now int64) {
	auditAppend(Record{
		Event: AuditEventSend, Pane: pane,
		Summary: state + ": " + auditLine(payload, auditSendMax),
	}, now)
}

// AuditReap journals a reap that reclaimed a dispatch — called before the
// ledger entry is removed, so the journal keeps what the ledger forgets.
func AuditReap(taskID, pane, actions string, now int64) {
	auditAppend(Record{
		Event: AuditEventReap, Pane: pane,
		Summary: taskID + ": " + auditLine(actions, auditReapMax),
	}, now)
}

// AuditRotate journals the `gtmux hq --rotate` act: the retiring session id
// (empty reads as unknown) and the reset input typed.
func AuditRotate(retiringSession, input string, now int64) {
	if retiringSession == "" {
		retiringSession = "?"
	}
	auditAppend(Record{
		Event:   AuditEventRotate,
		Summary: "session " + retiringSession + " → reset (" + input + ")",
	}, now)
}

// AuditHQSession journals the successor chain when the health sensor observes
// the HQ session id REPLACE a known predecessor. A first observation is not a
// replacement and writes nothing (callers guard): nothing is being lost — the
// live resume record still names that session — and a healthy first sight
// keeping the stream untouched is the sensors' silence discipline.
func AuditHQSession(successor, predecessor string, now int64) {
	auditAppend(Record{
		Event:   AuditEventHQSession,
		Summary: successor + " replaces " + predecessor,
	}, now)
}
