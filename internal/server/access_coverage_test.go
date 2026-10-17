package server

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
)

// Derive subjects from the router, not a second handwritten endpoint inventory.
// An intentional asset/stream exclusion is explicit and cannot mask a new route.
func TestAccessVocabularyCoversRegisteredRoutes(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "server.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	exceptional := map[string]string{
		"/api/health": "public probe", "/api/enroll": "explicit redemption receipt",
		"/api/icon": "asset", "/api/theme": "asset", "/api/events": "actual stream boundaries", "/api/attach": "actual stream boundaries",
	}
	routes := map[string]bool{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "Handler" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) < 2 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (sel.Sel.Name != "Handle" && sel.Sel.Name != "HandleFunc") {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok {
				return true
			}
			path, _ := strconv.Unquote(lit.Value)
			if !strings.HasPrefix(path, "/api/") {
				return true
			}
			routes[path] = true
			if exceptional[path] == "" && accessRoutes[path] == "" && accessWrites[path] == "" {
				t.Errorf("authenticated route %s has no explicit access classification", path)
			}
			return true
		})
	}
	if len(routes) < 40 {
		t.Fatalf("router inventory was not discovered: %d", len(routes))
	}
	t.Setenv("HOME", t.TempDir())
	for _, vocabulary := range []map[string]string{accessRoutes, accessWrites} {
		for path, op := range vocabulary {
			if !routes[path] {
				t.Errorf("classification for absent route %s", path)
			}
			diag.For("serve").Access(diag.AccessEntry{Origin: diag.AccessExternal, Operation: op, Outcome: diag.OK})
		}
	}
	page, err := diag.ReadAccess(diag.AccessQuery{Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Entries) != len(accessRoutes)+len(accessWrites) {
		t.Fatal("vocabulary projection lost records")
	}
	for _, e := range page.Entries {
		if e.Operation == "other" {
			t.Fatal("route classification absent from safe bilingual vocabulary")
		}
	}
}

func TestAccessUnsupportedMethodKeepsIntendedOperation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := New(Config{Token: "owner"}, Deps{})
	r := httptest.NewRequest("GET", "/api/send", nil)
	r.Header.Set("Authorization", "Bearer owner")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	page, err := diag.ReadAccess(diag.AccessQuery{})
	if w.Code != 405 || err != nil || len(page.Entries) != 1 || page.Entries[0].Operation != "send" {
		t.Fatalf("lost rejected request category: %d %+v %v", w.Code, page, err)
	}
}

func TestAccessArchiveExportsAreIndividualAndGenerationFailureIsVisible(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	fail, partial := false, false
	s := New(Config{Token: "owner"}, Deps{HQMemory: func(w io.Writer) (int64, error) {
		if fail && !partial {
			return 0, fmt.Errorf("archive source unavailable")
		}
		n, err := io.WriteString(w, "synthetic-archive")
		if fail {
			return 0, fmt.Errorf("archive source unavailable")
		}
		return int64(n), err
	}})
	request := func(method string) int {
		r := httptest.NewRequest(method, "/api/hq/memory", nil)
		r.Header.Set("Authorization", "Bearer owner")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if partial && method == "GET" && w.Body.String() != "synthetic-archive" {
			t.Fatal("JSON error appended to already-started archive")
		}
		return w.Code
	}
	for i := 0; i < 2; i++ {
		if request("GET") != 200 {
			t.Fatal("archive request failed")
		}
	}
	fail = true
	if request("GET") != 500 {
		t.Fatal("archive failure before output must not be an empty successful download")
	}
	partial = true
	if request("GET") != 200 {
		t.Fatal("an already committed status must not be rewritten")
	}
	if request("POST") != 405 {
		t.Fatal("archive endpoint accepted an unsupported method")
	}
	page, err := diag.ReadAccess(diag.AccessQuery{})
	if err != nil || len(page.Entries) != 5 {
		t.Fatalf("exports coalesced or failures missing: %+v %v", page, err)
	}
	for _, e := range page.Entries {
		if e.Operation != "archive_export" {
			t.Fatalf("not an export: %+v", e)
		}
	}
	if page.Entries[1].Outcome != diag.Failed || page.Entries[1].Status != 200 || page.Entries[2].Outcome != diag.Failed || page.Entries[2].Status != 500 {
		t.Fatalf("false archive success: %+v", page.Entries)
	}
}

func TestAccessDesktopTargetsDoNotCoalesceOtherConversations(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	s := New(Config{Token: "owner"}, Deps{
		SessionTranscript: func(string, string) ([]byte, TranscriptMeta, error) { return []byte(`[]`), TranscriptMeta{}, nil },
		SessionFollow: func(string, *sessionpolicy.Settings, string, string) (sessionpolicy.Settings, error) {
			return sessionpolicy.Settings{}, nil
		},
	})
	a, b := "12345678-1111-2222-3333-123456789abc", "12345678-1111-2222-3333-123456789abd"
	for _, id := range []string{a, a, b} {
		r := httptest.NewRequest("GET", "/api/session/transcript?session_id="+id, nil)
		r.Header.Set("Authorization", "Bearer owner")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(w.Code)
		}
	}
	for i := 0; i < 2; i++ {
		r := httptest.NewRequest("POST", "/api/session-follow", strings.NewReader(`{"session_id":"`+b+`","settings":{"hq":true}}`))
		r.Header.Set("Authorization", "Bearer owner")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(w.Code)
		}
	}
	page, err := diag.ReadAccess(diag.AccessQuery{})
	if err != nil || len(page.Entries) != 4 {
		t.Fatalf("conversation attribution/coalescing: %+v %v", page, err)
	}
	if page.Entries[0].SessionID != b || page.Entries[1].SessionID != b || page.Entries[2].SessionID != b || page.Entries[3].SessionID != a {
		t.Fatalf("session target lost: %+v", page.Entries)
	}
}

func TestAccessCreatedSessionCarriesReturnedPane(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := New(Config{Token: "owner"}, Deps{CreateSession: func(string, string) (SessionCreated, error) { return SessionCreated{PaneID: "%81"}, nil }})
	r := httptest.NewRequest("POST", "/api/sessions", strings.NewReader(`{"request_id":"request-id-123456789","name":"PRIVATE-NAME"}`))
	r.Header.Set("Authorization", "Bearer owner")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	page, err := diag.ReadAccess(diag.AccessQuery{})
	if w.Code != 200 || err != nil || len(page.Entries) != 1 || page.Entries[0].Target != "%81" {
		t.Fatalf("creation target missing: %d %+v %v", w.Code, page, err)
	}
	if strings.Contains(todaysLog(t), "PRIVATE-NAME") {
		t.Fatal("request text entered audit")
	}
}

func TestAccessFilterAPIRejectsUnknownValuesAndEchoesAppliedQuery(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := New(Config{Token: "owner"}, Deps{})
	for _, path := range []string{"/api/access-history?group_by=name", "/api/access-history?outcome=unknown", "/api/access-history?operation=unknown"} {
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("Authorization", "Bearer owner")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatalf("invalid filter: %s %d", path, w.Code)
		}
	}
	r := httptest.NewRequest("GET", "/api/access-history?device_id=full-id&group_by=device&outcome=failed", nil)
	r.Header.Set("Authorization", "Bearer owner")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"device_id":"full-id"`) || !strings.Contains(w.Body.String(), `"group_by":"device"`) || !strings.Contains(w.Body.String(), `"groups":[]`) {
		t.Fatal(w.Code, w.Body.String())
	}
}
