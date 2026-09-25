package knowledge

import (
	"strings"
	"testing"
)

// The whole reason weighting was added: two entries share one word each with the query,
// and the one sharing the word almost nothing else carries is the one meant. Unweighted,
// the two score the same and the tie falls to the id, which is how the right entry used to
// sit below four wrong ones. The ids here are chosen so an unweighted ranking puts the
// wrong entry first.
func TestARareSharedWordOutranksACommonOne(t *testing.T) {
	var live []knowledgeOp
	// Most of the base talks about deploying.
	for _, n := range []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel"} {
		live = append(live, knowledgeOp{
			ID: "pitfalls/common-" + n, Topic: "pitfalls",
			Title: "deploy notes " + n, Body: "the deploy step for " + n + " needs the usual checks",
		})
	}
	live = append(live,
		knowledgeOp{ID: "pitfalls/aaa-common", Topic: "pitfalls",
			Title: "deploy the release build", Body: "the deploy step runs after the gate is green"},
		knowledgeOp{ID: "pitfalls/zzz-rare", Topic: "pitfalls",
			Title: "chisel refuses the account", Body: "the chisel authfile must hold the sentinel account"},
	)
	hits := search(live, Query{Text: "deploy chisel"})
	if len(hits) == 0 {
		t.Fatal("no hits at all")
	}
	if hits[0].ID != "pitfalls/zzz-rare" {
		t.Errorf("ranked %s first; the entry sharing the rare word should win\ngot: %v",
			hits[0].ID, ids(hits))
	}
}

// A base of two entries is the state every base starts in. The plain log(N/(1+df)) is zero
// or negative there, so every weight collapses and nothing matches anything.
func TestATinyBaseStillMatches(t *testing.T) {
	live := []knowledgeOp{
		{ID: "pitfalls/one", Topic: "pitfalls", Title: "the release tag needs a user block", Body: "goreleaser copies it"},
		{ID: "pitfalls/two", Topic: "pitfalls", Title: "disk hygiene on the build host", Body: "prune old derived data"},
	}
	hits := search(live, Query{Text: "release tag"})
	if len(hits) == 0 {
		t.Fatal("a two-entry base returned nothing for a query that clearly matches one of them")
	}
	if hits[0].ID != "pitfalls/one" {
		t.Errorf("ranked %s first, want pitfalls/one", hits[0].ID)
	}
}

// Topics narrow the field; they are not a ranking hint.
func TestSearchNarrowsByTopic(t *testing.T) {
	live := []knowledgeOp{
		{ID: "pitfalls/p", Topic: "pitfalls", Title: "the release tag needs a user block"},
		{ID: "workflows/w", Topic: "workflows", Title: "the release flow: tag then wait"},
	}
	all := search(live, Query{Text: "release tag"})
	if len(all) != 2 {
		t.Fatalf("unfiltered search returned %v, want both", ids(all))
	}
	only := search(live, Query{Text: "release tag", Topics: []string{"workflows"}})
	if len(only) != 1 || only[0].ID != "workflows/w" {
		t.Errorf("topic filter returned %v, want only workflows/w", ids(only))
	}
}

func TestSearchIsQuietWhenNothingIsClose(t *testing.T) {
	live := []knowledgeOp{
		{ID: "pitfalls/p", Topic: "pitfalls", Title: "the release tag needs a user block", Body: "goreleaser copies it"},
	}
	if hits := search(live, Query{Text: "octopus migration patterns in postgres"}); len(hits) != 0 {
		t.Errorf("returned %v for a query about nothing in the base", ids(hits))
	}
	if hits := search(live, Query{Text: "   "}); len(hits) != 0 {
		t.Errorf("an empty query returned %v", ids(hits))
	}
}

func ids(ns []Neighbour) []string {
	var out []string
	for _, n := range ns {
		out = append(out, n.ID+" "+strings.TrimSpace(n.Title))
	}
	return out
}
