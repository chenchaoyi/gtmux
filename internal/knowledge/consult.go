package knowledge

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
)

type ConsultRequest struct {
	Query         string            `json:"query"`
	Action        string            `json:"action,omitempty"`
	Project       string            `json:"project,omitempty"`
	Role          string            `json:"role,omitempty"`
	Tools         map[string]string `json:"tools,omitempty"`
	TokenBudget   int               `json:"token_budget,omitempty"`
	Purpose       string            `json:"purpose,omitempty"`        // task, audit, evaluation; always read-only
	IncludeLegacy bool              `json:"include_legacy,omitempty"` // explicit local HQ investigation
}

type ConsultItem struct {
	ID              string   `json:"id"`
	Revision        string   `json:"revision"`
	Title           string   `json:"title"`
	Card            string   `json:"card"`
	ContentHash     string   `json:"content_hash"`
	Why             []string `json:"why_selected"`
	NeedsVerify     []string `json:"needs_verify,omitempty"`
	EstimatedTokens int      `json:"estimated_tokens"`
}

type ConsultResult struct {
	PlanID          string         `json:"plan_id"`
	Snapshot        string         `json:"snapshot"`
	Items           []ConsultItem  `json:"items"`
	Excluded        map[string]int `json:"excluded"`
	MissingRequired []string       `json:"missing_required,omitempty"`
	EstimatedTokens int            `json:"estimated_tokens"`
	TokenEstimate   string         `json:"token_estimate"`
}

// conservativeTokens is deliberately an estimate, never a claimed tokenizer.
// Whole cards are selected or omitted; exceptions are not truncated to fit.
func conservativeTokens(s string) int {
	han, other := 0, 0
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			han++
		} else {
			other++
		}
	}
	return han*2 + (other+2)/3
}

var actionAliases = map[string]string{
	"copy":          "copy replace overwrite 覆盖 替换 复制 内容 核验",
	"release":       "release publish 发布 访问 权限 限制",
	"dispatch":      "dispatch spawn goal 派发 目标 经验",
	"authorization": "authorization approval 授权 批准 同意 重复 请批",
	"send":          "send composer draft 发送 输入 草稿",
	"test":          "test isolation 测试 隔离 临时 用户 数据",
	"diagnose":      "diagnose network proxy 诊断 网络 代理 环境",
}

func consultText(q ConsultRequest) string {
	s := q.Query + " " + actionAliases[q.Action]
	for _, p := range [][2]string{{"replacement", "覆盖 替换"}, {"overwrite", "覆盖"}, {"copy", "复制"}, {"已同意", "授权 批准"}, {"重复请批", "授权 批准"}, {"编写派发目标", "goal 派发 经验"}} {
		if strings.Contains(strings.ToLower(q.Query), p[0]) {
			s += " " + p[1]
		}
	}
	return s
}

func consultEligible(e knowledgeOp, q ConsultRequest, now int64) (string, []string) {
	if e.Sensitive || e.Topic == "accounts" {
		return "private", nil
	}
	if e.Status == StatusHypothesis {
		return "hypothesis", nil
	}
	m := e.Memory
	if m == nil {
		switch e.Audience {
		case AudienceMachine, AudienceEveryone:
			return "", []string{"legacy rule: verify current applicability"}
		case AudienceRepo:
			if q.Project != "" && cleanRepo(q.Project) == cleanRepo(e.AudienceRepo) {
				return "", []string{"legacy rule: verify current applicability"}
			}
		case AudienceHQ:
			if q.Role == "hq" {
				return "", []string{"legacy rule: verify current applicability"}
			}
		default:
			if q.Role == "hq" && q.IncludeLegacy {
				return "", []string{"scope unknown; investigation only"}
			}
		}
		return "scope_unknown_or_mismatch", nil
	}
	if m.Scope == "project" && (q.Project == "" || cleanRepo(q.Project) != m.Project) {
		return "project_mismatch", nil
	}
	if m.Scope == "hq" && q.Role != "hq" {
		return "role_mismatch", nil
	}
	if !contains([]string{"local", "project", "hq", "global"}, m.Scope) {
		return "scope_unknown_or_mismatch", nil
	}
	for tool, version := range m.ToolVersions {
		if q.Tools[tool] != version {
			return "tool_version_mismatch", nil
		}
	}
	if m.Confidence != "supported" && m.Confidence != "reported" {
		return "needs_review", nil
	}
	if m.ReviewAfter > 0 && now >= m.ReviewAfter {
		return "review_due", nil
	}
	if m.Confidence == "reported" {
		return "", []string{"reported evidence; verify before relying on it"}
	}
	return "", nil
}

func memoryCard(e knowledgeOp) string {
	if e.Memory == nil {
		return e.Title + "\n" + e.Body
	}
	m := e.Memory
	s := e.Title + "\n" + m.Summary
	for _, exception := range m.Exceptions {
		s += "\nException: " + exception
	}
	return s
}

