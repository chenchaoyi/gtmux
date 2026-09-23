package server

import (
	"encoding/json"
	"net/http"
)

// The Direct routes this Mac can take, and moving between them from the owner's phone
// (openspec/changes/phone-moves-the-route).
//
// Which route a Mac uses is the Mac's, and the paired app is the Mac at a distance: it
// types into panes and sends work, so choosing the route is the same kind of act from the
// same place. A GUEST is a different thing — someone watching a pane through a share link
// — and may neither see this nor do it.
//
// What the phone does NOT get from here is a round trip. A user on the other side of the
// world is asking what THEIR connection costs, and the Mac's own measurement answers a
// different question. Each route carries its address so the phone can time it itself.

// RouteInfo is one route as the owner meets it: a place, the address this Mac has on it,
// and whether it is the one in use.
type RouteInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	EN      string `json:"en,omitempty"`
	ZH      string `json:"zh,omitempty"`
	URL     string `json:"url"`
	Current bool   `json:"current"`
}

func (s *Server) handleRoutes(w http.ResponseWriter, r *http.Request) {
	// A guest never moves someone else's Mac, and never learns where else it could be.
	if callerScope(r.Context()) == scopeGuest {
		writeJSON(w, http.StatusForbidden, errBody("not allowed for a guest connection"))
		return
	}
	switch r.Method {
	case http.MethodGet:
		if s.deps.Routes == nil {
			writeJSON(w, http.StatusServiceUnavailable, errBody("routes not available"))
			return
		}
		list, err := s.deps.Routes()
		if err != nil {
			writeJSON(w, http.StatusBadGateway, errBody("could not read the routes"))
			return
		}
		if list == nil {
			list = []RouteInfo{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"routes": list})
	case http.MethodPost:
		if s.deps.MoveRoute == nil {
			writeJSON(w, http.StatusServiceUnavailable, errBody("moving is not available"))
			return
		}
		var body struct {
			Route string `json:"route"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Route == "" {
			writeJSON(w, http.StatusBadRequest, errBody("which route?"))
			return
		}
		if err := s.deps.MoveRoute(body.Route); err != nil {
			// A route this Mac may not use is a refusal, not a server fault: the phone
			// asked for something that is not on its own list.
			writeJSON(w, http.StatusConflict, errBody(err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"route": body.Route})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, errBody("GET or POST"))
	}
}
