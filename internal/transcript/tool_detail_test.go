package transcript

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCodexDescriptiveToolDetail(t *testing.T) {
	for _, tc := range []struct{ args, want string }{
		{`{"title":"Inspect the command panel","code":"await tab.snapshot()"}`, "Inspect the command panel"},
		{`{"description":"Run focused tests","cmd":"go test ./internal/transcript"}`, "Run focused tests"},
		{`{"title":" ","description":"Check layout","cmd":"ignored"}`, "Check layout"},
		{`{"title":42,"cmd":"go test ./..."}`, "go test ./..."},
		{`{"cmd":["go","test","./..."]}`, "go test ./..."},
		{`{"cell_id":"private-id"}`, ""},
	} {
		t.Run(tc.args, func(t *testing.T) {
			if got := codexToolDetail(tc.args); got != tc.want {
				t.Fatalf("detail = %q, want %q", got, tc.want)
			}
		})
	}
	args, _ := json.Marshal(map[string]string{"title": strings.Repeat("鲸", 100)})
	if got := codexToolDetail(string(args)); got != strings.Repeat("鲸", 80)+"…" {
		t.Fatalf("summary bound lost: %q", got)
	}

	st := &parseState{}
	codexStep(`{"type":"event_msg","payload":{"type":"user_message","message":"check layout"}}`, st)
	codexStep(`{"type":"response_item","payload":{"type":"function_call","name":"js","arguments":"{\"title\":\"Inspect the command panel\",\"code\":\"private implementation\"}"}}`, st)
	got := st.cur.Segments[0].Steps[0]
	if got.Title != "js" || got.Detail != "Inspect the command panel" {
		t.Fatalf("parsed step: %#v", got)
	}
}
