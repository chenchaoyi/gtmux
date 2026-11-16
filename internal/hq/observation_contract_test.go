package hq

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/relay"
)

func TestDPC005UncertainObservationsStayInEveryCoreSelection(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	root := t.TempDir()
	r, _, err := relay.Put(relay.Request{ID: "archive-source", Kind: "report", Pane: "%9", Session: "synthetic", Body: "synthetic only", For: "hq"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal([]relay.Request{r})
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	files["notes/relay-valid-pane.json"] = string(b)
	r.ID, r.Pane, r.Agent, r.HomeID, r.ThreadID = "archive-thread", "", "codex", "synthetic-home", "thread-1"
	r, _, err = relay.Put(r)
	if err != nil {
		t.Fatal(err)
	}
	b, err = json.Marshal([]relay.Request{r})
	if err != nil {
		t.Fatal(err)
	}
	files["notes/saved/relay-valid-thread.json"] = string(b)
	for _, mode := range []string{"unknown-field", "missing-source", "missing-blocking", "partial-notice"} {
		var record []map[string]any
		if err := json.Unmarshal(b, &record); err != nil {
			t.Fatal(err)
		}
		switch mode {
		case "unknown-field":
			record[0]["recovery_payload"] = map[string]any{"summary": "unique retained note"}
		case "missing-source":
			delete(record[0], "agent")
		case "missing-blocking":
			delete(record[0], "blocking")
		case "partial-notice":
			record[0]["notice"] = map[string]any{"turn_id": "t"}
		}
		body, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		files["notes/relay-"+mode+".json"] = string(body)
	}
	for name, body := range files {
		processFixture(t, root, name, body)
	}
	for _, scope := range []string{"core", "include-file", "include-subtree", "full"} {
		var include []string
		switch scope {
		case "include-file":
			include = []string{"notes/relay-valid-pane.json"}
		case "include-subtree":
			include = []string{"notes/saved"}
		}
		entries, err := selectArchive(root, scope != "full", include)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for _, entry := range entries {
			seen[entry.name] = true
		}
		for name, body := range files {
			want := true
			if name == "notes/relay-valid-pane.json" {
				want = scope == "include-file" || scope == "full"
			}
			if name == "notes/saved/relay-valid-thread.json" {
				want = scope == "include-subtree" || scope == "full"
			}
			if seen[name] != want {
				t.Fatalf("%s selected=%v for %s", scope, seen[name], name)
			}
			actual, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
			if err != nil || string(actual) != body {
				t.Fatal("selection modified protected data", name)
			}
		}
	}
}
