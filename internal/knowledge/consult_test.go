package knowledge

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func cardFixture(id, scope string) knowledgeOp {
	return knowledgeOp{Op: knowledgeOpAdd, ID: "pitfalls/" + id, Topic: "pitfalls", Title: "Preserve release access policy", Body: "Release evidence", Memory: &MemoryRule{Actions: []string{"release"}, Summary: "Check the current access policy before publication.", Exceptions: []string{"Change audience only with explicit user instruction."}, Scope: scope, Confidence: "supported", Evidence: []string{"synthetic:test"}, ReviewAfter: 200}}
}

func TestConsultScopeVersionAndEvidenceBoundaries(t *testing.T) {
	cases := []struct {
		name   string
		change func(*knowledgeOp)
		req    ConsultRequest
		want   int
	}{
		{"local", func(e *knowledgeOp) {}, ConsultRequest{Action: "release"}, 1},
		{"global", func(e *knowledgeOp) { e.Memory.Scope = "global" }, ConsultRequest{Action: "release"}, 1},
		{"reported", func(e *knowledgeOp) { e.Memory.Confidence = "reported" }, ConsultRequest{Action: "release"}, 1},
		{"contested", func(e *knowledgeOp) { e.Memory.Confidence = "contested" }, ConsultRequest{Action: "release"}, 0},
		{"unknown scope", func(e *knowledgeOp) { e.Memory.Scope = "magic" }, ConsultRequest{Action: "release"}, 0},
		{"HQ match", func(e *knowledgeOp) { e.Memory.Scope = "hq" }, ConsultRequest{Action: "release", Role: "hq"}, 1},
		{"legacy unscoped", func(e *knowledgeOp) { e.Memory = nil }, ConsultRequest{Action: "release"}, 0},
		{"legacy machine", func(e *knowledgeOp) { e.Memory = nil; e.Audience = AudienceMachine }, ConsultRequest{Query: "preserve release access policy"}, 1},
		{"accounts", func(e *knowledgeOp) { e.Topic = "accounts" }, ConsultRequest{Action: "release"}, 0},
		{"version changed", func(e *knowledgeOp) { e.Memory.ToolVersions = map[string]string{"tool": "2"} }, ConsultRequest{Action: "release", Tools: map[string]string{"tool": "3"}}, 0},
		{"private", func(e *knowledgeOp) { e.Sensitive = true }, ConsultRequest{Action: "release"}, 0},
		{"hypothesis", func(e *knowledgeOp) { e.Status = StatusHypothesis }, ConsultRequest{Action: "release"}, 0},
		{"unknown confidence", func(e *knowledgeOp) { e.Memory.Confidence = "magic" }, ConsultRequest{Action: "release"}, 0},
		{"tentative", func(e *knowledgeOp) { e.Memory.Confidence = "tentative" }, ConsultRequest{Action: "release"}, 0},
		{"review due", func(e *knowledgeOp) { e.Memory.ReviewAfter = 100 }, ConsultRequest{Action: "release"}, 0},
		{"project mismatch", func(e *knowledgeOp) { e.Memory.Scope = "project"; e.Memory.Project = "/project/a" }, ConsultRequest{Action: "release", Project: "/project/b"}, 0},
		{"project match", func(e *knowledgeOp) { e.Memory.Scope = "project"; e.Memory.Project = "/project/a" }, ConsultRequest{Action: "release", Project: "/project/a"}, 1},
		{"HQ only", func(e *knowledgeOp) { e.Memory.Scope = "hq" }, ConsultRequest{Action: "release", Role: "worker"}, 0},
		{"version missing", func(e *knowledgeOp) { e.Memory.ToolVersions = map[string]string{"tool": "2"} }, ConsultRequest{Action: "release"}, 0},
		{"version match", func(e *knowledgeOp) { e.Memory.ToolVersions = map[string]string{"tool": "2"} }, ConsultRequest{Action: "release", Tools: map[string]string{"tool": "2"}}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := cardFixture("release", "local")
			tc.change(&e)
			before, _ := json.Marshal(e)
			r, err := consultEntries([]knowledgeOp{e}, tc.req, 100)
			if err != nil || len(r.Items) != tc.want {
				t.Fatalf("got %+v %v", r, err)
			}
			after, _ := json.Marshal(e)
			if !bytes.Equal(before, after) {
				t.Fatal("consult changed entry")
			}
		})
	}
}

func TestConsultWholeCardsProtectedOverflowAndNegativeQuery(t *testing.T) {
	e := cardFixture("release", "local")
	e.Memory.ProtectedReason = "user access correction"
	r, err := consultEntries([]knowledgeOp{e}, ConsultRequest{Action: "release", TokenBudget: 1}, 100)
	if err != nil || len(r.Items) != 0 || len(r.MissingRequired) != 1 {
		t.Fatalf("silent truncation: %+v %v", r, err)
	}
	r, err = consultEntries([]knowledgeOp{e}, ConsultRequest{Action: "release"}, 100)
	if err != nil || len(r.Items) != 1 || !strings.Contains(r.Items[0].Card, "explicit user instruction") {
		t.Fatalf("exception lost: %+v %v", r, err)
	}
	r, err = consultEntries([]knowledgeOp{e}, ConsultRequest{Query: "write a birthday poem"}, 100)
	if err != nil || len(r.Items) != 0 {
		t.Fatalf("irrelevant injection: %+v %v", r, err)
	}
	e.Memory.ReviewAfter = 50
	r, err = consultEntries([]knowledgeOp{e}, ConsultRequest{Action: "release"}, 100)
	if err != nil || len(r.Items) != 0 || len(r.MissingRequired) != 1 {
		t.Fatalf("silently lost stale protected rule: %+v %v", r, err)
	}
	e.Memory = nil
	e.Provenance = ProvCorrection
	e.Audience = AudienceMachine
	r, err = consultEntries([]knowledgeOp{e}, ConsultRequest{Query: "preserve release access policy", TokenBudget: 1}, 100)
	if err != nil || len(r.MissingRequired) != 1 {
		t.Fatalf("legacy protection disappeared under budget: %+v %v", r, err)
	}
}
