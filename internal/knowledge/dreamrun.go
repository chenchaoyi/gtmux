package knowledge

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// Dreaming owns one durable journal, not another KB or scheduler. Each item is
// a recovery boundary. A process lock serializes runners; no model call holds
// the knowledge lock. A completed result and its cost settle in one publication.
const dreamJournalLimit = 16 << 20

type DreamChoice struct {
	Agent          string `json:"agent"`
	Model          string `json:"model"`
	Effort         string `json:"effort,omitempty"`
	Reason         string `json:"reason"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type DreamStart struct {
	Selection  DreamRequest `json:"selection"`
	MaxCalls   int          `json:"max_calls"`
	MaxSeconds int64        `json:"max_seconds"`
}

// Absolute limits make an identical extension retry idempotent. Extension is
// an explicit owner action; it never launches a model or erases prior charges.
type DreamAllowance struct {
	MaxCalls   int    `json:"max_calls"`
	MaxSeconds int64  `json:"max_seconds"`
	Reason     string `json:"reason"`
}

func validDreamAllowance(q DreamAllowance) bool {
	return q.MaxCalls >= 1 && q.MaxCalls <= 60 && q.MaxSeconds >= 1 && q.MaxSeconds <= 14400 && strings.TrimSpace(q.Reason) != "" && len(q.Reason) <= 8000
}

type DreamDecision struct {
	ID           string        `json:"id"`
	BaseRevision string        `json:"base_revision"`
	Disposition  string        `json:"disposition"` // keep, defer, revise, retire
	Note         string        `json:"note"`
	Change       *ReviewChange `json:"change,omitempty"`
}

// Unknown usage is not zero. CLI execution has hard call/time bounds; reported
// token totals are additional stop signals, never a promised billing hard cap.
type DreamReply struct {
	Text             string `json:"text,omitempty"`
	UsageKnown       bool   `json:"usage_known"`
	InputTokens      int64  `json:"input_tokens,omitempty"`
	GenerationTokens int64  `json:"generation_tokens,omitempty"`
	UsageSource      string `json:"usage_source,omitempty"`
	FailureCode      string `json:"failure_code,omitempty"`
	ExitCode         *int   `json:"exit_code,omitempty"`
}

// Executor diagnostics are evidence, never arbitrary provider text. Keep this
// vocabulary closed so secrets/session URLs cannot enter the journal via errors.
func dreamFailureCode(code string) string {
	switch code {
	case "launch", "cancelled", "invalid_config", "authentication", "model_unavailable", "rate_limit", "network", "permission", "unexpected_action", "output_limit", "provider", "incomplete", "exit":
		return code
	default:
		return "executor"
	}
}

type DreamCall struct {
	RunID   string      `json:"run_id"`
	Attempt int         `json:"attempt"`
	Item    DreamItem   `json:"item"`
	Choice  DreamChoice `json:"choice"`
}

type DreamEvent struct {
	Version   int             `json:"version"`
	Seq       int             `json:"seq"`
	At        int64           `json:"at"`
	RunID     string          `json:"run_id"`
	Kind      string          `json:"kind"`
	Previous  string          `json:"previous,omitempty"`
	Hash      string          `json:"hash"`
	Plan      *DreamPlan      `json:"plan,omitempty"`
	Limits    *DreamStart     `json:"limits,omitempty"`
	Allowance *DreamAllowance `json:"allowance,omitempty"`
	Call      *DreamCall      `json:"call,omitempty"`
	Attempt   int             `json:"attempt,omitempty"`
	ItemID    string          `json:"item_id,omitempty"`
	Decision  *DreamDecision  `json:"decision,omitempty"`
	Reply     *DreamReply     `json:"reply,omitempty"`
	Seconds   int64           `json:"seconds,omitempty"`
	Reason    string          `json:"reason,omitempty"`
}

type DreamRun struct {
	ID               string          `json:"id"`
	State            string          `json:"state"`
	Plan             DreamPlan       `json:"plan"`
	MaxCalls         int             `json:"max_calls"`
	MaxSeconds       int64           `json:"max_seconds"`
	Calls            int             `json:"calls"`
	Seconds          int64           `json:"seconds"`
	InputTokens      int64           `json:"reported_input_tokens"`
	GenerationTokens int64           `json:"reported_generation_tokens"`
	UnknownUsage     int             `json:"unknown_usage_attempts"`
	Pending          []string        `json:"pending"`
	Decisions        []DreamDecision `json:"decisions"`
	Active           *DreamCall      `json:"active,omitempty"`
	Reason           string          `json:"reason,omitempty"`
}

type DreamExecutor func(context.Context, DreamCall) (DreamReply, error)

func dreamJournalPath() string { return filepath.Join(state.Dir(), "knowledge-dream", "actions.jsonl") }

func withDreamLock(run func() error) error {
	root := filepath.Dir(dreamJournalPath())
	if err := state.MkdirAll(root, 0700); err != nil {
		return err
	}
	f, err := state.OpenFile(filepath.Join(root, ".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("another dreaming operation is active; inspect status and retry later: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return run()
}

func dreamEventHash(e DreamEvent) string {
	e.Hash = ""
	b, _ := json.Marshal(e)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

func readDreamEvents() ([]DreamEvent, error) {
	f, err := os.Open(dreamJournalPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if st.Size() > dreamJournalLimit {
		return nil, fmt.Errorf("dream journal exceeds 16 MiB; preserve the complete history; do not reset budget counters")
	}
	if st.Size() > 0 {
		var last [1]byte
		if _, err := f.ReadAt(last[:], st.Size()-1); err != nil || last[0] != '\n' {
			return nil, fmt.Errorf("incomplete dream journal; preserve and recover it before continuing")
		}
	}
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 4096), 1<<20)
	var events []DreamEvent
	previous := ""
	for s.Scan() {
		var e DreamEvent
		if err := json.Unmarshal(s.Bytes(), &e); err != nil {
			return nil, err
		}
		if e.Version != 1 || e.Seq != len(events)+1 || e.Previous != previous || e.Hash != dreamEventHash(e) || !validDigest(e.RunID) {
			return nil, fmt.Errorf("dream journal integrity failure at sequence %d", len(events)+1)
		}
		previous = e.Hash
		events = append(events, e)
	}
	return events, s.Err()
}

func appendDreamEvent(events *[]DreamEvent, e DreamEvent) error {
	e.Version, e.Seq, e.At = 1, len(*events)+1, time.Now().Unix()
	if len(*events) > 0 {
		e.Previous = (*events)[len(*events)-1].Hash
	}
	e.Hash = dreamEventHash(e)
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	var size int64
	if st, err := os.Stat(dreamJournalPath()); err == nil {
		size = st.Size()
	} else if !os.IsNotExist(err) {
		return err
	}
	// Keep enough room for a terminal receipt after accepting another call.
	margin := int64(0)
	if e.Kind == "started" {
		margin = 256 << 10
	}
	if size+int64(len(b))+1+margin > dreamJournalLimit {
		return fmt.Errorf("dream journal full; retain history and preserve the complete history; do not reset budget counters")
	}
	committed, err := atomicAppend(dreamJournalPath(), append(b, '\n'))
	if committed {
		*events = append(*events, e)
	}
	return err
}

func foldDream(events []DreamEvent, id string) (DreamRun, error) {
	r := DreamRun{ID: id, Pending: []string{}, Decisions: []DreamDecision{}}
	seen := map[string]bool{}
	items := map[string]DreamItem{}
	for _, e := range events {
		if e.RunID != id {
			continue
		}
		switch e.Kind {
		case "created":
			if r.State != "" || e.Plan == nil || e.Limits == nil || e.Plan.ID != id {
				return r, fmt.Errorf("invalid dream creation")
			}
			for _, item := range e.Plan.Items {
				if item.ID == "" || !validDigest(item.Revision) || items[item.ID].ID != "" {
					return r, fmt.Errorf("invalid dream input identity")
				}
				items[item.ID] = item
			}
			r.Plan, r.MaxCalls, r.MaxSeconds, r.State = *e.Plan, e.Limits.MaxCalls, e.Limits.MaxSeconds, "ready"
		case "allowance_extended":
			if r.State == "" || r.Active != nil || e.Allowance == nil || !validDreamAllowance(*e.Allowance) || e.Allowance.MaxCalls < r.MaxCalls || e.Allowance.MaxSeconds < r.MaxSeconds || e.Allowance.MaxCalls == r.MaxCalls && e.Allowance.MaxSeconds == r.MaxSeconds {
				return r, fmt.Errorf("invalid dream allowance extension")
			}
			r.MaxCalls, r.MaxSeconds = e.Allowance.MaxCalls, e.Allowance.MaxSeconds
		case "started":
			if r.State == "" || r.Active != nil || e.Call == nil || e.Call.Attempt != r.Calls+1 || seen[e.Call.Item.ID] || e.Call.RunID != id || items[e.Call.Item.ID].Revision != e.Call.Item.Revision {
				return r, fmt.Errorf("invalid dream attempt sequence")
			}
			r.Active, r.Calls, r.State, r.Reason = e.Call, e.Call.Attempt, "running", ""
		case "finished":
			if r.Active == nil || r.Active.Attempt != e.Attempt || e.Seconds < 0 {
				return r, fmt.Errorf("dream completion without its attempt")
			}
			r.Seconds += e.Seconds
			if e.Reply != nil && e.Reply.UsageKnown {
				if e.Reply.InputTokens < 0 || e.Reply.GenerationTokens < 0 {
					return r, fmt.Errorf("negative recorded usage")
				}
				r.InputTokens += e.Reply.InputTokens
				r.GenerationTokens += e.Reply.GenerationTokens
			} else {
				r.UnknownUsage++
			}
			if e.Decision != nil && (e.Decision.ID != r.Active.Item.ID || e.Decision.BaseRevision != r.Active.Item.Revision) {
				return r, fmt.Errorf("dream completion identity mismatch")
			}
			r.Active, r.State, r.Reason = nil, "paused", e.Reason
			if e.Decision != nil {
				if seen[e.Decision.ID] {
					return r, fmt.Errorf("duplicate completed dream item")
				}
				r.Decisions = append(r.Decisions, *e.Decision)
				seen[e.Decision.ID], r.State = true, "ready"
			}
		case "deferred":
			if seen[e.ItemID] || r.State == "" || r.Active != nil || items[e.ItemID].ID == "" {
				return r, fmt.Errorf("invalid deferred dream item")
			}
			seen[e.ItemID] = true
			r.Decisions = append(r.Decisions, DreamDecision{ID: e.ItemID, BaseRevision: items[e.ItemID].Revision, Disposition: "defer", Note: e.Reason})
		case "paused":
			if r.State == "" || r.Active != nil {
				return r, fmt.Errorf("invalid dream pause")
			}
			r.State, r.Reason = "paused", e.Reason
		default:
			return r, fmt.Errorf("unknown dream event %q", e.Kind)
		}
	}
	if r.State == "" {
		return r, fmt.Errorf("dream run not found")
	}
	for _, item := range r.Plan.Items {
		if !seen[item.ID] {
			r.Pending = append(r.Pending, item.ID)
		}
	}
	if len(r.Pending) == 0 && r.Active == nil {
		r.State = "review_ready"
	}
	return r, nil
}

// StartDream freezes only eligible KB entries. Repeated creation of the same
// packet returns its existing history instead of resetting its budget/progress.
func StartDream(q DreamStart) (DreamRun, error) {
	var result DreamRun
	err := withDreamLock(func() error {
		events, err := readDreamEvents()
		if err != nil {
			return err
		}
		if q.MaxCalls == 0 {
			q.MaxCalls = 40
		}
		if q.MaxSeconds == 0 {
			q.MaxSeconds = 7200
		}
		if q.MaxCalls < 1 || q.MaxCalls > 60 || q.MaxSeconds < 1 || q.MaxSeconds > 14400 {
			return fmt.Errorf("dream limits must be 1–60 calls and 1–14400 seconds")
		}
		if q.Selection.AllowBurst {
			return fmt.Errorf("CLI runs do not support token burst overrides; choose a bounded campaign mode")
		}
		// Internal progress is authoritative; a request cannot erase completed work.
		if q.Selection.ReviewedRevisions == nil {
			q.Selection.ReviewedRevisions = map[string]string{}
		}
		for _, e := range events {
			if e.Decision != nil && (e.Decision.Disposition == "keep" || e.Decision.Disposition == "revise" || e.Decision.Disposition == "retire") {
				q.Selection.ReviewedRevisions[e.Decision.ID] = e.Decision.BaseRevision
			}
		}
		plan, err := PlanDream(q.Selection)
		if err != nil {
			return err
		}
		if len(plan.Items) == 0 || len(plan.Items) > 25 {
			return fmt.Errorf("a resumable run needs 1–25 eligible entries")
		}
		for _, e := range events {
			if e.RunID == plan.ID {
				result, err = foldDream(events, plan.ID)
				return err
			}
		}
		// The journal owns counters; caller-supplied token totals are not reservations.
		q.Selection.InputSpent, q.Selection.GenerationSpent, q.Selection.UsageKnown = 0, 0, false
		if err := appendDreamEvent(&events, DreamEvent{RunID: plan.ID, Kind: "created", Plan: &plan, Limits: &q}); err != nil {
			return err
		}
		result, err = foldDream(events, plan.ID)
		return err
	})
	return result, err
}

func DreamStatus(id string) (DreamRun, error) {
	if !validDigest(id) {
		return DreamRun{}, fmt.Errorf("run id must be SHA-256")
	}
	events, err := readDreamEvents()
	if err != nil {
		return DreamRun{}, err
	}
	return foldDream(events, id)
}

func ExtendDream(id string, q DreamAllowance) (DreamRun, error) {
	var result DreamRun
	if !validDigest(id) || !validDreamAllowance(q) {
		return result, fmt.Errorf("extension requires a run ID, 1–60 total calls, 1–14400 total seconds and a 1–8000 byte reason")
	}
	err := withDreamLock(func() error {
		events, err := readDreamEvents()
		if err != nil {
			return err
		}
		r, err := foldDream(events, id)
		if err != nil {
			return err
		}
		result = r
		if r.Active != nil {
			return fmt.Errorf("recover the interrupted attempt before extending its allowance")
		}
		if q.MaxCalls < r.MaxCalls || q.MaxSeconds < r.MaxSeconds {
			return fmt.Errorf("extension cannot reduce existing limits")
		}
		if q.MaxCalls == r.MaxCalls && q.MaxSeconds == r.MaxSeconds {
			return nil
		}
		if len(r.Pending) == 0 {
			return fmt.Errorf("completed run needs no allowance extension")
		}
		if err = appendDreamEvent(&events, DreamEvent{RunID: id, Kind: "allowance_extended", Allowance: &q}); err != nil {
			return err
		}
		result, err = foldDream(events, id)
		return err
	})
	return result, err
}

func DreamHistory(id string) ([]DreamEvent, error) {
	if !validDigest(id) {
		return nil, fmt.Errorf("run id must be SHA-256")
	}
	events, err := readDreamEvents()
	if err != nil {
		return nil, err
	}
	var result []DreamEvent
	for _, e := range events {
		if e.RunID == id {
			result = append(result, e)
		}
	}
	return result, nil
}

func DreamRuns() ([]DreamRun, error) {
	events, err := readDreamEvents()
	if err != nil {
		return nil, err
	}
	result := []DreamRun{}
	for _, e := range events {
		if e.Kind != "created" {
			continue
		}
		r, err := foldDream(events, e.RunID)
		if err != nil {
			return nil, err
		}
		// List is a compact index; status/history provide the frozen evidence.
		r.Plan.Items = nil
		r.Decisions = nil
		result = append(result, r)
	}
	return result, nil
}

func ValidateDreamChoice(c DreamChoice) error {
	if !contains([]string{"codex", "claude"}, c.Agent) || strings.TrimSpace(c.Model) == "" || len(c.Model) > 160 || strings.HasPrefix(c.Model, "-") || strings.ContainsAny(c.Model, "\r\n\x00") {
		return fmt.Errorf("HQ must choose a codex/claude model explicitly")
	}
	if strings.TrimSpace(c.Reason) == "" || len(c.Reason) > 2000 {
		return fmt.Errorf("record HQ's model-selection reason (1–2000 bytes)")
	}
	if c.Effort != "" && !contains([]string{"low", "medium", "high", "xhigh", "max", "ultra"}, c.Effort) {
		return fmt.Errorf("unknown effort")
	}
	if c.TimeoutSeconds < 1 || c.TimeoutSeconds > 1800 {
		return fmt.Errorf("each call needs a 1–1800 second timeout")
	}
	return nil
}

func parseDreamDecision(text string, item DreamItem, ops []knowledgeOp) (DreamDecision, error) {
	var d DreamDecision
	if len(text) > 64<<10 {
		return d, fmt.Errorf("model result exceeds 64 KiB")
	}
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&d); err != nil {
		return d, fmt.Errorf("model result is not a decision JSON object")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return d, fmt.Errorf("model returned multiple JSON values")
	}
	if d.ID != item.ID || d.BaseRevision != item.Revision || strings.TrimSpace(d.Note) == "" || len(d.Note) > 8000 {
		return d, fmt.Errorf("model decision identity/revision/note mismatch")
	}
	if !contains([]string{"keep", "defer", "revise", "retire"}, d.Disposition) {
		return d, fmt.Errorf("unknown decision")
	}
	if d.Disposition == "keep" || d.Disposition == "defer" {
		if d.Change != nil {
			return d, fmt.Errorf("keep/defer must not include a mutation")
		}
		return d, nil
	}
	if d.Change == nil || d.Change.ID != item.ID || d.Change.BaseRevision != item.Revision || d.Change.Operation != d.Disposition {
		return d, fmt.Errorf("change does not match the reviewed item")
	}
	// No tool/source access occurs in this executor. Its prose cannot establish
	// newly verified facts; HQ must review proposals before applying them.
	if d.Change.Memory != nil && (d.Change.Memory.Confidence == "supported" || d.Change.Memory.VerifiedAt != 0) {
		return d, fmt.Errorf("model-only review cannot claim new verification")
	}
	allowed := map[string]bool{"kb:" + item.ID + "@" + item.Revision: true}
	for _, ref := range item.Evidence {
		allowed[ref] = true
	}
	refs := append([]string{}, d.Change.Evidence...)
	if d.Change.Memory != nil {
		refs = append(refs, d.Change.Memory.Evidence...)
	}
	for _, ref := range refs {
		if !allowed[ref] {
			return d, fmt.Errorf("model cited evidence outside the authorized packet")
		}
	}
	p := ReviewProposal{Version: 1, Origin: "dreaming", Changes: []ReviewChange{*d.Change}}
	p.ID = proposalDigest(p)
	_, _, err := prepareReview(p, ops, time.Now().Unix())
	return d, err
}

// ResumeDream executes only unfinished items. The caller (HQ) chooses the model
// for this continuation. There is no model-selected escalation or auto-apply.
func ResumeDream(ctx context.Context, id string, choice DreamChoice, execute DreamExecutor) (DreamRun, error) {
	var result DreamRun
	if err := ValidateDreamChoice(choice); err != nil {
		return result, err
	}
	if execute == nil || !validDigest(id) {
		return result, fmt.Errorf("executor and valid run id required")
	}
	err := withDreamLock(func() error {
		events, err := readDreamEvents()
		if err != nil {
			return err
		}
		r, err := foldDream(events, id)
		if err != nil {
			return err
		}
		if r.Active != nil {
			// The prior controller no longer owns the lock. Its remote computation
			// may still have run: preserve uncertainty and require another explicit
			// HQ continuation before retrying this item, never silently relaunch it.
			if err := appendDreamEvent(&events, DreamEvent{RunID: id, Kind: "finished", Attempt: r.Active.Attempt, Seconds: int64(r.Active.Choice.TimeoutSeconds), Reason: "interrupted before a durable result; usage unknown"}); err != nil {
				return err
			}
			result, err = foldDream(events, id)
			if err != nil {
				return err
			}
			return fmt.Errorf("interrupted attempt archived; HQ must inspect history before another run")
		}
		for {
			r, err = foldDream(events, id)
			if err != nil {
				return err
			}
			result = r
			if len(r.Pending) == 0 {
				return nil
			}
			pause := func(reason string) error {
				if err := appendDreamEvent(&events, DreamEvent{RunID: id, Kind: "paused", Reason: reason}); err != nil {
					return err
				}
				result, err = foldDream(events, id)
				if err != nil {
					return err
				}
				return fmt.Errorf("dream paused: %s", reason)
			}
			if ctx.Err() != nil {
				return pause("cancelled; completed items retained")
			}
			if r.Calls >= r.MaxCalls || r.Seconds >= r.MaxSeconds {
				return pause("run call/time allowance exhausted")
			}
			// One journal means starting another run cannot reset this week's
			// operational ceiling. Unknown CLI tokens stay visibly unknown.
			week := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -int((time.Now().UTC().Weekday()+6)%7)).Unix()
			weeklyCalls := 0
			var weeklySeconds, input, generation int64
			modes := map[string]string{}
			for _, e := range events {
				if e.Kind == "created" && e.Plan != nil {
					modes[e.RunID] = e.Plan.Mode
				}
			}
			for _, e := range events {
				if e.At >= week {
					if e.Kind == "started" {
						weeklyCalls++
					}
					if e.Kind == "finished" {
						weeklySeconds += e.Seconds
					}
				}
				// Pilot/bootstrap are lifetime campaign thresholds shared by their
				// packets; routine counts all model activity in the current week.
				inBudget := r.Plan.Mode == "routine" && e.At >= week || r.Plan.Mode != "routine" && modes[e.RunID] == r.Plan.Mode
				if inBudget && e.Kind == "finished" && e.Reply != nil && e.Reply.UsageKnown {
					input += e.Reply.InputTokens
					generation += e.Reply.GenerationTokens
				}
			}
			// An abandoned run also retains its time reservation when a different
			// run is requested. Do not hide unresolved work behind a new run ID.
			for _, e := range events {
				if e.Kind != "created" || e.RunID == id {
					continue
				}
				other, foldErr := foldDream(events, e.RunID)
				if foldErr != nil {
					return foldErr
				}
				if other.Active != nil {
					return pause("another run has an interrupted attempt; inspect and recover it first")
				}
			}
			if weeklyCalls >= 200 || weeklySeconds >= 12*3600 {
				return pause("shared weekly CLI call/time ceiling reached")
			}
			inputLimit, generationLimit, _ := dreamLimits(r.Plan.Mode, false)
			if input >= inputLimit || generation >= generationLimit {
				return pause("reported token stop threshold reached; CLI cannot enforce an in-flight token cap")
			}
			var item DreamItem
			for _, candidate := range r.Plan.Items {
				if candidate.ID == r.Pending[0] {
					item = candidate
					break
				}
			}
			attempts := 0
			for _, e := range events {
				if e.RunID == id && e.Call != nil && e.Call.Item.ID == item.ID {
					attempts++
				}
			}
			if attempts >= 3 {
				return pause("item failed three attempts; inspect history and resolve its evidence/protocol")
			}
			ops, err := readKnowledgeOps()
			if err != nil {
				return err
			}
			live, ok := findLive(foldKnowledge(ops), item.ID)
			if !ok || entryRevision(live) != item.Revision || live.Sensitive || live.Topic == "accounts" {
				if err := appendDreamEvent(&events, DreamEvent{RunID: id, Kind: "deferred", ItemID: item.ID, Reason: "source changed, retired or became private; reselect its authorized current version"}); err != nil {
					return err
				}
				continue
			}
			callChoice := choice
			callChoice.TimeoutSeconds = int(min(int64(choice.TimeoutSeconds), r.MaxSeconds-r.Seconds, 12*3600-weeklySeconds))
			call := DreamCall{RunID: id, Attempt: r.Calls + 1, Item: item, Choice: callChoice}
			if err := appendDreamEvent(&events, DreamEvent{RunID: id, Kind: "started", Call: &call}); err != nil {
				return err
			}
			start := time.Now()
			callCtx, cancel := context.WithTimeout(ctx, time.Duration(callChoice.TimeoutSeconds)*time.Second)
			reply, callErr := execute(callCtx, call)
			cancel()
			elapsed := max(int64(1), int64(time.Since(start).Seconds()+0.999))
			finish := DreamEvent{RunID: id, Kind: "finished", Attempt: call.Attempt, Reply: &reply, Seconds: elapsed}
			if len(reply.Text) > 64<<10 {
				reply.Text = reply.Text[:64<<10]
				callErr = fmt.Errorf("output limit exceeded")
			}
			if reply.InputTokens < 0 || reply.GenerationTokens < 0 {
				reply.UsageKnown = false
				reply.InputTokens, reply.GenerationTokens = 0, 0
				callErr = fmt.Errorf("invalid usage receipt")
			}
			if callErr != nil {
				reply.FailureCode = dreamFailureCode(reply.FailureCode)
				finish.Reason = "executor failed: " + reply.FailureCode
				if reply.ExitCode != nil {
					finish.Reason += fmt.Sprintf(" (exit %d)", *reply.ExitCode)
				}
			} else {
				reply.FailureCode, reply.ExitCode = "", nil
				// A concurrent KB edit invalidates this proposal, not its cost record.
				current, readErr := readKnowledgeOps()
				if readErr != nil {
					callErr = readErr
				} else {
					nowEntry, exists := findLive(foldKnowledge(current), item.ID)
					if !exists || entryRevision(nowEntry) != item.Revision || nowEntry.Sensitive {
						callErr = fmt.Errorf("source changed during review")
					} else {
						d, parseErr := parseDreamDecision(reply.Text, item, current)
						callErr = parseErr
						if parseErr == nil {
							finish.Decision = &d
						}
					}
				}
				if callErr != nil {
					finish.Reason = "result failed identity, revision or proposal validation"
				}
			}
			if err := appendDreamEvent(&events, finish); err != nil {
				return err
			}
			result, err = foldDream(events, id)
			if err != nil {
				return err
			}
			if callErr != nil {
				return fmt.Errorf("dream attempt %d paused: %s", call.Attempt, finish.Reason)
			}
		}
	})
	return result, err
}

// DeferDream lets HQ move past an unresolved item without claiming it was
// reviewed. It never clears attempts, cost or the evidence snapshot.
func DeferDream(id, itemID, reason string) (DreamRun, error) {
	var result DreamRun
	if !validDigest(id) || strings.TrimSpace(reason) == "" || len(reason) > 8000 {
		return result, fmt.Errorf("valid run and a 1–8000 byte defer reason required")
	}
	err := withDreamLock(func() error {
		events, err := readDreamEvents()
		if err != nil {
			return err
		}
		r, err := foldDream(events, id)
		if err != nil {
			return err
		}
		if r.Active != nil {
			return fmt.Errorf("recover the interrupted attempt before deferring it")
		}
		if !contains(r.Pending, itemID) {
			return fmt.Errorf("item is not pending in this run")
		}
		if err := appendDreamEvent(&events, DreamEvent{RunID: id, Kind: "deferred", ItemID: itemID, Reason: reason}); err != nil {
			return err
		}
		result, err = foldDream(events, id)
		return err
	})
	return result, err
}

func DreamProposal(id string) (ReviewProposal, error) {
	r, err := DreamStatus(id)
	if err != nil {
		return ReviewProposal{}, err
	}
	p := ReviewProposal{Version: 1, Origin: "dreaming", Changes: []ReviewChange{}}
	for _, d := range r.Decisions {
		if d.Change != nil {
			p.Changes = append(p.Changes, *d.Change)
		}
	}
	p.ID = proposalDigest(p)
	return p, nil
}