func consultEntries(ops []knowledgeOp, q ConsultRequest, now int64) (ConsultResult, error) {
	r := ConsultResult{Items: []ConsultItem{}, Excluded: map[string]int{}, TokenEstimate: "Han characters ×2 + other characters /3; estimate, not measured tokens"}
	if q.Purpose == "" {
		q.Purpose = "task"
	}
	if q.Role == "" {
		q.Role = "worker"
	}
	if !contains([]string{"worker", "hq"}, q.Role) {
		return r, fmt.Errorf("role must be worker or hq")
	}
	if !contains([]string{"task", "audit", "evaluation"}, q.Purpose) {
		return r, fmt.Errorf("purpose must be task, audit, or evaluation")
	}
	if q.TokenBudget == 0 {
		q.TokenBudget = 1200
	}
	if q.TokenBudget < 1 || q.TokenBudget > 2000 {
		return r, fmt.Errorf("token budget must be 1–2000")
	}
	if len(q.Query) > 8000 || len(q.Action) > 100 || len(q.Tools) > 16 {
		return r, fmt.Errorf("consult request is too large")
	}
	if q.IncludeLegacy && q.Role != "hq" {
		return r, fmt.Errorf("legacy investigation requires the explicit hq role")
	}
	b, _ := json.Marshal(ops)
	r.Snapshot = fmt.Sprintf("%x", sha256.Sum256(b))
	live := foldKnowledge(ops)
	var eligible []knowledgeOp
	needs := map[string][]string{}
	for _, e := range live {
		why, verify := consultEligible(e, q, now)
		if why != "" {
			r.Excluded[why]++
			if (why == "review_due" || why == "needs_review") && e.Memory != nil && q.Action != "" && contains(e.Memory.Actions, q.Action) && protectedEntry(e) {
				r.MissingRequired = append(r.MissingRequired, e.ID)
			}
			continue
		}
		eligible = append(eligible, e)
		needs[e.ID] = verify
	}
	// Search the reviewed action card as well as its source prose, without
	// changing legacy search ranking or mutating the stored entry.
	searchable := append([]knowledgeOp(nil), eligible...)
	for i := range searchable {
		if searchable[i].Memory != nil {
			searchable[i].Body += "\n" + memoryCard(searchable[i]) + " " + strings.Join(searchable[i].Memory.Actions, " ")
		}
	}
	c := newCorpus(searchable)
	probe := tokens(consultText(q))
	type ranked struct {
		entry    knowledgeOp
		score    float64
		why      []string
		required bool
	}
	var ranks []ranked
	for i, e := range eligible {
		score := c.score(probe, i)
		reasons := []string{"text relevance"}
		actionMatch := e.Memory != nil && q.Action != "" && contains(e.Memory.Actions, q.Action)
		if actionMatch {
			score += 1
			reasons = append(reasons, "action:"+q.Action)
		}
		if !actionMatch && (shared(probe, c.tok[i]) < 2 || score < 0.18) {
			continue
		}
		score += 0.2 * overlap(probe, tokens(e.Title))
		required := protectedEntry(e) && (actionMatch || e.Memory == nil)
		if required {
			score += 2
			reasons = append(reasons, "protected rule relevant to this query/action")
		}
		ranks = append(ranks, ranked{e, score, reasons, required})
	}
	sort.Slice(ranks, func(i, j int) bool {
		if ranks[i].score != ranks[j].score {
			return ranks[i].score > ranks[j].score
		}
		return ranks[i].entry.ID < ranks[j].entry.ID
	})
	for _, rank := range ranks {
		e := rank.entry
		card := memoryCard(e)
		cost := conservativeTokens(card)
		if r.EstimatedTokens+cost > q.TokenBudget || len(r.Items) >= 6 {
			r.Excluded["budget"]++
			if rank.required {
				r.MissingRequired = append(r.MissingRequired, e.ID)
			}
			continue
		}
		r.Items = append(r.Items, ConsultItem{ID: e.ID, Revision: entryRevision(e), Title: e.Title, Card: card, ContentHash: fmt.Sprintf("%x", sha256.Sum256([]byte(card))), Why: rank.why, NeedsVerify: needs[e.ID], EstimatedTokens: cost})
		r.EstimatedTokens += cost
	}
	b, _ = json.Marshal(struct {
		Request ConsultRequest
		Result  ConsultResult
	}{q, r})
	r.PlanID = fmt.Sprintf("%x", sha256.Sum256(b))
	return r, nil
}

// Consult is read-only for every purpose; recording a separate usage receipt is
// explicit and cannot turn a query into a hit or a successful outcome.
func Consult(q ConsultRequest) (ConsultResult, error) {
	ops, err := readKnowledgeOps()
	if err != nil {
		return ConsultResult{}, err
	}
	return consultEntries(ops, q, time.Now().Unix())
}
