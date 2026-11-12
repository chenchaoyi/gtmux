package knowledge

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// DreamPlan is a bounded evidence packet for the existing HQ distill executor.
// It does not call a model, read native sessions, advance a queue, or spend tokens.
// Reported cost is kept outside the knowledge ledger; no new scheduler is needed.
type DreamRequest struct {
	Mode              string            `json:"mode"`
	IDs               []string          `json:"ids,omitempty"`
	Limit             int               `json:"limit,omitempty"`
	InputSpent        int64             `json:"input_spent"`
	GenerationSpent   int64             `json:"generation_spent"`
	UsageKnown        bool              `json:"usage_known"`
	AllowBurst        bool              `json:"allow_burst,omitempty"`
	ReviewedRevisions map[string]string `json:"reviewed_revisions,omitempty"`
}
type DreamItem struct {
	ID           string      `json:"id"`
	Revision     string      `json:"revision"`
	Title        string      `json:"title"`
	Body         string      `json:"body"`
	Reasons      []string    `json:"reasons"`
	Protected    bool        `json:"protected"`
	Evidence     []string    `json:"evidence,omitempty"`
	Audience     string      `json:"audience,omitempty"`
	AudienceRepo string      `json:"audience_repo,omitempty"`
	Memory       *MemoryRule `json:"memory,omitempty"`
}
type DreamBudget struct {
	InputLimit            int64  `json:"input_limit"`
	GenerationLimit       int64  `json:"billed_generation_limit"`
	InputRemaining        int64  `json:"input_remaining"`
	GenerationRemaining   int64  `json:"generation_remaining"`
	UsageKnown            bool   `json:"usage_known"`
	CanStart              bool   `json:"can_start"`
	EstimatedPacketTokens int    `json:"estimated_packet_tokens"`
	SuggestedGeneration   int64  `json:"suggested_generation_cap"`
	Note                  string `json:"note"`
}
type DreamPlan struct {
	Version      int            `json:"version"`
	ID           string         `json:"id"`
	Mode         string         `json:"mode"`
	Items        []DreamItem    `json:"items"`
	Budget       DreamBudget    `json:"budget"`
	Instructions []string       `json:"instructions"`
	Excluded     map[string]int `json:"excluded"`
}

func dreamLimits(mode string, burst bool) (int64, int64, error) {
	switch mode {
	case "pilot":
		return 500000, 200000, nil
	case "bootstrap":
		return 6000000, 1800000, nil
	case "routine":
		if burst {
			return 3000000, 1200000, nil
		}
		return 1000000, 400000, nil
	}
	return 0, 0, fmt.Errorf("mode must be pilot, bootstrap, or routine")
}

