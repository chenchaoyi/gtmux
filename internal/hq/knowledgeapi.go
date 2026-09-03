package hq

// The knowledge base, as an API for callers that are not the CLI (hq-knowledge-on-phone).
//
// Two doors, one ledger. The CLI's verbs are gated on the HQ home by cwd, which keeps
// WORKERS out of the quality gate — the right rule for the machine's other sessions. The
// commander is a different caller: they outrank the supervisor, and `land` in particular
// is a fact only they hold, since only the person who carried a lesson into a runbook or a
// PR knows it landed. So serve gets its own door, authorized by owner scope rather than by
// cwd, and both doors run the SAME verbs below and journal the same audit record.
//
// The read side is split index/detail on a size judgment: this machine's base is 330
// entries, whose bodies together are megabytes and whose identities are tens of kilobytes.
// A phone polls the index and asks for a body when a finger lands on one.

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
)

// KnowledgeEntryRow is one live entry WITHOUT its body — the index unit.
type KnowledgeEntryRow struct {
	ID    string `json:"id"`
	Topic string `json:"topic"`
	Title string `json:"title"`
	At    int64  `json:"at"`
	Seq   int64  `json:"seq,omitempty"`
	// Provenance of a consumed capture candidate, when this entry folded one in.
	Pane    string `json:"pane,omitempty"`
	Task    string `json:"task,omitempty"`
	Capture string `json:"capture,omitempty"`
	Legacy  bool   `json:"legacy,omitempty"`
	// Promotion lifecycle. Pending is PromotedAt > 0 with LandedAt == 0.
	PromotedAt    int64  `json:"promoted_at,omitempty"`
	PromoteWhy    string `json:"promote_why,omitempty"`
	PromoteTarget string `json:"promote_target,omitempty"`
	LandedAt      int64  `json:"landed_at,omitempty"`
	LandedRef     string `json:"landed_ref,omitempty"`
}

// KnowledgeEntryFull is one entry WITH its body, for the detail read.
type KnowledgeEntryFull struct {
	KnowledgeEntryRow
	Body string `json:"body"`
}

// KnowledgeTopicRow is one topic in the vocabulary, with what it holds.
type KnowledgeTopicRow struct {
	Name    string `json:"name"`
	Count   int    `json:"count"`
	Desc    string `json:"desc,omitempty"`
	Builtin bool   `json:"builtin"`
}

// KnowledgeQueue is a maintenance figure: how much is outstanding and how old the
// oldest of it is. Used for both the promotion queue and the candidate spool, because
// the question ("is this being kept up with?") is the same for both.
type KnowledgeQueue struct {
	Pending   int   `json:"pending"`
	OldestSec int64 `json:"oldest_sec,omitempty"`
}

// KnowledgeIndexPayload is the whole index: entries newest first, the vocabulary, and
// the two maintenance figures.
type KnowledgeIndexPayload struct {
	Entries    []KnowledgeEntryRow `json:"entries"`
	Topics     []KnowledgeTopicRow `json:"topics"`
	Promotions KnowledgeQueue      `json:"promotions"`
	Candidates KnowledgeQueue      `json:"candidates"`
}

// rowOf projects a folded live op onto the index row.
func rowOf(op knowledgeOp) KnowledgeEntryRow {
	return KnowledgeEntryRow{
		ID: op.ID, Topic: op.Topic, Title: op.Title, At: op.At, Seq: op.Seq,
		Pane: op.Pane, Task: op.Task, Capture: op.Capture, Legacy: op.Legacy,
		PromotedAt: op.PromotedAt, PromoteWhy: op.PromoteWhy, PromoteTarget: op.PromoteTarget,
		LandedAt: op.LandedAt, LandedRef: op.LandedRef,
	}
}

