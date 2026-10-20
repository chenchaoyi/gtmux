package hqworkspace

import (
	"sort"
	"strings"
)

// Overview is a bounded read projection of explicit workspace records, not an
// inference about goals, progress or task completion from a transcript.
type Overview struct {
	GeneratedAt int64            `json:"generated_at"`
	Since       int64            `json:"since"`
	NeedsInput  []OverviewItem   `json:"needs_input"`
	Following   []OverviewItem   `json:"following"`
	Recent      []OverviewChange `json:"recent"`
	RecentTotal int              `json:"recent_total"`
}

type OverviewItem struct {
	MatterID string           `json:"matter_id"`
	Preview  *OverviewPreview `json:"preview,omitempty"`
}

type OverviewPreview struct {
	MessageID string `json:"message_id,omitempty"`
	Kind      string `json:"kind"`
	Body      string `json:"body"`
	At        int64  `json:"at,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

type OverviewChange struct {
	Seq      uint64 `json:"seq"`
	At       int64  `json:"at"`
	MatterID string `json:"matter_id"`
	Kind     string `json:"kind"`
	State    string `json:"state,omitempty"`
}

func BuildOverview(d Database, now int64) Overview {
	out := Overview{GeneratedAt: now, Since: now - 86400, NeedsInput: []OverviewItem{}, Following: []OverviewItem{}, Recent: []OverviewChange{}}
	wanted := map[string]bool{}
	for _, m := range Matters(d) {
		item := OverviewItem{MatterID: m.ID}
		switch {
		case m.State == "needs-input" && len(out.NeedsInput) < 3:
			out.NeedsInput = append(out.NeedsInput, item)
		case m.State == "open" && len(out.Following) < 3:
			out.Following = append(out.Following, item)
		default:
			continue
		}
		wanted[m.ID] = true
	}
	latest := map[string]Message{}
	canonical := map[string]string{}
	resolve := func(id string) string {
		if value, ok := canonical[id]; ok {
			return value
		}
		value := CanonicalMatter(d, id)
		canonical[id] = value
		return value
	}
	for _, msg := range d.Messages {
		if msg.Speaker != "hq" || !validKind(msg.Kind) {
			continue
		}
		id := resolve(msg.MatterID)
		if !wanted[id] {
			continue
		}
		previous, exists := latest[id]
		if !exists || msg.Seq > previous.Seq || msg.Seq == previous.Seq && msg.ID > previous.ID {
			latest[id] = msg
		}
	}
	for _, items := range [][]OverviewItem{out.NeedsInput, out.Following} {
		for i := range items {
			if msg, ok := latest[items[i].MatterID]; ok {
				body, clipped := overviewExcerpt(msg.Body, 768)
				items[i].Preview = &OverviewPreview{MessageID: msg.ID, Kind: msg.Kind, Body: body, At: msg.At, Truncated: clipped}
			} else if body, clipped := overviewExcerpt(d.Matters[items[i].MatterID].Assessment, 768); body != "" {
				// UpdatedAt also changes on linking/renaming, so it is not an
				// assessment timestamp or the start of a wait.
				items[i].Preview = &OverviewPreview{Kind: "assessment", Body: body, Truncated: clipped}
			}
		}
	}
	changes := append([]Change(nil), d.Changes...)
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].At != changes[j].At {
			return changes[i].At > changes[j].At
		}
		return changes[i].Seq > changes[j].Seq
	})
	seen := map[string]bool{}
	for _, c := range changes {
		if c.At <= 0 || c.At < out.Since || c.At > now {
			continue
		}
		state := ""
		switch c.Kind {
		case "update":
			if !validState(c.Detail) {
				continue
			}
			state = c.Detail
		case "delivery-uncertain":
			if d.Messages[c.Target].Delivery != "uncertain" {
				continue
			}
		case "register", "hq-result", "hq-decision", "hq-error", "merge", "split":
			state = d.Messages[c.Target].MatterState
		default:
			continue
		}
		id := resolve(c.Matter)
		if c.Kind == "split" {
			id = resolve(c.Target)
		}
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out.RecentTotal++
		if len(out.Recent) < 5 {
			out.Recent = append(out.Recent, OverviewChange{Seq: c.Seq, At: c.At, MatterID: id, Kind: c.Kind, State: state})
		}
	}
	return out
}

func overviewExcerpt(body string, limit int) (string, bool) {
	body = strings.TrimSpace(body)
	n := 0
	for i := range body {
		if n == limit {
			return body[:i], true
		}
		n++
	}
	return body, false
}
