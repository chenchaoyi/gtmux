package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestDTV005WorktreePathIsOwnerOnlyAndPaneGateRunsFirst(t *testing.T) {
	enroll := NewEnrollManager(nil, nil)
	guest := enroll.MintGuest("fixture", []string{"%1"}, nil, 0).Token
	const prefix = "# worktree \"/host-private/task-root\"\n"
	const patch = "# branch feature/review\ndiff --git a/a b/a\n--- a/a\n+++ b/a\n@@ -1 +1 @@\n-old\n+# worktree \"/literal-code\"\n"
	calls := 0
	h := New(Config{Token: testToken}, Deps{
		Enroll: enroll,
		Diff:   func(string) (string, error) { calls++; return prefix + patch, nil },
	}).Handler()
	for _, tc := range []struct {
		token, pane string
		status      int
		body        string
	}{
		{testToken, "%1", 200, prefix + patch},
		{guest, "%1", 200, patch},
		{guest, "%2", 403, ""},
		{"", "%1", 401, ""},
	} {
		before := calls
		r := do(t, h, http.MethodGet, "/api/diff?id="+strings.ReplaceAll(tc.pane, "%", "%25"), tc.token)
		if r.Code != tc.status {
			t.Fatalf("status got %d want %d", r.Code, tc.status)
		}
		if tc.status != http.StatusOK {
			if calls != before {
				t.Fatal("unauthorized request reached diff capture")
			}
			continue
		}
		var got diffResponse
		if err := json.Unmarshal(r.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Diff != tc.body {
			t.Fatalf("unexpected scoped diff: %q", got.Diff)
		}
	}
}
