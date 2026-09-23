package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Moving a Mac between Direct routes from the owner's phone
// (openspec/changes/phone-moves-the-route). A paired device is the owner at a distance; a
// guest is someone watching a pane through a share link and may do neither.

func routeServer(t *testing.T, moved *[]string, fail error) *Server {
	t.Helper()
	return New(Config{Addr: "x", Token: "master"}, Deps{
		Routes: func() ([]RouteInfo, error) {
			return []RouteInfo{
				{ID: "sh", Name: "上海", EN: "Shanghai", ZH: "上海", URL: "https://sh.example.test/p35047", Current: true},
				{ID: "la", Name: "United States (West)", EN: "United States (West)", ZH: "美国西部", URL: "https://la.example.test/p35047"},
			}, nil
		},
		MoveRoute: func(id string) error {
			if fail != nil {
				return fail
			}
			*moved = append(*moved, id)
			return nil
		},
	})
}

func call(t *testing.T, s *Server, method, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, "/api/routes", nil)
	} else {
		r = httptest.NewRequest(method, "/api/routes", strings.NewReader(body))
	}
	r.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	return rec
}

func TestTheOwnerSeesEveryRouteWithItsOwnAddress(t *testing.T) {
	var moved []string
	rec := call(t, routeServer(t, &moved, nil), http.MethodGet, "", "master")
	if rec.Code != 200 {
		t.Fatalf("GET = %d: %s", rec.Code, rec.Body.String())
	}
	var reply struct {
		Routes []RouteInfo `json:"routes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &reply); err != nil {
		t.Fatal(err)
	}
	if len(reply.Routes) != 2 || !reply.Routes[0].Current {
		t.Fatalf("routes = %+v", reply.Routes)
	}
	// The address is what lets the phone time the route ITSELF, which is the whole point
	// of not sending a round trip from here.
	for _, r := range reply.Routes {
		if r.URL == "" {
			t.Fatalf("a route with no address: %+v", r)
		}
	}
}

func TestTheOwnerMovesTheMac(t *testing.T) {
	var moved []string
	rec := call(t, routeServer(t, &moved, nil), http.MethodPost, `{"route":"la"}`, "master")
	if rec.Code != 200 {
		t.Fatalf("POST = %d: %s", rec.Code, rec.Body.String())
	}
	if len(moved) != 1 || moved[0] != "la" {
		t.Fatalf("moved = %v, want the route the caller named", moved)
	}
}

func TestAGuestNeitherSeesTheRoutesNorMovesTheMac(t *testing.T) {
	var moved []string
	s := routeServer(t, &moved, nil)
	en := NewEnrollManager(nil, func([]EnrolledDevice) {})
	s.deps.Enroll = en
	guest := en.MintGuest("a share link", []string{"%1"}, nil, 0)
	tok := guest.Token

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		body := ""
		if method == http.MethodPost {
			body = `{"route":"la"}`
		}
		rec := call(t, s, method, body, tok)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s as a guest = %d, want 403", method, rec.Code)
		}
	}
	if len(moved) != 0 {
		t.Fatalf("a guest moved the Mac: %v", moved)
	}
}

func TestARouteThisMacMayNotUseIsRefusedWithoutMoving(t *testing.T) {
	var moved []string
	s := routeServer(t, &moved, fmt.Errorf("no Direct server called nope"))
	rec := call(t, s, http.MethodPost, `{"route":"nope"}`, "master")
	if rec.Code != http.StatusConflict {
		t.Fatalf("POST = %d, want 409", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "nope") {
		t.Fatalf("the refusal does not say which route: %s", rec.Body.String())
	}
}

func TestNamingNoRouteIsABadRequest(t *testing.T) {
	var moved []string
	rec := call(t, routeServer(t, &moved, nil), http.MethodPost, `{}`, "master")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST with no route = %d, want 400", rec.Code)
	}
}

// A Mac with no Direct configured answers plainly rather than pretending to have routes.
func TestNoRoutesConfiguredSaysSo(t *testing.T) {
	s := New(Config{Addr: "x", Token: "master"}, Deps{})
	if rec := call(t, s, http.MethodGet, "", "master"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET = %d, want 503", rec.Code)
	}
	if rec := call(t, s, http.MethodPost, `{"route":"la"}`, "master"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("POST = %d, want 503", rec.Code)
	}
}
