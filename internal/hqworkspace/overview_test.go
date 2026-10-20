package hqworkspace

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestOverviewBoundsReportsAndKeepsReadProvenance(t *testing.T) {
	d := Empty()
	for i := range 12 {
		id := fmt.Sprintf("goal-%02d", i)
		state := "open"
		if i < 6 {
			state = "needs-input"
		}
		d.Matters[id] = Matter{ID: id, State: state, UpdatedAt: int64(i + 100)}
		d.Messages[id] = Message{ID: id, MatterID: id, Speaker: "hq", Kind: "result", Body: strings.Repeat("鲸", 1000), At: 80, Seq: 1}
	}
	// A later user prompt must not become the alleged HQ report.
	d.Messages["user"] = Message{ID: "user", MatterID: "goal-05", Speaker: "user", Body: "do something else", Seq: 99}
	d.Messages["older"] = Message{ID: "older", MatterID: "goal-05", Speaker: "hq", Kind: "progress", Body: "outdated", Seq: 0}
	before, _ := json.Marshal(d)
	out := BuildOverview(d, 200000)
	if len(out.NeedsInput) != 3 || len(out.Following) != 3 || out.NeedsInput[0].MatterID != "goal-05" {
		t.Fatal("unbounded or unstable selection", out)
	}
	p := out.NeedsInput[0].Preview
	if p == nil || p.MessageID != "goal-05" || p.At != 80 || !p.Truncated || !utf8.ValidString(p.Body) || utf8.RuneCountInString(p.Body) != 768 {
		t.Fatal("lost report provenance or Unicode bounds", p)
	}
	after, _ := json.Marshal(d)
	if string(before) != string(after) || d.Matters["goal-05"].State != "needs-input" {
		t.Fatal("overview changed explicit state or inferred completion from a result")
	}
}

func TestOverviewRecentUsesExplicitChangesAndDeduplicatesCanonicalMatters(t *testing.T) {
	d := Empty()
	for i := range 9 {
		id := fmt.Sprintf("goal-%d", i)
		d.Matters[id] = Matter{ID: id, State: "open"}
		d.Changes = append(d.Changes, Change{Seq: uint64(i + 1), At: 200000 - int64(i), Kind: "update", Matter: id, Detail: "completed"})
	}
	d.Matters["old"] = Matter{ID: "old", MergedInto: "goal-0"}
	d.Changes = append(d.Changes,
		Change{Seq: 20, At: 200000, Kind: "merge", Matter: "old", Target: "goal-0"},
		Change{Seq: 21, At: 200000, Kind: "hq-progress", Matter: "goal-0"},
		Change{Seq: 22, At: 200001, Kind: "hq-error", Matter: "future"},
		Change{Seq: 23, At: 1, Kind: "hq-error", Matter: "old"},
		Change{Seq: 24, At: 200000, Kind: "delivery-delivered", Matter: "goal-1"},
		Change{Seq: 25, At: 200000, Kind: "update", Matter: "goal-1", Detail: "renamed"},
		Change{Seq: 26, At: 200000, Kind: "hq-decision"},
	)
	out := BuildOverview(d, 200000)
	if len(out.Recent) != 5 || out.RecentTotal != 9 || out.Recent[0].Kind != "merge" || out.Recent[0].MatterID != "goal-0" {
		t.Fatal("recent summary includes noise or loses canonical identity", out)
	}
	if out.Recent[1].State != "completed" || out.Since != 200000-86400 {
		t.Fatal("transition or time boundary is not explicit", out)
	}
}

func TestOverviewAssessmentAndMissingSourcesDoNotInventClocks(t *testing.T) {
	d := Empty()
	d.Matters["target"] = Matter{ID: "target", State: "needs-input", UpdatedAt: 199999, Assessment: "Recorded context"}
	d.Matters["missing"] = Matter{ID: "missing", State: "open"}
	out := BuildOverview(d, 200000)
	p := out.NeedsInput[0].Preview
	if p == nil || p.Kind != "assessment" || p.At != 0 || p.MessageID != "" || out.Following[0].Preview != nil {
		t.Fatal("invented report source, report time or wait duration", out)
	}
	d.Matters["merged"] = Matter{ID: "merged", MergedInto: "target"}
	d.Messages["original"] = Message{ID: "original", MatterID: "merged", Speaker: "hq", Kind: "report", Body: "Original source survives merge", At: 123, Seq: 10}
	out = BuildOverview(d, 200000)
	if out.NeedsInput[0].Preview.MessageID != "original" || out.NeedsInput[0].Preview.At != 123 {
		t.Fatal("explicit merge edge lost original source")
	}
	d.Changes = []Change{{Seq: 3, At: 200000, Kind: "split", Matter: "target", Target: "missing"}}
	out = BuildOverview(d, 200000)
	if len(out.Recent) != 1 || out.Recent[0].MatterID != "missing" {
		t.Fatal("split must open the new matter")
	}
}

func TestOverviewDoesNotResurfaceResolvedDeliveryAsUnconfirmed(t *testing.T) {
	d := Empty()
	d.Matters["goal"] = Matter{ID: "goal", State: "open"}
	d.Messages["request"] = Message{ID: "request", MatterID: "goal", Delivery: "delivered"}
	d.Changes = []Change{{Seq: 1, At: 200000, Kind: "delivery-uncertain", Matter: "goal", Target: "request"}}
	if len(BuildOverview(d, 200000).Recent) != 0 {
		t.Fatal("resolved delivery resurfaced as a problem")
	}
	d.Messages["request"] = Message{ID: "request", MatterID: "goal", Delivery: "uncertain"}
	if len(BuildOverview(d, 200000).Recent) != 1 {
		t.Fatal("actual unconfirmed delivery disappeared")
	}
}
