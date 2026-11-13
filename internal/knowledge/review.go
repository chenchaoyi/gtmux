package knowledge

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
)

// ReviewProposal is shared by a human editor and a dreaming pass. It is inert
// data until the HQ/owner write path applies it. No inferred use or recurrence
// is recorded by reviewing, exporting, or applying a proposal.
type ReviewProposal struct {
	Version   int            `json:"version"`
	ID        string         `json:"id"`
	Origin    string         `json:"origin"` // external-editor, dreaming, review
	Changes   []ReviewChange `json:"changes"`
	Conflicts []string       `json:"conflicts,omitempty"`
	Missing   []string       `json:"missing_local_files,omitempty"`
	Drafts    []string       `json:"unfiled_drafts,omitempty"`
}

type ReviewChange struct {
	ID           string      `json:"id"`
	BaseRevision string      `json:"base_revision"`
	Operation    string      `json:"operation"` // revise, retire
	Title        string      `json:"title,omitempty"`
	Body         string      `json:"body,omitempty"`
	Why          string      `json:"why"`
	Evidence     []string    `json:"evidence"`
	Memory       *MemoryRule `json:"memory,omitempty"`
}

type ReviewResult struct {
	ProposalID      string   `json:"proposal_id"`
	Committed       bool     `json:"committed"`
	AlreadyApplied  bool     `json:"already_applied,omitempty"`
	Entries         []string `json:"entries"`
	PendingCarriers []string `json:"pending_carriers,omitempty"`
}

