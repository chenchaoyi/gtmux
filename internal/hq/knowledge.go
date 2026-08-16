// The knowledge LEDGER (hq-knowledge-ledger): the knowledge base's authority is an
// append-only operation log, and the topic markdown files are deterministic renders
// of its live entries. This is what gives a lesson PROVENANCE — the capture
// candidate's pane/seq/task stop evaporating at merge time — and gives the
// charter's "update-over-append / prune" discipline a mechanical form (supersede /
// retire operations whose history survives).
//
// This file is the substrate: the op record, append/read, validation, and the
// live-set fold. The renders live in knowledgerender.go, the verbs in
// knowledgecmd.go.
package hq

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// knowledgeSchemaV is the ledger's record schema version, stamped on every op.
const knowledgeSchemaV = 1

// Ledger op names.
const (
	knowledgeOpAdd       = "add"
	knowledgeOpSupersede = "supersede"
	knowledgeOpRetire    = "retire"
)

// Content bounds. They refuse LOUDLY at write time — knowledge is curated
// content, and a silent truncation would corrupt exactly the thing the ledger
// exists to keep trustworthy.
const (
	knowledgeTitleMax = 200
	knowledgeBodyMax  = 8 << 10
	knowledgeWhyMax   = 300
)

// knowledgeOp is one ledger line. Adds and supersedes carry content; retires
// carry a reason. Provenance fields are optional individually, but the write
// path always stamps Seq (the event high-water mark) so every entry can at least
// be placed on the stream.
type knowledgeOp struct {
	V     int    `json:"v"`
	Op    string `json:"op"`
	ID    string `json:"id"`
	Topic string `json:"topic"`
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
	At    int64  `json:"at"`
	// Provenance: where this lesson came from.
	Seq      int64   `json:"seq"`                 // event high-water mark at write
	SeqRange string  `json:"seq_range,omitempty"` // the distill delta, "a..b"
	Seqs     []int64 `json:"seqs,omitempty"`      // consumed candidates' seqs
	Pane     string  `json:"pane,omitempty"`      // consumed candidate's pane
	Task     string  `json:"task,omitempty"`      // consumed candidate's task
	Capture  string  `json:"capture,omitempty"`   // consumed candidate key
	Legacy   bool    `json:"legacy,omitempty"`    // migrated from a legacy file
	// Supersedes names the predecessor for op=supersede; Why the reason for
	// op=retire (and optionally colors a supersede).
	Supersedes string `json:"supersedes,omitempty"`
	Why        string `json:"why,omitempty"`
}

// knowledgeLedgerPath is the append-only authority, dot-prefixed like the
// pending-distill spool so it stays out of the curated topic listing.
func knowledgeLedgerPath() string { return filepath.Join(hqKnowledgeDir(), ".ledger.jsonl") }

// knowledgeTopics is the topic vocabulary an entry may be filed under: the
// capture topics plus `environment` (machine-specific, so not a capture target,
// but still curated knowledge). README is an index, never a topic.
func knowledgeTopics() []string { return append(append([]string{}, captureTopics...), "environment") }

func validKnowledgeTopic(topic string) bool {
	for _, t := range knowledgeTopics() {
		if t == topic {
			return true
		}
	}
	return false
}

// validateKnowledgeContent enforces the bounds, loudly. An empty title is the
// caller's error to have prevented (add/supersede require one).
func validateKnowledgeContent(title, body, why string) error {
	if strings.ContainsAny(title, "\n\r") {
		return fmt.Errorf("title must be a single line")
	}
	if len(title) > knowledgeTitleMax {
		return fmt.Errorf("title is %d bytes — the limit is %d; move detail into the body", len(title), knowledgeTitleMax)
	}
	if len(body) > knowledgeBodyMax {
		return fmt.Errorf("body is %d bytes — the limit is %d; trim it (knowledge is curated, not archived)", len(body), knowledgeBodyMax)
	}
	if len(why) > knowledgeWhyMax {
		return fmt.Errorf("--why is %d bytes — the limit is %d", len(why), knowledgeWhyMax)
	}
	return nil
}

// appendKnowledgeOp validates and appends one ledger line (O_APPEND, one line,
// atomic enough across concurrent writers — the capture spool's discipline).
func appendKnowledgeOp(op knowledgeOp) error {
	if op.V == 0 {
		op.V = knowledgeSchemaV
	}
	if err := validateKnowledgeContent(op.Title, op.Body, op.Why); err != nil {
		return err
	}
	if err := os.MkdirAll(hqKnowledgeDir(), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(op)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(knowledgeLedgerPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}

// readKnowledgeOps loads the ledger (empty when absent). A malformed line is
// skipped, never fatal — one bad write must not take the whole base hostage.
func readKnowledgeOps() ([]knowledgeOp, error) {
	f, err := os.Open(knowledgeLedgerPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []knowledgeOp
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var op knowledgeOp
		if json.Unmarshal([]byte(line), &op) == nil && op.Op != "" && op.ID != "" {
			out = append(out, op)
		}
	}
	return out, sc.Err()
}

// foldKnowledge replays the ledger into the LIVE entry set, in ledger order.
// add → live; supersede → predecessor dead, successor live; retire → dead.
// Robustness over ceremony: a same-id re-add is last-wins here (the WRITE path
// refuses the collision; the fold must still read a ledger that has one).
func foldKnowledge(ops []knowledgeOp) []knowledgeOp {
	index := map[string]int{} // id → position in out; -1 = dead
	var out []knowledgeOp
	place := func(op knowledgeOp) {
		if i, ok := index[op.ID]; ok && i >= 0 {
			out[i] = op // last-wins on the same id, keeping the original position
			return
		}
		index[op.ID] = len(out)
		out = append(out, op)
	}
	kill := func(id string) {
		if i, ok := index[id]; ok && i >= 0 {
			out[i].Op = "" // tombstone in place; compacted below
			index[id] = -1
		}
	}
	for _, op := range ops {
		switch op.Op {
		case knowledgeOpAdd:
			place(op)
		case knowledgeOpSupersede:
			kill(op.Supersedes)
			place(op)
		case knowledgeOpRetire:
			kill(op.ID)
		}
	}
	live := out[:0]
	for _, op := range out {
		if op.Op != "" {
			live = append(live, op)
		}
	}
	return live
}

// liveKnowledge reads and folds in one step.
func liveKnowledge() ([]knowledgeOp, error) {
	ops, err := readKnowledgeOps()
	if err != nil {
		return nil, err
	}
	return foldKnowledge(ops), nil
}

// findLive returns the live entry with id, if any.
func findLive(live []knowledgeOp, id string) (knowledgeOp, bool) {
	for _, op := range live {
		if op.ID == id {
			return op, true
		}
	}
	return knowledgeOp{}, false
}
