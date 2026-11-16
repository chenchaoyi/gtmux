package relay

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDPC005ObservationRequiresRealCompleteKnownSerialization(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	inputs := []Request{
		{ID: "source-pane", Kind: "report", Pane: "%9", Session: "synthetic", PanePID: "100", Body: "synthetic only", For: "hq"},
		{ID: "source-thread", Kind: "ask", Agent: "codex", HomeID: "synthetic-home", ThreadID: "thread-1", Session: "synthetic", Body: "synthetic only", For: "user", Blocking: true},
	}
	encode := func(value any) string {
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	for _, input := range inputs {
		r, fresh, err := Put(input)
		if err != nil || !fresh {
			t.Fatalf("real synthetic producer rejected valid source: %v", err)
		}
		serialized := encode([]Request{r})
		if !IsObservationExport(strings.NewReader(serialized)) {
			t.Fatal("real producer serialization was not recognized")
		}
		invalidText := strings.Replace(serialized, "synthetic only", string([]byte{0xff}), 1)
		if IsObservationExport(strings.NewReader(invalidText)) {
			t.Fatal("invalid UTF-8 must not be silently replaced and classified")
		}
		for _, field := range []string{"id", "kind", "pane", "session", "pane_pid", "body", "for", "blocking", "state", "created_at"} {
			for _, mode := range []string{"missing", "null"} {
				var record []map[string]any
				if err := json.Unmarshal([]byte(serialized), &record); err != nil {
					t.Fatal(err)
				}
				if mode == "missing" {
					delete(record[0], field)
				} else {
					record[0][field] = nil
				}
				if IsObservationExport(strings.NewReader(encode(record))) {
					t.Fatalf("%s serialized key %s was accepted", mode, field)
				}
			}
		}
		for _, mutate := range []func(map[string]any){
			func(v map[string]any) { v["recovery_payload"] = map[string]any{"summary": "unique note"} },
			func(v map[string]any) { v["blocking"] = "false" },
			func(v map[string]any) { v["notice"] = map[string]any{"turn_id": "t", "state": "accepted", "at": 1, "unique_note": "keep"} },
			func(v map[string]any) { v["notice"] = map[string]any{"turn_id": "t"} },
			func(v map[string]any) { v["notice"] = map[string]any{"turn_id": "t", "state": nil, "at": 1} },
		} {
			var record []map[string]any
			if err := json.Unmarshal([]byte(serialized), &record); err != nil {
				t.Fatal(err)
			}
			mutate(record[0])
			if IsObservationExport(strings.NewReader(encode(record))) {
				t.Fatal("unknown, partial or mistyped data was classified as a complete observation")
			}
			mixed := serialized[:len(serialized)-1] + "," + encode(record)[1:]
			if IsObservationExport(strings.NewReader(mixed)) {
				t.Fatal("an uncertain member must preserve the entire mixed export")
			}
		}
		object := encode(r)
		duplicate := "[" + object[:len(object)-1] + `,"body":"second unique value"}]`
		if IsObservationExport(strings.NewReader(duplicate)) {
			t.Fatal("duplicate keys must remain protected")
		}
		r.Notice = &Notice{TurnID: "t", State: "accepted", At: 1}
		if !IsObservationExport(strings.NewReader(encode([]Request{r}))) {
			t.Fatal("complete known nested serialization was lost")
		}
		r.Pane, r.Agent, r.HomeID, r.ThreadID = "", "", "", ""
		if IsObservationExport(strings.NewReader(encode([]Request{r}))) {
			t.Fatal("complete keys without a Put source were accepted")
		}
		r.Agent, r.HomeID, r.ThreadID = "codex", "synthetic-home", strings.Repeat("t", 257)
		if IsObservationExport(strings.NewReader(encode([]Request{r}))) {
			t.Fatal("overlong thread source was accepted")
		}
	}
}
