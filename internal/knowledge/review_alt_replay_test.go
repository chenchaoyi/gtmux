package knowledge

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// KBAR-001: old schema records had an intentional translation-reset contract.
func TestHistoricalReviewAltReadFallback(t *testing.T) {
	e := reviewAltFixture(t, true)
	old := knowledgeOp{Op: knowledgeOpSupersede, ID: e.ID, Supersedes: e.ID,
		Lang: "zh", Title: "历史修订后的中文", Body: "旧合同让先前译文失效。", Reviewed: true, At: 100}
	raw, err := json.Marshal(old)
	if err != nil || bytes.Contains(raw, []byte(`"alt"`)) {
		t.Fatal("not an omitted historical alternate", err)
	}
	if err := appendKnowledgeOp(old); err != nil {
		t.Fatal(err)
	}
	before := lineageLedger(t)
	live := lineageLive(t, e.ID)
	if live.Alt != nil {
		t.Fatal("historical review resurrected an intentionally invalidated alternate")
	}
	entry, ok := KnowledgeEntry(e.ID)
	row := rowOf(live)
	if !ok || entry.AltLang != "" || entry.AltTitle != "" || entry.AltBody != "" || row.AltLang != "" || row.AltTitle != "" {
		t.Fatal("public projection exposed historical stale English")
	}
	if title, body, tag := pick(live, "en"); title != old.Title || body != old.Body || tag != "zh" {
		t.Fatal("English reader did not get the historical Chinese fallback")
	}
	out := captureStdout(t, func() {
		if rc := CmdKnowledge([]string{"show", e.ID, "--lang", "en"}); rc != 0 {
			t.Fatalf("show exit %d", rc)
		}
	})
	if !strings.Contains(out, old.Title) || strings.Contains(out, strings.TrimSpace(e.Alt.Title)) || !bytes.Equal(before, lineageLedger(t)) {
		t.Fatal("CLI fallback exposed stale English or read rewrote the ledger")
	}
}

// KBAR-002: future retention is explicit data in the newly prepared operation.
func TestPreparedReviewAltReplay(t *testing.T) {
	e := reviewAltFixture(t, true)
	ops, err := readKnowledgeOps()
	if err != nil {
		t.Fatal(err)
	}
	p := proposalFor(e)
	p.Changes[0].Title, p.Changes[0].Body = "核对新写入的字节", "先核对授权，再比较完整内容。"
	p.ID = proposalDigest(p)
	prepared, _, err := prepareReview(p, ops, 100)
	if err != nil || len(prepared) != 1 || !reflect.DeepEqual(prepared[0].Alt, e.Alt) {
		t.Fatal("prepare omitted retained alternate", err)
	}
	raw, err := json.Marshal(prepared[0])
	if err != nil || !bytes.Contains(raw, []byte(`"alt":`)) {
		t.Fatal("new operation did not serialize retained alternate", err)
	}
	var recorded knowledgeOp
	if err := json.Unmarshal(raw, &recorded); err != nil {
		t.Fatal(err)
	}
	ops = append(ops, recorded)
	live := foldKnowledge(ops)
	if len(live) != 1 || !reflect.DeepEqual(live[0].Alt, e.Alt) {
		t.Fatal("new persisted alternate did not survive replay")
	}
	ops = append(ops, knowledgeOp{Op: knowledgeOpAlt, ID: e.ID})
	cleared := foldKnowledge(ops)[0]
	prepared, _, err = prepareReview(proposalFor(cleared), ops, 200)
	if err != nil || len(prepared) != 1 || prepared[0].Alt != nil {
		t.Fatal("new prepare resurrected an explicitly cleared alternate", err)
	}
	live = foldKnowledge(append(ops, prepared...))
	if len(live) != 1 || live[0].Alt != nil {
		t.Fatal("new review replay resurrected a cleared alternate")
	}
}

// KBAR-003: historical omitted serialized fields retain their reset semantics.
func TestReviewedAltReplayBoundaries(t *testing.T) {
	old := &altHalf{Lang: "en", Title: "Old English", Body: "  Original bytes.\r\n"}
	newAlt := &altHalf{Lang: "en", Title: "New English", Body: "New bytes.\n"}
	seed := knowledgeOp{Op: knowledgeOpAdd, ID: "pitfalls/a", Topic: "pitfalls", Lang: "zh", Title: "旧原文", Alt: old}
	review := knowledgeOp{Op: knowledgeOpSupersede, ID: seed.ID, Supersedes: seed.ID, Lang: "zh", Title: "新原文", Reviewed: true}
	clear := knowledgeOp{Op: knowledgeOpAlt, ID: seed.ID}
	replace := clear
	replace.Alt = newAlt
	explicit := review
	explicit.Alt = newAlt
	ordinary := review
	ordinary.Reviewed = false
	retitle := review
	retitle.ID = "pitfalls/b"
	retired := knowledgeOp{Op: knowledgeOpRetire, ID: seed.ID}
	readded := seed
	readded.Alt = nil
	for _, tc := range []struct {
		name string
		ops  []knowledgeOp
		want *altHalf
	}{
		{"historical omitted", []knowledgeOp{seed, review}, nil},
		{"consecutive reviews", []knowledgeOp{seed, review, review}, nil},
		{"current predecessor", []knowledgeOp{seed, replace, review}, nil},
		{"explicit replacement", []knowledgeOp{seed, explicit}, newAlt},
		{"clear before review", []knowledgeOp{seed, clear, review}, nil},
		{"clear after review", []knowledgeOp{seed, review, clear, review}, nil},
		{"ordinary reset", []knowledgeOp{seed, ordinary, review}, nil},
		{"dead predecessor", []knowledgeOp{seed, retired, review}, nil},
		{"readded predecessor", []knowledgeOp{seed, retired, readded, review}, nil},
		{"live retitle", []knowledgeOp{seed, retitle}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before, _ := json.Marshal(tc.ops)
			live := foldKnowledge(tc.ops)
			if len(live) != 1 || !reflect.DeepEqual(live[0].Alt, tc.want) {
				t.Fatalf("replay alternate: %+v, want %+v", live, tc.want)
			}
			if live[0].Alt != nil {
				live[0].Alt.Title = "mutated projection"
			}
			after, _ := json.Marshal(tc.ops)
			if !bytes.Equal(before, after) {
				t.Fatal("replay aliased input alternate")
			}
		})
	}
}