// KnowledgeIndex reports the base without bodies.
//
// An unreadable or absent ledger is an EMPTY base, not an error: a Mac with no HQ home is
// an ordinary machine, and a client that has to tell "no knowledge" apart from "the read
// failed" would render the same thing for both anyway.
func KnowledgeIndex(now int64) KnowledgeIndexPayload {
	out := KnowledgeIndexPayload{Entries: []KnowledgeEntryRow{}, Topics: []KnowledgeTopicRow{}}
	live, custom, err := readKnowledgeState()
	if err != nil {
		return out
	}
	counts := map[string]int{}
	for _, op := range live {
		out.Entries = append(out.Entries, rowOf(op))
		counts[op.Topic]++
	}
	// Newest first: the review question on a phone is "what did it just learn", and the
	// ledger's own order is the order it was written.
	for i, j := 0, len(out.Entries)-1; i < j; i, j = i+1, j-1 {
		out.Entries[i], out.Entries[j] = out.Entries[j], out.Entries[i]
	}
	descs := map[string]string{}
	for _, t := range custom {
		descs[t.ID] = t.Body
	}
	for _, name := range builtinTopics {
		out.Topics = append(out.Topics, KnowledgeTopicRow{Name: name, Count: counts[name], Builtin: true})
	}
	for _, t := range custom {
		out.Topics = append(out.Topics, KnowledgeTopicRow{Name: t.ID, Count: counts[t.ID], Desc: descs[t.ID]})
	}
	pending, oldestAt := pendingPromotions(live)
	out.Promotions = KnowledgeQueue{Pending: len(pending)}
	if len(pending) > 0 && oldestAt > 0 {
		out.Promotions.OldestSec = now - oldestAt
	}
	if cands, err := readCandidates(); err == nil && len(cands) > 0 {
		oldest := cands[0].At
		for _, c := range cands {
			if c.At < oldest {
				oldest = c.At
			}
		}
		out.Candidates = KnowledgeQueue{Pending: len(cands), OldestSec: now - oldest}
	}
	return out
}

// KnowledgeIndexJSON is KnowledgeIndex marshaled, for the serve dep.
func KnowledgeIndexJSON(now int64) ([]byte, error) { return json.Marshal(KnowledgeIndex(now)) }

// KnowledgeEntry returns one live entry with its body. ok=false means no such live entry
// (retired entries are gone from the live set by design — the ledger keeps the history).
func KnowledgeEntry(id string) (KnowledgeEntryFull, bool) {
	live, err := liveKnowledge()
	if err != nil {
		return KnowledgeEntryFull{}, false
	}
	op, ok := findLive(live, id)
	if !ok {
		return KnowledgeEntryFull{}, false
	}
	return KnowledgeEntryFull{KnowledgeEntryRow: rowOf(op), Body: op.Body}, true
}

// KnowledgeEntryJSON is KnowledgeEntry marshaled, for the serve dep.
func KnowledgeEntryJSON(id string) ([]byte, bool, error) {
	e, ok := KnowledgeEntry(id)
	if !ok {
		return nil, false, nil
	}
	b, err := json.Marshal(e)
	return b, true, err
}

// KnowledgeLand closes a pending promotion: the lesson reached somewhere durable, and
// `ref` names where. The brief is removed and the ledger keeps the whole lifecycle.
//
// This is the verb that most needed a second door. HQ can decide a lesson is
// charter-level, but it cannot know it was carried — only the person who carried it can,
// and they are not always at the Mac.
func KnowledgeLand(id, ref string) error {
	if id == "" || ref == "" {
		return fmt.Errorf("land needs <id> and a ref (where it landed: a PR, a spec, a runbook)")
	}
	live, err := liveKnowledge()
	if err != nil {
		return err
	}
	entry, ok := findLive(live, id)
	if !ok {
		return fmt.Errorf("no live entry %q (gtmux knowledge list)", id)
	}
	if !promotionPending(entry) {
		return fmt.Errorf("%s has no pending promotion to land (gtmux knowledge promotions)", id)
	}
	op := knowledgeOp{
		Op: knowledgeOpLand, ID: id, Topic: entry.Topic,
		At: time.Now().Unix(), Seq: events.LatestSeq(), Ref: ref,
	}
	return commitKnowledgeOp(op, "land "+id+" → "+ref)
}

// KnowledgeRetire removes a live entry, with a reason that survives in the ledger.
//
// The reason is required for the same purpose everywhere else in this subsystem: an entry
// that was wrong enough to remove is worth knowing about later, and "why did this go away"
// is unanswerable from an absence.
func KnowledgeRetire(id, why string) error {
	if id == "" || why == "" {
		return fmt.Errorf("retire needs <id> and a reason (it survives; make it worth reading)")
	}
	live, err := liveKnowledge()
	if err != nil {
		return err
	}
	entry, ok := findLive(live, id)
	if !ok {
		return fmt.Errorf("no live entry %q (gtmux knowledge list)", id)
	}
	op := knowledgeOp{
		Op: knowledgeOpRetire, ID: id, Topic: entry.Topic,
		At: time.Now().Unix(), Seq: events.LatestSeq(), Why: why,
	}
	return commitKnowledgeOp(op, "retire "+id+": "+why)
}
