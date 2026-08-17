// Package hqfeed names HQ's perception-surface vocabulary: the journal
// control-record event names the slow-tick sensors raise, and the surfacing
// tiers/threshold that gate what HQ prints to the user (the `gtmux quiet`
// switch). The event stream itself lives in internal/events — HQ perceives by
// wake-knock plus `gtmux events --since-seq` pull, with a retention gap warned
// at read time (retire-perception-spool); there is no push copy of the stream.
package hqfeed

// Control-record event names. All are JOURNAL-borne — appended to the session
// event stream, where they count toward HQ's consumption debt and reach it
// through the completeness net (#647: a record written anywhere else reaches no
// reader). They ride the same events.Record shape as every other event.
const (
	// ControlWakeDegraded reports that the WAKE side is broken — knocks are queued
	// but never confirmed on HQ's screen. It must reach the pull stream precisely
	// because the channel that would otherwise announce it is the one that failed.
	ControlWakeDegraded = "gtmux:wake-degraded"
	// ControlSelfCheck asks HQ to run its periodic housekeeping pass (ledger
	// archival, memory health). Raised on a cadence by the serve slow-tick.
	ControlSelfCheck = "gtmux:self-check"
	// ControlDistill asks HQ to run a periodic knowledge-distillation pass: distil the
	// fleet's event delta since the last distill into the knowledge base and prune
	// stale. A low-urgency maintenance signal like self-check (a journal control
	// record, not a typed wake) — HQ does the curation; gtmux only raises it on a
	// cadence.
	ControlDistill = "gtmux:distill"
)
