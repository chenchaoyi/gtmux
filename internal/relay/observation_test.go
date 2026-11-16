package relay

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestDPC001ObservationExportRequiresCompleteRelayContract(t *testing.T) {
	r := Request{ID: "custom-ID_01", Kind: "report", Pane: "%9", Session: "synthetic", Body: "synthetic evidence", For: "hq", State: "pending", CreatedAt: 1}
	encode := func(value any) string {
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	valid := encode([]Request{r})
	if !IsObservationExport(strings.NewReader(valid + " \n")) {
		t.Fatal("valid custom IDs must follow the existing relay contract")
	}
	for _, field := range []string{"id", "kind", "session", "body", "for", "state", "created_at"} {
		t.Run(field, func(t *testing.T) {
			var value []map[string]any
			if err := json.Unmarshal([]byte(valid), &value); err != nil {
				t.Fatal(err)
			}
			delete(value[0], field)
			if IsObservationExport(strings.NewReader(encode(value))) {
				t.Fatal("partial structured note must remain protected")
			}
		})
	}
	for _, input := range []string{"[]", "null", "{}", "[null]", valid[:len(valid)-1], valid + "{}", valid + "[", `[{"body":"ordinary note"}]`} {
		if IsObservationExport(strings.NewReader(input)) {
			t.Fatal("unknown or incomplete input was classified as an observation")
		}
	}
	for _, id := range []string{"aa", "../foreign", strings.Repeat("a", 81), "a b"} {
		r.ID = id
		if IsObservationExport(strings.NewReader(encode([]Request{r}))) {
			t.Fatal("invalid relay ID classified as export")
		}
	}
	r.ID, r.Kind, r.For, r.State = strings.Repeat("a", 80), "ask", "user", "replied"
	if !IsObservationExport(strings.NewReader(encode([]Request{r}))) {
		t.Fatal("valid ask/user observation lost")
	}
	item := encode(r)
	oversized := "[" + strings.Repeat(item+",", 10000) + item + "]"
	if IsObservationExport(strings.NewReader(oversized)) {
		t.Fatal("unbounded request list accepted")
	}
	if IsObservationExport(io.MultiReader(strings.NewReader(valid), observationReadError{})) {
		t.Fatal("read failure after an array must not be treated as EOF")
	}
}

type observationReadError struct{}

func (observationReadError) Read([]byte) (int, error) { return 0, errors.New("synthetic read failure") }