func entryRevision(op knowledgeOp) string {
	b, _ := json.Marshal(op)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func proposalDigest(p ReviewProposal) string {
	p.ID = ""
	b, _ := json.Marshal(p)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func validateMemory(m *MemoryRule) error {
	if m == nil {
		return nil
	}
	if !contains([]string{"local", "project", "hq", "global"}, m.Scope) {
		return fmt.Errorf("memory scope must be local, project, hq, or global")
	}
	if m.Scope == "project" && (!filepath.IsAbs(m.Project) || filepath.Clean(m.Project) != m.Project) {
		return fmt.Errorf("project scope needs a canonical absolute project path")
	}
	if m.Scope != "project" && m.Project != "" {
		return fmt.Errorf("project is only valid for project scope")
	}
	if !contains([]string{"reported", "supported", "tentative", "contested"}, m.Confidence) {
		return fmt.Errorf("memory confidence must be reported, supported, tentative, or contested")
	}
	if len(m.Actions) == 0 || strings.TrimSpace(m.Summary) == "" || len(m.Evidence) == 0 {
		return fmt.Errorf("memory needs actions, an actionable summary, and evidence references")
	}
	if len(m.Actions) > 16 || len(m.Exceptions) > 16 || len(m.Evidence) > 32 || len(m.ToolVersions) > 16 {
		return fmt.Errorf("memory metadata is too large")
	}
	for _, items := range [][]string{m.Actions, m.Exceptions, m.Evidence} {
		for _, v := range items {
			if strings.TrimSpace(v) == "" || len(v) > 500 {
				return fmt.Errorf("memory values must be 1–500 bytes")
			}
		}
	}
	if len(m.Summary) > 4096 || len(m.ProtectedReason) > 500 {
		return fmt.Errorf("memory summary/protection reason is too large")
	}
	for k, v := range m.ToolVersions {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" || len(k) > 100 || len(v) > 100 {
			return fmt.Errorf("tool versions must have bounded, nonempty names and values")
		}
	}
	if m.VerifiedAt < 0 || m.ReviewAfter < 0 || m.VerifiedAt > time.Now().Unix() {
		return fmt.Errorf("invalid verification time")
	}
	return nil
}

func protectedEntry(e knowledgeOp) bool {
	return e.Provenance == ProvCorrection || e.Topic == "corrections" || e.PromotedAt > 0 || e.Memory != nil && e.Memory.ProtectedReason != ""
}

// prepareReview is pure: even a dry run does not create a lock, consume a
// candidate, change usage, or advance any HQ watermark.
func prepareReview(p ReviewProposal, ops []knowledgeOp, now int64) ([]knowledgeOp, ReviewResult, error) {
	r := ReviewResult{ProposalID: p.ID, Entries: []string{}}
	if len(p.Conflicts) > 0 {
		return nil, r, fmt.Errorf("resolve proposal conflicts before applying")
	}
	if p.Version != 1 || len(p.Changes) == 0 || len(p.Changes) > 25 || p.ID != proposalDigest(p) {
		return nil, r, fmt.Errorf("proposal needs version 1, 1–25 changes, and its content digest as id")
	}
	if !contains([]string{"external-editor", "dreaming", "review"}, p.Origin) {
		return nil, r, fmt.Errorf("unknown review origin")
	}
	prefix := "review-" + p.ID + "-"
	already := 0
	for _, op := range ops {
		if strings.HasPrefix(op.OpID, prefix) {
			already++
		}
	}
	if already > 0 {
		if already != len(p.Changes) {
			return nil, r, fmt.Errorf("incomplete receipt set for proposal %s; inspect before retry", p.ID)
		}
		r.Committed, r.AlreadyApplied = true, true
		live := foldKnowledge(ops)
		for _, c := range p.Changes {
			r.Entries = append(r.Entries, c.ID)
			if e, ok := findLive(live, c.ID); ok && promotionPending(e) {
				r.PendingCarriers = append(r.PendingCarriers, c.ID)
			}
		}
		return nil, r, nil
	}
	live := foldKnowledge(ops)
	seen := map[string]bool{}
	var out []knowledgeOp
	for i, c := range p.Changes {
		if seen[c.ID] {
			return nil, r, fmt.Errorf("duplicate change for %s", c.ID)
		}
		seen[c.ID] = true
		e, ok := findLive(live, c.ID)
		if !ok || c.BaseRevision != entryRevision(e) {
			return nil, r, fmt.Errorf("revision conflict for %s; refresh and compare before applying", c.ID)
		}
		if strings.TrimSpace(c.Why) == "" || len(c.Evidence) == 0 || len(c.Evidence) > 32 {
			return nil, r, fmt.Errorf("%s needs a reason and evidence references", c.ID)
		}
		for _, ref := range c.Evidence {
			if strings.TrimSpace(ref) == "" || len(ref) > 500 {
				return nil, r, fmt.Errorf("invalid evidence reference for %s", c.ID)
			}
		}
		if err := validateKnowledgeContent(c.Title, c.Body, c.Why); err != nil {
			return nil, r, err
		}
		op := knowledgeOp{ID: e.ID, Topic: e.Topic, At: now, Why: c.Why, OpID: fmt.Sprintf("%s%d", prefix, i), ReviewEvidence: c.Evidence, ReviewOrigin: p.Origin}
		switch c.Operation {
		case "retire":
			if protectedEntry(e) {
				return nil, r, fmt.Errorf("%s is protected; ordinary review cannot retire a correction or distributed rule", e.ID)
			}
			op.Op = knowledgeOpRetire
		case "revise":
			if strings.TrimSpace(c.Title) == "" {
				return nil, r, fmt.Errorf("%s needs a title", e.ID)
			}
			if err := validateMemory(c.Memory); err != nil {
				return nil, r, err
			}
			if e.Memory == nil && c.Memory != nil {
				// Retrieval metadata cannot silently widen a previously declared audience.
				m := c.Memory
				if e.Audience == AudienceHQ && m.Scope != "hq" || e.Audience == AudienceRepo && (m.Scope != "project" || m.Project != cleanRepo(e.AudienceRepo)) || e.Audience == AudienceMachine && m.Scope == "global" {
					return nil, r, fmt.Errorf("memory scope would widen the declared audience of %s", e.ID)
				}
			}
			if e.Memory != nil && c.Memory != nil {
				if e.Memory.ProtectedReason != "" && c.Memory.ProtectedReason != e.Memory.ProtectedReason {
					return nil, r, fmt.Errorf("ordinary review cannot remove/change protection for %s", e.ID)
				}
				if e.Memory.Scope != c.Memory.Scope || e.Memory.Project != c.Memory.Project {
					return nil, r, fmt.Errorf("this review command cannot change the existing scope of %s", e.ID)
				}
			}
			op.Op, op.Supersedes, op.Title, op.Body = knowledgeOpSupersede, e.ID, c.Title, c.Body
			op.Reviewed, op.Memory, op.Lang = true, c.Memory, e.Lang
			inheritSourceOrigins(&op, e)
			op.Status = e.Status
			// Store the reopened distribution intent explicitly as well, so a
			// previous schema-3 reader does not claim the old landing or lose it.
			op.PromotedAt, op.PromoteWhy, op.PromoteTarget = e.PromotedAt, e.PromoteWhy, e.PromoteTarget
			op.Audience, op.AudienceRepo = e.Audience, e.AudienceRepo
			// A prose edit is not evidence that the old action card is still valid.
			// Preserve its scope/protection, but stop automatically using it until
			// a reviewer supplies an updated card with evidence.
			if op.Memory == nil && e.Memory != nil {
				copy := *e.Memory
				copy.Confidence, copy.VerifiedAt = "tentative", 0
				op.Memory = &copy
			}
			if e.PromotedAt > 0 {
				r.PendingCarriers = append(r.PendingCarriers, e.ID)
			}
		default:
			return nil, r, fmt.Errorf("unknown review operation %q", c.Operation)
		}
		if err := validateAxes(op); err != nil {
			return nil, r, err
		}
		out = append(out, op)
		r.Entries = append(r.Entries, e.ID)
	}
	return out, r, nil
}

// ApplyReview is the single owner/HQ transaction used by editors and dreaming.
// The caller authenticates the owner (API) or passes knowledgeMutation (CLI).
func ApplyReview(p ReviewProposal, dryRun bool) (ReviewResult, error) {
	if dryRun {
		ops, err := readKnowledgeOps()
		if err != nil {
			return ReviewResult{}, err
		}
		_, r, err := prepareReview(p, ops, time.Now().Unix())
		return r, err
	}
	var result ReviewResult
	err := withKnowledgeLock(func() error {
		ops, err := readKnowledgeOps()
		if err != nil {
			return err
		}
		now := time.Now().Unix()
		prepared, r, err := prepareReview(p, ops, now)
		result = r
		if err != nil {
			return err
		}
		if !result.AlreadyApplied {
			lines := make([][]byte, 0, len(prepared))
			seq := events.LatestSeq()
			for _, op := range prepared {
				op.V, op.Seq = knowledgeSchemaV, seq
				b, e := json.Marshal(op)
				if e != nil {
					return e
				}
				lines = append(lines, append(b, '\n'))
			}
			committed, e := atomicAppendLines(knowledgeLedgerPath(), lines)
			result.Committed = committed
			if e != nil {
				return &CommitError{OpID: p.ID, Committed: committed, Phase: "ledger", Err: e}
			}
			events.AuditKnowledgeOutcome("review "+p.ID, now, p.ID, "committed", "ledger")
		}
		live, custom, err := readKnowledgeState()
		if err == nil {
			err = renderAllTopics(live, custom, now)
		}
		if err == nil {
			err = renderPromotions(live)
		}
		if err != nil {
			return &CommitError{OpID: p.ID, Committed: true, Phase: "render", Err: err}
		}
		return nil
	})
	return result, err
}

func readReview(path string) (ReviewProposal, error) {
	var p ReviewProposal
	f, err := os.Open(path)
	if err != nil {
		return p, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return p, err
	}
	if info.Size() > 512<<10 {
		return p, fmt.Errorf("proposal exceeds 512 KiB")
	}
	d := json.NewDecoder(io.LimitReader(f, (512<<10)+1))
	d.DisallowUnknownFields()
	err = d.Decode(&p)
	if err == nil {
		var extra any
		if e := d.Decode(&extra); e != io.EOF {
			return p, fmt.Errorf("proposal must contain exactly one JSON object")
		}
	}
	if err == nil && p.ID == "" {
		p.ID = proposalDigest(p)
	}
	return p, err
}
