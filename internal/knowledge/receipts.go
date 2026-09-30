package knowledge

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/i18n"
)

func mergeSources(a, b []Candidate) []Candidate {
	seen := map[string]bool{}
	var out []Candidate
	for _, sources := range [][]Candidate{a, b} {
		for _, c := range sources {
			if !seen[c.ID] {
				seen[c.ID] = true
				out = append(out, c)
			}
		}
	}
	return out
}

// selectCandidateKeys is read-only. A miss refuses the whole request before commit.
// It must run inside the same lock as the accepting/dismissing ledger append.
func selectCandidateKeys(keys []string) ([]Candidate, error) {
	cands, err := readCandidates()
	if err != nil {
		return nil, err
	}
	want := map[string]bool{}
	for _, k := range keys {
		want[k] = false
	}
	var selected []Candidate
	for _, c := range cands {
		if _, ok := want[c.Key]; !ok {
			continue
		}
		if c.Digest != candidateDigest(c) {
			return nil, fmt.Errorf("candidate %s source digest does not match", c.ID)
		}
		c.Group = 0
		selected = append(selected, c)
		want[c.Key] = true
	}
	for _, k := range keys {
		if !want[k] {
			return nil, fmt.Errorf("no pending candidate with key %q (gtmux capture --list; gtmux knowledge receipts --capture %s)", k, k)
		}
	}
	return selected, nil
}

// CommitError distinguishes a safe retry from a committed operation whose derived
// views need repair. Errors never rely on telemetry having succeeded.
type CommitError struct {
	OpID      string
	Committed bool
	Phase     string
	Err       error
}

func (e *CommitError) Error() string {
	if e.Committed {
		return i18n.Tr("operation "+e.OpID+" committed; "+e.Phase+" failed: "+e.Err.Error()+"; inspect knowledge receipts/show and run `gtmux knowledge render` to repair views",
			"操作 "+e.OpID+" 已提交；"+e.Phase+" 失败："+e.Err.Error()+"；请查 receipts/show，并运行 `gtmux knowledge render` 修复视图")
	}
	return i18n.Tr("operation "+e.OpID+" not committed; candidates remain pending: "+e.Err.Error(),
		"操作 "+e.OpID+" 未提交，候选仍待处理："+e.Err.Error())
}
func (e *CommitError) Unwrap() error { return e.Err }

func commitKnowledgeWithSources(op knowledgeOp, keys []string, note string) error {
	op.OpID = diag.NewOpID()
	committed, phase := false, "selection"
	err := withKnowledgeLock(func() error {
		if len(keys) > 0 {
			selected, err := selectCandidateKeys(keys)
			if err != nil {
				return err
			}
			op.Sources, op.Capture = selected, strings.Join(keys, ",")
			op.CandidateResult = "accepted"
			if op.Op == knowledgeOpDismiss {
				op.CandidateResult = "dismissed"
			}
			mined := false
			for _, c := range selected {
				if c.Seq > 0 {
					op.Seqs = append(op.Seqs, c.Seq)
				}
				if c.Source == "transcript" {
					mined = true
				}
				op.Hits += max(1, c.Count)
			}
			newest := selected[len(selected)-1]
			op.Pane, op.Task = newest.Pane, newest.Task
			if op.Provenance == "" {
				op.Provenance = ProvCapture
				if mined {
					op.Provenance = ProvMined
				}
			}
			if op.Op == knowledgeOpDismiss {
				op.ID = "dismiss/" + op.OpID
				note = fmt.Sprintf("dismiss %s ×%d: %s", op.Capture, len(selected), op.Why)
			} else {
				note += fmt.Sprintf(" (capture %s ×%d)", op.Capture, len(selected))
			}
		}
		phase = "ledger"
		ops, err := readKnowledgeOps()
		if err != nil {
			return err
		}
		live := foldKnowledge(ops)
		if op.Op == knowledgeOpAdd {
			if _, exists := findLive(live, op.ID); exists {
				return fmt.Errorf("id %s is a live entry; use supersede", op.ID)
			}
		}
		if op.Op == knowledgeOpSupersede {
			if _, exists := findLive(live, op.Supersedes); !exists {
				return fmt.Errorf("no live entry %q", op.Supersedes)
			}
			if op.ID != op.Supersedes {
				if _, exists := findLive(live, op.ID); exists {
					return fmt.Errorf("id %s is a live entry", op.ID)
				}
			}
		}
		committed, err = appendKnowledgeOpLocked(op)
		if !committed {
			return err
		}
		outcome := op.CandidateResult
		if outcome == "" {
			outcome = "committed"
		}
		events.AuditKnowledgeOutcome(note, time.Now().Unix(), op.OpID, outcome, "ledger")
		if err != nil {
			phase = "directory-sync"
			return err
		}
		if op.Op == knowledgeOpDismiss {
			i18n.Say(fmt.Sprintf("dismissed %d candidate(s) under %s", len(op.Sources), op.Capture), fmt.Sprintf("已驳回 %d 条候选（%s）", len(op.Sources), op.Capture))
			return nil
		}
		phase = "render"
		live, custom, err := readKnowledgeState()
		if err != nil {
			return err
		}
		if err := renderAllTopics(live, custom, time.Now().Unix()); err != nil {
			return err
		}
		if err := renderPromotions(live); err != nil {
			return err
		}
		i18n.Say("✓ "+note, "✓ "+note)
		return nil
	})
	if err != nil {
		outcome := "failed"
		if committed {
			outcome = "committed-view-failed"
		}
		events.AuditKnowledgeOutcome(note, time.Now().Unix(), op.OpID, outcome, phase)
		return &CommitError{OpID: op.OpID, Committed: committed, Phase: phase, Err: err}
	}
	return nil
}

type CandidateReceipt struct {
	OpID    string      `json:"op_id"`
	At      int64       `json:"at"`
	Outcome string      `json:"outcome"`
	EntryID string      `json:"entry_id,omitempty"`
	Why     string      `json:"why,omitempty"`
	Sources []Candidate `json:"sources"`
}

func knowledgeReceipts(args []string) int {
	f, err := parseKnowledgeFlags(args)
	if err != nil || len(f.positional) != 0 {
		i18n.Sae("receipts takes [--capture key] [--json]", "receipts 接受 [--capture 键] [--json]")
		return 2
	}
	ops, err := readKnowledgeOps()
	if err != nil {
		i18n.Sae(err.Error(), err.Error())
		return 1
	}
	out := make([]CandidateReceipt, 0)
	for _, op := range ops {
		if op.CandidateResult == "" {
			continue
		}
		if len(f.captures) > 0 {
			match := false
			for _, c := range op.Sources {
				if contains(f.captures, c.Key) {
					match = true
				}
			}
			if !match {
				continue
			}
		}
		r := CandidateReceipt{OpID: op.OpID, At: op.At, Outcome: op.CandidateResult, Why: op.Why, Sources: op.Sources}
		if op.Op != knowledgeOpDismiss {
			r.EntryID = op.ID
		}
		out = append(out, r)
	}
	if f.jsonOut {
		if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
			i18n.Sae(err.Error(), err.Error())
			return 1
		}
		return 0
	}
	for _, r := range out {
		fmt.Printf("%s · %s · %s · %d sources\n", r.OpID, r.Outcome, r.EntryID, len(r.Sources))
		if r.Why != "" {
			fmt.Println("  " + r.Why)
		}
		for _, c := range r.Sources {
			fmt.Printf("  %s · %s · %s\n", c.ID, c.Key, c.Lesson)
		}
	}
	return 0
}
