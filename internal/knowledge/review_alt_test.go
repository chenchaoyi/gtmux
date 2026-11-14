package knowledge

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func reviewAltFixture(t *testing.T, bilingual bool) knowledgeOp {
	t.Helper()
	asHQ(t)
	body := filepath.Join(t.TempDir(), "english.md")
	if err := os.WriteFile(body, []byte("  Compare `bytes`.\r\n\n- keep tabs:\t✓\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"add", "--topic", "pitfalls", "--title", "write-check: 核对写入字节", "--lang", "zh"}
	if bilingual {
		args = append(args, "--alt-lang", "en", "--alt-title", "  Check written bytes  ", "--alt-body-file", body)
	}
	captureStdout(t, func() {
		if rc := CmdKnowledge(args); rc != 0 {
			t.Fatalf("normal add: %d", rc)
		}
	})
	return mustLiveAll(t)[0]
}

// Exercise the actual strict JSON reader, including its automatic digest. No new
// Go struct field is needed to reproduce the pre-fix rejection of explicit Alt.
func reviewAltFile(t *testing.T, p ReviewProposal, fields map[int]any) (string, ReviewProposal, error) {
	t.Helper()
	p.ID = ""
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err := json.Unmarshal(b, &obj); err != nil {
		t.Fatal(err)
	}
	for i, alt := range fields {
		obj["changes"].([]any)[i].(map[string]any)["alt"] = alt
	}
	b, err = json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "proposal.json")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	parsed, err := readReview(path)
	return path, parsed, err
}

// KBA-001, KBA-002: normal add -> JSON -> check/prepare -> CLI apply -> readers/retry.
func TestReviewAltJSONTransaction(t *testing.T) {
	for _, mode := range []string{"omitted", "null", "replace", "monolingual"} {
		t.Run(mode, func(t *testing.T) {
			e := reviewAltFixture(t, mode != "monolingual")
			want := e.Alt
			fields := map[int]any{}
			if mode == "null" {
				fields[0] = nil
			}
			if mode == "replace" {
				want = &altHalf{Lang: "en", Title: " Verify the new bytes ", Body: "New evidence.\r\n\n- compare:\t✓\n"}
				fields[0] = want
			}
			path, p, err := reviewAltFile(t, proposalFor(e), fields)
			if err != nil {
				t.Fatalf("read proposal: %v", err)
			}
			_, omitted, err := reviewAltFile(t, proposalFor(e), nil)
			if err != nil || mode == "replace" && omitted.ID == p.ID {
				t.Fatal("alternate replacement was ignored by proposal digest", err)
			}
			ops, err := readKnowledgeOps()
			if err != nil {
				t.Fatal(err)
			}
			input, _ := json.Marshal(ops)
			prepared, _, err := prepareReview(p, ops, 100)
			if err != nil || len(prepared) != 1 || !reflect.DeepEqual(prepared[0].Alt, want) {
				t.Fatalf("prepared alternate: %+v, want %+v; %v", prepared, want, err)
			}
			if prepared[0].Alt != nil {
				prepared[0].Alt.Body = "mutated prepared copy"
			}
			afterInput, _ := json.Marshal(ops)
			if !bytes.Equal(input, afterInput) || p.ID != proposalDigest(p) || !reflect.DeepEqual(e.Alt, want) && mode != "replace" {
				t.Fatal("preparation aliased ledger/proposal alternate")
			}
			files, before := lineageFiles(t), lineageLedger(t)
			captureStdout(t, func() {
				if rc := CmdKnowledge([]string{"review", "check", path}); rc != 0 {
					t.Fatalf("check exit %d", rc)
				}
			})
			if !reflect.DeepEqual(files, lineageFiles(t)) {
				t.Fatal("check changed files")
			}
			captureStdout(t, func() {
				if rc := CmdKnowledge([]string{"review", "apply", path}); rc != 0 {
					t.Fatalf("apply exit %d", rc)
				}
			})
			ops, err = readKnowledgeOps()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(lineageLedger(t), before) || !reflect.DeepEqual(ops[len(ops)-1].Alt, want) || !reflect.DeepEqual(lineageLive(t, e.ID).Alt, want) {
				t.Fatal("append/reload lost alternate bytes or rewrote history")
			}
			if want != nil {
				entry, ok := KnowledgeEntry(e.ID)
				if !ok || entry.AltLang != want.Lang || entry.AltTitle != want.Title || entry.AltBody != want.Body {
					t.Fatal("API projection lost alternate bytes")
				}
				if title, body, tag := pick(lineageLive(t, e.ID), "en"); title != want.Title || body != want.Body || tag != "" {
					t.Fatal("reader did not receive retained/replaced English")
				}
			}
			committed := lineageLedger(t)
			if r, err := ApplyReview(p, false); err != nil || !r.AlreadyApplied || !bytes.Equal(committed, lineageLedger(t)) {
				t.Fatal("identical retry changed ledger", err)
			}
			stale := proposalFor(e)
			stale.Changes[0].Body += " Competing stale edit."
			stale.ID = proposalDigest(stale)
			if _, err := ApplyReview(stale, false); err == nil || !bytes.Equal(committed, lineageLedger(t)) {
				t.Fatal("stale competing proposal changed ledger")
			}
		})
	}
}

// KBA-002: known fields must be validated, never silently discarded. A valid
// first change cannot commit when the second change has an invalid alternate.
func TestReviewAltInvalidBatch(t *testing.T) {
	for _, tc := range []struct {
		name               string
		alt                any
		parseError, retire bool
	}{
		{name: "unknown language", alt: altHalf{Lang: "fr", Title: "x"}},
		{name: "same language", alt: altHalf{Lang: "zh", Title: "x"}},
		{name: "empty title", alt: altHalf{Lang: "en", Title: "  "}},
		{name: "multiline title", alt: altHalf{Lang: "en", Title: "x\ny"}},
		{name: "long title", alt: altHalf{Lang: "en", Title: strings.Repeat("x", knowledgeTitleMax+1)}},
		{name: "long body", alt: altHalf{Lang: "en", Title: "x", Body: strings.Repeat("x", knowledgeBodyMax+1)}},
		{name: "unknown field", alt: map[string]any{"lang": "en", "title": "x", "typo": "must fail"}, parseError: true},
		{name: "wrong type", alt: "x", parseError: true},
		{name: "retire with alternate", alt: altHalf{Lang: "en", Title: "x"}, retire: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := reviewAltFixture(t, true)
			other := e
			other.ID = "pitfalls/other"
			if err := commitKnowledgeOp(other, "synthetic second entry"); err != nil {
				t.Fatal(err)
			}
			p := proposalFor(e)
			second := proposalFor(lineageLive(t, other.ID)).Changes[0]
			if tc.retire {
				second.Operation = "retire"
			}
			p.Changes = append(p.Changes, second)
			before := lineageLedger(t)
			_, parsed, err := reviewAltFile(t, p, map[int]any{1: tc.alt})
			if tc.parseError {
				if err == nil {
					t.Fatal("accepted invalid JSON alternate")
				}
			} else {
				if err != nil {
					t.Fatalf("known alternate field failed to parse: %v", err)
				}
				if _, err := ApplyReview(parsed, false); err == nil {
					t.Fatal("accepted invalid alternate")
				}
			}
			if !bytes.Equal(before, lineageLedger(t)) {
				t.Fatal("invalid alternate partially committed batch")
			}
		})
	}
}