func dreamPlan(ops []knowledgeOp, q DreamRequest, now int64) (DreamPlan, error) {
	p := DreamPlan{Version: 1, Mode: q.Mode, Items: []DreamItem{}, Excluded: map[string]int{}}
	input, gen, err := dreamLimits(q.Mode, q.AllowBurst)
	if err != nil {
		return p, err
	}
	if q.InputSpent < 0 || q.GenerationSpent < 0 {
		return p, fmt.Errorf("spent tokens must be nonnegative")
	}
	if q.Limit == 0 {
		q.Limit = 20
	}
	if q.Limit < 1 || q.Limit > 30 || len(q.IDs) > 30 {
		return p, fmt.Errorf("a packet holds 1–30 entries")
	}
	requested := map[string]bool{}
	if len(q.ReviewedRevisions) > 10000 {
		return p, fmt.Errorf("review progress is too large")
	}
	for id, rev := range q.ReviewedRevisions {
		if len(id) > 300 || !validDigest(rev) {
			return p, fmt.Errorf("review progress needs IDs and revision digests")
		}
	}
	for _, id := range q.IDs {
		if len(id) > 300 {
			return p, fmt.Errorf("requested id exceeds 300 bytes")
		}
		if requested[id] {
			return p, fmt.Errorf("duplicate requested id")
		}
		requested[id] = true
	}
	type candidate struct {
		e        knowledgeOp
		reasons  []string
		priority int
	}
	var list []candidate
	for _, e := range foldKnowledge(ops) {
		if len(requested) > 0 && !requested[e.ID] {
			continue
		}
		if e.Sensitive || e.Topic == "accounts" {
			p.Excluded["private"]++
			continue
		}
		if len(requested) == 0 && q.ReviewedRevisions[e.ID] == entryRevision(e) {
			p.Excluded["already_reviewed_revision"]++
			continue
		}
		reasons := []string{}
		priority := 0
		if e.Memory == nil {
			reasons = append(reasons, "missing action/scope/evidence card")
			priority += 2
		}
		if e.Memory != nil && (e.Memory.ReviewAfter > 0 && e.Memory.ReviewAfter <= now || e.Memory.Confidence == "tentative" || e.Memory.Confidence == "contested") {
			reasons = append(reasons, "review due or disputed")
			priority += 8
		}
		if e.Status == StatusHypothesis {
			reasons = append(reasons, "hypothesis needs evidence, not automatic confirmation")
			priority += 3
		}
		if protectedEntry(e) {
			reasons = append(reasons, "protected: verify applicability; do not retire by popularity")
			priority += 4
		}
		if len(requested) > 0 {
			reasons = append(reasons, "explicit review selection")
		}
		if len(reasons) == 0 {
			p.Excluded["no_review_signal"]++
			continue
		}
		list = append(list, candidate{e, reasons, priority})
	}
	if len(requested) > 0 && len(list)+p.Excluded["private"] != len(requested) {
		return p, fmt.Errorf("requested id is no longer live; refresh the selection")
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].priority != list[j].priority {
			return list[i].priority > list[j].priority
		}
		return list[i].e.ID < list[j].e.ID
	})
	// Reserve one fifth for lower-priority exploration, never just high-hit rules.
	picked := list
	if len(list) > q.Limit {
		n := q.Limit - q.Limit/5
		picked = append([]candidate{}, list[:n]...)
		picked = append(picked, list[len(list)-(q.Limit-n):]...)
	}
	for _, c := range picked {
		e := c.e
		refs := append([]string(nil), e.ReviewEvidence...)
		if e.Memory != nil {
			refs = append(refs, e.Memory.Evidence...)
		}
		p.Items = append(p.Items, DreamItem{ID: e.ID, Revision: entryRevision(e), Title: e.Title, Body: e.Body, Reasons: c.reasons, Protected: protectedEntry(e), Evidence: refs, Audience: e.Audience, AudienceRepo: e.AudienceRepo, Memory: e.Memory})
	}
	p.Instructions = []string{"Use only authorized KB evidence; missing evidence remains unknown. No native-session scan.", "For each selected ID: keep, revise with actionable trigger/scope/evidence, defer pending evidence, or propose reversible retirement with a concrete reason. Low hits never prove uselessness.", "Submit a version-1 ReviewProposal with origin dreaming and these base_revision values. Use knowledge review check then the existing HQ/owner apply path. No direct file rewriting.", "Remember completed packet IDs externally in the project review record. Repeating an unchanged packet without new evidence is not another learning pass.", "Read neighboring entries only for a specific conflict; preserve scenario differences and source references. Do not convert a repeated assertion into evidence.", "Record provider input and billed generation (including reasoning), or mark usage unknown. This plan is advisory: the existing executor must reserve/enforce the cap, reconcile actual usage, and defer when review backlog is high."}
	data, _ := json.Marshal(p.Items)
	estimate := conservativeTokens(string(data)) + 2000
	remainingInput, remainingGen := max(int64(0), input-q.InputSpent), max(int64(0), gen-q.GenerationSpent)
	suggested := min(int64(len(p.Items))*6000+8000, remainingGen)
	p.Budget = DreamBudget{InputLimit: input, GenerationLimit: gen, InputRemaining: remainingInput, GenerationRemaining: remainingGen, UsageKnown: q.UsageKnown, CanStart: q.UsageKnown && len(p.Items) > 0 && remainingInput >= int64(estimate) && remainingGen >= int64(len(p.Items))*6000+8000, EstimatedPacketTokens: estimate, SuggestedGeneration: suggested, Note: "Pilot/bootstrap are whole-run caps; routine is a week. Generation includes billed reasoning. Supplied usage is reported, not billing verification. This command never reserves or spends tokens."}
	// Identity includes content and instructions, not mutable spend counters.
	identity, _ := json.Marshal(struct {
		Mode         string
		Items        []DreamItem
		Instructions []string
	}{p.Mode, p.Items, p.Instructions})
	p.ID = fmt.Sprintf("%x", sha256.Sum256(identity))
	return p, nil
}
func PlanDream(q DreamRequest) (DreamPlan, error) {
	ops, err := readKnowledgeOps()
	if err != nil {
		return DreamPlan{}, err
	}
	q.Mode = strings.TrimSpace(q.Mode)
	return dreamPlan(ops, q, time.Now().Unix())
}
