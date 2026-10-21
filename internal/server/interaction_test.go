package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInteractionScreenAndAskGates(t *testing.T) {
	text := "Allow this tool?\n› 1. Allow once\n  2. Cancel"
	asked := true
	s := New(Config{Addr: "x", Token: "tok"}, Deps{
		PaneScreen:    func(id string) (string, bool) { return text, id == "%1" },
		PaneText:      func(string) (string, bool) { t.Fatal("interaction must not read scrollback"); return "", false },
		HasPendingAsk: func(string) bool { return asked },
	})
	get := func(method, id, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/interaction?id="+id, nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w
	}
	read := func() Interaction {
		r := get(http.MethodGet, "%251", "tok")
		if r.Code != 200 {
			t.Fatalf("status %d: %s", r.Code, r.Body.String())
		}
		var view Interaction
		if err := json.Unmarshal(r.Body.Bytes(), &view); err != nil {
			t.Fatal(err)
		}
		return view
	}
	if view := read(); len(view.Options) != 2 || !view.AwaitingReply || !strings.Contains(view.Text, "Allow this tool?") {
		t.Fatalf("view = %+v", view)
	}
	asked = false
	if view := read(); len(view.Options) != 0 || view.AwaitingReply || view.Text == "" {
		t.Fatalf("manual preview must not infer a wait: %+v", view)
	}
	asked = true
	text = "Choose projects\n❯ 1. [ ] Alpha\n  2. [x] Beta\nPress Space to toggle, Enter to submit"
	if view := read(); len(view.Options) != 0 || !strings.Contains(view.Text, "Space") {
		t.Fatalf("rich picker needs screen and keys, not digit buttons: %+v", view)
	}
	text = "Finished\n1. First finding\n2. Second finding\n› "
	if view := read(); len(view.Options) != 0 {
		t.Fatalf("prose became choices: %+v", view)
	}
	for _, tc := range []struct {
		method, id, token string
		code              int
	}{
		{http.MethodGet, "%251", "", 401}, {http.MethodGet, "", "tok", 400},
		{http.MethodPost, "%251", "tok", 405}, {http.MethodGet, "%259", "tok", 404},
	} {
		if r := get(tc.method, tc.id, tc.token); r.Code != tc.code {
			t.Errorf("%+v: status %d", tc, r.Code)
		}
	}
}

func TestInteractionTextBoundedAndPlain(t *testing.T) {
	text, cut := interactionText("\x1b[31mQuestion\x1b[0m\n\t中\x00文\r\n\n")
	if cut || text != "Question\n\t中文" {
		t.Fatalf("%q, %v", text, cut)
	}
	text, cut = interactionText(strings.Repeat("old\n", 90) + "CURRENT")
	if !cut || len(strings.Split(text, "\n")) != 80 || !strings.HasSuffix(text, "CURRENT") {
		t.Fatal("line bound lost")
	}
	text, cut = interactionText(strings.Repeat("中", 20000))
	if !cut || len([]rune(text)) != 16000 {
		t.Fatal("rune bound lost")
	}
}
